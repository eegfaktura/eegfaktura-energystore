package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
	"time"
)

type TargetMP struct {
	MeteringPoint string `json:"meteringPoint"`
}

type periodRange struct {
	start time.Time
	end   time.Time
}

type EngineContext struct {
	start           time.Time
	end             time.Time
	metaMap         map[string]*model.CounterPointMeta
	meta            []*model.CounterPointMeta
	info            *model.CounterPointMetaInfo
	countCons       int
	countProd       int
	periodsConsumer map[int]periodRange
	periodsProducer map[int]periodRange
	qovLogArray     []model.RawSourceLine
	checkBegin      func(lineDate, mDate time.Time) bool
}

func createEngineContext(db IBowStorage, start, end time.Time) (*EngineContext, error) {
	metaMap, info, err := GetMetaInfo(db)
	if err != nil {
		return nil, err
	}

	metaRangeConsumer := map[int]periodRange{}
	metaRangeProducer := map[int]periodRange{}
	for _, v := range metaMap {
		ts, _ := utils.ParseTime(v.PeriodStart, 0)
		te, _ := utils.ParseTime(v.PeriodEnd, 0)
		if v.Dir == model.CONSUMER_DIRECTION {
			metaRangeConsumer[v.SourceIdx] = periodRange{start: ts, end: te}
		} else {
			metaRangeProducer[v.SourceIdx] = periodRange{start: ts, end: te}
		}
	}

	metaCon := []*model.CounterPointMeta{}
	metaPro := []*model.CounterPointMeta{}
	for _, v := range metaMap {
		if v.Dir == model.CONSUMER_DIRECTION {
			metaCon = append(metaCon, v)
		} else {
			metaPro = append(metaPro, v)
		}
	}
	meta := append(metaCon, metaPro...)
	countCons, countProd := utils.CountConsumerProducer(meta)

	return &EngineContext{
		start: time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local), /*start*/
		end:   time.Date(end.Year(), end.Month(), end.Day(), 23, 45, 0, 0, time.Local),     /*end*/
		//cps:             cps,
		metaMap:         metaMap,
		meta:            meta,
		info:            info,
		countProd:       countProd,
		countCons:       countCons,
		periodsConsumer: metaRangeConsumer,
		periodsProducer: metaRangeProducer,
		checkBegin: func(lineDate, mDate time.Time) bool {
			if lineDate.Before(mDate) {
				return true
			}
			return false
		},
	}, nil
}

type EnergyConsumer interface {
	HandleStart(ctx *EngineContext) error
	HandleLine(ctx *EngineContext, line *model.RawSourceLine) error
	HandleEnd(ctx *EngineContext) error
}

type AddTo func(*EngineContext, time.Time, *model.RawSourceLine) error

type Cache struct {
	cacheTs   time.Duration
	cache     model.RawSourceLine
	cacheTime time.Time
}

func (ca *Cache) CacheLine(ctx *EngineContext, ts time.Time, line *model.RawSourceLine, addTo AddTo) error {
	if ts.Before(ca.cacheTime) {
		return ca.addToCache(line)
	}

	err := addTo(ctx, ca.cacheTime, &ca.cache)
	if err != nil {
		return err
	}

	ca.cache = line.DeepCopy(ctx.countCons, ctx.countProd)
	ca.cacheTime = ca.cacheTime.Add(ca.cacheTs)
	return nil
}

func (ca *Cache) InitCache(ctx *EngineContext) error {
	ca.cacheTime = ctx.start.Add(ca.cacheTs)
	ca.cache = model.RawSourceLine{
		Consumers:    make([]float64, ctx.countCons*3),
		Producers:    make([]float64, ctx.countProd*2),
		QoVConsumers: make([]int, ctx.countCons*3),
		QoVProducers: make([]int, ctx.countProd*2)}

	ca.cache.QoVConsumers = utils.InitSlice(1, ca.cache.QoVConsumers)
	ca.cache.QoVProducers = utils.InitSlice(1, ca.cache.QoVProducers)
	return nil
}

func (ca *Cache) addToCache(line *model.RawSourceLine) error {
	ca.cache.Id = line.Id
	for i := range line.Consumers {
		if len(line.Consumers) > i {
			ca.cache.Consumers[i] += line.Consumers[i]
			if len(line.QoVConsumers) > i {
				ca.cache.QoVConsumers[i] = calcQoV(ca.cache.QoVConsumers[i], line.QoVConsumers[i])
			} else {
				ca.cache.QoVConsumers[i] = calcQoV(ca.cache.QoVConsumers[i], 0)
			}
		} else {
			break
		}
	}
	for i := range line.Producers {
		if len(line.Producers) > i {
			ca.cache.Producers[i] += line.Producers[i]
			if len(line.QoVProducers) > i {
				ca.cache.QoVProducers[i] = calcQoV(ca.cache.QoVProducers[i], line.QoVProducers[i])
			} else {
				ca.cache.QoVProducers[i] = calcQoV(ca.cache.QoVProducers[i], 0)
			}
		} else {
			break
		}
	}
	return nil
}

type Engine struct {
	Consumer EnergyConsumer
}

func (e *Engine) Query(tenant, ecid string, start, end time.Time) error {

	db, err := OpenStorage(tenant, ecid)
	if err != nil {
		return err
	}
	defer db.Close()

	sYear, sMonth, sDay := start.Year(), int(start.Month()), start.Day()
	eYear, eMonth, eDay := end.Year(), int(end.Month()), end.Day()

	iterCP := db.GetLineRange("CP", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterCP.Close()

	var _lineG1 model.RawSourceLine
	g1Ok := iterCP.Next(&_lineG1)
	if !g1Ok {
		return errors.New("no Rows found")
	}
	_, lineStart, err := utils.ConvertRowIdToTimeString("CP", _lineG1.Id, time.Local)
	if err != nil {
		return err
	}
	ctx, err := createEngineContext(db, *lineStart, end)
	if err != nil {
		return err
	}

	err = e.Consumer.HandleStart(ctx)
	if err != nil {
		return err
	}

	var pt *time.Time = nil
	for g1Ok {
		_, t, err := utils.ConvertRowIdToTimeString("CP", _lineG1.Id, time.Local)
		if rowOk := utils.CheckTime(pt, t); !rowOk {
			diff := ((t.Unix() - pt.Unix()) / (60 * 15)) - 1
			if diff > 0 {
				for i := int64(0); i < diff; i += 1 {
					nTime := pt.Add(time.Minute * time.Duration(15*(int(i)+1)))
					newId, _ := utils.ConvertUnixTimeToRowId("CP/", nTime)
					fillLine := model.MakeRawSourceLine(newId,
						ctx.countCons*3, ctx.countProd*2).Copy(ctx.countCons * 3)
					if err = e.Consumer.HandleLine(ctx, &fillLine); err != nil {
						return err
					}
				}
			}
		}
		ct := time.Unix(t.Unix(), 0).UTC()
		pt = &ct

		if err = e.Consumer.HandleLine(ctx, &_lineG1); err != nil {
			return err
		}
		g1Ok = iterCP.Next(&_lineG1)
	}

	return e.Consumer.HandleEnd(ctx)
}
