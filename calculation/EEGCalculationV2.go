package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/golang/glog"
	"math"
	"time"
)

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

func CalculateParticipantPeriod(db *store.BowStorage, allocFunc AllocationHandlerV2, year, segment int, meterings map[string][]model.MeterReport) error {
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
	startDay time.Time, rowPrefix string, dayCb func(day time.Time, results *calcResults) error) error {
	daySummary := newCalcResult(metaInfo)
	day := startDay
	var _line model.RawSourceLine
	for iter.Next(&_line) {
		line := _line.Copy(0)
		currentTimeStamp, err := utils.ConvertRowIdToTime(rowPrefix, line.Id)
		if err != nil {
			continue
		}

		if currentTimeStamp.YearDay() != day.YearDay() {
			if err := dayCb(day, daySummary); err != nil {
				glog.Errorf("Error Daily Summary: %s", err.Error())
			}
			daySummary = newCalcResult(metaInfo)
			day = currentTimeStamp
		}

		if err := appendResults(&line, allocFunc, daySummary); err != nil {
			return err
		}
	}
	return dayCb(day, daySummary)
}

func CalculateMonthlyPeriodV2(db *store.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year, segment int) error {
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

func CalculateBiAnnualPeriodV2(db *store.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year, segment int) error {
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

func CalculateQuarterlyPeriodV2(db *store.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year, segment int) error {
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

func CalculateAnnualPeriodV2(db *store.BowStorage, report *model.ReportResponse, allocFunc AllocationHandlerV2, year int) error {
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
		fmt.Printf("Intermediate Idx: %+v -- (week: %d startWeek: %d = %d\n", int(math.Max(float64((a%b+b)%b)+1, 1)), month, startMonth, month-startMonth)
		return int(math.Max(float64((a%b+b)%b)+1, 1))
	})
	return err
}

func calcParticipantReport(iter ebow.IRange,
	reportValues *reportValues,
	allocFunc AllocationHandlerV2,
	cpMeta map[string]*model.CounterPointMeta,
	metaInfo *model.CounterPointMetaInfo, rowPrefix string, startDate time.Time, switchIntermediate func(time.Time) int) error {
	//intermediateReport := model.Recort{}
	err := calcDailyScope(iter, allocFunc, metaInfo, startDate, rowPrefix,
		func(currentDate time.Time, summary *calcResults) error {
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
						//ir.Consumption += values[0]
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
						//ir.Production += values[0]
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
		}
	}
	return err
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
			glog.Warningf("Metering point %s has no energy values received yet", meterId)
		}
	}
	return nil
}
