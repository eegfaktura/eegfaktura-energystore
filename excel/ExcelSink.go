package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/xuri/excelize/v2"
)

func ExportExcel(tenant string, year, month int) error {

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return err
	}
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	// Create a new sheet.
	index := f.NewSheet("Sheet1")
	f.SetActiveSheet(index)

	if err != nil {
		fmt.Println(err)
		return err
	}
	defer func() { db.Close() }()

	iterG1 := db.GetLinePrefix(fmt.Sprintf("CP-G.01/%.4d/%.2d/", year, month))
	defer iterG1.Close()
	iterG2 := db.GetLinePrefix(fmt.Sprintf("CP-G.02/%.4d/%.2d/", year, month))
	defer iterG2.Close()
	iterG3 := db.GetLinePrefix(fmt.Sprintf("CP-G.03/%.4d/%.2d/", year, month))
	defer iterG3.Close()

	var _lineG1 model.RawSourceLine
	var _lineG2 model.RawSourceLine
	var _lineG3 model.RawSourceLine

	g1Ok := iterG1.Next(&_lineG1)
	g2Ok := iterG2.Next(&_lineG2)
	g3Ok := iterG3.Next(&_lineG3)
	_ = g3Ok

	sw, err := f.NewStreamWriter("Sheet1")

	meta, _ := db.GetMeta(fmt.Sprintf("cpmeta/%s", "0"))

	sw.SetRow("A2",
		append([]interface{}{excelize.Cell{Value: "MeteringpointID"}},
			addHeader(meta, func(m *model.CounterPointMeta) interface{} { return m.Name })...))

	sw.SetRow("A4",
		append([]interface{}{excelize.Cell{Value: "Energy direction"}},
			addHeader(meta, func(m *model.CounterPointMeta) interface{} { return m.Dir })...))

	sw.SetRow("A5",
		append([]interface{}{excelize.Cell{Value: "Period start"}},
			addHeader(meta, func(m *model.CounterPointMeta) interface{} {
				return fmt.Sprintf("01.%.2d.%.4d 00:00:00", month, year)
			})...))

	sw.SetRow("A6",
		append([]interface{}{excelize.Cell{Value: "Period end"}},
			addHeader(meta, func(m *model.CounterPointMeta) interface{} {
				return fmt.Sprintf("01.%.2d.%.4d 00:00:00", month+1, year)
			})...))

	sw.SetRow("A7",
		append([]interface{}{excelize.Cell{Value: "Metercode"}},
			addHeaderMeterCode(meta, func(m MeterCodeType) interface{} {
				switch m {
				case Total:
					return "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]"
				case Share:
					return "Anteil gemeinschaftliche Erzeugung [KWH]"
				case Coverage:
					return "Eigendeckung gemeinschaftliche Erzeugung [KWH]"
				default:
					return "No Data"
				}
			})...))
	//line := map[string][]float64{}
	lineNum := 0
	for g1Ok && g2Ok {
		lineNum = lineNum + 1
		lineDate, err := utils.ConvertRowIdToTimeString("CP-G.01", _lineG1.Id)
		if err != nil {
			return err
		}

		//line[lineDate] = addLine(&_lineG1, &_lineG2, &_lineG3, meta)

		sw.SetRow(fmt.Sprintf("A%d", lineNum+10),
			append([]interface{}{excelize.Cell{Value: lineDate}}, addLine(&_lineG1, &_lineG2, &_lineG3, meta)...))

		g1Ok = iterG1.Next(&_lineG1)
		g2Ok = iterG2.Next(&_lineG2)
		g3Ok = iterG3.Next(&_lineG3)
	}

	sw.Flush()
	return f.SaveAs("TestOutput.xlsx")
}

func addLine(g1, g2, g3 *model.RawSourceLine, meta *model.RawSourceMeta) []interface{} {

	lineData := make([]interface{}, len(meta.CounterPoints)*3)
	//line := map[string][]float64{}

	for _, m := range meta.CounterPoints {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			lineData[baseIdx] = excelize.Cell{Value: g1.Consumers[m.SourceIdx]}
			lineData[baseIdx+1] = excelize.Cell{Value: g2.Consumers[m.SourceIdx]}
			lineData[baseIdx+2] = excelize.Cell{Value: g3.Consumers[m.SourceIdx]}
		}
	}
	return lineData
}

func addHeader(meta *model.RawSourceMeta, value func(meta *model.CounterPointMeta) interface{}) []interface{} {
	lineData := make([]interface{}, len(meta.CounterPoints)*3)
	for _, m := range meta.CounterPoints {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			lineData[baseIdx] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+2] = excelize.Cell{Value: value(m)}
		}
	}
	return lineData
}

func addHeaderMeterCode(meta *model.RawSourceMeta, value func(code MeterCodeType) interface{}) []interface{} {
	lineData := make([]interface{}, len(meta.CounterPoints)*3)
	for _, m := range meta.CounterPoints {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			lineData[baseIdx] = excelize.Cell{Value: value(Total)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(Share)}
			lineData[baseIdx+2] = excelize.Cell{Value: value(Coverage)}
		}
	}
	return lineData
}
