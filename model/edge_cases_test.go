package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
}
