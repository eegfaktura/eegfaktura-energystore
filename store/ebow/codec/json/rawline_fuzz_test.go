package json

import (
	"testing"
)

// FuzzRowDecoder compares the codec (fast path with fallback) with encoding/json on arbitrary input:
// both must agree on error and row as checkSame defines it (the row also after a type error, not after
// a syntax error into a prefilled row). Run locally: go test ./store/ebow/codec/json -run '^$'
// -fuzz FuzzRowDecoder -fuzztime 2m. Without -fuzz only the seeds run.
func FuzzRowDecoder(f *testing.F) {
	for _, row := range monthRows(4, 5, 2) {
		f.Add(row)
	}
	for _, s := range []string{
		`{"Id":"CP/2026/10/01/00/15/00","Consumers":[0,0.000077,1.5,-0,1e-7],"Producers":[],"QoVConsumers":[1,2,3,1,1],"QoVProducers":null}`,
		`{"Consumers":[1],"consumers":[2]}`, `{"Id":"aä"}`, `{"Consumers":[1,null]}`, `{"QoVConsumers":[1.0]}`,
		` {"Producers" : [ 1e400 ] } `, `{"Consumers":[01]}`, `{}`, `null`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		checkSame(t, data)
	})
}
