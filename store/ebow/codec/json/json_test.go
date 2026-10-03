package json

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type line struct {
	Id        string    `bow:"key"`
	Consumers []float64 `bow:"consumers"`
	QoV       []int
}

// Stored values are JSON (AGENTS.md section 8): field names as in the struct, floats unchanged.
func TestRoundTrip(t *testing.T) {
	c := Codec{}
	in := line{Id: "CP/2026/06/01/00/00/00", Consumers: []float64{0.000077, 1.5}, QoV: []int{1, 2}}
	b, err := c.Marshal(in, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"Id":"CP/2026/06/01/00/00/00","Consumers":[0.000077,1.5],"QoV":[1,2]}`, string(b))

	var out line
	require.NoError(t, c.Unmarshal(b, &out))
	assert.Equal(t, in, out)
	assert.Error(t, c.Unmarshal([]byte("{"), &out))
	assert.Equal(t, 1, int(c.Format()), "JSON")
}
