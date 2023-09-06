package utils

import (
	"at.ourproject/energystore/model"
	"math"
	"strings"
)

func DetermineDirection(meteringPoint string) model.MeterDirection {
	// 012345678901234567890123456789012
	// AT0030000000000000000000030032764
	// AT0070000907310000000000000633966

	netoperator := meteringPoint[:8]

	println(netoperator)

	if netoperator == "AT003000" {
		switch meteringPoint[25] {
		case '1', '2', '3':
			return model.PRODUCER_DIRECTION
		default:
			return model.CONSUMER_DIRECTION
		}
	} else if netoperator == "AT007000" {
		println(string(meteringPoint[13]))
		switch meteringPoint[13] {
		case '1', '2':
			return model.PRODUCER_DIRECTION
		default:
			return model.CONSUMER_DIRECTION
		}
	}
	return model.CONSUMER_DIRECTION
}

// ExamineDirection exermine the meteringpoint direction accourding to the metercode of energy values.
// It is expected that a GENERATOR have a metercode with profit values CODE_PLUS (1-1:2.9.0 P.01)
func ExamineDirection(energydata []model.MqttEnergyData) model.MeterDirection {
	for _, d := range energydata {
		if d.MeterCode == model.CODE_PLUS {
			return model.PRODUCER_DIRECTION
		}
	}
	return model.CONSUMER_DIRECTION
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

var InsertInt = func(orig []int, index int, value int) []int {
	l := len(orig)
	if index >= l {
		target := make([]int, index+1)
		copy(target, orig)
		orig = target
	}
	orig[index] = value
	return orig
}

func CastQoVStringToInt(qov string) int {
	switch strings.ToUpper(qov) {
	case "L1":
		return 1
	case "L2":
		return 2
	case "L3":
		return 3
	}
	return 0
}

func RoundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func DecodeMeterCode(meterCode model.MeterCodeValue, sourceIdx int) *model.MeterCodeMeta {
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

func CountConsumerProducer(meta []*model.CounterPointMeta) (int, int) {
	consumer := 0
	producer := 0

	for _, m := range meta {
		if m.Dir == model.CONSUMER_DIRECTION {
			consumer = consumer + 1
		} else {
			producer = producer + 1
		}
	}
	return consumer, producer
}

func ConvertLineToMatrix(line *model.RawSourceLine) (*model.Matrix, *model.Matrix) {
	lenConsumers := int(math.Max(float64(len(line.Consumers)-1), 1))
	lenProducers := int(math.Max(float64(len(line.Producers)-1), 1))

	rowConsumers := (lenConsumers + 3 - (lenConsumers % 3)) / 3
	rowProducers := (lenProducers + 2 - (lenProducers % 2)) / 2

	consumerMatrix := model.MakeMatrix(line.Consumers, rowConsumers, 3)
	producerMatrix := model.MakeMatrix(line.Producers, rowProducers, 2)

	return consumerMatrix, producerMatrix

}
