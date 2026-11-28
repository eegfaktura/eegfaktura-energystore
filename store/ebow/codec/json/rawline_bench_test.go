package json

import (
	"testing"

	"at.ourproject/energystore/model"
)

// BenchmarkUnmarshalMonth decodes a month of 500 consumers and 100 producers into one reused row, as the
// report's iterator does. Not run in CI: go test ./store/ebow/codec/json -run '^$' -bench . -benchmem
func BenchmarkUnmarshalMonth(b *testing.B) {
	month := monthRows(96*30, 500, 100)
	c := Codec{}
	for b.Loop() {
		var l model.RawSourceLine
		for _, r := range month {
			if err := c.Unmarshal(r, &l); err != nil {
				b.Fatal(err)
			}
		}
	}
}
