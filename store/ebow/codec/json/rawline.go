package json

import (
	"strconv"

	"at.ourproject/energystore/model"
)

// decodeRawSourceLine decodes an energy row the way encoding/json does, without reflection. It only
// accepts the shape the store writes: one object with the exact keys Id, Consumers, Producers,
// QoVConsumers and QoVProducers, an Id without escapes or non-ASCII bytes, arrays of JSON numbers or null.
// On anything else it returns false without assigning r's fields, so the caller decodes the row with
// encoding/json. Like encoding/json it reuses the backing arrays of r's slices, sets a null array to nil
// and an empty one to an empty non-nil slice, keeps fields the row does not name, and lets the last of
// duplicate keys win.
//
// It writes into those backing arrays before it may give up. When encoding/json then decodes the row,
// the result is the same: it writes the same elements again. On a syntax error encoding/json writes
// nothing, so the contents of r's slices can differ from what plain encoding/json leaves ([9 9 9 9]
// becomes [1 2 9 9]). Every caller discards the row on a decode error.
func decodeRawSourceLine(data []byte, r *model.RawSourceLine) bool {
	d := rowDecoder{b: data}
	id, cons, prod, qovCons, qovProd := r.Id, r.Consumers, r.Producers, r.QoVConsumers, r.QoVProducers
	if !d.consume('{') {
		return false
	}
	if !d.consume('}') {
		for {
			key, ok := d.str()
			if !ok || !d.consume(':') {
				return false
			}
			d.skipWS()
			switch string(key) { // a switch on a converted []byte does not allocate
			case "Id":
				var b []byte
				b, ok = d.str()
				id = string(b)
			case "Consumers":
				cons, ok = floats(&d, cons)
			case "Producers":
				prod, ok = floats(&d, prod)
			case "QoVConsumers":
				qovCons, ok = ints(&d, qovCons)
			case "QoVProducers":
				qovProd, ok = ints(&d, qovProd)
			default: // also a key that encoding/json matches case-insensitively
				ok = false
			}
			if !ok {
				return false
			}
			if d.consume('}') {
				break
			}
			if !d.consume(',') {
				return false
			}
		}
	}
	d.skipWS()
	if d.i != len(d.b) {
		return false
	}
	r.Id, r.Consumers, r.Producers, r.QoVConsumers, r.QoVProducers = id, cons, prod, qovCons, qovProd
	return true
}

type rowDecoder struct {
	b []byte
	i int
}

func (d *rowDecoder) skipWS() {
	for d.i < len(d.b) {
		switch d.b[d.i] {
		case ' ', '\t', '\n', '\r':
			d.i++
		default:
			return
		}
	}
}

// consume skips whitespace and then c, if it comes next.
func (d *rowDecoder) consume(c byte) bool {
	d.skipWS()
	if d.i < len(d.b) && d.b[d.i] == c {
		d.i++
		return true
	}
	return false
}

// str reads a string of printable ASCII without escapes, after optional whitespace, and returns the
// bytes between the quotes.
func (d *rowDecoder) str() ([]byte, bool) {
	if !d.consume('"') {
		return nil, false
	}
	start := d.i
	for ; d.i < len(d.b); d.i++ {
		switch c := d.b[d.i]; {
		case c == '"':
			d.i++
			return d.b[start : d.i-1], true
		case c < 0x20 || c >= 0x80 || c == '\\':
			return nil, false
		}
	}
	return nil, false
}

// null reads the literal null.
func (d *rowDecoder) null() bool {
	if len(d.b)-d.i >= 4 && string(d.b[d.i:d.i+4]) == "null" {
		d.i += 4
		return true
	}
	return false
}

// number reads a number by the JSON grammar and reports whether it has neither fraction nor exponent.
// strconv would also accept forms JSON does not have (+1, .5, 0x10, inf, 1_0), so the grammar is checked
// here first.
func (d *rowDecoder) number() (s []byte, integer bool, ok bool) {
	b, start := d.b, d.i
	i := start
	if i < len(b) && b[i] == '-' {
		i++
	}
	switch {
	case i < len(b) && b[i] == '0':
		i++
	case i < len(b) && b[i] >= '1' && b[i] <= '9':
		i = digits(b, i)
	default:
		return nil, false, false
	}
	integer = true
	if i < len(b) && b[i] == '.' {
		if i+1 >= len(b) || b[i+1] < '0' || b[i+1] > '9' {
			return nil, false, false
		}
		i, integer = digits(b, i+1), false
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || b[i] < '0' || b[i] > '9' {
			return nil, false, false
		}
		i, integer = digits(b, i), false
	}
	d.i = i
	return b[start:i], integer, true
}

func digits(b []byte, i int) int {
	for i < len(b) && b[i] >= '0' && b[i] <= '9' {
		i++
	}
	return i
}

// count returns an upper bound for the number of elements of the array that starts after '['.
func count(b []byte, i int) int {
	n := 1
	for ; i < len(b) && b[i] != ']'; i++ {
		if b[i] == ',' {
			n++
		}
	}
	return n
}

// array reads null (nil, like encoding/json) or an array, calling elem for each element. dst is the
// slice the field held; its backing array is reused as encoding/json does.
func array[T any](d *rowDecoder, dst []T, elem func() (T, bool)) ([]T, bool) {
	if d.null() {
		return nil, true
	}
	if !d.consume('[') {
		return nil, false
	}
	if d.consume(']') {
		return []T{}, true // encoding/json: a fresh empty slice, never nil
	}
	if cap(dst) == 0 {
		dst = make([]T, 0, count(d.b, d.i))
	}
	dst = dst[:0]
	for {
		d.skipWS()
		v, ok := elem()
		if !ok {
			return nil, false
		}
		dst = append(dst, v)
		if d.consume(']') {
			return dst, true
		}
		if !d.consume(',') {
			return nil, false
		}
	}
}

// floats parses like encoding/json: strconv.ParseFloat with 64 bits, out of range is an error.
func floats(d *rowDecoder, dst []float64) ([]float64, bool) {
	return array(d, dst, func() (float64, bool) {
		s, _, ok := d.number()
		if !ok {
			return 0, false
		}
		f, err := strconv.ParseFloat(string(s), 64)
		return f, err == nil
	})
}

// ints parses like encoding/json: an integer literal that fits int; 1.0 or 1e2 is an error there.
func ints(d *rowDecoder, dst []int) ([]int, bool) {
	return array(d, dst, func() (int, bool) {
		s, integer, ok := d.number()
		if !ok || !integer {
			return 0, false
		}
		n, err := strconv.ParseInt(string(s), 10, strconv.IntSize)
		return int(n), err == nil
	})
}
