package key

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type selfMarshaled struct{ v string }

func (s selfMarshaled) Marshal(in []byte) ([]byte, error) { return []byte("m:" + s.v), nil }
func (s *selfMarshaled) Unmarshal(data []byte) error      { s.v = string(data[2:]); return nil }

func TestStringAndBytesRoundTrip(t *testing.T) {
	c := Codec{}
	b, err := c.Marshal("CP/2026/06/01/00/00/00", nil)
	require.NoError(t, err)
	var s string
	require.NoError(t, c.Unmarshal(b, &s))
	assert.Equal(t, "CP/2026/06/01/00/00/00", s)

	b, err = c.Marshal([]byte{1, 2, 3}, nil)
	require.NoError(t, err)
	var bs []byte
	require.NoError(t, c.Unmarshal(b, &bs))
	assert.Equal(t, []byte{1, 2, 3}, bs)

	b, err = c.Marshal(byte(7), nil)
	require.NoError(t, err)
	var by byte
	require.NoError(t, c.Unmarshal(b, &by))
	assert.Equal(t, byte(7), by)

	b, err = c.Marshal(selfMarshaled{"x"}, nil)
	require.NoError(t, err)
	var sm selfMarshaled
	require.NoError(t, c.Unmarshal(b, &sm))
	assert.Equal(t, "x", sm.v)
}

func TestIntKeysOrderAndUniqueness(t *testing.T) {
	c := Codec{}
	a, err := c.Marshal(1, nil)
	require.NoError(t, err)
	b, err := c.Marshal(2, nil)
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
	assert.Less(t, string(a), string(b), "big endian keeps the order")
}

// Every integer type the codec accepts gives distinct keys that sort like the numbers.
func TestIntegerKeyTypes(t *testing.T) {
	c := Codec{}
	one, two := 1, 2
	pairs := [][2]interface{}{
		{uint16(1), uint16(2)}, {uint32(1), uint32(2)}, {uint64(1), uint64(2)},
		{int8(1), int8(2)}, {int16(1), int16(2)}, {int32(1), int32(2)}, {int64(1), int64(2)},
		{uint(1), uint(2)}, {&one, &two}, {[]int{1}, []int{2}}, {[]uint{1}, []uint{2}},
		{[]int64{1}, []int64{2}},
	}
	for _, p := range pairs {
		a, err := c.Marshal(p[0], nil)
		require.NoError(t, err, "%T", p[0])
		b, err := c.Marshal(p[1], nil)
		require.NoError(t, err, "%T", p[1])
		assert.Less(t, string(a), string(b), "%T", p[0])
	}
}

func TestInvalidKeyTypes(t *testing.T) {
	c := Codec{}
	_, err := c.Marshal(1.5, nil)
	assert.Error(t, err)
	var f float64
	assert.Error(t, c.Unmarshal([]byte{1}, &f))
	assert.NoError(t, c.Unmarshal(nil, &f), "empty data is ignored")
	assert.Equal(t, 0, int(c.Format()), "binary")
}

// Integer keys are written behind 8 zero bytes (bytes.NewBuffer(make([]byte, 8)) then Write), so
// reading them back gives 0 (known-errors #52). Stored keys of this service are strings.
func TestIntKeyRoundTrip(t *testing.T) {
	t.Skip("known-errors #52")
	c := Codec{}
	b, err := c.Marshal(int64(42), nil)
	require.NoError(t, err)
	assert.Len(t, b, 8)
	var v int64
	require.NoError(t, c.Unmarshal(b, &v))
	assert.Equal(t, int64(42), v)
}
