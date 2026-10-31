package store

import (
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
)

type IntraDay struct {
	Cache
	Result  map[int]*ReportData
	aligned bool
}

func NewIntraDayFunction() (EnergyConsumer, error) {
	return &IntraDay{Cache: Cache{cacheTsFn: AddDuration(time.Hour)}, Result: make(map[int]*ReportData)}, nil
}

func (id *IntraDay) HandleStart(ctx *EngineContext) error {
	return id.InitCache(ctx)
}

func (id *IntraDay) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	if !id.aligned {
		// The rows begin at the first stored one, which can lie after the period's start; the cache moves
		// one hour per row after a finished hour, so it has to start in the hour of the first row.
		id.aligned = true
		id.cacheTime = CacheTime{ts.Truncate(time.Hour)}
	}
	return id.CacheLine(ctx, ts, line, id.addToResult)
}

func (id *IntraDay) HandleEnd(ctx *EngineContext) error {
	return id.addToResult(ctx, id.cacheTime.Time, &id.cache)
}

func (id *IntraDay) GetResult() []interface{} {
	data := make([]interface{}, 24)
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
	hour := t.Hour() // t is the start of the cached hour

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
		id.Result[hour].Unused += line.Producers[i+1]
		id.Result[hour].QoVProducer = calcQoV(id.Result[hour].QoVProducer, line.QoVProducers[i])
	}
	return nil
}
