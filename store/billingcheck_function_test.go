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

	// Quarter hours: first consumer 28 days, second 5.-20.02. (16 days), earlier owner 1.-9.02.
	slots := float64((28 + 16 + 9) * 96)
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
	viper.Set("persistence.path", t.TempDir())
	db, err := ebow.OpenStorage(bcTenant, bcEcId)
	require.NoError(t, err)
	require.NoError(t, db.SetMeta(&model.RawSourceMeta{Id: "cpmeta/0", CounterPoints: []*model.CounterPointMeta{
		{ID: "cpmeta/0", Name: bcCons, SourceIdx: 0, Dir: model.CONSUMER_DIRECTION},
	}}))
	require.NoError(t, db.SetLines([]*model.RawSourceLine{
		{Id: "CP/2026/02/02/00/00/00", Consumers: []float64{1, 0, 0}, QoVConsumers: []int{1, 1, 1}},
		{Id: "CP/2026/02/04/00/00/00", Consumers: []float64{1, 0, 0}, QoVConsumers: []int{1, 1, 1}},
	}))
	db.Close()

	res, err := QueryBillingCheck(bcTenant, bcEcId, bcRequest(cons(bcCons, 0, 0)))
	require.NoError(t, err)
	assert.Equal(t, bcDay(2).UnixMilli(), *res.Meters[0].FirstData)
	assert.Equal(t, bcDay(4).UnixMilli(), *res.Meters[0].LastData)
	assert.Equal(t, 0, res.Meters[0].L3Count)
	assert.InDelta(t, 2.0, res.Totals.Consumed, 1e-6)
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
