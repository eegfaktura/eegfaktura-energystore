package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Aggregate struct {
	ParentFunction
	cacheTs   time.Duration
	cache     model.RawSourceLine
	cacheTime time.Time
}

func NewAggregateFunction(args []string, cps []TargetMP) (IQueryFunction, error) {

	if len(args) != 1 {
		return nil, errors.New("only 1 argument for function 'Aggregate' allowed")
	}

	cacheTs, err := parseArgument(args[0])
	if err != nil {
		return nil, err
	}

	return &Aggregate{ParentFunction: ParentFunction{cps: cps},
		cacheTs: cacheTs}, nil
}

func parseArgument(arg string) (time.Duration, error) {
	d := arg[len(arg)-1]
	switch d {
	case 'h':
		break
	case 'd':
		v, err := strconv.ParseInt(arg[:len(arg)-1], 10, 16)
		if err != nil {
			return time.Second, err
		}
		arg = fmt.Sprintf("%dh", v*24)
	default:
		return time.Second, errors.New(fmt.Sprintf("detect wrong duration. Got '%s'. Expected (h..Hour, d..Day)", string(d)))
	}
	return time.ParseDuration(arg)
}

func (agg *Aggregate) HandleInit(ctx *EngineContext) error {
	agg.Result = make(map[string]*RawDataResult)
	agg.cacheTime = ctx.start.Add(agg.cacheTs)
	agg.cache = model.RawSourceLine{
		Consumers:    make([]float64, ctx.countCons*3),
		Producers:    make([]float64, ctx.countProd*2),
		QoVConsumers: make([]int, ctx.countCons*3),
		QoVProducers: make([]int, ctx.countProd*2)}

	agg.cache.QoVConsumers = utils.InitSlice(1, agg.cache.QoVConsumers)
	agg.cache.QoVProducers = utils.InitSlice(1, agg.cache.QoVProducers)
	return nil
}

func (agg *Aggregate) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {

	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	if ts.Before(agg.cacheTime) {
		return agg.addToCache(line)
	}

	err = agg.addToResult(ctx, agg.cacheTime, &agg.cache)
	if err != nil {
		return err
	}

	agg.cache = line.DeepCopy(ctx.countCons, ctx.countProd)
	agg.cacheTime = agg.cacheTime.Add(agg.cacheTs)
	return nil
}

func (agg *Aggregate) HandleFinish(ctx *EngineContext) error {
	return agg.addToResult(ctx, agg.cacheTime, &agg.cache)
}

func (agg *Aggregate) addToCache(line *model.RawSourceLine) error {
	agg.cache.Id = line.Id
	for i := range line.Consumers {
		if len(line.Consumers) > i {
			agg.cache.Consumers[i] += line.Consumers[i]
			agg.cache.QoVConsumers[i] = calcQoV(agg.cache.QoVConsumers[i], line.QoVConsumers[i])
		} else {
			break
		}
	}
	for i := range line.Producers {
		if len(line.Producers) > i {
			agg.cache.Producers[i] += line.Producers[i]
			agg.cache.QoVProducers[i] = calcQoV(agg.cache.QoVProducers[i], line.QoVProducers[i])
		} else {
			break
		}
	}
	return nil
}

func calcQoV(current, target int) int {
	if current != 1 {
		if target > current && target != 1 {
			return target
		}
		return current
	}
	return target
}
