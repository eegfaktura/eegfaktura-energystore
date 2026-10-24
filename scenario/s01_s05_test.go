package scenario_test

import (
	"testing"
	"time"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S1 — one consumer, one producer, 2026-06-01, imported over MQTT: 96 slots each, values and daily
// sums as sent, the monthly report and the summary carry the sums.
func TestS01_OneConsumerOneProducer(t *testing.T) {
	community(t)
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(day, 96, "L1"), producerMsg(day, 96, "L1"))

	c := raw(t, consumer, day, day)
	p := raw(t, producer, day, day)
	require.Len(t, c, 96)
	require.Len(t, p, 96)
	for i, ts := range testsupport.QuarterHours(day) {
		assert.Equal(t, ts.UnixMilli(), c[i].Ts, "slot %d", i)
		testsupport.InDelta(t, cons(i), c[i].Value[0], "G.01 slot %d", i)
		testsupport.InDelta(t, share(i), c[i].Value[1], "G.02 slot %d", i)
		testsupport.InDelta(t, cover(i), c[i].Value[2], "G.03 slot %d", i)
		testsupport.InDelta(t, prod(i), p[i].Value[0], "producer G.01 slot %d", i)
		testsupport.InDelta(t, surp(i), p[i].Value[1], "producer P.01 slot %d", i)
		assert.Equal(t, []int{1, 1, 1}, c[i].Qov)
	}

	from, until := day.UnixMilli(), day.UnixMilli()
	report, err := calculation.EnergyReportV2(tenant, ecId, []model.ParticipantReport{
		{ParticipantId: "C", Meters: []*model.MeterReport{{MeterId: consumer, MeterDir: "CONSUMPTION", From: from, Until: until}}},
		{ParticipantId: "P", Meters: []*model.MeterReport{{MeterId: producer, MeterDir: "GENERATION", From: from, Until: until}}},
	}, 2026, 6, "YM")
	require.NoError(t, err)
	assert.Equal(t, "YM/2026/06", report.Id)
	cr := report.ParticipantReports[0].Meters[0].Report
	pr := report.ParticipantReports[1].Meters[0].Report
	require.NotNil(t, cr)
	require.NotNil(t, pr)
	testsupport.InDelta(t, sum(values(cons, 96)), cr.Summary.Consumption, "consumption")
	testsupport.InDelta(t, sum(values(share, 96)), cr.Summary.Allocation, "allocation (G.02)")
	testsupport.InDelta(t, sum(values(cover, 96)), cr.Summary.Utilization, "utilization (G.03)")
	testsupport.InDelta(t, sum(values(prod, 96)), pr.Summary.Production, "production")
	testsupport.InDelta(t, sum(values(cons, 96)), report.TotalConsumption, "total consumption")
	require.Len(t, report.Meta, 2)

	summary, err := calculation.EnergySummary(tenant, ecId, 2026, 6, "YM")
	require.NoError(t, err)
	s := summary.([]interface{})[0].(*store.ReportData)
	testsupport.InDelta(t, sum(values(cons, 96)), s.Consumed)
	testsupport.InDelta(t, sum(values(share, 96)), s.Allocated)
	testsupport.InDelta(t, sum(values(cover, 96)), s.Distributed)
	testsupport.InDelta(t, sum(values(prod, 96)), s.Produced)
}

// The summary of all-L1 data must report quality 1; it starts at 0 and calcQoV(0, 1) stays 0
// (known-errors #30, F15 — the same start value as the load curve and the intra-day report).
func TestS01_SummaryQualityOfL1Data(t *testing.T) {
	t.Skip("known-errors #30")
	community(t)
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(day, 96, "L1"), producerMsg(day, 96, "L1"))
	summary, err := calculation.EnergySummary(tenant, ecId, 2026, 6, "YM")
	require.NoError(t, err)
	s := summary.([]interface{})[0].(*store.ReportData)
	assert.Equal(t, 1, s.QoVConsumer)
	assert.Equal(t, 1, s.QoVProducer)
}

// S2 — the spring day 2026-03-29 has 92 quarter hours; 01:45 is followed by 03:00.
func TestS02_SpringDayStores92Slots(t *testing.T) {
	community(t)
	day := testsupportSpring()
	importMQTT(t, consumerMsg(day, 92, "L1"))

	ids := storedIds(t, day)
	require.Len(t, ids, 92)
	assert.Contains(t, ids, "CP/2026/03/29/01/45/00")
	assert.Contains(t, ids, "CP/2026/03/29/03/00/00")
	assert.NotContains(t, ids, "CP/2026/03/29/02/00/00")
}

// The read side must not see the 01:45 → 03:00 jump as a gap: 92 slots, no fill rows, no L0
// (known-errors #22, F7).
func TestS02_SpringDayReadsWithoutPhantomSlots(t *testing.T) {
	t.Skip("known-errors #22")
	community(t)
	day := testsupportSpring()
	importMQTT(t, consumerMsg(day, 92, "L1"))

	c := raw(t, consumer, day, day)
	assert.Len(t, c, 92)
	for _, d := range c {
		assert.Equal(t, 1, d.Qov[0], "no fill row (QoV 0) at %s", time.UnixMilli(d.Ts).In(vienna))
	}
}

func testsupportSpring() time.Time { return time.Date(2026, 3, 29, 0, 0, 0, 0, vienna) }

// S3 — the autumn day 2026-10-25 has 100 quarter hours. Over MQTT the total is kept.
func TestS03_AutumnDayTotalIsKept(t *testing.T) {
	community(t)
	day := time.Date(2026, 10, 25, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(day, 100, "L1"))

	var total float64
	for _, d := range raw(t, consumer, day, day) {
		total += d.Value[0]
	}
	testsupport.InDelta(t, sum(values(cons, 100)), total, "daily total of G.01")
}

// Both 02:xx hours must be kept as separate slots: 100 slots (known-errors #25, F10; today the
// second hour is summed into the first key: 96 keys).
func TestS03_AutumnDayKeeps100Slots(t *testing.T) {
	t.Skip("known-errors #25")
	community(t)
	day := time.Date(2026, 10, 25, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(day, 100, "L1"))
	assert.Len(t, raw(t, consumer, day, day), 100)
}

// S4 — one hour missing (10:00 – 10:45) on a normal day: the read side fills exactly those four
// slots with zero and quality 0 (known-errors #22, F7: today the fill rows land 2 h later and
// duplicate real slots).
func TestS04_MissingHourIsFilledInPlace(t *testing.T) {
	t.Skip("known-errors #22")
	community(t)
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(day, 40, "L1"), consumerMsg(day.Add(11*time.Hour), 52, "L1"))

	c := raw(t, consumer, day, day)
	require.Len(t, c, 96)
	seen := map[int64]bool{}
	for i, ts := range testsupport.QuarterHours(day) {
		assert.Equal(t, ts.UnixMilli(), c[i].Ts, "slot %d", i)
		assert.False(t, seen[c[i].Ts], "duplicate timestamp")
		seen[c[i].Ts] = true
		if ts.Hour() == 10 {
			assert.Equal(t, 0.0, c[i].Value[0], "fill row %s", ts)
			assert.Equal(t, 0, c[i].Qov[0], "fill row is L0")
		}
	}
}

func TestS04_MissingHourIsNotStored(t *testing.T) {
	community(t)
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(day, 40, "L1"), consumerMsg(day.Add(11*time.Hour), 52, "L1"))
	ids := storedIds(t, day)
	assert.Len(t, ids, 92)
	assert.NotContains(t, ids, "CP/2026/06/01/10/00/00")
}

// S5 — year end, leap day and month boundaries: each monthly report holds exactly its days.
func TestS05_PeriodBoundaries(t *testing.T) {
	community(t)
	dec31 := time.Date(2027, 12, 31, 0, 0, 0, 0, vienna)
	feb28 := time.Date(2028, 2, 28, 0, 0, 0, 0, vienna)
	importMQTT(t, consumerMsg(dec31, 2*96, "L1"), consumerMsg(feb28, 3*96, "L1")) // 31.12.–1.1., 28.2.–1.3.

	assert.Len(t, raw(t, consumer, dec31, dec31.AddDate(0, 0, 1)), 192, "over the year end")
	assert.Len(t, raw(t, consumer, feb28.AddDate(0, 0, 1), feb28.AddDate(0, 0, 1)), 96, "29.02.2028")

	day := sum(values(cons, 96))
	for _, tc := range []struct {
		year, month int
		want        float64
	}{{2027, 12, day}, {2028, 1, day}, {2028, 2, 2 * day}, {2028, 3, day}} {
		from := time.Date(tc.year, time.Month(tc.month), 1, 0, 0, 0, 0, vienna)
		until := from.AddDate(0, 1, -1)
		report, err := calculation.EnergyReportV2(tenant, ecId, []model.ParticipantReport{{ParticipantId: "C",
			Meters: []*model.MeterReport{{MeterId: consumer, MeterDir: "CONSUMPTION", From: from.UnixMilli(), Until: until.UnixMilli()}}}},
			tc.year, tc.month, "YM")
		require.NoError(t, err, "%d/%d", tc.year, tc.month)
		r := report.ParticipantReports[0].Meters[0].Report
		require.NotNil(t, r, "%d/%d", tc.year, tc.month)
		testsupport.InDelta(t, tc.want, r.Summary.Consumption, "%d/%02d", tc.year, tc.month)
	}
}
