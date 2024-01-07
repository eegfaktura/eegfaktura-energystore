package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/xuri/excelize/v2"
	"time"
)

type SummarySheet struct {
	name             string
	excel            *excelize.File
	report           *model.EnergyReport
	qovConsumerSlice []bool
	qovProducerSlice []bool
}

func (ss *SummarySheet) initSheet(ctx *RunnerContext) error {
	ss.report = &model.EnergyReport{
		Consumed:    make([]float64, ctx.info.ConsumerCount),
		Allocated:   make([]float64, ctx.info.ConsumerCount),
		Shared:      make([]float64, ctx.info.ConsumerCount),
		Produced:    make([]float64, ctx.info.ProducerCount),
		Distributed: make([]float64, ctx.info.ProducerCount),
	}

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
	for i := 0; i < consumerMatrix.Rows; i += 1 {
		ss.report.Consumed[i] += consumerMatrix.GetElm(i, 0)
		ss.report.Shared[i] += consumerMatrix.GetElm(i, 1)
		ss.report.Allocated[i] += consumerMatrix.GetElm(i, 2)
		if (i*3)+2 < len(line.QoVConsumers) {
			ss.qovConsumerSlice[i] = ss.qovConsumerSlice[i] && (ctx.checkBegin(lineDate, ctx.periodsConsumer[i].start) || ((line.QoVConsumers[(i*3)] == 1) && (line.QoVConsumers[(i*3)+1] == 1) && (line.QoVConsumers[(i*3)+2] == 1)))
		}
	}
	for i := 0; i < producerMatrix.Rows; i += 1 {
		ss.report.Produced[i] += producerMatrix.GetElm(i, 0)
		ss.report.Distributed[i] += producerMatrix.GetElm(i, 1)
		if (i*2)+1 < len(line.QoVProducers) {
			ss.qovProducerSlice[i] = ss.qovProducerSlice[i] && (ctx.checkBegin(lineDate, ctx.periodsProducer[i].start) || ((line.QoVProducers[(i*2)] == 1) && (line.QoVProducers[(i*2)+1] == 1)))
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

	styleIdQov := map[bool]int{true: styleIdQoVGood, false: styleIdQoVBad}

	sw, err := f.NewStreamWriter(ss.name)
	if err != nil {
		return err
	}

	beginDate := time.Date(ctx.start.Year(), ctx.start.Month(), ctx.start.Day(), 0, 0, 0, 0, time.Local)
	endDate := time.Date(ctx.end.Year(), ctx.end.Month(), ctx.end.Day(), 0, 0, 0, 0, time.Local)

	_ = sw.SetColWidth(1, 1, float64(40))
	_ = sw.SetColWidth(2, 2, float64(35))
	_ = sw.SetColWidth(3, 4, float64(22))
	_ = sw.SetColWidth(5, 5, float64(15))
	_ = sw.SetColWidth(6, 10, float64(22))

	rowOpts := excelize.RowOpts{StyleID: styleIdRowSummary}
	err = sw.SetRow("A2",
		[]interface{}{excelize.Cell{Value: "Gemeinschafts-ID", StyleID: styleIdBold}, excelize.Cell{Value: ctx.cps.CommunityId}}, rowOpts)
	err = sw.SetRow("A3",
		[]interface{}{excelize.Cell{Value: "Zeitraum von", StyleID: styleIdBold}, excelize.Cell{Value: utils.DateToString(beginDate)}}, rowOpts)
	err = sw.SetRow("A4",
		[]interface{}{excelize.Cell{Value: "Zeitraum bis", StyleID: styleIdBold}, excelize.Cell{Value: utils.DateToString(endDate)}}, rowOpts)
	err = sw.SetRow("A5",
		[]interface{}{excelize.Cell{Value: "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummaryMeterResult) float64 { return e.Total })}},
		excelize.RowOpts{StyleID: styleIdRowSummary, Height: 0.34 * 72})
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
			excelize.Cell{Value: "Daten vollständig? Ja/Nein"},
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
				excelize.Cell{Value: c.DataOk, StyleID: styleIdQov[c.DataOk]},
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
			excelize.Cell{Value: "Daten vollständig? Ja/Nein"},
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
				excelize.Cell{Value: c.DataOk, StyleID: styleIdQov[c.DataOk]},
				excelize.Cell{Value: utils.RoundToFixed(c.Share, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Total, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Coverage, 6)},
			}, excelize.RowOpts{StyleID: styleId})

	}
	return sw.Flush()
}

func (ss *SummarySheet) summaryMeteringPoints(ctx *RunnerContext) (*SummaryResult, error) {

	summary := &SummaryResult{Consumer: []SummaryMeterResult{}, Producer: []SummaryMeterResult{}}
	for _, cp := range ctx.cps.Cps {
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
				DataOk:        ss.qovConsumerSlice[m.SourceIdx],
				Total:         returnFloatValue(ss.report.Consumed, m.SourceIdx),
				Coverage:      returnFloatValue(ss.report.Shared, m.SourceIdx),
				Share:         returnFloatValue(ss.report.Allocated, m.SourceIdx),
			})
		} else {
			summary.Producer = append(summary.Producer, SummaryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				DataOk:        ss.qovProducerSlice[m.SourceIdx],
				Total:         returnFloatValue(ss.report.Produced, m.SourceIdx),
				Coverage:      returnFloatValue(ss.report.Produced, m.SourceIdx) - returnFloatValue(ss.report.Distributed, m.SourceIdx),
				Share:         returnFloatValue(ss.report.Distributed, m.SourceIdx),
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
	return utils.RoundFloat(sum, 6)
}
