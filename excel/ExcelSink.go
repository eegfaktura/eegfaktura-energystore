package excel

import (
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

type SummaryMeterResult struct {
	MeteringPoint string
	Name          string
	BeginDate     string
	EndDate       string
	DataOk        bool
	Total         float64
	Coverage      float64
	Share         float64
}

type SummaryResult struct {
	Consumer []SummaryMeterResult
	Producer []SummaryMeterResult
}

func returnFloatValue(array []float64, idx int) float64 {
	if idx < len(array) {
		return array[idx]
	}
	return 0
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
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local)

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

	if err = generateSummaryDataSheet(db, f, start, end, cps); err != nil {
		return nil, err
	}
	if err := generateEnergyDataSheetV2(db, f, start, end, cps.Cps); err != nil {
		return nil, err
	}

	_ = f.DeleteSheet("Sheet1")
	return f.WriteToBuffer()
}

func generateSummaryDataSheet(db store.IBowStorage, f *excelize.File, start, end time.Time, cps *ExportCPs) error {

	sheet := "Summary"
	counterpoints, err := summaryCounterPointsV2(db, start, end, cps)
	if err != nil {
		return err
	}

	_, err = f.NewSheet(sheet)
	if err != nil {
		return err
	}
	//index := f.NewSheet(sheet)
	//f.SetActiveSheet(index)

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

	sw, err := f.NewStreamWriter(sheet)
	if err != nil {
		return err
	}

	beginDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local)
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)

	_ = sw.SetColWidth(1, 1, float64(40))
	_ = sw.SetColWidth(2, 2, float64(35))
	_ = sw.SetColWidth(3, 4, float64(22))
	_ = sw.SetColWidth(5, 5, float64(15))
	_ = sw.SetColWidth(6, 10, float64(22))

	rowOpts := excelize.RowOpts{StyleID: styleIdRowSummary}
	err = sw.SetRow("A2",
		[]interface{}{excelize.Cell{Value: "Gemeinschafts-ID", StyleID: styleIdBold}, excelize.Cell{Value: cps.CommunityId}}, rowOpts)
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

func generateEnergyDataSheetV2(db store.IBowStorage, f *excelize.File, start, end time.Time, meters []InvestigatorCP) error {

	participantMeterMap := map[string]string{}
	for _, m := range meters {
		participantMeterMap[m.MeteringPoint] = m.Name
	}

	_metaMap, err := store.GetMetaMap(db)
	if err != nil {
		return err
	}

	//metaMap := store.MetaMapType{}
	metaCon := []*model.CounterPointMeta{}
	metaPro := []*model.CounterPointMeta{}
	for _, k := range meters {
		if v, ok := _metaMap[k.MeteringPoint]; ok {
			if v.Dir == model.CONSUMER_DIRECTION {
				metaCon = append(metaCon, v)
			} else {
				metaPro = append(metaPro, v)
			}
		}
	}

	meta := append(metaCon, metaPro...)
	//sort.Slice(meta, func(i, j int) bool {
	//	return meta[i].Dir < meta[j].Dir
	//})

	// Create a new sheet.
	_, err = f.NewSheet("Energiedaten")
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

	sYear, sMonth, sDay := start.Year(), int(start.Month()), start.Day()
	eYear, eMonth, eDay := end.Year(), int(end.Month()), end.Day()

	iterCP := db.GetLineRange("CP", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterCP.Close()

	var _lineG1 model.RawSourceLine
	g1Ok := iterCP.Next(&_lineG1)

	sw, err := f.NewStreamWriter("Energiedaten")
	if err != nil {
		return err
	}

	_ = sw.SetColWidth(1, 1, 30)
	_ = sw.SetColWidth(2, 1000, 25)

	//meta, _ := db.GetMeta(fmt.Sprintf("cpmeta/%s", "0"))
	countCons, countProd := utils.CountConsumerProducer(meta)

	_ = sw.SetRow("A2",
		append([]interface{}{excelize.Cell{Value: "MeteringpointID"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} { return m.Name })...))

	_ = sw.SetRow("A3",
		append([]interface{}{excelize.Cell{Value: "Name"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} {
				if p, ok := participantMeterMap[m.Name]; ok {
					return p
				}
				return "unknown"
			})...))

	_ = sw.SetRow("A4",
		append([]interface{}{excelize.Cell{Value: "Energy direction"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} { return m.Dir })...))

	_ = sw.SetRow("A5",
		append([]interface{}{excelize.Cell{Value: "Period start"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} {
				return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", sDay, sMonth, sYear)
			})...))

	_ = sw.SetRow("A6",
		append([]interface{}{excelize.Cell{Value: "Period end"}},
			addHeader(meta, countCons, countProd, func(m *model.CounterPointMeta) interface{} {
				return fmt.Sprintf("%.2d.%.2d.%.4d 00:00:00", eDay, eMonth, eYear)
			})...))

	_ = sw.SetRow("A7",
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
	lineNum := 0
	var pt *time.Time = nil
	for g1Ok {
		lineNum = lineNum + 1
		lineDate, t, err := utils.ConvertRowIdToTimeString("CP", _lineG1.Id)
		if err != nil {
			return err
		}

		if rowOk := utils.CheckTime(pt, t); !rowOk {
			diff := ((t.Unix() - pt.Unix()) / (60 * 15)) - 1
			if diff > 0 {
				//nTime := time.Unix(pt.Unix(), 0)
				//fmt.Printf("Time: %v\n", nTime)
				for i := int64(0); i < diff; i += 1 {
					nTime := pt.Add(time.Minute * time.Duration(15*(int(i)+1)))
					newId, _ := utils.ConvertUnixTimeToRowId("CP/", nTime)
					fillLine := model.MakeRawSourceLine(newId,
						countCons*3, countProd*2).Copy(countCons * 3)
					_ = sw.SetRow(fmt.Sprintf("A%d", lineNum+10),
						append([]interface{}{excelize.Cell{Value: utils.ConvertTimeToStringExcel(nTime)}},
							addLineV2(&fillLine, countCons, meta, stylesQoV)...))
					lineNum += 1
				}
			}
		}
		pt = t
		_ = sw.SetRow(fmt.Sprintf("A%d", lineNum+10),
			append([]interface{}{excelize.Cell{Value: lineDate}}, addLineV2(&_lineG1, countCons, meta, stylesQoV)...))

		g1Ok = iterCP.Next(&_lineG1)
	}

	_ = sw.Flush()

	err = f.SetColWidth("Energiedaten", "A", "A", float64(25.0))
	if err != nil {
		return err
	}

	return nil
}

func addLineV2(g1 *model.RawSourceLine, countCon int, meta []*model.CounterPointMeta, stylesQoV []int) []interface{} {

	lineData := make([]interface{}, len(meta)*3)
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

	cCnt := 0
	pCnt := 0
	for _, m := range meta {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := cCnt * 3
			cCnt += 1
			lineData[baseIdx] = setCellValue(len(g1.Consumers), m.SourceIdx*3, g1.Consumers, g1.QoVConsumers)
			lineData[baseIdx+1] = setCellValue(len(g1.Consumers), (m.SourceIdx*3)+1, g1.Consumers, g1.QoVConsumers)
			lineData[baseIdx+2] = setCellValue(len(g1.Consumers), (m.SourceIdx*3)+2, g1.Consumers, g1.QoVConsumers)
		} else if m.Dir == model.PRODUCER_DIRECTION {
			//baseIdx := (countCon * 3) + (m.SourceIdx * 2)
			baseIdx := (countCon * 3) + (pCnt * 2)
			pCnt += 1
			lineData[baseIdx] = setCellValue(len(g1.Producers), m.SourceIdx*2, g1.Producers, g1.QoVProducers) //excelize.Cell{Value: g1.Producers[m.SourceIdx]}
			lineData[baseIdx+1] = setCellValue(len(g1.Producers), (m.SourceIdx*2)+1, g1.Producers, g1.QoVProducers)
		}
	}
	return lineData
}

func addHeader(meta []*model.CounterPointMeta, countCon int, countProd int, value func(meta *model.CounterPointMeta) interface{}) []interface{} {
	cCnt := 0
	pCnt := 0
	lineData := make([]interface{}, len(meta)*3)
	for _, m := range meta {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := cCnt * 3
			cCnt += 1
			lineData[baseIdx] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+2] = excelize.Cell{Value: value(m)}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (countCon * 3) + (pCnt * 2)
			pCnt += 1
			lineData[baseIdx] = excelize.Cell{Value: value(m)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(m)}
		}
	}
	return lineData
}

func addHeaderMeterCode(meta []*model.CounterPointMeta, countCon int, countProd int, value func(code MeterCodeType) interface{}) []interface{} {
	cCnt := 0
	pCnt := 0
	lineData := make([]interface{}, len(meta)*3)
	for _, m := range meta {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := cCnt * 3
			cCnt += 1
			lineData[baseIdx] = excelize.Cell{Value: value(Total)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(Share)}
			lineData[baseIdx+2] = excelize.Cell{Value: value(Coverage)}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (countCon * 3) + (pCnt * 2)
			pCnt += 1
			lineData[baseIdx] = excelize.Cell{Value: value(Total)}
			lineData[baseIdx+1] = excelize.Cell{Value: value(Profit)}
		}
	}
	return lineData
}

func summaryCounterPointsV2(db store.IBowStorage, start, end time.Time, cps *ExportCPs) (*SummaryResult, error) {

	meta, info, err := store.GetMetaInfo(db)
	if err != nil {
		return nil, err
	}

	report, qovConsumer, qovProducer, err := sumEnergyOfPeriod(db, start, end, info)

	if err != nil {
		return nil, err
	}
	eegModel := &model.EegEnergy{}
	eegModel.Report = report

	for _, m := range meta {
		if m.Dir == "CONSUMPTION" || m.Dir == "GENERATION" {
			eegModel.Meta = append(eegModel.Meta, m)
		}
	}

	summary := &SummaryResult{Consumer: []SummaryMeterResult{}, Producer: []SummaryMeterResult{}}
	for _, cp := range cps.Cps {
		//m, err := findMeterMeta(eegModel.Meta, cp.MeteringPoint)
		//if err != nil {
		//	continue
		//}
		m, ok := meta[cp.MeteringPoint]
		if !ok {
			continue
		}
		if cp.Direction == "CONSUMPTION" {
			summary.Consumer = append(summary.Consumer, SummaryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				DataOk:        qovConsumer[m.SourceIdx],
				Total:         returnFloatValue(report.Consumed, m.SourceIdx),
				Coverage:      returnFloatValue(report.Shared, m.SourceIdx),
				Share:         returnFloatValue(report.Allocated, m.SourceIdx),
			})
		} else {
			summary.Producer = append(summary.Producer, SummaryMeterResult{
				MeteringPoint: cp.MeteringPoint,
				Name:          cp.Name,
				BeginDate:     m.PeriodStart,
				EndDate:       m.PeriodEnd,
				DataOk:        qovProducer[m.SourceIdx],
				Total:         returnFloatValue(report.Produced, m.SourceIdx),
				Coverage:      returnFloatValue(report.Produced, m.SourceIdx) - returnFloatValue(report.Distributed, m.SourceIdx),
				Share:         returnFloatValue(report.Distributed, m.SourceIdx),
			})
		}
	}

	return summary, nil
}

func sumEnergyOfPeriod(db store.IBowStorage, start, end time.Time, info *model.CounterPointMetaInfo) (*model.EnergyReport, []bool, []bool, error) {
	sYear, sMonth, sDay := start.Year(), int(start.Month()), start.Day()
	eYear, eMonth, eDay := end.Year(), int(end.Month()), end.Day()

	iterCP := db.GetLineRange("CP", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterCP.Close()

	var _lineG1 model.RawSourceLine
	g1Ok := iterCP.Next(&_lineG1)

	if !g1Ok {
		return nil, []bool{}, []bool{}, errors.New("no Rows found")
	}

	//consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(&_lineG1)
	report := &model.EnergyReport{
		Consumed:    make([]float64, info.ConsumerCount),
		Allocated:   make([]float64, info.ConsumerCount),
		Shared:      make([]float64, info.ConsumerCount),
		Produced:    make([]float64, info.ProducerCount),
		Distributed: make([]float64, info.ProducerCount),
	}

	qovConsumerSlice := model.CreateInitializedBoolSlice(info.ConsumerCount, true)
	qovProducerSlice := model.CreateInitializedBoolSlice(info.ProducerCount, true)
	for g1Ok {
		consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(&_lineG1)
		for i := 0; i < consumerMatrix.Rows; i += 1 {
			report.Consumed[i] += consumerMatrix.GetElm(i, 0)
			report.Shared[i] += consumerMatrix.GetElm(i, 1)
			report.Allocated[i] += consumerMatrix.GetElm(i, 2)
			if (i*3)+2 < len(_lineG1.QoVConsumers) {
				qovConsumerSlice[i] = qovConsumerSlice[i] && (_lineG1.QoVConsumers[(i*3)] == 1) && (_lineG1.QoVConsumers[(i*3)+1] == 1) && (_lineG1.QoVConsumers[(i*3)+2] == 1)
			}
		}
		for i := 0; i < producerMatrix.Rows; i += 1 {
			report.Produced[i] += producerMatrix.GetElm(i, 0)
			report.Distributed[i] += producerMatrix.GetElm(i, 1)
			if (i*2)+1 < len(_lineG1.QoVProducers) {
				qovProducerSlice[i] = qovProducerSlice[i] && (_lineG1.QoVProducers[(i*2)] == 1) && (_lineG1.QoVProducers[(i*2)+1] == 1)
			}
		}

		g1Ok = iterCP.Next(&_lineG1)
	}

	return report, qovConsumerSlice, qovProducerSlice, nil
}

func findMeterMeta(meta []*model.CounterPointMeta, meterId string) (*model.CounterPointMeta, error) {
	for i := range meta {
		if meta[i].Name == meterId {
			return meta[i], nil
		}
	}
	return nil, errors.New("metering Point not found in Metadata")
}

func sumMeterResult(s []SummaryMeterResult, elem func(e *SummaryMeterResult) float64) float64 {
	sum := 0.0
	for _, e := range s {
		sum = sum + elem(&e)
	}
	return utils.RoundFloat(sum, 6)
}
