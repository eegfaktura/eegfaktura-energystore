package scenario_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var june1 = time.Date(2026, 6, 1, 0, 0, 0, 0, vienna)

// S7 — mixed quality: L1, L2 and L3 blocks are stored and read with their quality.
func TestS07_QualityIsStoredPerSlot(t *testing.T) {
	community(t)
	msg := testsupport.CrMsg(ecId, consumer).Direction(model.CONSUMER_DIRECTION)
	for k, method := range []string{"L1", "L2", "L3"} {
		start := june1.Add(time.Duration(k*8) * time.Hour)
		msg.Energy(start, method, map[model.MeterCodeValue][]float64{model.CODE_CON: values(cons, 32)})
	}
	importMQTT(t, msg.Message())

	c := raw(t, consumer, june1, june1)
	require.Len(t, c, 96)
	for i, d := range c {
		assert.Equal(t, i/32+1, d.Qov[0], "slot %d", i)
	}
}

// A later L2 value replaces a stored L1 value today (known-errors #35, F20). Whether it may is the
// open business question ES-15; this records today's behaviour, it does not say it is right.
func TestS07_LateMessageOverwrites(t *testing.T) {
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	late := testsupport.CrMsg(ecId, consumer).Direction(model.CONSUMER_DIRECTION).
		Energy(june1, "L2", map[model.MeterCodeValue][]float64{model.CODE_CON: {9.99}}).Message()
	importMQTT(t, late)

	c := raw(t, consumer, june1, june1)
	testsupport.InDelta(t, 9.99, c[0].Value[0], "ES-15: today the late L2 value wins")
	assert.Equal(t, 2, c[0].Qov[0])
	testsupport.InDelta(t, cons(1), c[1].Value[0], "the next slot is untouched")
}

// The load curve of all-L1 data must report quality 1 (known-errors #30, F15).
func TestS07_LoadCurveQualityOfL1Data(t *testing.T) {
	t.Skip("known-errors #30")
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	curve, err := store.QueryLoadCurveReport(tenant, ecId, june1, june1.Add(23*time.Hour+45*time.Minute), nil)
	require.NoError(t, err)
	require.NotEmpty(t, curve)
	for _, e := range curve {
		for _, d := range reportData(e) {
			assert.Equal(t, 1, d.QoVConsumer)
		}
	}
}

// reportData extracts the ReportData values of a load-curve entry (a ReportNamedData).
func reportData(e interface{}) []*store.ReportData {
	switch v := e.(type) {
	case *store.ReportData:
		return []*store.ReportData{v}
	case *store.ReportNamedData:
		return []*store.ReportData{v.ReportData}
	}
	return nil
}

// S8 — an export that names a metering point without stored data must not panic
// (known-errors #24, F9): an error or an empty column.
func TestS08_ExportForAMeterWithoutData(t *testing.T) {
	t.Skip("known-errors #24")
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	cps := &excel.ExportParticipantEnergy{Start: june1.UnixMilli(), End: june1.UnixMilli(), CommunityId: ecId,
		Cps: []excel.ParticipantCp{
			{MeteringPoint: consumer, Direction: model.CONSUMER_DIRECTION, ActiveSince: june1.UnixMilli(), InactiveSince: june1.AddDate(1, 0, 0).UnixMilli()},
			{MeteringPoint: "AT0030000000000000000000000009999", Direction: model.CONSUMER_DIRECTION, ActiveSince: june1.UnixMilli(), InactiveSince: june1.AddDate(1, 0, 0).UnixMilli()},
		}}
	assert.NotPanics(t, func() {
		_, _ = excel.ExportEnergyToExcel(tenant, ecId, june1, june1, cps)
	})
}

// S10 — deleting a range of one metering point zeroes exactly that range (end included); the
// neighbours and the report outside it stay.
func TestS10_RawDataDelete(t *testing.T) {
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	from, to := june1.Add(10*time.Hour), june1.Add(11*time.Hour)

	affected, kwh, err := store.DeleteRawDataForMeteringPoint(tenant, ecId, consumer, from, to, false)
	require.NoError(t, err)
	assert.Equal(t, 5, affected, "10:00 … 11:00, the end slot included")
	deleted := 0.0
	for i := 40; i <= 44; i++ {
		deleted += cons(i)
	}
	testsupport.InDelta(t, deleted, kwh)

	c := raw(t, consumer, june1, june1)
	for i, d := range c {
		if i >= 40 && i <= 44 {
			assert.Equal(t, 0.0, d.Value[0], "deleted slot %d", i)
		} else {
			testsupport.InDelta(t, cons(i), d.Value[0], "slot %d", i)
		}
	}
	report, err := calculation.EnergyReportV2(tenant, ecId, []model.ParticipantReport{{ParticipantId: "C",
		Meters: []*model.MeterReport{{MeterId: consumer, MeterDir: "CONSUMPTION", From: june1.UnixMilli(), Until: june1.UnixMilli()}}}}, 2026, 6, "YM")
	require.NoError(t, err)
	testsupport.InDelta(t, sum(values(cons, 96))-deleted, report.ParticipantReports[0].Meters[0].Report.Summary.Consumption)
}

// S11 — a stored row that cannot be decoded must make the read fail, not return a short report
// (known-errors #31, F16).
func TestS11_CorruptRowIsAnError(t *testing.T) {
	t.Skip("known-errors #31")
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	ebow.ClosePool()

	db, err := ebow.Open(filepath.Join(viper.GetString("persistence.path"), strings.ToLower(tenant), ecId))
	require.NoError(t, err)
	type badLine struct {
		Id        string `bow:"key"`
		Consumers string
	}
	require.NoError(t, db.Bucket("rawdata").Put(badLine{Id: "CP/2026/06/01/12/00/00", Consumers: "garbage"}))
	require.NoError(t, db.Close())

	_, err = store.QueryRawData(tenant, ecId, june1, june1, []store.TargetMP{{MeteringPoint: consumer}}, nil)
	assert.Error(t, err, "raw data")
	_, err = calculation.EnergyReportV2(tenant, ecId, []model.ParticipantReport{{ParticipantId: "C",
		Meters: []*model.MeterReport{{MeterId: consumer, MeterDir: "CONSUMPTION", From: june1.UnixMilli(), Until: june1.UnixMilli()}}}}, 2026, 6, "YM")
	assert.Error(t, err, "report")
}

// S12 — intra-day report and load curve over S1's day add up to the day.
func TestS12_IntraDayAndLoadCurveSumToTheDay(t *testing.T) {
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	end := june1.Add(23*time.Hour + 45*time.Minute)

	hours, err := store.QueryIntraDayReport(tenant, ecId, june1, end)
	require.NoError(t, err)
	require.Len(t, hours, 24)
	var total float64
	for _, h := range hours {
		total += h.(*store.ReportData).Consumed
	}
	testsupport.InDelta(t, sum(values(cons, 96)), total, "intra-day")

	curve, err := store.QueryLoadCurveReport(tenant, ecId, june1, end, nil)
	require.NoError(t, err)
	total = 0
	for _, e := range curve {
		for _, d := range reportData(e) {
			total += d.Consumed
		}
	}
	testsupport.InDelta(t, sum(values(cons, 96)), total, "load curve")
}

// Hour key h of the intra-day report holds the slots h:00 – h:45 (known-errors #26, F11: today
// 00:00 – 01:00 lands in hour 23).
func TestS12_IntraDayHourKeys(t *testing.T) {
	t.Skip("known-errors #26")
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"))
	hours, err := store.QueryIntraDayReport(tenant, ecId, june1, june1.Add(23*time.Hour+45*time.Minute))
	require.NoError(t, err)
	for h := 0; h < 24; h++ {
		want := cons(4*h) + cons(4*h+1) + cons(4*h+2) + cons(4*h+3)
		testsupport.InDelta(t, want, hours[h].(*store.ReportData).Consumed, "hour %d", h)
	}
}

// F28 (known-errors #43, reproduced in M3): a meta record whose consumer indices have a gap (0 and 2)
// and a stored line of three consumers; the hourly aggregation of /raw must not panic.
func TestS12_AggregateWithIndexGap(t *testing.T) {
	t.Skip("known-errors #43")
	db := testsupport.TempStore(t, tenant, ecId)
	require.NoError(t, db.SetMeta(testsupport.CpMeta(
		testsupport.Consumer(consumer, 0, "01.06.2026 00:00:00", "01.06.2026 23:45:00"),
		testsupport.Consumer("AT0030000000000000000000000000003", 2, "01.06.2026 00:00:00", "01.06.2026 23:45:00"))))
	for i, ts := range testsupport.QuarterHours(june1) {
		require.NoError(t, db.SetLine(testsupport.RawLine(testsupport.RowId(ts), []float64{cons(i), 0, 0, 0, 0, 0, 1, 0, 0}, nil)))
	}
	assert.NotPanics(t, func() {
		_, err := store.QueryRawData(tenant, ecId, june1, june1, []store.TargetMP{{MeteringPoint: consumer}}, map[string][]string{"f": {"agg(1h)"}})
		assert.NoError(t, err)
	})
}
