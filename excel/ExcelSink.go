package excel

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"bytes"
	"errors"
	"fmt"
	"github.com/xuri/excelize/v2"
	"time"
)

type ExportCPs struct {
	Start       int64            `json:"start"`
	End         int64            `json:"end"`
	CommunityId string           `json:"communityId"`
	Cps         []InvestigatorCP `json:"cps"`
}

type InvestigatorCP struct {
	MeteringPoint string `json:"meteringPoint"`
	Direction     string `json:"direction"`
	Name          string `json:"name"`
}

type SummeryMeterResult struct {
	MeteringPoint string
	Name          string
	BeginDate     string
	EndDate       string
	DataOk        bool
	Total         float64
	Coverage      float64
	Share         float64
}

type SummeryResult struct {
	Consumer []SummeryMeterResult
	Producer []SummeryMeterResult
}

func ExportEnergyDataToMail(tenant, to string, year, month int, cps *ExportCPs) error {

	buf, err := ExportExcel(tenant, year, month, cps)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-Energie Report-%d%.2d.xlsx", tenant, year, month)
	return utils.SendMail(tenant, to, fmt.Sprintf("EEG (%s) - Excel Report", tenant), nil, &filename, buf)
}

func ExportExcel(tenant string, year, month int, cps *ExportCPs) (*bytes.Buffer, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)

	return CreateExcelFile(tenant, start, end, cps)
}

func CreateExcelFile(tenant string, start, end time.Time, cps *ExportCPs) (*bytes.Buffer, error) {

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return nil, err
	}
	defer func() { db.Close() }()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	if err = generateSummeryDataSheet(db, f, start, end, cps); err != nil {
		return nil, err
	}
	if err := generateEnergyDataSheet(db, f, start, end, cps.Cps); err != nil {
		return nil, err
	}

	_ = f.DeleteSheet("Sheet1")
	return f.WriteToBuffer()
}

func generateSummeryDataSheet(db *store.BowStorage, f *excelize.File, start, end time.Time, cps *ExportCPs) error {

	sheet := "Summery"
	counterpoints, err := summaryCounterPoints(db, start, end, cps)
	if err != nil {
		return err
	}

	f.NewSheet(sheet)
	//index := f.NewSheet(sheet)
	//f.SetActiveSheet(index)

	styleId, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Size: 10.0}})
	styleIdBold, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10.0, Bold: true},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	styleIdRowSummery, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10.0},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	styleIdHeader, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return err
	}

	beginDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	sw.SetColWidth(1, 1, float64(40))
	sw.SetColWidth(2, 2, float64(35))
	sw.SetColWidth(3, 4, float64(22))
	sw.SetColWidth(5, 5, float64(15))
	sw.SetColWidth(6, 10, float64(22))

	rowOpts := excelize.RowOpts{StyleID: styleIdRowSummery}
	err = sw.SetRow("A2",
		[]interface{}{excelize.Cell{Value: "Gemeinschafts-ID", StyleID: styleIdBold}, excelize.Cell{Value: cps.CommunityId}}, rowOpts)
	err = sw.SetRow("A3",
		[]interface{}{excelize.Cell{Value: "Zeitraum von", StyleID: styleIdBold}, excelize.Cell{Value: utils.DateToString(beginDate)}}, rowOpts)
	err = sw.SetRow("A4",
		[]interface{}{excelize.Cell{Value: "Zeitraum bis", StyleID: styleIdBold}, excelize.Cell{Value: utils.DateToString(endDate)}}, rowOpts)
	err = sw.SetRow("A5",
		[]interface{}{excelize.Cell{Value: "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummeryMeterResult) float64 { return e.Total })}},
		excelize.RowOpts{StyleID: styleIdRowSummery, Height: 0.34 * 72})
	err = sw.SetRow("A6",
		[]interface{}{excelize.Cell{Value: "Anteil gemeinschaftliche Erzeugung [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummeryMeterResult) float64 { return e.Coverage })}},
		rowOpts)
	err = sw.SetRow("A7",
		[]interface{}{excelize.Cell{Value: "Eigendeckung gemeinschaftliche Erzeugung [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Consumer, func(e *SummeryMeterResult) float64 { return e.Share })}},
		excelize.RowOpts{StyleID: styleIdRowSummery, Height: 0.34 * 72})
	err = sw.SetRow("A8",
		[]interface{}{excelize.Cell{Value: "Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Producer, func(e *SummeryMeterResult) float64 { return e.Coverage })}},
		excelize.RowOpts{StyleID: styleIdRowSummery, Height: 0.34 * 72})
	err = sw.SetRow("A9",
		[]interface{}{excelize.Cell{Value: "Gesamte gemeinschaftliche Erzeugung [KWH]", StyleID: styleIdBold},
			excelize.Cell{Value: sumMeterResult(counterpoints.Producer, func(e *SummeryMeterResult) float64 { return e.Total })}},
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
				excelize.Cell{Value: c.DataOk},
				excelize.Cell{Value: c.Total},
				excelize.Cell{Value: c.Coverage},
				excelize.Cell{Value: c.Share},
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
				excelize.Cell{Value: c.DataOk},
				excelize.Cell{Value: c.Coverage},
				excelize.Cell{Value: c.Total},
				excelize.Cell{Value: c.Share},
			}, excelize.RowOpts{StyleID: styleId})

	}
	return sw.Flush()
}

func generateEnergyDataSheet(db *store.BowStorage, f *excelize.File, start, end time.Time, meters []InvestigatorCP) error {

	participantMeterMap := map[string]string{}
	for _, m := range meters {
		participantMeterMap[m.MeteringPoint] = m.Name
	}

	// Create a new sheet.
	_, err := f.NewSheet("Energiedaten")
	if err != nil {
		return err
	}

	//iterG1 := db.GetLinePrefix(fmt.Sprintf("CP-G.01/%.4d/%.2d/", year, month))
	//defer iterG1.Close()
	//iterG2 := db.GetLinePrefix(fmt.Sprintf("CP-G.02/%.4d/%.2d/", year, month))
	//defer iterG2.Close()
	//iterG3 := db.GetLinePrefix(fmt.Sprintf("CP-G.03/%.4d/%.2d/", year, month))
	//defer iterG3.Close()

	sYear, sMonth, sDay := start.Year(), int(start.Month()), start.Day()
	eYear, eMonth, eDay := end.Year(), int(end.Month()), end.Day()

	iterG1 := db.GetLineRange("CP-G.01", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterG1.Close()
	iterG2 := db.GetLineRange("CP-G.02", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterG2.Close()
	iterG3 := db.GetLineRange("CP-G.03", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterG3.Close()

	var _lineG1 model.RawSourceLine
	var _lineG2 model.RawSourceLine
	var _lineG3 model.RawSourceLine

	g1Ok := iterG1.Next(&_lineG1)
	g2Ok := iterG2.Next(&_lineG2)
	g3Ok := iterG3.Next(&_lineG3)
	_ = g3Ok

	sw, err := f.NewStreamWriter("Energiedaten")
	if err != nil {
		return err
	}

	sw.SetColWidth(1, 1, 30)
	sw.SetColWidth(2, 1000, 25)

	meta, _ := db.GetMeta(fmt.Sprintf("cpmeta/%s", "0"))
	countCons, countProd := utils.CountConsumerProducer(meta)

	sw.SetRow("A2",
		append([]interface{}{excelize.Cell{Value: "MeteringpointID"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} { return m.Name })...))

	sw.SetRow("A3",
		append([]interface{}{excelize.Cell{Value: "Name"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} {
				if p, ok := participantMeterMap[m.Name]; ok {
					return p
				}
				return "unknown"
			})...))

	sw.SetRow("A4",
		append([]interface{}{excelize.Cell{Value: "Energy direction"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} { return m.Dir })...))

	sw.SetRow("A5",
		append([]interface{}{excelize.Cell{Value: "Period start"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} {
				return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", sDay, sMonth, sYear)
			})...))

	sw.SetRow("A6",
		append([]interface{}{excelize.Cell{Value: "Period end"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} {
				return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", eDay, eMonth, eYear)
			})...))

	sw.SetRow("A7",
		append([]interface{}{excelize.Cell{Value: "Metercode"}},
			addHeaderMeterCode(meta, countCons, countProd, func(m MeterCodeType) interface{} {
				switch m {
				case Total:
					return "Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]"
				case Share:
					return "Anteil gemeinschaftliche Erzeugung [KWH]"
				case Coverage:
					return "Eigendeckung gemeinschaftliche Erzeugung [KWH]"
				case TotalProd:
					return "Gesamte gemeinschaftliche Erzeugung [KWH]"
				case Profit:
					return "Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]"
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
			append([]interface{}{excelize.Cell{Value: lineDate}}, addLine(&_lineG1, &_lineG2, &_lineG3, countCons, countProd, meta)...))

		g1Ok = iterG1.Next(&_lineG1)
		g2Ok = iterG2.Next(&_lineG2)
		g3Ok = iterG3.Next(&_lineG3)
	}

	sw.Flush()

	err = f.SetColWidth("Energiedaten", "A", "A", float64(25.0))
	if err != nil {
		return err
	}

	return nil
}

func addLine(g1, g2, g3 *model.RawSourceLine, countCon, countProd int, meta *model.RawSourceMeta) []interface{} {

	lineData := make([]interface{}, len(meta.CounterPoints)*3)
	//line := map[string][]float64{}

	setCellValue := func(length, sourceIdx int, raw []float64) excelize.Cell {
		if length > sourceIdx {
			return excelize.Cell{Value: raw[sourceIdx]}
		} else {
			return excelize.Cell{Value: 0}
		}
	}

	for _, m := range meta.CounterPoints {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			lineData[baseIdx] = setCellValue(len(g1.Consumers), m.SourceIdx, g1.Consumers) //excelize.Cell{Value: g1.Consumers[m.SourceIdx]}
			lineData[baseIdx+1] = setCellValue(len(g2.Consumers), m.SourceIdx, g2.Consumers)
			lineData[baseIdx+2] = setCellValue(len(g3.Consumers), m.SourceIdx, g3.Consumers)
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (countCon * 3) + (m.SourceIdx * 2)
			lineData[baseIdx] = setCellValue(len(g1.Producers), m.SourceIdx, g1.Producers) //excelize.Cell{Value: g1.Producers[m.SourceIdx]}
			lineData[baseIdx+1] = setCellValue(len(g2.Producers), m.SourceIdx, g2.Producers)
		}
	}
	return lineData
}

func addHeader(meta *model.RawSourceMeta, countCon, countProd int, value func(meta *model.CounterPointMeta) interface{}) []interface{} {
	lineData := make([]interface{}, len(meta.CounterPoints)*3)
	for _, m := range meta.CounterPoints {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			lineData[baseIdx] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+2] = excelize.Cell{Value: value(m)}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (countCon * 3) + (m.SourceIdx * 2)
			lineData[baseIdx] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(m)}
		}
	}
	return lineData
}

func addHeaderMeterCode(meta *model.RawSourceMeta, countCon, countProd int, value func(code MeterCodeType) interface{}) []interface{} {
	lineData := make([]interface{}, len(meta.CounterPoints)*3)
	for _, m := range meta.CounterPoints {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := m.SourceIdx * 3
			lineData[baseIdx] = excelize.Cell{Value: value(Total)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(Share)}
			lineData[baseIdx+2] = excelize.Cell{Value: value(Coverage)}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (countCon * 3) + (m.SourceIdx * 2)
			lineData[baseIdx] = excelize.Cell{Value: value(Total)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(Profit)}
		}
	}
	return lineData
}

func summaryCounterPoints(db *store.BowStorage, start, end time.Time, cps *ExportCPs) (*SummeryResult, error) {

	results, report, err := calculation.CalculateReport(db, start, end, calculation.CalculateEEG)
	if err != nil {
		return nil, err
	}
	eegModel := &model.EegEnergy{}
	eegModel.Results = append(eegModel.Results, results...)
	eegModel.Report = report

	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%d", 0)); err != nil {
		return nil, err
	} else {
		for _, m := range meta.CounterPoints {
			if m.Dir == "CONSUMPTION" || m.Dir == "GENERATION" {
				eegModel.Meta = append(eegModel.Meta, m)
			}
		}
	}

	summery := &SummeryResult{Consumer: []SummeryMeterResult{}, Producer: []SummeryMeterResult{}}
	for _, cp := range cps.Cps {
		m, err := findMeterMeta(eegModel.Meta, cp.MeteringPoint)
		if err != nil {
			continue
		}
		if cp.Direction == "CONSUMPTION" {
			summery.Consumer = append(summery.Consumer, SummeryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				DataOk:        true,
				Total:         report.Consumed[m.SourceIdx],
				Coverage:      report.Shared[m.SourceIdx],
				Share:         report.Allocated[m.SourceIdx],
			})
		} else {
			summery.Producer = append(summery.Producer, SummeryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				DataOk:        true,
				Total:         report.Produced[m.SourceIdx],
				Coverage:      report.Produced[m.SourceIdx] - report.Distributed[m.SourceIdx],
				Share:         report.Distributed[m.SourceIdx],
			})
		}
	}

	return summery, nil
}

func findMeterMeta(meta []*model.CounterPointMeta, meterId string) (*model.CounterPointMeta, error) {
	for i := range meta {
		if meta[i].Name == meterId {
			return meta[i], nil
		}
	}
	return nil, errors.New("metering Point not found in Metadata")
}

func sumMeterResult(s []SummeryMeterResult, elem func(e *SummeryMeterResult) float64) float64 {
	sum := 0.0
	for _, e := range s {
		sum = sum + elem(&e)
	}
	return sum
}
