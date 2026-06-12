package calculation

import (
	"errors"
	"fmt"
	"strings"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"github.com/golang/glog"
)

//const StorageException = errors.New()

// EnergyReportV2 generate cumulated energy values over a time period.
// year - select year
// segment - period segment
// peroidCode - can have those values:
//   - Y:        cumulate one year
//   - YQ1-YQ4:  cumulate quarter years
//   - YH1-YH2:  cumulate half years
//   - YM1-YM12: cumulate months
func EnergyReportV2(tenant, ecid string, participants []model.ParticipantReport, year, segment int, periodCode string) (*model.ReportResponse, error) {

	db, err := ebow.OpenStorage(tenant, ecid)
	if err != nil {
		return nil, err
	}
	defer func() { db.Close() }()

	response := &model.ReportResponse{Id: fmt.Sprintf("%s/%.4d/%.2d", strings.ToUpper(periodCode), year, segment),
		ParticipantReports: participants}

	code := []byte(strings.ToUpper(periodCode))
	if len(code) < 2 {
		code = append(code, 'X')
	}

	switch code[1] {
	case 'M':
		err = CalculateMonthlyPeriodV2(db, response, AllocDynamicV2, year, segment)
		if err != nil && !errors.Is(err, ebow.ErrNotFound) {
			return nil, err
		}
		break
	case 'H':
		err = CalculateBiAnnualPeriodV2(db, response, AllocDynamicV2, year, segment)
		if err != nil && !errors.Is(err, ebow.ErrNotFound) {
			return nil, err
		}
		break
	case 'Q':
		err = CalculateQuarterlyPeriodV2(db, response, AllocDynamicV2, year, segment)
		if err != nil && !errors.Is(err, ebow.ErrNotFound) {
			return nil, err
		}
		break
	default:
		err = CalculateAnnualPeriodV2(db, response, AllocDynamicV2, year)
		if err != nil && !errors.Is(err, ebow.ErrNotFound) {
			return nil, err
		}
	}

	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%d", 0)); err != nil {
		return nil, err
	} else {
		for _, m := range meta.CounterPoints {
			glog.V(6).Infof("Meta: %+v\n", m)
			if m.Dir == "CONSUMPTION" || m.Dir == "GENERATION" {
				response.Meta = append(response.Meta, m)
			} else {
				glog.V(6).Infof("Omitted Meta: %+v\n", m)
			}
		}
	}
	return response, nil
}

func EnergySummary(tenant, ecid string, year, segment int, periodCode string) (interface{}, error) {
	c, _ := store.NewEnergySummary()
	e := &store.Engine{c}

	start, end, err := utils.PeriodToStartEndTime(year, segment, periodCode)
	if err != nil {
		return nil, err
	}

	if err := e.Query(tenant, ecid, start, end); err != nil && !errors.Is(err, ebow.ErrNoRows) {
		return nil, err
	}
	return (c.(*store.EnergySummary)).GetResult(), nil
}
