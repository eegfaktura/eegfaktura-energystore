package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"bytes"
	"errors"
	"fmt"
	"github.com/golang/glog"
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

type ExportParticipantEnergy struct {
	Start       int64           `json:"start"`
	End         int64           `json:"end"`
	CommunityId string          `json:"communityId"`
	Cps         []ParticipantCp `json:"cps"`
}

type ParticipantCp struct {
	MeteringPoint string                  `json:"meteringPoint"`
	Direction     model.MeterDirection    `json:"direction"`
	ActiveSince   int64                   `json:"activeSince"`
	InactiveSince int64                   `json:"inactiveSince"`
	Name          string                  `json:"name"`
	Report        model.EnergyDescription `json:"report"`
	QoV           bool                    `json:"qov"`
	QoVSum        [3]bool                 `json:"qoVSum,omitempty"`
}

type SummaryMeterResult struct {
	MeteringPoint string
	Name          string
	BeginDate     string
	EndDate       string
	ActivePeriod  string
	DataOk        bool
	DataL0        bool
	DataL2        bool
	DataL3        bool
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

func ExportEnergyDataToMail(tenant, ecid, to string, year, month int, cps *ExportParticipantEnergy) error {

	buf, err := ExportExcel(tenant, ecid, year, month, cps)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-Energie Report-%d%.2d.xlsx", tenant, year, month)
	return utils.SendMail(tenant, to, fmt.Sprintf("EEG (%s) - Excel Report", tenant), nil, &filename, buf)
}

func ExportExcel(tenant, ecid string, year, month int, cps *ExportParticipantEnergy) (*bytes.Buffer, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local)

	//return CreateExcelFile(tenant, start, end, cps)
	return ExportEnergyToExcel(tenant, ecid, start, end, cps)
}

type periodRange struct {
	start time.Time
	end   time.Time
}

type Sheet interface {
	initSheet(ctx *RunnerContext) error
	handleLine(ctx *RunnerContext, line *model.RawSourceLine) error
	closeSheet(ctx *RunnerContext) error
}

type RunnerContext struct {
	start       time.Time
	end         time.Time
	communityId string
	cps         []*ParticipantCp
	producers   []*ParticipantCp
	consumers   []*ParticipantCp
	orderedCps  []*ParticipantCp
	metaMap     map[string]*model.CounterPointMeta
	//meta            []*model.CounterPointMeta
	info            *model.CounterPointMetaInfo
	countCons       int
	countProd       int
	periodsConsumer map[int]periodRange
	periodsProducer map[int]periodRange
	qovLogArray     []model.RawSourceLine
	checkBegin      func(lineDate, mDate time.Time) bool
}

func createRunnerContext(db ebow.IBowStorage, start, end time.Time, cps *ExportParticipantEnergy) (*RunnerContext, error) {
	metaMap, info, err := store.GetMetaInfo(db)
	if err != nil {
		return nil, err
	}

	metaRangeConsumer := map[int]periodRange{}
	metaRangeProducer := map[int]periodRange{}
	for _, v := range metaMap {
		ts, _ := utils.ParseTime(v.PeriodStart, 0)
		te, _ := utils.ParseTime(v.PeriodEnd, 0)
		if v.Dir == model.CONSUMER_DIRECTION {
			metaRangeConsumer[v.SourceIdx] = periodRange{start: ts, end: te}
		} else {
			metaRangeProducer[v.SourceIdx] = periodRange{start: ts, end: te}
		}
	}

	//metaCon := []*model.CounterPointMeta{}
	//metaPro := []*model.CounterPointMeta{}
	//for _, k := range cps.Cps {
	//	if v, ok := metaMap[k.MeteringPoint]; ok {
	//		if v.Dir == model.CONSUMER_DIRECTION {
	//			metaCon = append(metaCon, v)
	//		} else {
	//			metaPro = append(metaPro, v)
	//		}
	//	}
	//}
	//meta := append(metaCon, metaPro...)
	//countCons, countProd := utils.CountConsumerProducer(meta)

	var _cps []*ParticipantCp
	var producers []*ParticipantCp
	var consumers []*ParticipantCp
	for i, _ := range cps.Cps {
		if cps.Cps[i].Direction == model.PRODUCER_DIRECTION {
			producers = append(producers, &cps.Cps[i])
		} else {
			consumers = append(consumers, &cps.Cps[i])
		}
		if _, ok := metaMap[cps.Cps[i].MeteringPoint]; ok {
			cps.Cps[i].QoV = true
		}
		_cps = append(_cps, &cps.Cps[i])
	}

	return &RunnerContext{
		start:       start,
		end:         end,
		cps:         _cps,
		orderedCps:  append(consumers, producers...),
		communityId: cps.CommunityId,
		consumers:   consumers,
		producers:   producers,
		metaMap:     metaMap,
		//meta:            meta,
		info:            info,
		countProd:       len(producers),
		countCons:       len(consumers),
		periodsConsumer: metaRangeConsumer,
		periodsProducer: metaRangeProducer,
		checkBegin: func(lineDate, mDate time.Time) bool {
			if lineDate.Before(mDate) {
				return true
			}
			return false
		},
	}, nil
}

func (c *RunnerContext) getPeriodRange(m *model.CounterPointMeta) periodRange {
	if m.Dir == model.CONSUMER_DIRECTION {
		return c.periodsConsumer[m.SourceIdx]
	}
	return c.periodsProducer[m.SourceIdx]
}

type EnergyRunner struct {
	sheets []Sheet
}

func NewEnergyRunner(sheets []Sheet) *EnergyRunner {
	return &EnergyRunner{sheets: sheets}
}

func (er *EnergyRunner) initSheets(ctx *RunnerContext) error {
	for _, s := range er.sheets {
		if err := s.initSheet(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (er *EnergyRunner) handleLine(ctx *RunnerContext, line *model.RawSourceLine) error {
	for _, s := range er.sheets {
		if err := s.handleLine(ctx, line); err != nil {
			return err
		}
	}
	return nil
}

func (er *EnergyRunner) closeSheets(ctx *RunnerContext) error {
	for _, s := range er.sheets {
		if err := s.closeSheet(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (er *EnergyRunner) run(db ebow.IBowStorage, f *excelize.File, start, end time.Time, cps *ExportParticipantEnergy) (*bytes.Buffer, error) {
	rCxt, err := createRunnerContext(db, start, end, cps)
	if err != nil {
		return nil, err
	}
	if err = er.initSheets(rCxt); err != nil {
		return nil, err
	}

	sYear, sMonth, sDay := start.Year(), int(start.Month()), start.Day()
	eYear, eMonth, eDay := end.Year(), int(end.Month()), end.Day()

	iterCP := db.GetLineRange("CP", fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iterCP.Close()

	var _lineG1 model.RawSourceLine
	g1Ok := iterCP.Next(&_lineG1)

	if !g1Ok {
		return nil, model.ErrNoEntries(errors.New("no Rows found"))
	}

	sm := time.Now()
	var pt *time.Time = nil
	for g1Ok {
		_, t, err := utils.ConvertRowIdToTimeString("CP", _lineG1.Id, time.UTC)
		if rowOk := utils.CheckTime(pt, t); !rowOk {
			diff := ((t.Unix() - pt.Unix()) / (60 * 15)) - 1
			if diff > 0 {
				for i := int64(0); i < diff; i += 1 {
					nTime := pt.Add(time.Minute * time.Duration(15*(int(i)+1)))
					newId, _ := utils.ConvertUnixTimeToRowId("CP/", nTime)
					fillLine := model.MakeRawSourceLine(newId,
						rCxt.countCons*3, rCxt.countProd*2).Copy(rCxt.countCons * 3)
					if err = er.handleLine(rCxt, &fillLine); err != nil {
						return nil, err
					}
				}
			}
		}
		ct := time.Unix(t.Unix(), 0).UTC()
		pt = &ct

		if err = er.handleLine(rCxt, &_lineG1); err != nil {
			return nil, err
		}
		g1Ok = iterCP.Next(&_lineG1)
	}

	if err = er.closeSheets(rCxt); err != nil {
		return nil, err
	}

	if rCxt.qovLogArray != nil && len(rCxt.qovLogArray) > 0 {
		if err = generateLogDataSheet(rCxt, f); err != nil {
			glog.Infof("LOG: %+v\n", err)
		}
	}

	glog.V(5).Infof("Export Energy Data took %v (%s)", time.Since(sm).Seconds(), cps.CommunityId)

	_ = f.DeleteSheet("Sheet1")
	return f.WriteToBuffer()
}

func ExportEnergyToExcel(tenant, ecid string, start, end time.Time, cps *ExportParticipantEnergy) (*bytes.Buffer, error) {
	db, err := ebow.OpenStorage(tenant, ecid)
	if err != nil {
		return nil, err
	}
	defer func() { db.Close() }()

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			glog.Errorf("tenant=%s err: %v", tenant, err)
		}
	}()

	runner := NewEnergyRunner([]Sheet{
		&SummarySheet{name: "Summary", excel: f},
		&EnergySheet{name: "Energiedaten", excel: f},
	})
	return runner.run(db, f, start, end, cps)
}

func addLine(ctx *RunnerContext, line *model.RawSourceLine, stylesQoV []int) []interface{} {
	lineDate, _ := utils.ConvertRowIdToTime("CP", line.Id)
	consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(line)
	setCellValue1 := func(row, col int, matrix *model.Matrix, qov int) excelize.Cell {
		_qov := qov
		if _qov == 1 {
			return excelize.Cell{Value: utils.RoundToFixed(matrix.GetElm(row, col), 6), StyleID: stylesQoV[0]}
		} else if _qov == 2 {
			return excelize.Cell{Value: utils.RoundToFixed(matrix.GetElm(row, col), 6), StyleID: stylesQoV[1]}
		} else if _qov == 3 {
			return excelize.Cell{Value: utils.RoundToFixed(matrix.GetElm(row, col), 6), StyleID: stylesQoV[2]}
		} else {
			//fmt.Printf("Quality of Value is %d Value: %f\n", _qov, utils.RoundToFixed(raw[sourceIdx], 6))
			return excelize.Cell{Value: ""}
		}
	}

	participantCells := func(p *ParticipantCp, m *model.CounterPointMeta) []interface{} {
		if p.Direction == model.CONSUMER_DIRECTION {
			if utils.IsLineDateOutOfRange(lineDate, [2]int64{p.ActiveSince, p.InactiveSince}) {
				//if lineDate.Before(time.UnixMilli(p.ActiveSince)) || lineDate.After(time.UnixMilli(p.InactiveSince)) {
				return []interface{}{
					excelize.Cell{Value: ""},
					excelize.Cell{Value: ""},
					excelize.Cell{Value: ""},
				}
			}
			return []interface{}{
				setCellValue1(m.SourceIdx, 0, consumerMatrix, utils.GetInt(line.QoVConsumers, (m.SourceIdx*3)+0)),
				setCellValue1(m.SourceIdx, 1, consumerMatrix, utils.GetInt(line.QoVConsumers, (m.SourceIdx*3)+1)),
				setCellValue1(m.SourceIdx, 2, consumerMatrix, utils.GetInt(line.QoVConsumers, (m.SourceIdx*3)+2)),
			}
		} else {
			if utils.IsLineDateOutOfRange(lineDate, [2]int64{p.ActiveSince, p.InactiveSince}) {
				//if lineDate.Before(time.UnixMilli(p.ActiveSince)) || lineDate.After(time.UnixMilli(p.InactiveSince)) {
				return []interface{}{
					excelize.Cell{Value: ""},
					excelize.Cell{Value: ""},
				}
			}
			return []interface{}{
				setCellValue1(m.SourceIdx, 0, producerMatrix, utils.GetInt(line.QoVProducers, (m.SourceIdx*2)+0)),
				setCellValue1(m.SourceIdx, 1, producerMatrix, utils.GetInt(line.QoVProducers, (m.SourceIdx*2)+1)),
			}
		}
	}

	lines := []interface{}{}
	for i := 0; i < len(ctx.orderedCps); i++ {
		p := ctx.orderedCps[i]
		lines = append(lines, participantCells(ctx.orderedCps[i], ctx.metaMap[p.MeteringPoint])...)
	}
	return lines
}

func addHeaderV2(ctx *RunnerContext, cellCon, cellProd int,
	value func(meta *model.CounterPointMeta, p *ParticipantCp, cellOffset int) interface{},
	style func(meta *model.CounterPointMeta, p *ParticipantCp, cellOffset int) int) []interface{} {
	cCnt := 0
	pCnt := 0
	lineData := make([]interface{}, (ctx.countCons*cellCon)+(ctx.countProd*cellProd))
	for _, cp := range ctx.orderedCps {
		m := ctx.metaMap[cp.MeteringPoint]
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := cCnt * cellCon
			cCnt += 1
			for i := 0; i < cellCon; i++ {
				lineData[baseIdx+i] = excelize.Cell{Value: value(m, cp, i), StyleID: style(m, cp, i)}
			}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (ctx.countCons * cellCon) + (pCnt * cellProd)
			pCnt += 1
			for i := 0; i < cellProd; i++ {
				lineData[baseIdx+i] = excelize.Cell{Value: value(m, cp, i), StyleID: style(m, cp, i)}
			}
		}
	}
	return lineData
}
