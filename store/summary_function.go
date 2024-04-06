package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"time"
)

type EnergySummary struct {
	Result *ReportData
}

func NewEnergySummary() (EnergyConsumer, error) {
	return &EnergySummary{Result: &ReportData{}}, nil
}

func (id *EnergySummary) HandleStart(ctx *EngineContext) error {
	return nil
}

func (id *EnergySummary) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	return id.addToResult(ctx, ts, line)
}

func (id *EnergySummary) HandleEnd(ctx *EngineContext) error {
	return nil
}

func (id *EnergySummary) GetResult() *ReportData {
	return id.Result
}

func (id *EnergySummary) addToResult(ctx *EngineContext, t time.Time, line *model.RawSourceLine) error {
	for i := 0; i < len(line.Consumers); i += 3 {
		id.Result.Consumed += line.Consumers[i]
		id.Result.Allocated += line.Consumers[i+1]
		id.Result.Distributed += line.Consumers[i+2]
		if len(line.QoVConsumers) > i+3 {
			id.Result.QoVConsumer = calcQoV(id.Result.QoVConsumer, line.QoVConsumers[i])
		} else {
			id.Result.QoVConsumer = calcQoV(id.Result.QoVConsumer, 1)
		}
	}
	for i := 0; i < len(line.Producers); i += 2 {
		id.Result.Produced += line.Producers[i]
		if len(line.QoVProducers) > i+2 {
			id.Result.QoVProducer = calcQoV(id.Result.QoVProducer, line.QoVProducers[i])
		} else {
			id.Result.QoVProducer = calcQoV(id.Result.QoVProducer, 1)
		}
	}
	return nil
}
