package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"
	"github.com/golang/glog"
	"strings"
)

//const StorageException = errors.New()

// EnergyReport generate cumulated energy values over a time period.
// year - select year
// segment - period segment
// peroidCode - can have those values:
//   - Y:        cumulate one year
//   - YQ1-YQ4:  cumulate quarter years
//   - YH1-YH2:  cumulate half years
//   - YM1-YM12: cumulate months
func EnergyReport(tenant string, year, segment int, periodCode string) (*model.EegEnergy, error) {

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return nil, err
	}
	defer func() { db.Close() }()

	var eegModel *model.EegEnergy
	var results []*model.EnergyReport
	var report *model.EnergyReport

	code := []byte(strings.ToUpper(periodCode))
	if len(code) < 2 {
		code = append(code, 'X')
	}

	switch code[1] {
	case 'M':
		results, report, err = CalculateMonthlyPeriod(db, AllocDynamic2, year, segment)
		if err != nil {
			return nil, err
		}
		break
	case 'H':
		results, report, err = CalculateBiAnnualPeriod(db, AllocDynamic2, year, segment)
		if err != nil {
			return nil, err
		}
		break
	case 'Q':
		results, report, err = CalculateQuarterlyPeriod(db, AllocDynamic2, year, segment)
		if err != nil {
			return nil, err
		}
		break
	default:
		results, report, err = CalculateAnnualPeriod(db, AllocDynamic2, year)
		if err != nil {
			return nil, err
		}
	}

	eegModel = &model.EegEnergy{}
	eegModel.Results = append(eegModel.Results, results...)
	eegModel.Report = report

	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%d", 0)); err != nil {
		return nil, err
	} else {
		//metaMap := map[int]*model.CounterPointMeta{}
		for _, m := range meta.CounterPoints {
			glog.V(4).Infof("Meta: %+v\n", m)
			if m.Dir == "CONSUMPTION" || m.Dir == "GENERATION" {
				eegModel.Meta = append(eegModel.Meta, m)
			} else {
				glog.V(4).Infof("Omitted Meta: %+v\n", m)
			}
		}
	}

	return eegModel, nil
}

func EnergyDashboard(tenant, function string, year, month int) (*model.EegEnergy, error) {
	var err error

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return nil, err
	}
	defer func() { db.Close() }()

	var eegModel *model.EegEnergy
	var results []*model.EnergyReport
	var report *model.EnergyReport

	calcF := GetCalcFunc(function)
	if calcF == nil {
		calcF = CalculateEEG
	}

	if month > 0 {
		results, report, err = CalculateWeeklyReport(db, year, month, calcF)
	} else {
		results, report, err = CalculateYearlyReport(db, year, calcF)
	}

	if err != nil {
		return nil, err
	}

	eegModel = &model.EegEnergy{}
	eegModel.Results = append(eegModel.Results, results...)
	eegModel.Report = report

	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%d", 0)); err != nil {
		return nil, err
	} else {
		//metaMap := map[int]*model.CounterPointMeta{}
		for _, m := range meta.CounterPoints {
			glog.V(4).Infof("Meta: %+v\n", m)
			if m.Dir == "CONSUMPTION" || m.Dir == "GENERATION" {
				eegModel.Meta = append(eegModel.Meta, m)
			} else {
				glog.V(4).Infof("Omitted Meta: %+v\n", m)
			}
		}
	}

	return eegModel, nil
}
