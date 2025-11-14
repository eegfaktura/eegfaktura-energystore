package store

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
)

type SeriesNameFunc func(time.Time) string

func monthDayNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		return fmt.Sprintf("D:%.2d:%.2d:%.2d", int(start.Month()), start.Day(), (start.Weekday()+6)%7)
	}
}

func weekYearNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		year, week := start.ISOWeek()
		return fmt.Sprintf("W:%.4d:%.2d:%.2d", year, week, week)
	}
}

func monthYearNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		return fmt.Sprintf("M:%.4d:%.2d:%.2d", start.Year(), start.Month(), start.Month())
	}
}

func dayRawNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		return fmt.Sprintf("R:%d:%.2d:%.2d", start.Unix(), (start.Hour()*4)+(start.Minute()/15), start.Day())
	}
}

var nameFunctionRepo = map[string]SeriesNameFunc{
	"DAYRAW":    dayRawNameFunc(),
	"MONTHYEAR": monthYearNameFunc(),
	"WEEKYEAR":  weekYearNameFunc(),
	"MONTHDAY":  monthDayNameFunc(),
}

type LoadCurve struct {
	Cache
	Result   map[string]*ReportData
	NameFunc func(time time.Time) string
}

func NewLoadCurveFunction(timeShift AddCacheTimeFunc, seriesName SeriesNameFunc, initTime InitCacheTimeFunc) (EnergyConsumer, error) {
	return &LoadCurve{Cache: Cache{cacheTsFn: timeShift, initTsFn: initTime}, Result: make(map[string]*ReportData), NameFunc: seriesName}, nil
}

func (lc *LoadCurve) HandleStart(ctx *EngineContext) error {
	return lc.InitCache(ctx)
}

func (lc *LoadCurve) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	//fmt.Printf("Count Consumers: %d, Count Producers: %d\n", int(len(line.Consumers)/3)+1, int(len(line.Producers)/3))
	return lc.CacheLine(ctx, ts, line, lc.addToResult)
}

func (lc *LoadCurve) HandleEnd(ctx *EngineContext) error {
	if len(lc.cache.Id) == 0 {
		return nil
	}
	return lc.addToResult(ctx, lc.cacheTime.Time, &lc.cache)
}

func (lc *LoadCurve) GetResult() []interface{} {
	data := []interface{}{}
	for k, v := range lc.Result {
		data = append(data, &ReportNamedData{v, k})
	}
	slices.SortFunc(data, func(a, b interface{}) int {
		return strings.Compare(a.(*ReportNamedData).Name, b.(*ReportNamedData).Name)
	})
	return data
}

func (lc *LoadCurve) addToResult(ctx *EngineContext, t time.Time, line *model.RawSourceLine) error {
	start := t
	//if lc.cacheTsFn != nil {
	//	start = CacheTime{t}.SubTs(lc.cacheTsFn).Time
	//}
	sn := lc.NameFunc(start)

	if _, ok := lc.Result[sn]; !ok {
		lc.Result[sn] = &ReportData{}
	}

	cLen := len(line.Consumers)
	cLen = cLen - (cLen % 3)
	for i := 0; i < cLen; i += 3 {
		lc.Result[sn].Consumed += line.Consumers[i]
		lc.Result[sn].Allocated += line.Consumers[i+1]
		lc.Result[sn].Distributed += line.Consumers[i+2]
		if len(line.QoVConsumers) > i {
			lc.Result[sn].QoVConsumer = calcQoV(lc.Result[sn].QoVConsumer, line.QoVConsumers[i])
		} else {
			lc.Result[sn].QoVConsumer = 0
		}
	}
	pLen := len(line.Producers)
	pLen = pLen - (pLen % 2)
	for i := 0; i < pLen; i += 2 {
		lc.Result[sn].Produced += line.Producers[i]
		lc.Result[sn].Unused += line.Producers[i+1]
		if len(line.QoVProducers) > i {
			lc.Result[sn].QoVProducer = calcQoV(lc.Result[sn].QoVProducer, line.QoVProducers[i])
		} else {
			lc.Result[sn].QoVProducer = 0
		}

	}

	cCon, cPro := CountMembersPeriod(ctx, start, t)
	lc.Result[sn].CntConsumer = cCon
	lc.Result[sn].CntProducer = cPro

	return nil
}

func CountMembersPeriod(ctx *EngineContext, start, end time.Time) (int, int) {
	cCnt, pCnt := 0, 0
	for _, c := range ctx.periodsConsumer {
		if c.start.Before(end) && c.end.After(start) {
			cCnt++
		}
	}
	for _, p := range ctx.periodsProducer {
		if p.start.Before(end) && p.end.After(start) {
			pCnt++
		}
	}
	return cCnt, pCnt
}
