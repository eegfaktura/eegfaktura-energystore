package model

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func quotaMatrix3x2() *QuotaMatrix {
	q := NewQuotaMatrix([]string{"ZP1", "ZP2", "ZP3"}, []string{"P1", "P2"})
	q.Add("ZP1", "P1", 0.4)
	q.Add("ZP1", "P2", 0.6)
	q.Add("ZP2", "P1", 0.5)
	q.Add("ZP2", "P2", 0.5)
	return q
}

func TestQuotaMatrixKnownNames(t *testing.T) {
	q := quotaMatrix3x2()
	assert.InDelta(t, 0.6, q.GetQuota("ZP1", "P2"), 1e-12)
	assert.InDelta(t, 0.4, q.GetAllocQuota("ZP1", "P1"), 1e-12, "share of P1 in ZP1's row")
	assert.InDelta(t, 0.5, q.GetAllocQuota("ZP2", "P2"), 1e-12)

	q.Add("UNKNOWN", "P1", 9) // ignored
	q.Add("ZP1", "UNKNOWN", 9)
	assert.InDelta(t, 0.4, q.GetQuota("ZP1", "P1"), 1e-12)
}

// QuotaMatrix is used by tests only today (known-errors #46, F31): an unknown name falls back to
// row/column 0, a row without quota divides by zero, Validate assumes three producers.
func TestQuotaMatrixEdgeCases(t *testing.T) {
	t.Skip("known-errors #46")
	q := quotaMatrix3x2()
	assert.Equal(t, 0.0, q.GetQuota("UNKNOWN", "P1"), "unknown metering point")
	assert.Equal(t, 0.0, q.GetQuota("ZP1", "UNKNOWN"), "unknown producer")
	alloc := q.GetAllocQuota("ZP3", "P1")
	assert.False(t, math.IsNaN(alloc) || math.IsInf(alloc, 0), "zero quota row: %v", alloc)
	assert.NotPanics(t, func() {
		sums := q.Validate()
		require.Equal(t, 3, sums.Rows)
		assert.InDelta(t, 1.0, sums.GetElm(0, 0), 1e-12)
	}, "two producers")
}

func TestMakeRawSourceLineSizes(t *testing.T) {
	l := MakeRawSourceLine("CP/2026/06/01/00/00/00", 6, 4)
	assert.Equal(t, "CP/2026/06/01/00/00/00", l.Id)
	assert.Len(t, l.Consumers, 6)
	assert.Len(t, l.QoVConsumers, 6)
	assert.Len(t, l.Producers, 4)
}

// QoVProducers is sized with the consumer size (known-errors #46, sourcemodel.go:43).
func TestMakeRawSourceLineProducerQoVSize(t *testing.T) {
	t.Skip("known-errors #46")
	assert.Len(t, MakeRawSourceLine("CP/2026/06/01/00/00/00", 6, 4).QoVProducers, 4)
}

func TestRawSourceLineCopies(t *testing.T) {
	src := RawSourceLine{Id: "x", Consumers: []float64{1, 2, 3}, Producers: []float64{4, 5},
		QoVConsumers: []int{1, 2, 3}, QoVProducers: []int{1, 1}}
	c := src.Copy(0)
	c.Consumers[0] = 9
	assert.Equal(t, 1.0, src.Consumers[0], "Copy is deep")
	assert.Equal(t, src.Producers, c.Producers)

	d := src.DeepCopy(2, 2)
	assert.Equal(t, []float64{1, 2, 3, 0, 0, 0}, d.Consumers, "grown to two consumers")
	assert.Equal(t, []int{1, 1, 0, 0}, d.QoVProducers)
	assert.Empty(t, d.Id, "DeepCopy does not copy the id")

	meta := RawSourceMeta{Id: "cpmeta/0", NumberOfMetering: 1,
		CounterPoints: []*CounterPointMeta{{ID: "000", Name: "ZP1", SourceIdx: 3, Dir: CONSUMER_DIRECTION}}}
	mc := meta.Copy()
	mc.CounterPoints[0].Name = "changed"
	assert.Equal(t, "ZP1", meta.CounterPoints[0].Name)
	assert.Equal(t, CONSUMER_DIRECTION, mc.CounterPoints[0].Dir)
}

// RawSourceMeta.Copy does not copy SourceIdx (commented out, sourcemodel.go:71-79): a copy written
// back would move every metering point to index 0 (known-errors #51; no caller today).
func TestRawSourceMetaCopyKeepsSourceIdx(t *testing.T) {
	t.Skip("known-errors #51")
	meta := RawSourceMeta{Id: "cpmeta/0", CounterPoints: []*CounterPointMeta{{Name: "ZP1", SourceIdx: 3}}}
	assert.Equal(t, 3, meta.Copy().CounterPoints[0].SourceIdx)
}
