package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"fmt"
	"slices"
	"strings"
	"time"
)

type SeriesNameFunc func(time.Time) string

func monthDayNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		return fmt.Sprintf("D:%.2d:%.2d", int(start.Month()), start.Day())
	}
}

func weekYearNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		year, week := start.ISOWeek()
		return fmt.Sprintf("W:%.4d:%.2d", year, week)
	}
}

func monthYearNameFunc() SeriesNameFunc {
	return func(start time.Time) string {
		return fmt.Sprintf("M:%.4d:%.2d", start.Year(), start.Month())
	}
}

func determineSeriesNameFunction(start, end time.Time) SeriesNameFunc {
	if start.AddDate(0, 1, 0).Add(time.Minute).After(end) {
		return monthDayNameFunc()
	} else if start.AddDate(0, 3, 0).Add(time.Minute).After(end) {
		return weekYearNameFunc()
	} else { //start.AddDate(0, 6, 0).Add(time.Minute).After(end) {
		return monthYearNameFunc()
	}
}

type LoadCurve struct {
	Cache
	Result   map[string]*ReportData
	NameFunc func(time time.Time) string
}

func NewLoadCurveFunction(timeShift AddCacheTimeFunc, seriesName SeriesNameFunc) (EnergyConsumer, error) {
	return &LoadCurve{Cache: Cache{cacheTs: timeShift}, Result: make(map[string]*ReportData), NameFunc: seriesName}, nil
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
	start := CacheTime{t}.SubTs(lc.cacheTs).Time
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
		lc.Result[sn].QoVConsumer = calcQoV(lc.Result[sn].QoVConsumer, line.QoVConsumers[i])
	}
	pLen := len(line.Producers)
	pLen = pLen - (pLen % 2)
	for i := 0; i < pLen; i += 2 {
		lc.Result[sn].Produced += line.Producers[i]
		lc.Result[sn].QoVProducer = calcQoV(lc.Result[sn].QoVProducer, line.QoVProducers[i])
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

//func (lc *LoadCurve) HandleStart(ctx *EngineContext) error {
//	return lc.InitCache(ctx)
//}
//
//func (lc *LoadCurve) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
//	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
//	if err != nil {
//		return err
//	}
//
//	return lc.CacheLine(ctx, ts, line, lc.addToResult)
//}
//
//func (lc *LoadCurve) HandleEnd(ctx *EngineContext) error {
//	return lc.addToResult(ctx, lc.cacheTime.Time, &lc.cache)
//}
//
//func (lc *LoadCurve) GetResult() []*ReportData {
//	data := make([]*ReportData, 24)
//	for i := range data {
//		if r, ok := lc.Result[i]; ok {
//			data[i] = r
//		} else {
//			data[i] = &ReportData{}
//		}
//	}
//	return data
//}
//
//func (lc *LoadCurve) addToResult(ctx *EngineContext, t time.Time, line *model.RawSourceLine) error {
//	hour := t.Add(-1 * CacheTime{time.Now()}.GetDuration(lc.cacheTs)).Hour()
//
//	if _, ok := lc.Result[hour]; !ok {
//		lc.Result[hour] = &ReportData{}
//	}
//
//	cLen := len(line.Consumers)
//	cLen = cLen - (cLen % 3)
//	for i := 0; i < cLen; i += 3 {
//		lc.Result[hour].Consumed += line.Consumers[i]
//		lc.Result[hour].Allocated += line.Consumers[i+1]
//		lc.Result[hour].Distributed += line.Consumers[i+2]
//		lc.Result[hour].QoVConsumer = calcQoV(lc.Result[hour].QoVConsumer, line.QoVConsumers[i])
//	}
//	pLen := len(line.Producers)
//	pLen = pLen - (pLen % 2)
//	for i := 0; i < pLen; i += 2 {
//		lc.Result[hour].Produced += line.Producers[i]
//		lc.Result[hour].QoVProducer = calcQoV(lc.Result[hour].QoVProducer, line.QoVProducers[i])
//	}
//	return nil
//}
