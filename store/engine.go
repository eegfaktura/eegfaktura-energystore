package store

import (
	"fmt"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
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

func (ec EngineContext) Info() *model.CounterPointMetaInfo {
	return ec.info
}

func (ec EngineContext) ConsumerCount() int {
	return ec.info.ConsumerCount
}

func (ec EngineContext) ProducerCount() int {
	return ec.info.ProducerCount
}

func (ec EngineContext) CheckBegin(lineDate, mDate time.Time) bool {
	if ec.checkBegin == nil {
		return true
	}
	return ec.checkBegin(lineDate, mDate)
}

func (ec EngineContext) MetaMap() map[string]*model.CounterPointMeta {
	return ec.metaMap
}

func createEngineContext(db ebow.IBowStorage, start, end time.Time) (*EngineContext, error) {
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

type EnergyReportConsumer interface {
	EnergyConsumer
	GetResult() []interface{}
}

type AddTo func(*EngineContext, time.Time, *model.RawSourceLine) error

type InitCacheTimeFunc func(ct CacheTime) CacheTime
type AddCacheTimeFunc func(dir int, ct CacheTime) CacheTime
type SubCacheTimeFunc func(ct CacheTime) CacheTime

func AddDuration(d time.Duration) AddCacheTimeFunc {
	duration := d
	return func(dir int, ct CacheTime) CacheTime {
		return CacheTime{ct.Add(time.Duration(dir) * duration)}
	}
}

func SubDuration(d time.Duration) SubCacheTimeFunc {
	duration := d
	return func(ct CacheTime) CacheTime {
		return CacheTime{ct.Add(-1 * duration)}
	}
}

func AddDate(y, m, d int) AddCacheTimeFunc {
	year, month, day := y, m, d
	return func(dir int, ct CacheTime) CacheTime {
		return CacheTime{ct.AddDate(dir*year, dir*month, dir*day)}
	}
}

func SubDate(y, m, d int) SubCacheTimeFunc {
	year, month, day := y, m, d
	return func(ct CacheTime) CacheTime {
		return CacheTime{ct.AddDate(year, month, day)}
	}
}

func InitDefault() InitCacheTimeFunc {
	return func(dt CacheTime) CacheTime {
		return dt
	}
}

func InitMonth() InitCacheTimeFunc {
	return func(dt CacheTime) CacheTime {
		return CacheTime{time.Date(dt.Year(), dt.Month(), 1, 0, 0, 0, 0, time.Local)}
	}
}

func InitWeek() InitCacheTimeFunc {
	return func(dt CacheTime) CacheTime {
		weekday := time.Duration(dt.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		year, month, day := dt.Date()
		currentZeroDay := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		return CacheTime{currentZeroDay.Add(-1 * (weekday - 1) * 24 * time.Hour)}
	}
}

type CacheTime struct {
	time.Time
}

func (ct CacheTime) AddTs(timeFunc AddCacheTimeFunc) CacheTime {
	return timeFunc(1, ct)
}

func (ct CacheTime) Init(timeFunc InitCacheTimeFunc) CacheTime {
	return timeFunc(ct)
}

func (ct CacheTime) SubTs(timeFunc AddCacheTimeFunc) CacheTime {
	return timeFunc(-1, ct)
}

func (ct CacheTime) GetDuration(cacheTs AddCacheTimeFunc) time.Duration {
	return ct.AddTs(cacheTs).Sub(ct.Time)
}

type Cache struct {
	cacheTsFn AddCacheTimeFunc
	initTsFn  InitCacheTimeFunc
	cache     model.RawSourceLine
	cacheTime CacheTime
}

func (ca *Cache) CacheLine(ctx *EngineContext, ts time.Time, line *model.RawSourceLine, addTo AddTo) error {
	if ca.cacheTsFn == nil {
		return addTo(ctx, ts, line)
	}

	if ts.Before(ca.cacheTime.AddTs(ca.cacheTsFn).Time) {
		return ca.addToCache(line)
	}

	err := addTo(ctx, ca.cacheTime.Time, &ca.cache)
	if err != nil {
		return err
	}

	ca.cache = line.DeepCopy(ctx.countCons, ctx.countProd)
	ca.cacheTime = ca.cacheTime.AddTs(ca.cacheTsFn)
	return nil
}

func (ca *Cache) InitCache(ctx *EngineContext) error {
	if ca.cacheTsFn == nil {
		return nil
	}

	if ca.initTsFn != nil {
		ca.cacheTime = CacheTime{ctx.start}.Init(ca.initTsFn)
	} else {
		ca.cacheTime = CacheTime{ctx.start}
	}

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

	db, err := ebow.OpenStorage(tenant, ecid)
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
		return ebow.ErrNoRows
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
		_, t, err := utils.ConvertRowIdToTimeString("CP", _lineG1.Id, time.UTC)
		if err != nil {
			g1Ok = iterCP.Next(&_lineG1)
			continue
		}
		if rowOk := utils.CheckTime(pt, t); !rowOk {
			diff := ((t.Unix() - pt.Unix()) / (60 * 15)) - 1
			if diff > 0 {
				for i := int64(0); i < diff; i += 1 {
					nTime := pt.Add(time.Minute * time.Duration(15*(int(i)+1)))
					newId, _ := utils.ConvertUnixTimeToRowId("CP/", nTime.Local())
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
