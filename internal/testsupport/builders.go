package testsupport

import (
	"fmt"
	"math"
	"testing"
	"time"

	"at.ourproject/energystore/model"
)

// KWhDelta is the tolerance for energy values: sums over 96 values differ in the last digits
// with the order of addition (AGENTS.md section 4).
const KWhDelta = 1e-6

// InDelta fails the test when |want - got| > KWhDelta.
func InDelta(t testing.TB, want, got float64, msgAndArgs ...any) bool {
	t.Helper()
	if math.Abs(want-got) <= KWhDelta && !math.IsNaN(got) {
		return true
	}
	msg := ""
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprintf(fmt.Sprint(msgAndArgs[0]), msgAndArgs[1:]...) + ": "
	}
	t.Errorf("%swant %.9f, got %.9f (tolerance %g)", msg, want, got, KWhDelta)
	return false
}

// RowId builds the stored row id "CP/yyyy/MM/dd/hh/mm/ss" of a local wall-clock time. It is
// written here independently of utils so the tests do not use the code under test as oracle.
func RowId(ts time.Time) string {
	l := ts.In(time.Local)
	return fmt.Sprintf("CP/%04d/%02d/%02d/%02d/%02d/%02d", l.Year(), int(l.Month()), l.Day(), l.Hour(), l.Minute(), l.Second())
}

// QuarterHours returns the quarter-hour starts of the local day of day (92, 96 or 100).
func QuarterHours(day time.Time) []time.Time {
	l := day.In(time.Local)
	start := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.Local)
	end := time.Date(l.Year(), l.Month(), l.Day()+1, 0, 0, 0, 0, time.Local)
	var slots []time.Time
	for ts := start; ts.Before(end); ts = ts.Add(15 * time.Minute) {
		slots = append(slots, ts)
	}
	return slots
}

// RawLine builds a raw line with the given values and quality 1 (L1) for every value.
// Consumers are triples (G.01, G.02, G.03) per metering point, producers pairs (G.01, P.01).
func RawLine(id string, consumers, producers []float64) *model.RawSourceLine {
	return RawLineQoV(id, consumers, producers, ones(len(consumers)), ones(len(producers)))
}

// RawLineQoV builds a raw line with explicit quality values.
func RawLineQoV(id string, consumers, producers []float64, qovC, qovP []int) *model.RawSourceLine {
	return &model.RawSourceLine{
		Id:           id,
		Consumers:    append([]float64{}, consumers...),
		Producers:    append([]float64{}, producers...),
		QoVConsumers: append([]int{}, qovC...),
		QoVProducers: append([]int{}, qovP...),
	}
}

func ones(n int) []int {
	r := make([]int, n)
	for i := range r {
		r[i] = 1
	}
	return r
}

// Consumer is a metering point of the meta record with direction CONSUMPTION.
func Consumer(name string, sourceIdx int, periodStart, periodEnd string) *model.CounterPointMeta {
	return &model.CounterPointMeta{ID: fmt.Sprintf("%03d", sourceIdx), Name: name, SourceIdx: sourceIdx,
		Dir: model.CONSUMER_DIRECTION, PeriodStart: periodStart, PeriodEnd: periodEnd}
}

// Producer is a metering point of the meta record with direction GENERATION.
func Producer(name string, sourceIdx int, periodStart, periodEnd string) *model.CounterPointMeta {
	return &model.CounterPointMeta{ID: fmt.Sprintf("%03d", sourceIdx), Name: name, SourceIdx: sourceIdx,
		Dir: model.PRODUCER_DIRECTION, PeriodStart: periodStart, PeriodEnd: periodEnd}
}

// CpMeta builds the meta record cpmeta/0 from metering points.
func CpMeta(cps ...*model.CounterPointMeta) *model.RawSourceMeta {
	return &model.RawSourceMeta{Id: "cpmeta/0", CounterPoints: cps, NumberOfMetering: len(cps)}
}
