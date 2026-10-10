package utils

import (
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDetermineDirection(t *testing.T) {
	type args struct {
		meteringPoint string
	}
	tests := []struct {
		name string
		args args
		want model.MeterDirection
	}{
		{name: "Kelag Producer", args: args{meteringPoint: "AT0070000907310000000000000633966"}, want: model.PRODUCER_DIRECTION},
		{name: "Kelag Consumer", args: args{meteringPoint: "AT007000090730000010190002516667A"}, want: model.CONSUMER_DIRECTION},
		{name: "Netzooe Producer", args: args{meteringPoint: "AT0030000000000000000000030032764"}, want: model.PRODUCER_DIRECTION},
		{name: "Netzooe Consumer", args: args{meteringPoint: "AT0030000000000000000000000032764"}, want: model.CONSUMER_DIRECTION},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, DetermineDirection(tt.args.meteringPoint), "DetermineDirection(%v)", tt.args.meteringPoint)
		})
	}
}

// DetermineDirection slices without a length check (known-errors #46, F31). No production caller
// today; a short id must not panic.
func TestDetermineDirectionShortId(t *testing.T) {
	t.Skip("known-errors #46")
	for _, id := range []string{"", "AT00", "AT003000", "AT0030000000000000"} {
		assert.NotPanics(t, func() {
			assert.Equal(t, model.CONSUMER_DIRECTION, DetermineDirection(id), "id %q", id)
		}, "id %q", id)
	}
}
