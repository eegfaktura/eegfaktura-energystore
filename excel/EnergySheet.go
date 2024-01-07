package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/xuri/excelize/v2"
	"time"
)

type EnergySheet struct {
	name      string
	excel     *excelize.File
	stylesQoV []int
	writer    *excelize.StreamWriter
	lineNum   int
}

func (es *EnergySheet) initSheet(ctx *RunnerContext) error {
	participantMeterMap := map[string]string{}
	for _, m := range ctx.cps.Cps {
		participantMeterMap[m.MeteringPoint] = m.Name
	}

	f := es.excel
	_, err := f.NewSheet(es.name)
	if err != nil {
		return err
	}

	styleIdL3, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"ff5429"}, Pattern: 1},
	})
	if err != nil {
		return err
	}
	styleIdL2, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FFFF00"}, Pattern: 1},
	})
	if err != nil {
		return err
	}

	numFmt := "#,##0.000000"
	styleIdNumFmt, err := f.NewStyle(&excelize.Style{
		CustomNumFmt: &numFmt,
	})
	if err != nil {
		return err
	}

	es.stylesQoV = []int{styleIdNumFmt, styleIdL2, styleIdL3}

	es.writer, err = f.NewStreamWriter(es.name)
	if err != nil {
		return err
	}

	_ = es.writer.SetColWidth(1, 1, 30)
	_ = es.writer.SetColWidth(2, 1000, 25)

	_ = es.writer.SetRow("A2",
		append([]interface{}{excelize.Cell{Value: "MeteringpointID"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, i int) interface{} { return m.Name }, func(m *model.CounterPointMeta, i int) int { return 0 })...))

	_ = es.writer.SetRow("A3",
		append([]interface{}{excelize.Cell{Value: "Name"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, i int) interface{} {
				if p, ok := participantMeterMap[m.Name]; ok {
					return p
				}
				return "unknown"
			}, func(m *model.CounterPointMeta, i int) int { return 0 })...))

	_ = es.writer.SetRow("A4",
		append([]interface{}{excelize.Cell{Value: "Energy direction"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, i int) interface{} { return m.Dir }, func(m *model.CounterPointMeta, i int) int { return 0 })...))

	sYear, sMonth, sDay := ctx.start.Year(), int(ctx.start.Month()), ctx.start.Day()
	eYear, eMonth, eDay := ctx.end.Year(), int(ctx.end.Month()), ctx.end.Day()

	_ = es.writer.SetRow("A5",
		append([]interface{}{excelize.Cell{Value: "Period start"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, i int) interface{} {
				d := ctx.getPeriodRange(m).start
				if d.After(ctx.start) {
					return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", d.Day(), int(d.Month()), d.Year())
				}
				return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", sDay, sMonth, sYear)
			}, func(m *model.CounterPointMeta, i int) int { return 0 })...))

	_ = es.writer.SetRow("A6",
		append([]interface{}{excelize.Cell{Value: "Period end"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, i int) interface{} {
				d := ctx.getPeriodRange(m).end
				if d.Before(ctx.end) {
					return fmt.Sprintf("%.2d.%.2d.%.4d 23:45:00", d.Day(), int(d.Month()), d.Year())
				}
				return fmt.Sprintf("%.2d.%.2d.%.4d 23:45:00", eDay, eMonth, eYear)
			}, func(m *model.CounterPointMeta, i int) int { return 0 })...))

	_ = es.writer.SetRow("A7",
		append([]interface{}{excelize.Cell{Value: "Metercode"}},
			addHeaderV2(ctx, 3, 2,
				func(m *model.CounterPointMeta, i int) interface{} {
					if m.Dir == model.CONSUMER_DIRECTION {
						switch i {
						case 0:
							return "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]"
						case 1:
							return "Anteil gemeinschaftliche Erzeugung [KWH]"
						case 2:
							return "Eigendeckung gemeinschaftliche Erzeugung [KWH]"
						default:
							return ""
						}
					} else {
						switch i {
						case 0:
							return "Gesamte gemeinschaftliche Erzeugung [KWH]"
						case 1:
							return "Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]"
						default:
							return ""
						}
					}
				},
				func(m *model.CounterPointMeta, i int) int { return 0 })...))

	return nil
}

func (es *EnergySheet) handleLine(ctx *RunnerContext, line *model.RawSourceLine) error {
	es.lineNum += 1
	lineDate, _, err := utils.ConvertRowIdToTimeString("CP", line.Id, time.Local)
	if err != nil {
		return err
	}
	_ = es.writer.SetRow(fmt.Sprintf("A%d", es.lineNum+10),
		append([]interface{}{excelize.Cell{Value: lineDate}}, addLine(line, ctx.countCons, ctx.meta, es.stylesQoV)...))

	if !checkQoV(line, ctx.meta) {
		ctx.qovLogArray = append(ctx.qovLogArray, line.Copy(0))
	}

	return nil
}

func (es *EnergySheet) closeSheet(ctx *RunnerContext) error {
	return es.writer.Flush()
}

func checkQoV(line *model.RawSourceLine, meta []*model.CounterPointMeta) bool {
	lineDate, _ := utils.ConvertRowIdToTime("CP", line.Id)

	checkDate := func(periodStart string, lineDate time.Time) bool {
		mDate, _ := utils.ParseTime(periodStart, 0)
		if lineDate.Before(mDate) {
			return true
		}
		return false
	}

	check := false
	for _, m := range meta {
		if m.Dir == model.CONSUMER_DIRECTION {
			if checkDate(m.PeriodStart, lineDate) {
				continue
			}
			check = line.QoVConsumers[m.SourceIdx] != 1 || line.QoVConsumers[m.SourceIdx+1] != 1 || line.QoVConsumers[m.SourceIdx+2] != 1
		} else {
			if checkDate(m.PeriodStart, lineDate) {
				continue
			}
			check = line.QoVProducers[m.SourceIdx] != 1 || line.QoVProducers[m.SourceIdx+1] != 1
		}
		if check {
			return false
		}
	}
	return true
}
