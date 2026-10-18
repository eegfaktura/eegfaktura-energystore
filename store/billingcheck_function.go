package store

import (
	"errors"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"github.com/golang/glog"
)

var noEndDate = time.Date(2999, 12, 31, 0, 0, 0, 0, time.UTC).UnixMilli()

// BillingCheck evaluates the checks before a billing run (platform#148) in one pass over the
// period: per requested entry the L3 quarter hours and the first/last quarter hour with a value,
// and the community totals. Like the Excel summary, a quarter hour counts once per entry even if
// several of its values are L3, and only the entry's active window counts.
type BillingCheck struct {
	cps    []model.BillingCheckCp
	Result *model.BillingCheckResponse
}

func NewBillingCheck(req *model.BillingCheckRequest) *BillingCheck {
	start := time.UnixMilli(req.Start)
	end := time.UnixMilli(req.End)
	result := &model.BillingCheckResponse{
		PeriodStart: time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local).UnixMilli(),
		PeriodEnd:   time.Date(end.Year(), end.Month(), end.Day(), 23, 45, 0, 0, time.Local).UnixMilli(),
		Meters:      make([]model.BillingCheckMeter, len(req.Cps)),
	}
	for i, cp := range req.Cps {
		result.Meters[i] = model.BillingCheckMeter{
			MeteringPoint: cp.MeteringPoint,
			Direction:     cp.Direction,
			ActiveSince:   cp.ActiveSince,
			InactiveSince: cp.InactiveSince,
			L3Days:        []string{},
		}
	}
	return &BillingCheck{cps: req.Cps, Result: result}
}

func (bc *BillingCheck) HandleStart(ctx *EngineContext) error {
	return nil
}

func (bc *BillingCheck) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}
	totals := &bc.Result.Totals
	for i, cp := range bc.cps {
		inactiveSince := cp.InactiveSince
		if inactiveSince <= 0 { // no end date: active without end
			inactiveSince = noEndDate
		}
		if utils.IsLineDateOutOfRange(ts, [2]int64{cp.ActiveSince, inactiveSince}) {
			continue
		}
		meta, ok := ctx.metaMap[cp.MeteringPoint]
		if !ok || meta.Dir != cp.Direction {
			continue
		}

		var values []float64
		var qovs []int
		if cp.Direction == model.CONSUMER_DIRECTION {
			values, qovs = slotValues(line.Consumers, line.QoVConsumers, meta.SourceIdx, 3)
			if values != nil {
				totals.Consumed += values[0]
				totals.Allocated += values[1]
				totals.SelfCovered += values[2]
			}
		} else {
			values, qovs = slotValues(line.Producers, line.QoVProducers, meta.SourceIdx, 2)
			if values != nil {
				totals.Produced += values[0]
				totals.Surplus += values[1]
			}
		}

		hasValue, hasL3 := false, false
		for _, q := range qovs {
			hasValue = hasValue || q != 0
			hasL3 = hasL3 || q == 3
		}

		m := &bc.Result.Meters[i]
		if hasValue {
			t := ts.UnixMilli()
			if m.FirstData == nil {
				m.FirstData = &t
			}
			m.LastData = &t
		}
		if hasL3 {
			m.L3Count += 1
			// Lines come in chronological order: comparing with the last day noted keeps the
			// list sorted and free of duplicates.
			day := ts.Format("2006-01-02")
			if len(m.L3Days) == 0 || m.L3Days[len(m.L3Days)-1] != day {
				m.L3Days = append(m.L3Days, day)
			}
		}
	}
	return nil
}

func (bc *BillingCheck) HandleEnd(ctx *EngineContext) error {
	return nil
}

// slotValues returns the values and qualities of one metering point (width 3 for consumers,
// 2 for producers), or nil if the line is too short for its index.
func slotValues(values []float64, qovs []int, idx, width int) ([]float64, []int) {
	from, to := idx*width, idx*width+width
	if idx < 0 || to > len(values) {
		return nil, nil
	}
	var q []int
	if to <= len(qovs) {
		q = qovs[from:to]
	}
	return values[from:to], q
}

func QueryBillingCheck(tenant, ecid string, req *model.BillingCheckRequest) (*model.BillingCheckResponse, error) {
	c := NewBillingCheck(req)
	e := &Engine{c}

	sm := time.Now()
	if err := e.Query(tenant, ecid, time.UnixMilli(req.Start), time.UnixMilli(req.End)); err != nil && !errors.Is(err, ebow.ErrNoRows) {
		return nil, err
	}
	glog.V(5).Infof("Query Billing Check took %v (%s)", time.Since(sm).Seconds(), tenant)
	return c.Result, nil
}
