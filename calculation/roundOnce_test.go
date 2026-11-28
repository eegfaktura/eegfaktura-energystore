package calculation

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/require"
)

// updateGolden rewrites testdata/report_golden.json:
// `go test ./calculation -run Golden -update-golden`.
// Regenerate only when the report output is meant to change.
var updateGolden = flag.Bool("update-golden", false, "rewrite testdata/report_golden.json")

// sliceRange is a plain ebow.IRange over prepared lines (no mock bookkeeping, so the
// benchmark measures the calculation).
type sliceRange struct {
	lines []*model.RawSourceLine
	pos   int
}

func (r *sliceRange) Next(result interface{}) bool {
	if r.pos >= len(r.lines) {
		return false
	}
	*result.(*model.RawSourceLine) = *r.lines[r.pos]
	r.pos++
	return true
}
func (r *sliceRange) Close()     {}
func (r *sliceRange) Err() error { return nil }

// reportFixture builds `days` days of quarter-hour lines from 2024-01-01 for the given
// consumers and producers. Values carry 7 decimals so the 6-digit rounding matters.
func reportFixture(consumers, producers, days int) ([]*model.RawSourceLine, map[string]*model.CounterPointMeta, *model.CounterPointMetaInfo) {
	rnd := rand.New(rand.NewSource(42))
	val := func(max float64) float64 { return float64(rnd.Int63n(int64(max*1e7))) / 1e7 }

	lines := []*model.RawSourceLine{}
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
	for d := 0; d < days; d++ {
		for q := 0; q < 96; q++ {
			ts := start.AddDate(0, 0, d).Add(time.Duration(q) * 15 * time.Minute)
			cons := make([]float64, 0, consumers*3)
			for c := 0; c < consumers; c++ {
				consumption := val(2)
				share := val(consumption + 1e-7)
				alloc := val(share + 1e-7)
				cons = append(cons, consumption, share, alloc)
			}
			prod := make([]float64, 0, producers*2)
			for p := 0; p < producers; p++ {
				production := val(20)
				prod = append(prod, production, val(production+1e-7))
			}
			lines = append(lines, &model.RawSourceLine{
				Id:        fmt.Sprintf("CP/%.4d/%.2d/%.2d/%.2d/%.2d/00", ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute()),
				Consumers: cons,
				Producers: prod,
			})
		}
	}

	cpMeta := map[string]*model.CounterPointMeta{}
	for c := 0; c < consumers; c++ {
		id := fmt.Sprintf("CONS%d", c)
		cpMeta[id] = &model.CounterPointMeta{Name: id, SourceIdx: c, Dir: model.CONSUMER_DIRECTION}
	}
	for p := 0; p < producers; p++ {
		id := fmt.Sprintf("PROD%d", p)
		cpMeta[id] = &model.CounterPointMeta{Name: id, SourceIdx: p, Dir: model.PRODUCER_DIRECTION}
	}
	return lines, cpMeta, &model.CounterPointMetaInfo{ConsumerCount: consumers, ProducerCount: producers}
}

// reportRequest asks for every meter of the fixture. Variants: ZVT windows on every
// second meter, a consumer with windows active on days 2-3 only, a consumer in two
// consecutive participations (owner change after day 2), and a requested meter without
// energy data.
func reportRequest(consumers, producers int) *model.ReportResponse {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	until := time.Date(2024, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli()
	windows := []model.TimeWindow{{Key: "T1", From: "06:00", To: "08:00"}, {Key: "T2", From: "20:00", To: "06:00"}}

	report := &model.ReportResponse{}
	add := func(id string, i int) {
		m := &model.MeterReport{MeterId: id, From: from, Until: until}
		if i%2 == 0 {
			m.TimeWindows = windows
		}
		report.ParticipantReports = append(report.ParticipantReports,
			model.ParticipantReport{ParticipantId: "P-" + id, Meters: []*model.MeterReport{m}})
	}
	for c := 0; c < consumers; c++ {
		add(fmt.Sprintf("CONS%d", c), c)
	}
	for p := 0; p < producers; p++ {
		add(fmt.Sprintf("PROD%d", p), p)
	}
	report.ParticipantReports[2].Meters[0].From = time.Date(2024, 1, 2, 0, 0, 0, 0, time.Local).UnixMilli()
	report.ParticipantReports[2].Meters[0].Until = time.Date(2024, 1, 3, 0, 0, 0, 0, time.Local).UnixMilli()
	report.ParticipantReports[0].Meters[0].Until = time.Date(2024, 1, 2, 0, 0, 0, 0, time.Local).UnixMilli()
	report.ParticipantReports = append(report.ParticipantReports, model.ParticipantReport{
		ParticipantId: "P-second",
		Meters: []*model.MeterReport{
			{MeterId: "CONS0", From: time.Date(2024, 1, 3, 0, 0, 0, 0, time.Local).UnixMilli(), Until: until, TimeWindows: windows[:1]},
			{MeterId: "NODATA", From: from, Until: until},
		},
	})
	return report
}

func runReport(tb testing.TB, lines []*model.RawSourceLine, cpMeta map[string]*model.CounterPointMeta,
	metaInfo *model.CounterPointMetaInfo, report *model.ReportResponse) {
	reportValues := ConvertToMeterMap(report)
	err := calcParticipantReport(&sliceRange{lines: lines}, &reportValues, AllocDynamicV2, cpMeta, metaInfo, "CP",
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local),
		func(currentDate time.Time) int { return currentDate.Day() })
	require.NoError(tb, err)
}

// TestCalcParticipantReportGolden pins the whole report bit for bit. The meter values
// (summaries, intermediates, buckets) are the output of the former per-meter rounding; the
// two community totals are those of the sorted summation (261128-mm1), before which their
// last bits depended on Go map order.
func TestCalcParticipantReportGolden(t *testing.T) {
	lines, cpMeta, metaInfo := reportFixture(8, 3, 4)
	report := reportRequest(8, 3)
	runReport(t, lines, cpMeta, metaInfo, report)

	const golden = "testdata/report_golden.json"
	if *updateGolden {
		out, err := json.MarshalIndent(report, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, append(out, '\n'), 0o644))
	}
	raw, err := os.ReadFile(golden)
	require.NoError(t, err)
	var want model.ReportResponse
	require.NoError(t, json.Unmarshal(raw, &want))

	wantJSON, err := json.MarshalIndent(want, "", "  ")
	require.NoError(t, err)
	gotJSON, err := json.MarshalIndent(report, "", "  ")
	require.NoError(t, err)
	require.Equal(t, string(wantJSON), string(gotJSON))
}

// BenchmarkCalcParticipantReport: 500 consumers, 10 producers, 30 days, every meter requested.
func BenchmarkCalcParticipantReport(b *testing.B) {
	lines, cpMeta, metaInfo := reportFixture(500, 10, 30)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		report := reportRequest(500, 10)
		b.StartTimer()
		runReport(b, lines, cpMeta, metaInfo, report)
	}
}
