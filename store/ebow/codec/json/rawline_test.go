package json

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sameRow reports whether a and b are equal, floats by their bits, nil and empty slices told apart.
func sameRow(a, b model.RawSourceLine) bool {
	sameFloats := func(x, y []float64) bool {
		if (x == nil) != (y == nil) || len(x) != len(y) {
			return false
		}
		for i := range x {
			if math.Float64bits(x[i]) != math.Float64bits(y[i]) {
				return false
			}
		}
		return true
	}
	sameInts := func(x, y []int) bool {
		if (x == nil) != (y == nil) || len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	return a.Id == b.Id && sameFloats(a.Consumers, b.Consumers) && sameFloats(a.Producers, b.Producers) &&
		sameInts(a.QoVConsumers, b.QoVConsumers) && sameInts(a.QoVProducers, b.QoVProducers)
}

// prefilled is a row as the previous iteration step leaves it in the caller's reused variable.
func prefilled() model.RawSourceLine {
	return model.RawSourceLine{Id: "CP/2026/01/01/00/00/00", Consumers: []float64{9, 9, 9, 9}, Producers: []float64{8},
		QoVConsumers: []int{7, 7, 7, 7}, QoVProducers: []int{6, 6}}
}

// checkSame decodes data with Codec and with encoding/json, into a zero and into a prefilled row, and
// fails unless both agree on error and result.
func checkSame(t testing.TB, data []byte) {
	for _, zero := range []bool{true, false} {
		var want, got model.RawSourceLine // own backing arrays each, so neither decoder writes into the other's
		if !zero {
			want, got = prefilled(), prefilled()
		}
		errWant := json.Unmarshal(data, &want)
		errGot := Codec{}.Unmarshal(data, &got)
		if (errWant == nil) != (errGot == nil) {
			t.Fatalf("%q: encoding/json error %v, codec error %v", data, errWant, errGot)
		}
		if errWant == nil && !sameRow(want, got) {
			t.Fatalf("%q: encoding/json %+v, codec %+v", data, want, got)
		}
	}
}

// randomValue covers what the store holds (kWh with up to 6 decimals, 0) and the edges of float64.
func randomValue(rnd *rand.Rand) float64 {
	switch rnd.Intn(8) {
	case 0:
		return 0
	case 1:
		return math.Copysign(0, -1)
	case 2:
		return math.SmallestNonzeroFloat64 * float64(rnd.Intn(1000))
	case 3:
		return math.MaxFloat64 * rnd.Float64()
	case 4:
		return -rnd.Float64() * 1e-7
	case 5:
		return rnd.NormFloat64() * 1e12
	default:
		return math.Round(rnd.Float64()*rnd.Float64()*2*1e6) / 1e6
	}
}

// Many generated rows: the fast path takes every row the store writes and decodes it bit for bit like
// encoding/json.
func TestRawSourceLineDifferential(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	for n := 0; n < 2000; n++ {
		l := model.RawSourceLine{Id: fmt.Sprintf("CP/2026/%02d/01/00/%02d/00", n%12+1, n%4*15)}
		size := func() int { return []int{0, 1, 3, 17, 300}[rnd.Intn(5)] }
		for i, k := 0, size(); i < k; i++ {
			l.Consumers = append(l.Consumers, randomValue(rnd))
			l.QoVConsumers = append(l.QoVConsumers, []int{0, 1, 2, 3, -1, math.MaxInt, math.MinInt}[rnd.Intn(7)])
		}
		for i, k := 0, size(); i < k; i++ {
			l.Producers = append(l.Producers, randomValue(rnd))
			l.QoVProducers = append(l.QoVProducers, rnd.Intn(4))
		}
		if rnd.Intn(4) == 0 {
			l.Producers, l.QoVProducers = []float64{}, []int{} // stored as []
		}
		data, err := json.Marshal(l)
		require.NoError(t, err)
		var fast model.RawSourceLine
		require.True(t, decodeRawSourceLine(data, &fast), "fast path must take %q", data)
		checkSame(t, data)
	}
	for _, row := range monthRows(96, 50, 10) {
		var fast model.RawSourceLine
		require.True(t, decodeRawSourceLine(row, &fast))
		checkSame(t, row)
	}
}

// Number forms encoding/json writes or reads, and forms strconv accepts but JSON does not.
func TestRawSourceLineNumbers(t *testing.T) {
	for _, n := range []string{"0", "-0", "1e-7", "1E+300", "-2.5e-3", "1e400", "-1e400", "4.9e-325",
		"0.1000000000000000055511151231257827021181583404541015625", strings.Repeat("9", 400),
		"0." + strings.Repeat("0", 400) + "1", "01", "+1", ".5", "1.", "1.e2", "0x10", "inf", "NaN", "1_0", "-", "1e"} {
		checkSame(t, []byte(`{"Consumers":[`+n+`],"QoVConsumers":[`+n+`]}`))
		checkSame(t, []byte(`{"Producers":[1,`+n+`,2]}`))
	}
	for _, n := range []string{strconv.Itoa(math.MaxInt), strconv.Itoa(math.MinInt), "9223372036854775808", "1.0", "1e2"} {
		checkSame(t, []byte(`{"QoVProducers":[`+n+`]}`))
	}
}

// Object shapes: whitespace, null, empty, missing and duplicate fields, key case, escapes, garbage.
func TestRawSourceLineShapes(t *testing.T) {
	for _, s := range []string{
		`{}`, ` { } `, "{\n\t\"Id\" : \"a\" ,\r\n\"Consumers\" : [ 1 , 2 ] }\n",
		`{"Id":"a","Consumers":null,"Producers":[],"QoVConsumers":null,"QoVProducers":[]}`,
		`{"Consumers":[1,2,3],"Consumers":[4]}`, `{"Consumers":[1],"consumers":[2]}`, `{"consumers":[2]}`,
		`{"CONSUMERS":[2]}`, `{"Id":"x","Other":1}`, `{"Id":"aäb"}`, `{"Id":"a\"b"}`, `{"Id":"ä"}`,
		"{\"Id\":\"\xff\"}", "{\"Id\":\"a\tb\"}", `{"Id":null}`, `{"Id":5}`, `{"Consumers":"1"}`,
		`{"Consumers":[1,null,2]}`, `{"Consumers":[null]}`, `{"Consumers":nul}`, `{"Consumers":nullx}`,
		`{"Consumers":[1,]}`, `{"Consumers":[,1]}`, `{"Consumers":[1 2]}`, `{"Consumers":[[1]]}`,
		`{"Consumers":[1]`, `{"Consumers":[1]}x`, `{"Consumers":[1]}{}`, `{"Id":"a",}`, `{"Id":"a""Consumers":[]}`,
		`{"Id" "a"}`, `[]`, `null`, ``, ` `, `"x"`, `{"Consumers":[1],"Id":"a","Producers":[2]}`,
	} {
		checkSame(t, []byte(s))
	}
}

// Like encoding/json the fast path writes into the backing array of the reused row and keeps fields
// the row does not name.
func TestRawSourceLineReusesSlices(t *testing.T) {
	l := prefilled()
	backing := l.Consumers
	require.NoError(t, Codec{}.Unmarshal([]byte(`{"Consumers":[1,2]}`), &l))
	assert.Equal(t, []float64{1, 2}, l.Consumers)
	assert.Equal(t, []float64{1, 2, 9, 9}, backing, "same backing array as encoding/json")
	assert.Equal(t, prefilled().Producers, l.Producers, "field not in the row is kept")
}
