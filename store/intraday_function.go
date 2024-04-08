package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"time"
)

type IntraDay struct {
	Cache
	Result map[int]*ReportData
}

func NewIntraDayFunction() (EnergyConsumer, error) {
	return &IntraDay{Cache: Cache{cacheTs: time.Hour}, Result: make(map[int]*ReportData)}, nil
}

func (id *IntraDay) HandleStart(ctx *EngineContext) error {
	return id.InitCache(ctx)
}

func (id *IntraDay) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	return id.CacheLine(ctx, ts, line, id.addToResult)
}

func (id *IntraDay) HandleEnd(ctx *EngineContext) error {
	return id.addToResult(ctx, id.cacheTime, &id.cache)
}

func (id *IntraDay) GetResult() []*ReportData {
	data := make([]*ReportData, 24)
	for i := range data {
		if r, ok := id.Result[i]; ok {
			data[i] = r
		} else {
			data[i] = &ReportData{}
		}
	}
	return data
}

func (id *IntraDay) addToResult(ctx *EngineContext, t time.Time, line *model.RawSourceLine) error {
	hour := t.Add(-1 * id.cacheTs).Hour()

	if _, ok := id.Result[hour]; !ok {
		id.Result[hour] = &ReportData{}
	}

	cLen := len(line.Consumers)
	cLen = cLen - (cLen % 3)
	for i := 0; i < cLen; i += 3 {
		id.Result[hour].Consumed += line.Consumers[i]
		id.Result[hour].Allocated += line.Consumers[i+1]
		id.Result[hour].Distributed += line.Consumers[i+2]
		id.Result[hour].QoVConsumer = calcQoV(id.Result[hour].QoVConsumer, line.QoVConsumers[i])
	}
	pLen := len(line.Producers)
	pLen = pLen - (pLen % 2)
	for i := 0; i < pLen; i += 2 {
		id.Result[hour].Produced += line.Producers[i]
		id.Result[hour].QoVProducer = calcQoV(id.Result[hour].QoVProducer, line.QoVProducers[i])
	}
	return nil
}
