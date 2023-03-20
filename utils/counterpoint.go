package utils

import (
	"at.ourproject/energystore/model"
	"math"
)

func DetermineDirection(meteringPoint string) string {
	// 012345678901234567890123456789012
	// AT0030000000000000000000030032764

	switch meteringPoint[25] {
	case '3':
		return model.PRODUCER_DIRECTION
	default:
		return model.CONSUMER_DIRECTION
	}

}

var Insert = func(orig []float64, index int, value float64) []float64 {
	l := len(orig)
	if index >= l {
		target := make([]float64, index+1)
		copy(target, orig)
		orig = target
	}
	orig[index] = value
	return orig
}

func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func DecodeMeterCode(meterCode string, sourceIdx int) *model.MeterCodeMeta {
	if meterCode == "1-1:2.9.0 G.01" {
		return &model.MeterCodeMeta{Type: "GEN", Code: "G.01", SourceInData: sourceIdx, SourceDelta: 0} // Erzeugung -- Erzeuger
	}
	if meterCode == "1-1:1.9.0 G.01" {
		return &model.MeterCodeMeta{Type: "CON", Code: "G.01", SourceInData: sourceIdx, SourceDelta: 0} // Verbrauch -- Verbraucher
	}
	if meterCode == "1-1:2.9.0 G.02" {
		return &model.MeterCodeMeta{Type: "SHARE", Code: "G.02", SourceInData: sourceIdx, SourceDelta: 1} // Anteil -- Verbraucher
	}
	if meterCode == "1-1:2.9.0 G.03" {
		return &model.MeterCodeMeta{Type: "COVER", Code: "G.03", SourceInData: sourceIdx, SourceDelta: 2} // Eigendeckung -- Verbraucher
	}
	if meterCode == "1-1:2.9.0 P.01" {
		return &model.MeterCodeMeta{Type: "PLUS", Code: "G.02", SourceInData: sourceIdx, SourceDelta: 1} // Überschuss -- Erzeuger
	}
	return nil
}
