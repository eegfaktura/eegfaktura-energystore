package excel

import (
	"fmt"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"github.com/xuri/excelize/v2"
)

type SummarySheet struct {
	name  string
	excel *excelize.File
	//report           *model.EnergyReport
	qovConsumerSlice []bool
	qovProducerSlice []bool
}

func (ss *SummarySheet) initSheet(ctx *RunnerContext) error {

	ss.qovConsumerSlice = model.CreateInitializedBoolSlice(ctx.info.ConsumerCount, true)
	ss.qovProducerSlice = model.CreateInitializedBoolSlice(ctx.info.ProducerCount, true)

	_, err := ss.excel.NewSheet(ss.name)
	if err != nil {
		return err
	}

	return nil
}

func (ss *SummarySheet) handleLine(ctx *RunnerContext, line *model.RawSourceLine) error {
	lineDate, _ := utils.ConvertRowIdToTime("CP", line.Id)
	consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(line)

	for _, p := range ctx.cps {
		_ = ss.handleParticipantReport(ctx, p, consumerMatrix, producerMatrix, lineDate, line.QoVConsumers, line.QoVProducers)
	}

	//for i := 0; i < consumerMatrix.Rows && i < ctx.info.ConsumerCount; i += 1 {
	//	ss.report.Consumed[i] += consumerMatrix.GetElm(i, 0)
	//	ss.report.Shared[i] += consumerMatrix.GetElm(i, 1)
	//	ss.report.Allocated[i] += consumerMatrix.GetElm(i, 2)
	//	if (i*3)+2 < len(line.QoVConsumers) {
	//		ss.qovConsumerSlice[i] = ss.qovConsumerSlice[i] && (ctx.checkBegin(lineDate, ctx.periodsConsumer[i].start) || ((line.QoVConsumers[(i*3)] == 1) && (line.QoVConsumers[(i*3)+1] == 1) && (line.QoVConsumers[(i*3)+2] == 1)))
	//	}
	//}
	//for i := 0; i < producerMatrix.Rows && i < ctx.info.ProducerCount; i += 1 {
	//	ss.report.Produced[i] += producerMatrix.GetElm(i, 0)
	//	ss.report.Distributed[i] += producerMatrix.GetElm(i, 1)
	//	if (i*2)+1 < len(line.QoVProducers) {
	//		ss.qovProducerSlice[i] = ss.qovProducerSlice[i] && (ctx.checkBegin(lineDate, ctx.periodsProducer[i].start) || ((line.QoVProducers[(i*2)] == 1) && (line.QoVProducers[(i*2)+1] == 1)))
	//	}
	//}
	return nil
}

func (ss *SummarySheet) handleParticipantReport(ctx *RunnerContext, participant *ParticipantCp,
		consumerMatrix, producerMatrix *model.Matrix, lineDate time.Time, QoVConsumers, QoVProducers []int) error {

	if utils.IsLineDateOutOfRange(lineDate, [2]int64{participant.ActiveSince, participant.InactiveSince}) {
		//if lineDate.Before(time.UnixMilli(participant.ActiveSince)) || lineDate.After(time.UnixMilli(participant.InactiveSince)) {
		return nil
	}

	meta, ok := ctx.metaMap[participant.MeteringPoint]
	// check metering point in metaMap as well
	if !ok {
		return nil
	}

	if participant.Direction == model.CONSUMER_DIRECTION {
		participant.Report.Consumed += consumerMatrix.GetElm(meta.SourceIdx, 0)
		participant.Report.Shared += consumerMatrix.GetElm(meta.SourceIdx, 1)
		participant.Report.Allocated += consumerMatrix.GetElm(meta.SourceIdx, 2)
		if (meta.SourceIdx*3)+2 < len(QoVConsumers) {
			participant.QoV = participant.QoV &&
					(ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) ||
							((QoVConsumers[(meta.SourceIdx*3)] == 1) && (QoVConsumers[(meta.SourceIdx*3)+1] == 1) && (QoVConsumers[(meta.SourceIdx*3)+2] == 1)))
			participant.QoVSum[0] = participant.QoVSum[0] ||
					(!ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) &&
							((QoVConsumers[(meta.SourceIdx*3)] == 0) || (QoVConsumers[(meta.SourceIdx*3)+1] == 0) || (QoVConsumers[(meta.SourceIdx*3)+2] == 0)))
			participant.QoVSum[1] = participant.QoVSum[1] ||
					(!ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) &&
							((QoVConsumers[(meta.SourceIdx*3)] == 2) || (QoVConsumers[(meta.SourceIdx*3)+1] == 2) || (QoVConsumers[(meta.SourceIdx*3)+2] == 2)))
			participant.QoVSum[2] = participant.QoVSum[2] ||
					(!ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) &&
							((QoVConsumers[(meta.SourceIdx*3)] == 3) || (QoVConsumers[(meta.SourceIdx*3)+1] == 3) || (QoVConsumers[(meta.SourceIdx*3)+2] == 3)))
		}
	} else {
		participant.Report.Produced += producerMatrix.GetElm(meta.SourceIdx, 0)
		participant.Report.Distributed += producerMatrix.GetElm(meta.SourceIdx, 1)
		if (meta.SourceIdx*2)+1 < len(QoVProducers) {
			// TODO: check quality of Value calculation. Could be kind of weird!!!
			//ss.qovProducerSlice[i] = ss.qovProducerSlice[i] && (ctx.checkBegin(lineDate, ctx.periodsProducer[i].start) || ((line.QoVProducers[(i*2)] == 1) && (line.QoVProducers[(i*2)+1] == 1)))
			participant.QoV = participant.QoV &&
					(ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) ||
							((QoVProducers[(meta.SourceIdx*2)] == 1) && (QoVProducers[(meta.SourceIdx*2)+1] == 1)))

			participant.QoVSum[0] = participant.QoVSum[0] ||
					(!ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) &&
							((QoVProducers[(meta.SourceIdx*2)] == 0) || (QoVProducers[(meta.SourceIdx*2)+1] == 0)))
			participant.QoVSum[1] = participant.QoVSum[1] ||
					(!ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) &&
							((QoVProducers[(meta.SourceIdx*2)] == 2) || (QoVProducers[(meta.SourceIdx*2)+1] == 2)))
			participant.QoVSum[2] = participant.QoVSum[2] ||
					(!ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince)) &&
							((QoVProducers[(meta.SourceIdx*2)] == 3) || (QoVProducers[(meta.SourceIdx*2)+1] == 3)))
		}
	}
	return nil
}

func (ss *SummarySheet) closeSheet(ctx *RunnerContext) error {
	counterpoints, err := ss.summaryMeteringPoints(ctx)
	if err != nil {
		return err
	}

	f := ss.excel
	styleId, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10.0}})
	styleIdBold, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10.0, Bold: true},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	styleIdRowSummary, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10.0},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	styleIdHeader, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})

	styleIdQoVGood, err := f.NewStyle(&excelize.Style{
		//Font:      &excelize.Font{Bold: true},
		//Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Font: &excelize.Font{Size: 10.0},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"00a933"}, Pattern: 1},
	})

	styleIdQoVBad, err := f.NewStyle(&excelize.Style{
		//Font:      &excelize.Font{Bold: true},
		//Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Font: &excelize.Font{Size: 10.0},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"ff4000"}, Pattern: 1},
	})

	styleIdQoVL0, err := f.NewStyle(&excelize.Style{
		//Font:      &excelize.Font{Bold: true},
		//Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Font: &excelize.Font{Size: 5.0},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"777777"}, Pattern: 1},
	})
	styleIdQoVL2, err := f.NewStyle(&excelize.Style{
		//Font:      &excelize.Font{Bold: true},
		//Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Font: &excelize.Font{Size: 5.0},
		Fill: excelize.Fill{Pattern: 0},
	})

	styleIdQov := map[bool]int{true: styleIdQoVGood, false: styleIdQoVBad}
	styleIdQovSum := map[bool]int{true: styleIdQoVL0, false: styleIdQoVL2}
	//styleIdQovSum := map[int]int{0: styleIdQoVL0, 1: styleIdQoVL2, 2: styleIdQoVL2}

	sw, err := f.NewStreamWriter(ss.name)
	if err != nil {
		return err
	}

	beginDate := time.Date(ctx.start.Year(), ctx.start.Month(), ctx.start.Day(), 0, 0, 0, 0, time.Local)
	endDate := time.Date(ctx.end.Year(), ctx.end.Month(), ctx.end.Day(), 23, 45, 0, 0, time.Local)

	_ = sw.SetColWidth(1, 1, 37.5)
	_ = sw.SetColWidth(2, 2, float64(33))
	_ = sw.SetColWidth(3, 4, float64(20))
	_ = sw.SetColWidth(5, 5, 20.78)
	_ = sw.SetColWidth(6, 6, float64(10))
	_ = sw.SetColWidth(7, 9, 2.1)
	_ = sw.SetColWidth(10, 14, float64(20))

	rowOpts := excelize.RowOpts{StyleID: styleIdRowSummary}
	err = sw.SetRow("A2",
		[]interface{}{excelize.Cell{Value: "Gemeinschafts-ID", StyleID: styleIdBold}, excelize.Cell{Value: ctx.communityId}}, rowOpts)
	err = sw.SetRow("A3",
		[]interface{}{excelize.Cell{Value: "Zeitraum von", StyleID: styleIdBold}, excelize.Cell{Value: utils.DateToString(beginDate)}}, rowOpts)
	err = sw.SetRow("A4",
		[]interface{}{excelize.Cell{Value: "Zeitraum bis", StyleID: styleIdBold}, excelize.Cell{Value: utils.DateToString(endDate)}}, rowOpts)
	err = sw.SetRow("A5",
		[]interface{}{excelize.Cell{Value: "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummaryMeterResult) float64 { return e.Total })}},
		excelize.RowOpts{StyleID: styleIdRowSummary, Height: 27.6})
	err = sw.SetRow("A6",
		[]interface{}{excelize.Cell{Value: "Anteil gemeinschaftliche Erzeugung [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummaryMeterResult) float64 { return e.Coverage })}},
		rowOpts)
	err = sw.SetRow("A7",
		[]interface{}{excelize.Cell{Value: "Eigendeckung gemeinschaftliche Erzeugung [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummaryMeterResult) float64 { return e.Share })}},
		excelize.RowOpts{StyleID: styleIdRowSummary, Height: 0.34 * 72})
	err = sw.SetRow("A8",
		[]interface{}{excelize.Cell{Value: "Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Producer, func(e *SummaryMeterResult) float64 { return e.Share })}},
		excelize.RowOpts{StyleID: styleIdRowSummary, Height: 0.34 * 72})
	err = sw.SetRow("A9",
		[]interface{}{excelize.Cell{Value: "Gesamte gemeinschaftliche Erzeugung [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Producer, func(e *SummaryMeterResult) float64 { return e.Total })}},
		rowOpts)

	line := 12
	err = sw.SetRow(fmt.Sprintf("A%d", line),
		[]interface{}{excelize.Cell{Value: "Verbrauchszählpunkt"},
			excelize.Cell{Value: "Name"},
			excelize.Cell{Value: "Beginn der Daten"},
			excelize.Cell{Value: "Ende der Daten"},
			excelize.Cell{Value: "Aktiviert"},
			excelize.Cell{Value: "Daten vollständig? Ja/Nein"},
			excelize.Cell{Value: "L0"},
			excelize.Cell{Value: "L2"},
			excelize.Cell{Value: "L3"},
			excelize.Cell{Value: "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]"},
			excelize.Cell{Value: "Anteil gemeinschaftliche Erzeugung [KWH]"},
			excelize.Cell{Value: "Eigendeckung gemeinschaftliche Erzeugung [KWH]"},
		}, excelize.RowOpts{StyleID: styleIdHeader, Height: 1.15 * 72})

	for _, c := range counterpoints.Consumer {
		line = line + 1
		err = sw.SetRow(fmt.Sprintf("A%d", line),
			[]interface{}{excelize.Cell{Value: c.MeteringPoint},
				excelize.Cell{Value: c.Name},
				excelize.Cell{Value: c.BeginDate},
				excelize.Cell{Value: c.EndDate},
				excelize.Cell{Value: c.ActivePeriod},
				excelize.Cell{Value: c.DataOk, StyleID: styleIdQov[c.DataOk]},
				excelize.Cell{Value: "", StyleID: styleIdQovSum[c.DataL0]},
				excelize.Cell{Value: "", StyleID: styleIdQovSum[c.DataL2]},
				excelize.Cell{Value: "", StyleID: styleIdQovSum[c.DataL3]},
				excelize.Cell{Value: utils.RoundToFixed(c.Total, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Coverage, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Share, 6)},
			}, excelize.RowOpts{StyleID: styleId})

	}

	line = line + 3
	err = sw.SetRow(fmt.Sprintf("A%d", line),
		[]interface{}{excelize.Cell{Value: "Einspeisezählpunkt"},
			excelize.Cell{Value: "Name"},
			excelize.Cell{Value: "Beginn der Daten"},
			excelize.Cell{Value: "Ende der Daten"},
			excelize.Cell{Value: "Aktiviert"},
			excelize.Cell{Value: "Daten vollständig? Ja/Nein"},
			excelize.Cell{Value: "L0"},
			excelize.Cell{Value: "L2"},
			excelize.Cell{Value: "L3"},
			excelize.Cell{Value: "Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]"},
			excelize.Cell{Value: "Gesamte gemeinschaftliche Erzeugung [KWH]"},
			excelize.Cell{Value: "Eigendeckung gemeinschaftliche Erzeugung [KWH]"},
		}, excelize.RowOpts{StyleID: styleIdHeader, Height: 1.15 * 72})

	for _, c := range counterpoints.Producer {
		line = line + 1
		err = sw.SetRow(fmt.Sprintf("A%d", line),
			[]interface{}{excelize.Cell{Value: c.MeteringPoint},
				excelize.Cell{Value: c.Name},
				excelize.Cell{Value: c.BeginDate},
				excelize.Cell{Value: c.EndDate},
				excelize.Cell{Value: c.ActivePeriod},
				excelize.Cell{Value: c.DataOk, StyleID: styleIdQov[c.DataOk]},
				excelize.Cell{Value: "", StyleID: styleIdQovSum[c.DataL0]},
				excelize.Cell{Value: "", StyleID: styleIdQovSum[c.DataL2]},
				excelize.Cell{Value: "", StyleID: styleIdQovSum[c.DataL3]},
				excelize.Cell{Value: utils.RoundToFixed(c.Share, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Total, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Coverage, 6)},
			}, excelize.RowOpts{StyleID: styleId})

	}
	return sw.Flush()
}

func (ss *SummarySheet) summaryMeteringPoints(ctx *RunnerContext) (*SummaryResult, error) {
	summary := &SummaryResult{Consumer: []SummaryMeterResult{}, Producer: []SummaryMeterResult{}}
	for _, cp := range ctx.cps {
		m, ok := ctx.metaMap[cp.MeteringPoint]
		if !ok {
			continue
		}
		if cp.Direction == "CONSUMPTION" {
			summary.Consumer = append(summary.Consumer, SummaryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				ActivePeriod: fmt.Sprintf("%s - %s",
					time.UnixMilli(cp.ActiveSince).Format("02-01-2006"),
					time.UnixMilli(cp.InactiveSince).Format("02-01-2006")),
				DataOk:   cp.QoV, //utils.GetBool(ss.qovConsumerSlice, m.SourceIdx),
				DataL0:   cp.QoVSum[0],
				DataL2:   cp.QoVSum[1],
				DataL3:   cp.QoVSum[2],
				Total:    cp.Report.Consumed,  //returnFloatValue(ss.report.Consumed, m.SourceIdx),
				Coverage: cp.Report.Shared,    //returnFloatValue(ss.report.Shared, m.SourceIdx),
				Share:    cp.Report.Allocated, //returnFloatValue(ss.report.Allocated, m.SourceIdx),
			})
		} else {
			summary.Producer = append(summary.Producer, SummaryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				ActivePeriod: fmt.Sprintf("%s - %s",
					time.UnixMilli(cp.ActiveSince).Format("02-01-2006"),
					time.UnixMilli(cp.InactiveSince).Format("02-01-2006")),
				DataOk:   cp.QoV, //utils.GetBool(ss.qovProducerSlice, m.SourceIdx),
				DataL0:   cp.QoVSum[0],
				DataL2:   cp.QoVSum[1],
				DataL3:   cp.QoVSum[2],
				Total:    cp.Report.Produced,                         //returnFloatValue(ss.report.Produced, m.SourceIdx),
				Coverage: cp.Report.Produced - cp.Report.Distributed, //returnFloatValue(ss.report.Produced, m.SourceIdx) - returnFloatValue(ss.report.Distributed, m.SourceIdx),
				Share:    cp.Report.Distributed,                      //returnFloatValue(ss.report.Distributed, m.SourceIdx),
			})
		}
	}

	return summary, nil
}

func sumMeterResult(s []SummaryMeterResult, elem func(e *SummaryMeterResult) float64) float64 {
	sum := 0.0
	for _, e := range s {
		sum = sum + elem(&e)
	}
	//return utils.RoundFloat(sum, 6)
	//fmt.Printf("Calc: SUM: %v\n", sum)
	return sum
}
