package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Aggregate struct {
	ParentFunction
	Cache
	//cacheTs   time.Duration
	//cache     model.RawSourceLine
	//cacheTime time.Time
}

func NewAggregateFunction(args []string, cps []TargetMP) (IQueryFunction, error) {

	if len(args) != 1 {
		return nil, errors.New("only 1 argument for function 'Aggregate' allowed")
	}

	cacheTs, err := parseArgument(args[0])
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		ParentFunction: ParentFunction{cps: cps},
		Cache:          Cache{cacheTs: cacheTs}}, nil
}

func parseArgument(arg string) (AddCacheTimeFunc, error) {
	arg = strings.TrimSpace(arg)
	d := arg[len(arg)-1]
	switch d {
	case 'h':
		duration, err := time.ParseDuration(arg)
		if err != nil {
			return nil, err
		}
		return AddDuration(duration), nil
	case 'd':
		v, err := strconv.ParseInt(arg[:len(arg)-1], 10, 16)
		if err != nil {
			return nil, err
		}
		//arg = fmt.Sprintf("%dh", v*24)
		return AddDate(0, 0, int(v)), nil
	case 'w':
		v, err := strconv.ParseInt(arg[:len(arg)-1], 10, 16)
		if err != nil {
			return nil, err
		}
		return AddDate(0, 0, int(v)*7), nil
	case 'm':
		v, err := strconv.ParseInt(arg[:len(arg)-1], 10, 16)
		if err != nil {
			return nil, err
		}
		return AddDate(0, int(v), 0), nil
	default:
		return nil, errors.New(fmt.Sprintf("detect wrong duration. Got '%s'. Expected (h..Hour, d..Day)", string(d)))
	}
}

func (agg *Aggregate) HandleInit(ctx *EngineContext) error {
	agg.Result = make(map[string]*RawDataResult)
	agg.cacheTime = CacheTime{ctx.start}
	agg.cacheTime.AddTs(agg.cacheTs)
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

	return agg.CacheLine(ctx, ts, line, agg.addToResult)
	//if ts.Before(agg.cacheTime.Time) {
	//	return agg.addToCache(line)
	//}
	//
	//err = agg.addToResult(ctx, agg.cacheTime.Time, &agg.cache)
	//if err != nil {
	//	return err
	//}
	//
	//agg.cache = line.DeepCopy(ctx.countCons, ctx.countProd)
	//agg.cacheTime = agg.cacheTime.AddTs(agg.cacheTs)
	//return nil
}

func (agg *Aggregate) HandleFinish(ctx *EngineContext) error {
	return agg.addToResult(ctx, agg.cacheTime.Time, &agg.cache)
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
