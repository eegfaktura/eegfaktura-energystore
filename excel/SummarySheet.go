package excel

import (
	"fmt"
	"strings"
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

	// Das Standardblatt "Sheet1" uebernehmen statt es am Ende zu loeschen:
	// DeleteSheet ruft SetActiveSheet, das jedes Blatt einliest -- auch die
	// schon geschriebenen Stream-Blaetter unter 16 MB, die excelize dafuer
	// komplett zurueck ins DOM parst und beim Speichern erneut kodiert. Bei
	// kleinen und mittleren Gemeinschaften war das rund die Haelfte der Zeit.
	var err error
	if idx, _ := ss.excel.GetSheetIndex("Sheet1"); idx != -1 {
		err = ss.excel.SetSheetName("Sheet1", ss.name)
	} else {
		_, err = ss.excel.NewSheet(ss.name)
	}
	return err
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
		return nil
	}

	meta, ok := ctx.metaMap[participant.MeteringPoint]
	// check metering point in metaMap as well
	if !ok {
		return nil
	}

	var qovs []int
	if participant.Direction == model.CONSUMER_DIRECTION {
		participant.Report.Consumed += consumerMatrix.GetElm(meta.SourceIdx, 0)
		participant.Report.Shared += consumerMatrix.GetElm(meta.SourceIdx, 1)
		participant.Report.Allocated += consumerMatrix.GetElm(meta.SourceIdx, 2)
		if (meta.SourceIdx*3)+2 < len(QoVConsumers) {
			qovs = QoVConsumers[meta.SourceIdx*3 : (meta.SourceIdx*3)+3]
		}
	} else {
		participant.Report.Produced += producerMatrix.GetElm(meta.SourceIdx, 0)
		participant.Report.Distributed += producerMatrix.GetElm(meta.SourceIdx, 1)
		if (meta.SourceIdx*2)+1 < len(QoVProducers) {
			qovs = QoVProducers[meta.SourceIdx*2 : (meta.SourceIdx*2)+2]
		}
	}
	if qovs == nil {
		return nil
	}

	allOk := true
	for _, q := range qovs {
		allOk = allOk && qovOk(q)
	}
	beforeBegin := ctx.checkBegin(lineDate, time.UnixMilli(participant.ActiveSince))
	participant.QoV = participant.QoV && (beforeBegin || allOk)
	if beforeBegin {
		return nil
	}

	// Je Stufe zaehlen, wie viele Viertelstunden betroffen sind, und an welchen
	// Tagen. Eine Viertelstunde zaehlt einmal, auch wenn mehrere Werte desselben
	// Zaehlpunkts betroffen sind.
	for slot, level := range [3]int{0, 2, 3} {
		hit := false
		for _, q := range qovs {
			if q == level {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		participant.QoVSum[slot] = true
		participant.QoVCount[slot] += 1
		// Die Zeilen kommen chronologisch: es genuegt, den zuletzt notierten Tag
		// zu vergleichen, um eine sortierte Liste ohne Dubletten zu bekommen.
		day := lineDate.Format("02.01.2006")
		days := participant.QoVDays[slot]
		if len(days) == 0 || days[len(days)-1] != day {
			participant.QoVDays[slot] = append(days, day)
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

	// Die Zaehler L0/L2/L3 tragen DIESELBEN Farben wie die Zellen im Blatt
	// "Energiedaten". Damit ist die Uebersicht zugleich die Legende: grau = kein
	// Messwert (L0), gelb = Ersatzwert (L2), rot = fehlerhaft (L3).
	markStyle := func(color string) int {
		id, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Size: 10.0},
			Fill: excelize.Fill{Type: "pattern", Color: []string{color}, Pattern: 1},
		})
		return id
	}
	markL0, markL2, markL3 := markStyle(qovColorL0), markStyle(qovColorL2), markStyle(qovColorL3)

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
	_ = sw.SetColWidth(7, 9, 6)
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

	// Die Tage je Stufe haengen als Kommentar an der Zahl.
	notes := []excelize.Comment{}
	addNote := func(row, col int, level string, count int, days []string) {
		text := qovDayComment(level, count, days)
		if text == "" {
			return
		}
		axis, err := excelize.CoordinatesToCellName(col, row)
		if err != nil {
			return
		}
		notes = append(notes, excelize.Comment{
			Cell: axis, Author: "eegfaktura", Width: 260, Height: 28 + 14*uint((len(days)/6)+1),
			Text: text,
		})
	}

	for _, c := range counterpoints.Consumer {
		line = line + 1
		err = sw.SetRow(fmt.Sprintf("A%d", line),
			[]interface{}{excelize.Cell{Value: c.MeteringPoint},
				excelize.Cell{Value: c.Name},
				excelize.Cell{Value: c.BeginDate},
				excelize.Cell{Value: c.EndDate},
				excelize.Cell{Value: c.ActivePeriod},
				excelize.Cell{Value: c.DataOk, StyleID: styleIdQov[c.DataOk]},
				countCell(c.CountL0, markL0),
				countCell(c.CountL2, markL2),
				countCell(c.CountL3, markL3),
				excelize.Cell{Value: utils.RoundToFixed(c.Total, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Coverage, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Share, 6)},
			}, excelize.RowOpts{StyleID: styleId})
		addNote(line, 7, "L0 (kein Messwert)", c.CountL0, c.Days[0])
		addNote(line, 8, "L2 (Ersatzwert)", c.CountL2, c.Days[1])
		addNote(line, 9, "L3 (fehlerhaft)", c.CountL3, c.Days[2])
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
				countCell(c.CountL0, markL0),
				countCell(c.CountL2, markL2),
				countCell(c.CountL3, markL3),
				excelize.Cell{Value: utils.RoundToFixed(c.Share, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Total, 6)},
				excelize.Cell{Value: utils.RoundToFixed(c.Coverage, 6)},
			}, excelize.RowOpts{StyleID: styleId})
		addNote(line, 7, "L0 (kein Messwert)", c.CountL0, c.Days[0])
		addNote(line, 8, "L2 (Ersatzwert)", c.CountL2, c.Days[1])
		addNote(line, 9, "L3 (fehlerhaft)", c.CountL3, c.Days[2])
	}
	// Kommentare VOR dem Flush setzen: erst dabei schreibt der StreamWriter das
	// Blatt endgueltig, samt der Verknuepfung <legacyDrawing> auf die Kommentare.
	// Danach gesetzte Kommentare landen zwar in der Datei, aber ohne diese
	// Verknuepfung -- Excel zeigt sie dann nicht an.
	for _, n := range notes {
		if err = f.AddComment(ss.name, n); err != nil {
			return err
		}
	}
	return sw.Flush()
}

// qovDayComment beschreibt, an welchen Tagen eine Qualitaetsstufe aufgetreten ist.
// Die Tage stehen als Kommentar an der Zahl, damit die Uebersicht schmal bleibt.
func qovDayComment(level string, count int, days []string) string {
	if count == 0 || len(days) == 0 {
		return ""
	}
	plural := func(n int, one, many string) string {
		if n == 1 {
			return one
		}
		return many
	}
	return fmt.Sprintf("%s: %d %s an %d %s\n%s",
		level, count, plural(count, "Viertelstunde", "Viertelstunden"),
		len(days), plural(len(days), "Tag", "Tagen"), strings.Join(days, ", "))
}

// countCell zeigt die Anzahl betroffener Viertelstunden auf der Farbe der Stufe.
// Null bleibt leer und ungefaerbt, damit nur echte Treffer ins Auge fallen.
func countCell(n, style int) excelize.Cell {
	if n == 0 {
		return excelize.Cell{Value: ""}
	}
	return excelize.Cell{Value: n, StyleID: style}
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
				CountL0:  cp.QoVCount[0],
				CountL2:  cp.QoVCount[1],
				CountL3:  cp.QoVCount[2],
				Days:     cp.QoVDays,
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
				CountL0:  cp.QoVCount[0],
				CountL2:  cp.QoVCount[1],
				CountL3:  cp.QoVCount[2],
				Days:     cp.QoVDays,
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
