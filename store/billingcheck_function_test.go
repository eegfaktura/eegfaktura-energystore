package store

import (
	"fmt"
	"testing"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bcTenant = "TE999990"
	bcEcId   = "ECBILLCHECK1"
	bcCons   = "AT0030000000000000000000000000001"
	bcCons2  = "AT0030000000000000000000000000002"
	bcProd   = "AT0030000000000000000000000000003"
)

func bcDay(d int) time.Time { return time.Date(2026, 2, d, 0, 0, 0, 0, time.Local) }

// bcSeed writes February 2026 for two consumers and one producer: every quarter hour, all values
// L1, except that the second consumer has no values after 20.02. and the given quality overrides.
func bcSeed(t *testing.T, override func(id string, line *model.RawSourceLine)) {
	viper.Set("persistence.path", t.TempDir())
	db, err := ebow.OpenStorage(bcTenant, bcEcId)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.SetMeta(&model.RawSourceMeta{Id: "cpmeta/0", CounterPoints: []*model.CounterPointMeta{
		{ID: "cpmeta/0", Name: bcCons, SourceIdx: 0, Dir: model.CONSUMER_DIRECTION},
		{ID: "cpmeta/0", Name: bcCons2, SourceIdx: 1, Dir: model.CONSUMER_DIRECTION},
		{ID: "cpmeta/0", Name: bcProd, SourceIdx: 0, Dir: model.PRODUCER_DIRECTION},
	}}))

	lines := []*model.RawSourceLine{}
	for d := 1; d <= 28; d++ {
		for h := 0; h < 24; h++ {
			for m := 0; m < 60; m += 15 {
				line := &model.RawSourceLine{
					Id:           fmt.Sprintf("CP/2026/02/%.2d/%.2d/%.2d/00", d, h, m),
					Consumers:    []float64{1, 0.5, 0.4, 1, 0.5, 0.4},
					QoVConsumers: []int{1, 1, 1, 1, 1, 1},
					Producers:    []float64{2, 0.3},
					QoVProducers: []int{1, 1},
				}
				if d > 20 {
					copy(line.Consumers[3:], []float64{0, 0, 0})
					copy(line.QoVConsumers[3:], []int{0, 0, 0})
				}
				if override != nil {
					override(line.Id, line)
				}
				lines = append(lines, line)
			}
		}
	}
	require.NoError(t, db.SetLines(lines))
}

func bcRequest(cps ...model.BillingCheckCp) *model.BillingCheckRequest {
	return &model.BillingCheckRequest{Start: bcDay(1).UnixMilli(), End: bcDay(28).UnixMilli(), Cps: cps}
}

func cons(mp string, active, inactive int64) model.BillingCheckCp {
	return model.BillingCheckCp{MeteringPoint: mp, Direction: model.CONSUMER_DIRECTION, ActiveSince: active, InactiveSince: inactive}
}

func TestBillingCheck_L3CountsQuarterHoursOncePerActiveEntry(t *testing.T) {
	bcSeed(t, func(id string, line *model.RawSourceLine) {
		switch id {
		case "CP/2026/02/10/08/00/00": // one L3 value
			line.QoVConsumers[1] = 3
		case "CP/2026/02/10/08/15/00": // two L3 values in the same quarter hour count once
			line.QoVConsumers[0], line.QoVConsumers[2] = 3, 3
		case "CP/2026/02/11/09/00/00": // L2 is no warning
			line.QoVConsumers[0] = 2
		case "CP/2026/02/03/10/00/00": // L3 of the second consumer before its activation
			line.QoVConsumers[3] = 3
		case "CP/2026/02/12/10/00/00": // L3 of the producer
			line.QoVProducers[1] = 3
		}
	})

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(
		cons(bcCons, 0, 0),
		cons(bcCons2, bcDay(5).UnixMilli(), 0),
		model.BillingCheckCp{MeteringPoint: bcProd, Direction: model.PRODUCER_DIRECTION},
	))
	require.NoError(t, err)

	assert.Equal(t, 2, res.Meters[0].L3Count)
	assert.Equal(t, []string{"2026-02-10"}, res.Meters[0].L3Days)
	assert.Equal(t, 0, res.Meters[1].L3Count, "L3 before activation does not count")
	assert.Equal(t, []string{}, res.Meters[1].L3Days)
	assert.Equal(t, 1, res.Meters[2].L3Count)
	assert.Equal(t, []string{"2026-02-12"}, res.Meters[2].L3Days)
}

func TestBillingCheck_CoverageAndTotals(t *testing.T) {
	bcSeed(t, nil)

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(
		cons(bcCons, 0, 0),
		cons(bcCons2, bcDay(5).UnixMilli(), 0),
		model.BillingCheckCp{MeteringPoint: bcProd, Direction: model.PRODUCER_DIRECTION},
		cons("AT0030000000000000000000000000099", 0, 0), // unknown metering point
		cons(bcCons, 0, bcDay(9).UnixMilli()),           // same metering point, earlier owner
	))
	require.NoError(t, err)

	periodEnd := time.Date(2026, 2, 28, 23, 45, 0, 0, time.Local).UnixMilli()
	assert.Equal(t, bcDay(1).UnixMilli(), res.PeriodStart)
	assert.Equal(t, periodEnd, res.PeriodEnd)

	// The first consumer covers the whole period.
	require.NotNil(t, res.Meters[0].FirstData)
	assert.Equal(t, res.PeriodStart, *res.Meters[0].FirstData)
	assert.Equal(t, res.PeriodEnd, *res.Meters[0].LastData)
	// The second one from its activation until its last value.
	assert.Equal(t, bcDay(5).UnixMilli(), *res.Meters[1].FirstData)
	assert.Equal(t, time.Date(2026, 2, 20, 23, 45, 0, 0, time.Local).UnixMilli(), *res.Meters[1].LastData)
	// Unknown metering point: no data, no error.
	assert.Nil(t, res.Meters[3].FirstData)
	assert.Nil(t, res.Meters[3].LastData)
	assert.Equal(t, "AT0030000000000000000000000000099", res.Meters[3].MeteringPoint)
	// Earlier owner: until the end of its inactive day.
	assert.Equal(t, time.Date(2026, 2, 9, 23, 45, 0, 0, time.Local).UnixMilli(), *res.Meters[4].LastData)
	assert.Equal(t, bcDay(9).UnixMilli(), res.Meters[4].InactiveSince, "request values are echoed")

	// Totals cover all metering points of the community, not the requested windows: the earlier
	// owner and the activation of the second consumer do not change them. First consumer 28 days,
	// second consumer has values until 20.02.
	slots := float64((28 + 20) * 96)
	assert.InDelta(t, slots*1, res.Totals.Consumed, 1e-6)
	assert.InDelta(t, slots*0.5, res.Totals.Allocated, 1e-6)
	assert.InDelta(t, slots*0.4, res.Totals.SelfCovered, 1e-6)
	assert.InDelta(t, 28*96*2.0, res.Totals.Produced, 1e-6)
	assert.InDelta(t, 28*96*0.3, res.Totals.Surplus, 1e-6)
}

func TestBillingCheck_MissingTailDays(t *testing.T) {
	bcSeed(t, func(id string, line *model.RawSourceLine) {
		if id >= "CP/2026/02/26" { // nobody delivered the last three days
			for i := range line.QoVConsumers {
				line.QoVConsumers[i], line.Consumers[i] = 0, 0
			}
			line.QoVProducers[0], line.QoVProducers[1] = 0, 0
		}
	})

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(cons(bcCons, 0, 0)))
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 2, 25, 23, 45, 0, 0, time.Local).UnixMilli(), *res.Meters[0].LastData)
	assert.Less(t, *res.Meters[0].LastData, res.PeriodEnd)
}

func TestBillingCheck_GapInsidePeriodIsNoValue(t *testing.T) {
	// Missing rows inside the period are filled by the engine with L0 lines; they are neither a
	// value nor L3.
	bcStore(t, []*model.CounterPointMeta{{ID: "cpmeta/0", Name: bcCons, SourceIdx: 0, Dir: model.CONSUMER_DIRECTION}},
		[]*model.RawSourceLine{
			{Id: "CP/2026/02/02/00/00/00", Consumers: []float64{1, 0, 0}, QoVConsumers: []int{1, 1, 1}},
			{Id: "CP/2026/02/04/00/00/00", Consumers: []float64{1, 0, 0}, QoVConsumers: []int{1, 1, 1}},
		})

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(cons(bcCons, 0, 0)))
	require.NoError(t, err)
	assert.Equal(t, bcDay(2).UnixMilli(), *res.Meters[0].FirstData)
	assert.Equal(t, bcDay(4).UnixMilli(), *res.Meters[0].LastData)
	assert.Equal(t, 0, res.Meters[0].L3Count)
	assert.InDelta(t, 2.0, res.Totals.Consumed, 1e-6)
}

// bcStore writes the given metadata and lines into a fresh store.
func bcStore(t *testing.T, meta []*model.CounterPointMeta, lines []*model.RawSourceLine) {
	viper.Set("persistence.path", t.TempDir())
	db, err := ebow.OpenStorage(bcTenant, bcEcId)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.SetMeta(&model.RawSourceMeta{Id: "cpmeta/0", CounterPoints: meta}))
	require.NoError(t, db.SetLines(lines))
}

func TestBillingCheck_ShortArrays(t *testing.T) {
	// The import grows the arrays only up to the highest index written: the second consumer has only
	// G.01 (array ends inside its block), the first line has no qualities at all.
	bcStore(t, []*model.CounterPointMeta{
		{ID: "cpmeta/0", Name: bcCons, SourceIdx: 0, Dir: model.CONSUMER_DIRECTION},
		{ID: "cpmeta/0", Name: bcCons2, SourceIdx: 1, Dir: model.CONSUMER_DIRECTION},
	}, []*model.RawSourceLine{
		{Id: "CP/2026/02/02/00/00/00", Consumers: []float64{1, 0.5, 0.4, 2}},
		{Id: "CP/2026/02/02/00/15/00", Consumers: []float64{1, 0.5, 0.4, 2}, QoVConsumers: []int{1, 1, 1, 3}},
	})

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(cons(bcCons, 0, 0), cons(bcCons2, 0, 0)))
	require.NoError(t, err)
	assert.Equal(t, bcDay(2).UnixMilli(), *res.Meters[0].FirstData, "a value without quality counts as L1")
	require.NotNil(t, res.Meters[1].FirstData, "values at the end of a short array count")
	assert.Equal(t, 1, res.Meters[1].L3Count)
	assert.InDelta(t, 6.0, res.Totals.Consumed, 1e-9)
}

func TestBillingCheck_InactiveDayWithClockChange(t *testing.T) {
	// 25.10.2026 has 25 hours: the old owner's window still ends at 23:59 local time, and the new
	// owner from the same day does not double the totals.
	lines := []*model.RawSourceLine{}
	for _, hm := range []string{"22/45", "23/00", "23/15", "23/30", "23/45"} {
		lines = append(lines, &model.RawSourceLine{Id: "CP/2026/10/25/" + hm + "/00",
			Consumers: []float64{1, 0, 0}, QoVConsumers: []int{1, 1, 1}})
	}
	bcStore(t, []*model.CounterPointMeta{{ID: "cpmeta/0", Name: bcCons, SourceIdx: 0, Dir: model.CONSUMER_DIRECTION}}, lines)

	day := time.Date(2026, 10, 25, 0, 0, 0, 0, time.Local).UnixMilli()
	res, err := QueryBillingCheck(bcTenant, bcEcId, &model.BillingCheckRequest{
		Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local).UnixMilli(),
		End:   time.Date(2026, 10, 31, 0, 0, 0, 0, time.Local).UnixMilli(),
		Cps:   []model.BillingCheckCp{cons(bcCons, 0, day), cons(bcCons, day, 0)},
	})
	require.NoError(t, err)
	last := time.Date(2026, 10, 25, 23, 45, 0, 0, time.Local).UnixMilli()
	assert.Equal(t, last, *res.Meters[0].LastData)
	assert.Equal(t, last, *res.Meters[1].LastData)
	assert.InDelta(t, 5.0, res.Totals.Consumed, 1e-9)
}

func TestBillingCheck_NoDataInPeriod(t *testing.T) {
	viper.Set("persistence.path", t.TempDir())

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(cons(bcCons, 0, 0)))
	require.NoError(t, err)
	require.Len(t, res.Meters, 1)
	assert.Nil(t, res.Meters[0].FirstData)
	assert.Equal(t, 0, res.Meters[0].L3Count)
	assert.Equal(t, model.BillingCheckTotals{}, res.Totals)
}

func TestBillingCheck_MissingQuarterHours(t *testing.T) {
	bcSeed(t, func(id string, line *model.RawSourceLine) {
		if id >= "CP/2026/02/14" && id < "CP/2026/02/15" { // first consumer: 14.02. missing
			copy(line.QoVConsumers[:3], []int{0, 0, 0})
			copy(line.Consumers[:3], []float64{0, 0, 0})
		}
	})

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(
		cons(bcCons, 0, 0),
		cons(bcCons2, bcDay(5).UnixMilli(), 0),               // values until 20.02.
		cons(bcCons, 0, bcDay(10).UnixMilli()),               // inactive since 10.02.: complete
		cons("AT0030000000000000000000000000099", 0, 0),      // unknown: everything missing
	))
	require.NoError(t, err)
	assert.Equal(t, 96, res.Meters[0].MissingCount)
	assert.Equal(t, []string{"2026-02-14"}, res.Meters[0].MissingDays)
	assert.Equal(t, 8*96, res.Meters[1].MissingCount)
	assert.Equal(t, "2026-02-21", res.Meters[1].MissingDays[0])
	assert.Len(t, res.Meters[1].MissingDays, 8)
	assert.Equal(t, 0, res.Meters[2].MissingCount)
	assert.Equal(t, []string{}, res.Meters[2].MissingDays)
	assert.Equal(t, 28*96, res.Meters[3].MissingCount)
}

func TestBillingCheck_MissingWithoutRows(t *testing.T) {
	viper.Set("persistence.path", t.TempDir())
	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(cons(bcCons, bcDay(27).UnixMilli(), 0)))
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-02-27", "2026-02-28"}, res.Meters[0].MissingDays)
}

// On clock change days only the wall-clock quarter hours that exist count (in Europe/Vienna 92 on
// 29.03., 96 on 25.10. as v1 folds the repeated hour into one row).
func TestBillingCheck_MissingOnClockChangeDays(t *testing.T) {
	lines := []*model.RawSourceLine{}
	for _, d := range []time.Time{time.Date(2026, 3, 29, 0, 0, 0, 0, time.Local), time.Date(2026, 10, 25, 0, 0, 0, 0, time.Local)} {
		for h := 0; h < 24; h++ {
			for m := 0; m < 60; m += 15 {
				ts := time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, time.Local)
				if ts.Hour() != h || ts.Minute() != m {
					continue // does not exist on this day
				}
				lines = append(lines, &model.RawSourceLine{Id: fmt.Sprintf("CP/%04d/%02d/%02d/%02d/%02d/00", d.Year(), d.Month(), d.Day(), h, m),
					Consumers: []float64{1, 0, 0}, QoVConsumers: []int{1, 1, 1}})
			}
		}
	}
	bcStore(t, []*model.CounterPointMeta{{ID: "cpmeta/0", Name: bcCons, SourceIdx: 0, Dir: model.CONSUMER_DIRECTION}}, lines)

	for _, d := range []time.Time{time.Date(2026, 3, 29, 0, 0, 0, 0, time.Local), time.Date(2026, 10, 25, 0, 0, 0, 0, time.Local)} {
		res, err := QueryBillingCheck(bcTenant, bcEcId, &model.BillingCheckRequest{
			Start: d.UnixMilli(), End: d.UnixMilli(),
			Cps: []model.BillingCheckCp{cons(bcCons, 0, 0)},
		})
		require.NoError(t, err)
		assert.Equal(t, 0, res.Meters[0].MissingCount, d.Format("2006-01-02"))
	}
}
