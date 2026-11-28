package json

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"at.ourproject/energystore/model"
)

// monthRows returns the stored rows of one month of a community: n quarter hours with cons consumers (3
// values each) and prod producers (2 values each), values in kWh with 6 decimals, a fifth of them 0.
func monthRows(n, cons, prod int) [][]byte {
	rnd := rand.New(rand.NewSource(1))
	val := func() float64 {
		if rnd.Intn(5) == 0 {
			return 0
		}
		return math.Round(rnd.Float64()*rnd.Float64()*2*1e6) / 1e6
	}
	out := make([][]byte, n)
	for k := range out {
		l := model.RawSourceLine{Id: fmt.Sprintf("CP/2026/10/%02d/%02d/%02d/00", k/96+1, (k%96)/4, (k%4)*15)}
		for i := 0; i < cons*3; i++ {
			l.Consumers = append(l.Consumers, val())
			l.QoVConsumers = append(l.QoVConsumers, 1+rnd.Intn(3))
		}
		for i := 0; i < prod*2; i++ {
			l.Producers = append(l.Producers, val())
			l.QoVProducers = append(l.QoVProducers, 1)
		}
		out[k], _ = json.Marshal(l)
	}
	return out
}

// BenchmarkUnmarshalMonth decodes a month of 500 consumers and 100 producers into one reused row, as the
// report's iterator does. Not run in CI: go test ./store/ebow/codec/json -run '^$' -bench . -benchmem
func BenchmarkUnmarshalMonth(b *testing.B) {
	month := monthRows(96*30, 500, 100)
	c := Codec{}
	b.ResetTimer()
	for b.Loop() {
		var l model.RawSourceLine
		for _, r := range month {
			if err := c.Unmarshal(r, &l); err != nil {
				b.Fatal(err)
			}
		}
	}
}
