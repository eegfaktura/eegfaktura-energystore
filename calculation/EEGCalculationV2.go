package calculation

import (
	"fmt"
	"math"
	"sort"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"github.com/golang/glog"
)

type AllocationHandlerV2 func(*model.Matrix, *model.Matrix) (*model.Matrix, *model.Matrix, *model.Matrix)

func AllocDynamicV2(consumerMatrix, producerMatrix *model.Matrix) (*model.Matrix, *model.Matrix, *model.Matrix) {

	// set identity matrix to filter allocated value
	consumerUnitMatix := model.MakeMatrix(make([]float64, 3), 3, 1)
	consumerUnitMatix.SetElm(2, 0, 1)
	allocResult := model.Multiply(consumerMatrix, consumerUnitMatix)

	// set identity matrix to filter shared value
	consumerUnitMatix.SetElm(2, 0, 0)
	consumerUnitMatix.SetElm(1, 0, 1)
	shareResult := model.Multiply(consumerMatrix, consumerUnitMatix)

	// set identity matrix to filter total produced value
	producerUnitMatix := model.MakeMatrix(make([]float64, 2), 2, 1)
	producerUnitMatix.SetElm(1, 0, 1)
	prodResult := model.Multiply(producerMatrix, producerUnitMatix)

	return allocResult, shareResult, prodResult
}

type ValueIterator interface {
	Next(result interface{}) bool
}

type calcResults struct {
	rAlloc *model.Matrix
	rCons  *model.Matrix
	rProd  *model.Matrix
	rDist  *model.Matrix
	rShar  *model.Matrix
	pSum   float64
}

func newCalcResult(metaInfo *model.CounterPointMetaInfo) *calcResults {
	return &calcResults{
		rCons:  model.NewMatrix(metaInfo.ConsumerCount, 1),
		rAlloc: model.NewMatrix(metaInfo.ConsumerCount, 1),
		rProd:  model.NewMatrix(metaInfo.ProducerCount, 1),
		rDist:  model.NewMatrix(metaInfo.ProducerCount, 1),
		rShar:  model.NewMatrix(metaInfo.ConsumerCount, 1),
		pSum:   0,
	}
}

func appendResults(line *model.RawSourceLine, allocFunc AllocationHandlerV2, results *calcResults) error {

	consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(line)
	m, s, p := allocFunc(consumerMatrix, producerMatrix)

	consumerUnitMatix := model.MakeMatrix(make([]float64, 3), 3, 1)
	consumerUnitMatix.SetElm(0, 0, 1)

	producerUnitMatix := model.MakeMatrix(make([]float64, 2), 2, 1)
	producerUnitMatix.SetElm(0, 0, 1)

	consumed := model.Multiply(consumerMatrix, consumerUnitMatix)
	produced := model.Multiply(producerMatrix, producerUnitMatix)

	if results.rCons == nil {
		results.rCons = model.NewCopiedMatrixFromElements(line.Consumers, len(line.Consumers), 1)
	} else {
		//results.rCons.Add(model.MakeMatrix(line.Consumers, len(line.Consumers), 1))
		results.rCons.Add(consumed)
	}

	if results.rProd == nil {
		results.rProd = model.NewCopiedMatrixFromElements(line.Producers, len(line.Producers), 1)
	} else {
		//results.rProd.Add(model.MakeMatrix(line.Producers, len(line.Producers), 1))
		results.rProd.Add(produced)
	}

	if results.rAlloc == nil {
		results.rAlloc = model.NewCopiedMatrixFromElements(m.Elements, m.CountRows(), m.CountCols())
	} else {
		results.rAlloc.Add(m)
	}

	if results.rDist == nil {
		results.rDist = model.NewCopiedMatrixFromElements(p.Elements, p.CountRows(), p.CountCols())
	} else {
		results.rDist.Add(p)
	}

	if results.rShar == nil {
		results.rShar = model.NewCopiedMatrixFromElements(s.Elements, s.CountRows(), s.CountCols())
	} else {
		results.rShar.Add(s)
	}
	results.pSum += utils.Sum(produced.Elements)

	return nil

}

type reportValues struct {
	meters           map[string][]*model.MeterReport
	totalProduction  *float64
	totalConsumption *float64
}

var EnsureIntermediateSlice = func(orig []model.Recort, size int) []model.Recort {
	l := len(orig)
	if size > l {
		target := make([]model.Recort, size)
		copy(target, orig)
		orig = target
	}
	return orig
}

var EnsureIntermediatValueSlice = func(orig []float64, size int) []float64 {
	l := len(orig)
	if size > l {
		target := make([]float64, size)
		copy(target, orig)
		orig = target
	}
	return orig
}

var ConvertToMeterMap = func(report *model.ReportResponse) reportValues {
	meters := map[string][]*model.MeterReport{}
	for _, mm := range report.ParticipantReports {
		for _, m := range mm.Meters {
			r, ok := meters[m.MeterId]
			if !ok {
				r = []*model.MeterReport{}
				meters[m.MeterId] = r
			}
			if m.Report == nil {
				m.Report = &model.Report{
					Id:      "",
					Summary: model.Recort{},
					Intermediate: model.IntermediateRecord{
						Id:          "",
						Consumption: []float64{},
						Utilization: []float64{},
						Allocation:  []float64{},
						Production:  []float64{},
					},
				}
			}
			meters[m.MeterId] = append(r, m)
		}
	}
	return reportValues{meters: meters, totalConsumption: &report.TotalConsumption, totalProduction: &report.TotalProduction}
}

func CalculateParticipantPeriod(db *ebow.BowStorage, allocFunc AllocationHandlerV2, year, segment int, meterings map[string][]model.MeterReport) error {
	rowPrefix := "CP"
	month := 1
	_, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return err
	}
	iter := db.GetLinePrefix(fmt.Sprintf("%s/%d/%.2d/", rowPrefix, year, month))
	defer iter.Close()

	//results := newCalcResult(metaInfo)
	intermediate := newCalcResult(metaInfo)

	dd := 1
	var _line model.RawSourceLine
	for iter.Next(&_line) {
		line := _line.Copy(0)

		ct, err := utils.ConvertRowIdToTime(rowPrefix, line.Id)
		if err != nil {
			glog.V(3).Infof("Error converting row id to timestamp: %s", err.Error())
			continue
		}

		cdd := ct.Day()
		if cdd > dd {

			dd = cdd
		}

		if err := appendResults(&line, allocFunc, intermediate); err != nil {
			return err
		}
	}
	return nil
}

func appendValues(line *model.RawSourceLine, lineTime time.Time, metaInfo map[string]*model.CounterPointMeta, meterpoints map[string][]*model.MeterReport, allocFunc AllocationHandlerV2, results *calcResults) error {
	var err error

	for k, v := range metaInfo {
		participants := meterpoints[k]
		switch v.Dir {
		case model.CONSUMER_DIRECTION:
			values := line.Consumers[v.SourceIdx : v.SourceIdx+3]
			appendToMeterSummary(participants, values, v.Dir, lineTime)
		case model.PRODUCER_DIRECTION:
			values := line.Consumers[v.SourceIdx : v.SourceIdx+2]
			appendToMeterSummary(participants, values, v.Dir, lineTime)
		}
	}

	return err
}

func appendToMeterSummary(participants []*model.MeterReport, values []float64, dir model.MeterDirection, lineTime time.Time) {
	for _, p := range participants {
		from := time.UnixMilli(p.From)
		until := time.UnixMilli(p.Until)
		if from.After(lineTime) && until.Before(lineTime) {
			switch dir {
			case model.CONSUMER_DIRECTION:
				p.Report.Summary.Consumption += values[0]
				p.Report.Summary.Allocation += values[1]
				p.Report.Summary.Utilization += values[2]
			case model.PRODUCER_DIRECTION:
				p.Report.Summary.Production += values[0]
				p.Report.Summary.Allocation += values[1]
			}
		}
	}
}

func calcDailyScope(iter ValueIterator, allocFunc AllocationHandlerV2, metaInfo *model.CounterPointMetaInfo,
	startDay time.Time, rowPrefix string, windows []timeWindowRange,
	dayCb func(day time.Time, results *calcResults, windowResults map[string]*calcResults) error) error {
	daySummary := newCalcResult(metaInfo)
	dayWindowSums := map[string]*calcResults{}
	day := startDay
	var _line model.RawSourceLine
	for iter.Next(&_line) {
		line := _line.Copy(0)
		currentTimeStamp, err := utils.ConvertRowIdToTime(rowPrefix, line.Id)
		if err != nil {
			continue
		}

		if currentTimeStamp.YearDay() != day.YearDay() {
			if err := dayCb(day, daySummary, dayWindowSums); err != nil {
				glog.Errorf("Error Daily Summary: %s", err.Error())
			}
			daySummary = newCalcResult(metaInfo)
			dayWindowSums = map[string]*calcResults{}
			day = currentTimeStamp
		}

		if err := appendResults(&line, allocFunc, daySummary); err != nil {
			return err
		}

		// ZVT: fold the quarter-hour into every matching time-of-use window.
		// Membership compares against the wall-clock HH:MM encoded in the
		// row id (local Vienna time, see timeWindows.go).
		if len(windows) > 0 {
			minuteOfDay := currentTimeStamp.Hour()*60 + currentTimeStamp.Minute()
			for _, w := range windows {
				if w.contains(minuteOfDay) {
					ws, ok := dayWindowSums[w.rangeKey]
					if !ok {
						ws = newCalcResult(metaInfo)
						dayWindowSums[w.rangeKey] = ws
					}
					if err := appendResults(&line, allocFunc, ws); err != nil {
						return err
					}
				}
			}
		}
	}
	return dayCb(day, daySummary, dayWindowSums)
}

func CalculateMonthlyPeriodV2(db *ebow.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year, segment int) error {
	rowPrefix := "CP"
	cpMeta, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return err
	}

	reportValues := ConvertToMeterMap(report)

	iter := db.GetLinePrefix(fmt.Sprintf("%s/%d/%.2d/", rowPrefix, year, segment))
	defer iter.Close()

	startDate := time.Date(year, time.Month(segment), 1, 0, 0, 0, 0, time.Local)
	return calcParticipantReport(iter, &reportValues, allocFunc, cpMeta, metaInfo, rowPrefix, startDate, func(currentDate time.Time) int {
		return currentDate.Day()
	})
}

func CalculateBiAnnualPeriodV2(db *ebow.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year, segment int) error {
	rowPrefix := "CP"
	cpMeta, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return err
	}

	reportValues := ConvertToMeterMap(report)

	iter := db.GetLineRange(rowPrefix,
		fmt.Sprintf("%.4d/%.2d/", year, ((segment-1)*6)+1),
		fmt.Sprintf("%.4d/%.2d/", year, segment*6),
	)
	defer iter.Close()

	startDate := time.Date(year, time.Month(((segment-1)*6)+1), 1, 0, 0, 0, 0, time.Local)
	_, startWeek := startDate.ISOWeek()

	err = calcParticipantReport(iter, &reportValues, allocFunc, cpMeta, metaInfo, rowPrefix, startDate, func(currentDate time.Time) int {
		_, week := currentDate.ISOWeek()
		a := week - startWeek
		b := 53
		return int(math.Max(float64((a%b+b)%b), 1))
	})
	return err
}

func CalculateQuarterlyPeriodV2(db *ebow.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year, segment int) error {
	rowPrefix := "CP"
	cpMeta, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return err
	}

	reportValues := ConvertToMeterMap(report)

	iter := db.GetLineRange(rowPrefix,
		fmt.Sprintf("%.4d/%.2d/", year, ((segment-1)*3)+1),
		fmt.Sprintf("%.4d/%.2d/", year, segment*3),
	)
	defer iter.Close()

	startDate := time.Date(year, time.Month(((segment-1)*3)+1), 1, 0, 0, 0, 0, time.Local)
	_, startWeek := startDate.ISOWeek()

	err = calcParticipantReport(iter, &reportValues, allocFunc, cpMeta, metaInfo, rowPrefix, startDate, func(currentDate time.Time) int {
		_, week := currentDate.ISOWeek()
		a := week - startWeek
		b := 52
		return int(math.Max(float64((a%b+b)%b), 1))
	})
	return err
}

func CalculateAnnualPeriodV2(db *ebow.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year int) error {
	rowPrefix := "CP"
	cpMeta, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return err
	}

	reportValues := ConvertToMeterMap(report)

	iter := db.GetLinePrefix(fmt.Sprintf("%s/%d/", rowPrefix, year))
	defer iter.Close()

	startDate := time.Date(year, time.Month(1), 1, 0, 0, 0, 0, time.Local)
	startMonth := startDate.Month()

	err = calcParticipantReport(iter, &reportValues, allocFunc, cpMeta, metaInfo, rowPrefix, startDate, func(currentDate time.Time) int {
		month := currentDate.Month()
		a := int(month - startMonth)
		b := 12
		return int(math.Max(float64((a%b+b)%b)+1, 1))
	})
	return err
}

func calcParticipantReport(iter ebow.IRange,
	reportValues *reportValues,
	allocFunc AllocationHandlerV2,
	cpMeta map[string]*model.CounterPointMeta,
	metaInfo *model.CounterPointMetaInfo, rowPrefix string, startDate time.Time, switchIntermediate func(time.Time) int) error {
	windows := distinctWindowRanges(reportValues.meters)
	bucketSums := map[*model.MeterReport]map[string]float64{}
	err := calcDailyScope(iter, allocFunc, metaInfo, startDate, rowPrefix, windows,
		func(currentDate time.Time, summary *calcResults, windowResults map[string]*calcResults) error {
			appendBucketsToParticipantMeter(windowResults, reportValues, cpMeta, currentDate, bucketSums)
			err := appendEnergyToParticipantMeter(summary, reportValues, cpMeta, currentDate,
				func(participantReport *model.MeterReport, values []float64, dir model.MeterDirection) {
					switch dir {
					case model.CONSUMER_DIRECTION:
						idx := switchIntermediate(currentDate)
						participantReport.Report.Intermediate.Id = "IRP/2023/01"
						participantReport.Report.Intermediate.Consumption = EnsureIntermediatValueSlice(participantReport.Report.Intermediate.Consumption, idx)
						participantReport.Report.Intermediate.Allocation = EnsureIntermediatValueSlice(participantReport.Report.Intermediate.Allocation, idx)
						participantReport.Report.Intermediate.Utilization = EnsureIntermediatValueSlice(participantReport.Report.Intermediate.Utilization, idx)

						participantReport.Report.Intermediate.Consumption[idx-1] = utils.RoundToFixed(participantReport.Report.Intermediate.Consumption[idx-1]+values[0], 6)
						participantReport.Report.Intermediate.Allocation[idx-1] = utils.RoundToFixed(participantReport.Report.Intermediate.Allocation[idx-1]+values[1], 6)
						participantReport.Report.Intermediate.Utilization[idx-1] = utils.RoundToFixed(participantReport.Report.Intermediate.Utilization[idx-1]+values[2], 6)

						//if len(participantReport.Report.Intermediate) < idx {
						//	participantReport.Report.Intermediate = EnsureIntermediateSlice(participantReport.Report.Intermediate, idx)
						//}
						//ir := &participantReport.Report.Intermediate[idx-1]
						//ir.Consumed += values[0]
						//ir.Allocation += values[1]
						//ir.Utilization += values[2]
						//ir.RoundToFixed(6)
					case model.PRODUCER_DIRECTION:
						idx := switchIntermediate(currentDate)
						participantReport.Report.Intermediate.Id = "IRP/2023/01"
						participantReport.Report.Intermediate.Production = EnsureIntermediatValueSlice(participantReport.Report.Intermediate.Production, idx)
						participantReport.Report.Intermediate.Allocation = EnsureIntermediatValueSlice(participantReport.Report.Intermediate.Allocation, idx)

						participantReport.Report.Intermediate.Production[idx-1] += values[0]
						participantReport.Report.Intermediate.Allocation[idx-1] += values[1]

						//if len(participantReport.Report.Intermediate) < idx {
						//	participantReport.Report.Intermediate = EnsureIntermediateSlice(participantReport.Report.Intermediate, idx)
						//}
						//ir := &participantReport.Report.Intermediate[idx-1]
						//ir.Produced += values[0]
						//ir.Allocation += values[1]
					}
				},
			)
			return err
		},
	)
	for _, s := range reportValues.meters {
		for _, r := range s {
			r.Report.RoundToFixed(6)
			buildBuckets(r, cpMeta, bucketSums)
		}
	}
	return err
}

// appendBucketsToParticipantMeter adds the per-window daily sums to the
// running bucket totals of every requested meter with time windows. It uses
// the same day-level from/until gating as appendEnergyToParticipantMeter so
// the bucket partition stays consistent with the summary.
func appendBucketsToParticipantMeter(
	windowSums map[string]*calcResults,
	reportValues *reportValues,
	cpMeta map[string]*model.CounterPointMeta,
	lineTime time.Time,
	bucketSums map[*model.MeterReport]map[string]float64) {

	if len(windowSums) == 0 {
		return
	}
	for meterId, meterReports := range reportValues.meters {
		meta, ok := cpMeta[meterId]
		if !ok {
			continue
		}
		for _, p := range meterReports {
			if len(p.TimeWindows) == 0 {
				continue
			}
			from := utils.TruncateToDay(time.UnixMilli(p.From))
			until := utils.TruncateToDay(time.UnixMilli(p.Until))
			if !(from.Unix() <= lineTime.Unix() && lineTime.Unix() <= until.Unix()) {
				continue
			}
			sums, ok := bucketSums[p]
			if !ok {
				sums = map[string]float64{}
				bucketSums[p] = sums
			}
			for _, tw := range p.TimeWindows {
				ws, ok := windowSums[tw.From+"-"+tw.To]
				if !ok {
					continue
				}
				switch meta.Dir {
				case model.CONSUMER_DIRECTION:
					// billing quantity of a consumer is the utilization
					sums[tw.Key] += ws.rAlloc.RoundToFixed(6).GetElm(meta.SourceIdx, 0)
				case model.PRODUCER_DIRECTION:
					// billing quantity of a producer is production - allocation
					sums[tw.Key] += ws.rProd.GetElm(meta.SourceIdx, 0) - ws.rDist.GetElm(meta.SourceIdx, 0)
				}
			}
		}
	}
}

// buildBuckets writes the final buckets of one meter: T1/T2 as summed window
// quantities, BASE as the residual against the (rounded) period total - the
// kWh partition is exact by construction.
func buildBuckets(r *model.MeterReport, cpMeta map[string]*model.CounterPointMeta, bucketSums map[*model.MeterReport]map[string]float64) {
	if len(r.TimeWindows) == 0 || r.Report == nil {
		return
	}
	total := r.Report.Summary.Utilization
	if meta, ok := cpMeta[r.MeterId]; ok && meta.Dir == model.PRODUCER_DIRECTION {
		total = r.Report.Summary.Production - r.Report.Summary.Allocation
	}
	windows := append([]model.TimeWindow{}, r.TimeWindows...)
	sort.Slice(windows, func(i, j int) bool { return windows[i].Key < windows[j].Key })

	buckets := make([]model.Bucket, 0, len(windows)+1)
	windowTotal := float64(0)
	for _, tw := range windows {
		kwh := utils.RoundToFixed(bucketSums[r][tw.Key], 6)
		windowTotal += kwh
		buckets = append(buckets, model.Bucket{Key: tw.Key, KWh: kwh})
	}
	base := utils.RoundToFixed(total-windowTotal, 6)
	r.Report.Buckets = append([]model.Bucket{{Key: "BASE", KWh: base}}, buckets...)
}

func appendEnergyToParticipantMeter(
	dailyReport *calcResults,
	reportValues *reportValues,
	cpMeta map[string]*model.CounterPointMeta,
	lineTime time.Time,
	appendIntermediate func(*model.MeterReport, []float64, model.MeterDirection)) error {

	//for meterId, meta := range cpMeta {
	//	meterReports := meters[meterId]
	//	for _, p := range meterReports {
	for meterId, meterReports := range reportValues.meters {
		if meta, ok := cpMeta[meterId]; ok {
			for _, p := range meterReports {
				from := utils.TruncateToDay(time.UnixMilli(p.From))
				until := utils.TruncateToDay(time.UnixMilli(p.Until))
				//if lineTime.After(p.From) && lineTime.Before(p.Until) {
				if from.Unix() <= lineTime.Unix() && lineTime.Unix() <= until.Unix() {
					if p.Report == nil {
						p.SetReport(&model.Report{})
					}

					switch meta.Dir {
					case model.CONSUMER_DIRECTION:
						values := []float64{
							dailyReport.rCons.RoundToFixed(6).GetElm(meta.SourceIdx, 0),
							dailyReport.rShar.RoundToFixed(6).GetElm(meta.SourceIdx, 0),
							dailyReport.rAlloc.RoundToFixed(6).GetElm(meta.SourceIdx, 0),
						}
						p.Report.Summary.Consumption += values[0]
						p.Report.Summary.Allocation += values[1]
						p.Report.Summary.Utilization += values[2]
						*reportValues.totalConsumption += values[0]
						appendIntermediate(p, values, meta.Dir)
					case model.PRODUCER_DIRECTION:
						//values := dailyReport.rCons.Elements[meta.SourceIdx : meta.SourceIdx+3]
						values := []float64{
							dailyReport.rProd.GetElm(meta.SourceIdx, 0),
							dailyReport.rDist.GetElm(meta.SourceIdx, 0),
						}
						p.Report.Summary.Production += values[0]
						p.Report.Summary.Allocation += values[1]
						*reportValues.totalProduction += values[0]

						appendIntermediate(p, values, meta.Dir)
					}
				}
			}
		} else {
			glog.V(6).Infof("Metering point %s has no energy values received yet", meterId)
		}
	}
	return nil
}
