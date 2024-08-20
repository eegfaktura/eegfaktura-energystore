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
	//participantMeterMap := map[string]string{}
	//for _, m := range ctx.cps {
	//	participantMeterMap[m.MeteringPoint] = m.Name
	//}

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

	numFmt := "#,##0.0000000"
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
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} { return m.Name },
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = es.writer.SetRow("A3",
		append([]interface{}{excelize.Cell{Value: "Name"}},
			addHeaderV2(ctx, 3, 2,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
					return p.Name
				},
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = es.writer.SetRow("A4",
		append([]interface{}{excelize.Cell{Value: "Energy direction"}},
			addHeaderV2(ctx, 3, 2,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} { return m.Dir },
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	sYear, sMonth, sDay := ctx.start.Year(), int(ctx.start.Month()), ctx.start.Day()
	eYear, eMonth, eDay := ctx.end.Year(), int(ctx.end.Month()), ctx.end.Day()

	_ = es.writer.SetRow("A5",
		append([]interface{}{excelize.Cell{Value: "Period start"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
				d := ctx.getPeriodRange(m).start
				if d.After(ctx.start) {
					return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", d.Day(), int(d.Month()), d.Year())
				}
				return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", sDay, sMonth, sYear)
			}, func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = es.writer.SetRow("A6",
		append([]interface{}{excelize.Cell{Value: "Period end"}},
			addHeaderV2(ctx, 3, 2, func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
				d := ctx.getPeriodRange(m).end
				if d.Before(ctx.end) {
					return fmt.Sprintf("%.2d.%.2d.%.4d 23:45:00", d.Day(), int(d.Month()), d.Year())
				}
				return fmt.Sprintf("%.2d.%.2d.%.4d 23:45:00", eDay, eMonth, eYear)
			}, func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = es.writer.SetRow("A7",
		append([]interface{}{excelize.Cell{Value: "Metercode"}},
			addHeaderV2(ctx, 3, 2,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
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
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	return nil
}

func (es *EnergySheet) handleLine(ctx *RunnerContext, line *model.RawSourceLine) error {
	es.lineNum += 1
	lineDate, _, err := utils.ConvertRowIdToTimeString("CP", line.Id, time.Local)
	if err != nil {
		return err
	}
	_ = es.writer.SetRow(fmt.Sprintf("A%d", es.lineNum+10),
		append([]interface{}{excelize.Cell{Value: lineDate}}, addLine(ctx, line, es.stylesQoV)...))

	if !checkQoV(ctx, line) {
		ctx.qovLogArray = append(ctx.qovLogArray, line.Copy(0))
	}

	return nil
}

func (es *EnergySheet) closeSheet(ctx *RunnerContext) error {
	return es.writer.Flush()
}

func checkQoV(ctx *RunnerContext, line *model.RawSourceLine) bool {
	lineDate, _ := utils.ConvertRowIdToTime("CP", line.Id)

	//checkDate := func(periodStart string, lineDate time.Time) bool {
	//	mDate, _ := utils.ParseTime(periodStart, 0)
	//	if lineDate.Before(mDate) {
	//		return true
	//	}
	//	return false
	//}

	checkDate := func(start, end int64, lineDate time.Time) bool {
		if lineDate.Before(time.UnixMilli(start)) || lineDate.After(time.UnixMilli(end)) {
			return true
		}
		return false
	}

	nok := false
	for _, cp := range ctx.cps {
		m, ok := ctx.metaMap[cp.MeteringPoint]
		if !ok {
			continue
		}
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			if checkDate(cp.ActiveSince, cp.InactiveSince, lineDate) {
				continue
			}
			nok =
				utils.GetInt(line.QoVConsumers, baseIdx) != 1 ||
					utils.GetInt(line.QoVConsumers, baseIdx+1) != 1 ||
					utils.GetInt(line.QoVConsumers, baseIdx+2) != 1
		} else {
			baseIdx := m.SourceIdx * 2
			if checkDate(cp.ActiveSince, cp.InactiveSince, lineDate) {
				continue
			}
			nok =
				utils.GetInt(line.QoVProducers, baseIdx) != 1 ||
					utils.GetInt(line.QoVProducers, baseIdx+1) != 1
		}
		if nok {
			return false
		}
	}
	return true
}
