package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/xuri/excelize/v2"
	"time"
)

func generateLogDataSheet(ctx *RunnerContext, f *excelize.File) error {
	sheetName := "QoV Log"
	//participantMeterMap := map[string]string{}
	//for _, m := range ctx.cps {
	//	participantMeterMap[m.MeteringPoint] = m.Name
	//}

	// Create a new sheet.
	_, err := f.NewSheet(sheetName)
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

	stylesQoV := []int{styleIdNumFmt, styleIdL2, styleIdL3}

	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return err
	}

	_ = sw.SetColWidth(1, 1, 30)

	for i := 0; i < (ctx.countCons*6)+(ctx.countProd*4); i++ {
		if i%2 == 0 {
			_ = sw.SetColWidth(i+2, i+2, 25)
		} else {
			_ = sw.SetColWidth(i+2, i+2, 5)
		}
	}

	_ = sw.SetRow("A2",
		append([]interface{}{excelize.Cell{Value: "MeteringpointID"}},
			addHeaderV2(ctx, 6, 4,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
					if i%2 == 0 {
						return m.Name
					} else {
						return "QoV"
					}
				},
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = sw.SetRow("A3",
		append([]interface{}{excelize.Cell{Value: "Name"}},
			addHeaderV2(ctx, 6, 4,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
					if i%2 == 0 {
						return p.Name
					} else {
						return ""
					}
				},
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = sw.SetRow("A4",
		append([]interface{}{excelize.Cell{Value: "Energy direction"}},
			addHeaderV2(ctx, 6, 4,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
					if i%2 == 0 {
						return m.Dir
					} else {
						return ""
					}
				},
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	_ = sw.SetRow("A7",
		append([]interface{}{excelize.Cell{Value: "Metercode"}},
			addHeaderV2(ctx, 6, 4,
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) interface{} {
					if m.Dir == model.CONSUMER_DIRECTION {
						switch i {
						case 0:
							return "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]"
						case 2:
							return "Anteil gemeinschaftliche Erzeugung [KWH]"
						case 4:
							return "Eigendeckung gemeinschaftliche Erzeugung [KWH]"
						default:
							return ""
						}
					} else {
						switch i {
						case 0:
							return "Gesamte gemeinschaftliche Erzeugung [KWH]"
						case 2:
							return "Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]"
						default:
							return ""
						}
					}
				},
				func(m *model.CounterPointMeta, p *ParticipantCp, i int) int { return 0 })...))

	lineNum := 0
	for _, l := range ctx.qovLogArray {
		lineNum = lineNum + 1
		lineDate, _, err := utils.ConvertRowIdToTimeString("CP", l.Id, time.Local)
		if err != nil {
			return err
		}

		_ = sw.SetRow(fmt.Sprintf("A%d", lineNum+8),
			append([]interface{}{excelize.Cell{Value: lineDate}}, addLineQoV(ctx, &l, stylesQoV)...))
	}

	_ = sw.Flush()
	return nil
}

func addLineQoV(ctx *RunnerContext, line *model.RawSourceLine, stylesQoV []int) []interface{} {

	countCon := ctx.countCons
	countProd := ctx.countProd
	lineDate, _ := utils.ConvertRowIdToTime("CP", line.Id)
	lineData := make([]interface{}, (countCon*6)+(countProd*4))
	//line := map[string][]float64{}
	setCellValue := func(length, sourceIdx int, raw []float64, qov []int) excelize.Cell {
		if length > sourceIdx {
			_qov := 1
			if len(qov) > sourceIdx {
				_qov = qov[sourceIdx]
			}
			if _qov == 1 {
				return excelize.Cell{Value: utils.RoundToFixed(raw[sourceIdx], 6), StyleID: stylesQoV[0]}
			} else if _qov == 2 {
				return excelize.Cell{Value: utils.RoundToFixed(raw[sourceIdx], 6), StyleID: stylesQoV[1]}
			} else if _qov == 3 {
				return excelize.Cell{Value: utils.RoundToFixed(raw[sourceIdx], 6), StyleID: stylesQoV[2]}
			} else {
				//fmt.Printf("Quality of Value is %d Value: %f\n", _qov, utils.RoundToFixed(raw[sourceIdx], 6))
				return excelize.Cell{Value: ""}
			}
		} else {
			return excelize.Cell{Value: ""}
		}
	}

	setCellOoVValue := func(length, sourceIdx int, qov []int) excelize.Cell {
		if length > sourceIdx {
			_qov := 1
			if len(qov) > sourceIdx {
				_qov = qov[sourceIdx]
			}
			if _qov == 1 {
				return excelize.Cell{Value: "L1", StyleID: stylesQoV[0]}
			} else if _qov == 2 {
				return excelize.Cell{Value: "L2", StyleID: stylesQoV[1]}
			} else if _qov == 3 {
				return excelize.Cell{Value: "L3", StyleID: stylesQoV[2]}
			} else if _qov == 0 {
				return excelize.Cell{Value: "L0"}
			} else {
				return excelize.Cell{Value: ""}
			}
		} else {
			return excelize.Cell{Value: ""}
		}
	}

	cCnt := 0
	pCnt := 0
	//for _, m := range meta {
	for i := 0; i < len(ctx.orderedCps); i++ {
		p := ctx.orderedCps[i]
		m := ctx.metaMap[p.MeteringPoint]
		if p.Direction == model.CONSUMER_DIRECTION {
			baseIdx := cCnt * 6
			cCnt += 1
			if utils.IsLineDateOutOfRange(lineDate, [2]int64{p.ActiveSince, p.InactiveSince}) {
				//if lineDate.Before(time.UnixMilli(p.ActiveSince)) || lineDate.After(time.UnixMilli(p.InactiveSince)) {
				for n := 0; n < 6; n++ {
					lineData[baseIdx+n] = excelize.Cell{Value: ""}
				}
			} else {
				lineData[baseIdx] = setCellValue(len(line.Consumers), m.SourceIdx*3, line.Consumers, line.QoVConsumers)
				lineData[baseIdx+1] = setCellOoVValue(len(line.Consumers), m.SourceIdx*3, line.QoVConsumers)
				lineData[baseIdx+2] = setCellValue(len(line.Consumers), (m.SourceIdx*3)+1, line.Consumers, line.QoVConsumers)
				lineData[baseIdx+3] = setCellOoVValue(len(line.Consumers), (m.SourceIdx*3)+1, line.QoVConsumers)
				lineData[baseIdx+4] = setCellValue(len(line.Consumers), (m.SourceIdx*3)+2, line.Consumers, line.QoVConsumers)
				lineData[baseIdx+5] = setCellOoVValue(len(line.Consumers), (m.SourceIdx*3)+2, line.QoVConsumers)
			}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (countCon * 6) + (pCnt * 4)
			pCnt += 1
			if utils.IsLineDateOutOfRange(lineDate, [2]int64{p.ActiveSince, p.InactiveSince}) {
				//if lineDate.Before(time.UnixMilli(p.ActiveSince)) || lineDate.After(time.UnixMilli(p.InactiveSince)) {
				for n := 0; n < 4; n++ {
					lineData[baseIdx+n] = excelize.Cell{Value: ""}
				}
			} else {
				lineData[baseIdx] = setCellValue(len(line.Producers), m.SourceIdx*2, line.Producers, line.QoVProducers) //excelize.Cell{Value: g1.Producers[m.SourceIdx]}
				lineData[baseIdx+1] = setCellOoVValue(len(line.Producers), m.SourceIdx*2, line.QoVProducers)
				lineData[baseIdx+2] = setCellValue(len(line.Producers), (m.SourceIdx*2)+1, line.Producers, line.QoVProducers)
				lineData[baseIdx+3] = setCellOoVValue(len(line.Producers), (m.SourceIdx*2)+1, line.QoVProducers)
			}
		}
	}
	return lineData
}
