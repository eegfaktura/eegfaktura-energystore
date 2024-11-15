package calculation

import (
	"fmt"
	"testing"

	"github.com/eegfaktura/eegfaktura-energystore/model"
	"github.com/eegfaktura/eegfaktura-energystore/utils"
	"github.com/stretchr/testify/assert"
)

func TestAllocDynamic2(t *testing.T) {
	var tests = []struct {
		name   string
		line   *model.RawSourceLine
		cols   int
		rows   int
		result []float64
	}{
		{
			name:   "Test One",
			line:   &model.RawSourceLine{Id: "CP/2022/11/09/00/00/00", Consumers: []float64{0.118}, Producers: []float64{}},
			cols:   1,
			rows:   1,
			result: []float64{0},
		},
		{
			name:   "Test Two",
			line:   &model.RawSourceLine{Id: "CP/2022/11/09/00/00/00", Consumers: []float64{0.1, 0.2, 0.3}, Producers: []float64{0.1}},
			cols:   1,
			rows:   3,
			result: []float64{0.016667, 0.033333, 0.05},
		},
		{
			name:   "Test Three",
			line:   &model.RawSourceLine{Id: "CP/2022/11/09/00/00/00", Consumers: []float64{0.1, 0.2, 0.3}, Producers: []float64{0.1, 0.2}},
			cols:   1,
			rows:   3,
			result: []float64{0.05, 0.1, 0.15},
		},
		{
			name: "Test Real",
			line: &model.RawSourceLine{Id: "CP/2022/11/09/00/00/00",
				Consumers: []float64{0.067, 0.197, 0.0, 0.0, 0.0, 0.0, 0.011, 0.0, 0.0, 0.0, 0.0, 0.0},
				Producers: []float64{0.001}},
			cols:   1,
			rows:   12,
			result: []float64{0.000244, 0.000716, 0.0, 0.0, 0.0, 0.0, 0.00004, 0.0, 0.0, 0.0, 0.0, 0.0},
		},
		{
			name: "Test Real 2",
			line: &model.RawSourceLine{Id: "CP/2022/11/09/00/00/00",
				Consumers: []float64{0.114, 0.059, 0.0, 1.026000, 0.03475, 0.024000, 0.017000, 0.081000, 0.031000, 1.314000, 0.0, 0.0},
				Producers: []float64{1.22}},
			cols:   1,
			rows:   12,
			result: []float64{0.051497, 0.026652, 0.0, 0.463471, 0.015697, 0.010841, 0.007679, 0.03659, 0.014004, 0.593568, 0.0, 0.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AllocDynamic1(tt.line)
			assert.Equal(t, tt.cols, result.CountCols())
			assert.Equal(t, tt.rows, result.CountRows())
			sum := 0.0
			for i := 0; i < tt.rows; i++ {
				value := utils.RoundFloat(result.GetElm(i, 0), 6)
				assert.Equal(t, tt.result[i], value)
				sum += value
			}
			assert.Equal(t, utils.Sum(tt.result), sum)
			fmt.Printf("Result %s: %+v (%v)\n", tt.name, result, sum)
		})
	}
}
