package store

import (
	"errors"
	"math"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"github.com/golang/glog"
)

// BillingCheck evaluates the checks before a billing run (platform#148) in one pass over the
// period: per requested entry the L3 quarter hours and the first/last quarter hour with a value,
// and the community totals. Like the Excel summary, a quarter hour counts once per entry even if
// several of its values are L3, and only the entry's active window counts. The totals cover all
// metering points of the community in the period, so a metering point that is not requested (or
// listed twice after an owner change) does not change them. Missing quarter hours are counted per
// entry and local day.
type BillingCheck struct {
	cps         []model.BillingCheckCp
	entries     []bcEntry
	periodStart time.Time
	present     map[int][]int16 // per entry: quarter hours with a value per day of the period
	Result      *model.BillingCheckResponse
}

// bcEntry is a requested entry resolved against the community's metadata.
type bcEntry struct {
	idx       int // index in the request and the result
	sourceIdx int
	consumer  bool
	from, to  time.Time // active window: activation day 00:00 to the end of the inactive day
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
			MissingDays:   []string{},
		}
	}
	return &BillingCheck{cps: req.Cps, Result: result, present: map[int][]int16{},
		periodStart: time.UnixMilli(result.PeriodStart)}
}

// dayIndex is the number of local days from the day of base to the day of t.
func dayIndex(base, t time.Time) int {
	b := time.Date(base.Year(), base.Month(), base.Day(), 12, 0, 0, 0, time.UTC)
	d := time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC)
	return int(math.Round(d.Sub(b).Hours() / 24))
}

// quarterHoursOnDay counts the local wall-clock quarter hours of a day as v1 stores them: 92 on the
// spring clock change day (02:00-02:45 do not exist), 96 otherwise (the autumn hour is one row).
func quarterHoursOnDay(day time.Time) int {
	n := 0
	for h := 0; h < 24; h++ {
		for m := 0; m < 60; m += 15 {
			t := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, time.Local)
			if t.Hour() == h && t.Minute() == m {
				n++
			}
		}
	}
	return n
}

// HandleStart resolves the entries once: metering points missing in the metadata or with another
// direction never match and are dropped.
func (bc *BillingCheck) HandleStart(ctx *EngineContext) error {
	for i, cp := range bc.cps {
		meta, ok := ctx.metaMap[cp.MeteringPoint]
		if !ok || meta.Dir != cp.Direction {
			continue
		}
		to := time.Date(2999, 12, 31, 23, 59, 59, 0, time.Local)
		if cp.InactiveSince > 0 { // no end date: active without end
			d := time.UnixMilli(cp.InactiveSince)
			to = time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 0, time.Local)
		}
		// the whole activation day counts, whatever time of day the caller sent
		a := time.UnixMilli(cp.ActiveSince)
		bc.present[i] = make([]int16, dayIndex(bc.periodStart, time.UnixMilli(bc.Result.PeriodEnd))+1)
		bc.entries = append(bc.entries, bcEntry{
			idx:       i,
			sourceIdx: meta.SourceIdx,
			consumer:  cp.Direction == model.CONSUMER_DIRECTION,
			from:      time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.Local),
			to:        to,
		})
	}
	return nil
}

func (bc *BillingCheck) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	totals := &bc.Result.Totals
	for i, v := range line.Consumers {
		switch i % 3 {
		case 0:
			totals.Consumed += v
		case 1:
			totals.Allocated += v
		case 2:
			totals.SelfCovered += v
		}
	}
	for i, v := range line.Producers {
		if i%2 == 0 {
			totals.Produced += v
		} else {
			totals.Surplus += v
		}
	}

	day := dayIndex(bc.periodStart, ts)
	for _, e := range bc.entries {
		if ts.Before(e.from) || ts.After(e.to) {
			continue
		}
		var hasValue, hasL3 bool
		if e.consumer {
			hasValue, hasL3 = slotQuality(line.Consumers, line.QoVConsumers, e.sourceIdx, 3)
		} else {
			hasValue, hasL3 = slotQuality(line.Producers, line.QoVProducers, e.sourceIdx, 2)
		}

		m := &bc.Result.Meters[e.idx]
		if hasValue {
			t := ts.UnixMilli()
			if m.FirstData == nil {
				m.FirstData = &t
			}
			m.LastData = &t
			if p := bc.present[e.idx]; day >= 0 && day < len(p) {
				p[day]++
			}
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

// countMissing compares, per entry and local day of its active part of the period, the quarter
// hours with a value with those of the day. Also runs when the store has no rows in the period. An
// entry without any value gets the count but no day list (the caller shows "no data").
func (bc *BillingCheck) countMissing() {
	last := dayIndex(bc.periodStart, time.UnixMilli(bc.Result.PeriodEnd))
	expectedPerDay := make([]int, last+1)
	for d := range expectedPerDay {
		expectedPerDay[d] = quarterHoursOnDay(bc.periodStart.AddDate(0, 0, d))
	}
	for i, cp := range bc.cps {
		from, to := 0, last
		if cp.ActiveSince > 0 {
			from = max(from, dayIndex(bc.periodStart, time.UnixMilli(cp.ActiveSince)))
		}
		if cp.InactiveSince > 0 {
			to = min(to, dayIndex(bc.periodStart, time.UnixMilli(cp.InactiveSince)))
		}
		m := &bc.Result.Meters[i]
		present := bc.present[i]
		for d := from; d <= to; d++ {
			expected, got := expectedPerDay[d], 0
			if present != nil {
				got = int(present[d])
			}
			if got < expected {
				m.MissingCount += expected - got
				if m.FirstData != nil {
					m.MissingDays = append(m.MissingDays, bc.periodStart.AddDate(0, 0, d).Format("2006-01-02"))
				}
			}
		}
	}
}

// slotQuality reports whether one metering point (width 3 for consumers, 2 for producers) has a
// value (quality not L0) and an L3 value in the line. The import grows the arrays only up to the
// highest index written, so they may end inside the block: a missing value is L0, a value without
// quality counts as L1 (as in EnergySummary).
func slotQuality(values []float64, qovs []int, idx, width int) (hasValue, hasL3 bool) {
	for k := idx * width; k < idx*width+width; k++ {
		if k >= len(values) {
			break
		}
		q := 1
		if k < len(qovs) {
			q = qovs[k]
		}
		hasValue = hasValue || q != 0
		hasL3 = hasL3 || q == 3
	}
	return hasValue, hasL3
}

func QueryBillingCheck(tenant, ecid string, req *model.BillingCheckRequest) (*model.BillingCheckResponse, error) {
	c := NewBillingCheck(req)
	e := &Engine{c}

	sm := time.Now()
	if err := e.Query(tenant, ecid, time.UnixMilli(req.Start), time.UnixMilli(req.End)); err != nil && !errors.Is(err, ebow.ErrNoRows) {
		return nil, err
	}
	c.countMissing()
	glog.V(5).Infof("Query Billing Check took %v (%s)", time.Since(sm).Seconds(), tenant)
	return c.Result, nil
}
