package scenario_test

import (
	"strconv"
	"testing"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func num(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	require.NoError(t, err, s)
	return v
}

// S1 (export) — the workbook communities download: the summary sheet carries the day's sums, the
// energy sheet one row per quarter hour with the values as imported (read back with excelize).
func TestS01_ExportWorkbook(t *testing.T) {
	community(t)
	importMQTT(t, consumerMsg(june1, 96, "L1"), producerMsg(june1, 96, "L1"))
	active := june1.AddDate(1, 0, 0).UnixMilli()
	b, err := excel.ExportEnergyToExcel(tenant, ecId, june1, june1, &excel.ExportParticipantEnergy{
		Start: june1.UnixMilli(), End: june1.UnixMilli(), CommunityId: ecId,
		Cps: []excel.ParticipantCp{
			{MeteringPoint: consumer, Direction: model.CONSUMER_DIRECTION, ActiveSince: june1.UnixMilli(), InactiveSince: active},
			{MeteringPoint: producer, Direction: model.PRODUCER_DIRECTION, ActiveSince: june1.UnixMilli(), InactiveSince: active},
		}})
	require.NoError(t, err)
	f, err := excelize.OpenReader(b)
	require.NoError(t, err)
	require.Equal(t, []string{"Summary", "Energiedaten"}, f.GetSheetList())

	summary, err := f.GetRows("Summary")
	require.NoError(t, err)
	assert.Equal(t, []string{"Gemeinschafts-ID", ecId}, summary[1])
	for row, want := range map[int]float64{4: sum(values(cons, 96)), 5: sum(values(share, 96)), 6: sum(values(cover, 96)),
		7: sum(values(surp, 96)), 8: sum(values(prod, 96))} {
		testsupport.InDelta(t, want, num(t, summary[row][1]), "summary %s", summary[row][0])
	}

	rows, err := f.GetRows("Energiedaten")
	require.NoError(t, err)
	assert.Equal(t, []string{"MeteringpointID", consumer, consumer, consumer, producer, producer}, rows[1])
	data := rows[10:]
	require.Len(t, data, 96)
	for i, ts := range testsupport.QuarterHours(june1) {
		assert.Equal(t, ts.Format("02.01.2006 15:04:05"), data[i][0])
		testsupport.InDelta(t, cons(i), num(t, data[i][1]), "G.01 row %d", i)
		testsupport.InDelta(t, cover(i), num(t, data[i][3]), "G.03 row %d", i)
		testsupport.InDelta(t, prod(i), num(t, data[i][4]), "production row %d", i)
		testsupport.InDelta(t, surp(i), num(t, data[i][5]), "surplus row %d", i)
	}
}

// S5 (periods) — quarter, half year and year add up the stored days. The number and the borders of
// the intermediate buckets are not asserted: that is open-points ES-14 (scenario S6).
func TestS05_PeriodTotals(t *testing.T) {
	community(t)
	importMQTT(t, consumerMsg(june1, 2*96, "L1")) // 1. and 2.6.2026
	day := sum(values(cons, 96))
	from, until := june1.AddDate(0, -5, 0).UnixMilli(), june1.AddDate(0, 6, 0).UnixMilli()
	for _, tc := range []struct {
		segment int
		code    string
	}{{2, "YQ"}, {1, "YH"}, {0, "Y"}} {
		report, err := calculation.EnergyReportV2(tenant, ecId, []model.ParticipantReport{{ParticipantId: "C",
			Meters: []*model.MeterReport{{MeterId: consumer, MeterDir: "CONSUMPTION", From: from, Until: until}}}}, 2026, tc.segment, tc.code)
		require.NoError(t, err, tc.code)
		r := report.ParticipantReports[0].Meters[0].Report
		require.NotNil(t, r, tc.code)
		testsupport.InDelta(t, 2*day, r.Summary.Consumption, tc.code)
	}
}
