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

	//return CreateExcelFile(tenant, start, end, cps)
	return ExportEnergyToExcel(tenant, start, end, cps)
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
	start           time.Time
	end             time.Time
	cps             *ExportCPs
	metaMap         map[string]*model.CounterPointMeta
	meta            []*model.CounterPointMeta
	info            *model.CounterPointMetaInfo
	countCons       int
	countProd       int
	periodsConsumer map[int]periodRange
	periodsProducer map[int]periodRange
	qovLogArray     []model.RawSourceLine
	checkBegin      func(lineDate, mDate time.Time) bool
}

func createRunnerContext(db store.IBowStorage, start, end time.Time, cps *ExportCPs) (*RunnerContext, error) {
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

	metaCon := []*model.CounterPointMeta{}
	metaPro := []*model.CounterPointMeta{}
	for _, k := range cps.Cps {
		if v, ok := metaMap[k.MeteringPoint]; ok {
			if v.Dir == model.CONSUMER_DIRECTION {
				metaCon = append(metaCon, v)
			} else {
				metaPro = append(metaPro, v)
			}
		}
	}
	meta := append(metaCon, metaPro...)
	countCons, countProd := utils.CountConsumerProducer(meta)

	return &RunnerContext{
		start:           start,
		end:             end,
		cps:             cps,
		metaMap:         metaMap,
		meta:            meta,
		info:            info,
		countProd:       countProd,
		countCons:       countCons,
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

func (er *EnergyRunner) run(db store.IBowStorage, f *excelize.File, start, end time.Time, cps *ExportCPs) (*bytes.Buffer, error) {
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
		return nil, errors.New("no Rows found")
	}

	var pt *time.Time = nil
	for g1Ok {
		_, t, err := utils.ConvertRowIdToTimeString("CP", _lineG1.Id)
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
		pt = t

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
			fmt.Printf("LOG: %+v\n", err)
		}
	}

	_ = f.DeleteSheet("Sheet1")
	return f.WriteToBuffer()
}

func ExportEnergyToExcel(tenant string, start, end time.Time, cps *ExportCPs) (*bytes.Buffer, error) {
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

	runner := NewEnergyRunner([]Sheet{
		&SummarySheet{name: "Summary", excel: f},
		&EnergySheet{name: "Energiedaten", excel: f},
	})

	return runner.run(db, f, start, end, cps)
}

func addLine(g1 *model.RawSourceLine, countCon int, meta []*model.CounterPointMeta, stylesQoV []int) []interface{} {

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

func addHeaderV2(ctx *RunnerContext, cellCon, cellProd int,
	value func(meta *model.CounterPointMeta, cellOffset int) interface{},
	style func(meta *model.CounterPointMeta, cellOffset int) int) []interface{} {
	cCnt := 0
	pCnt := 0
	lineData := make([]interface{}, (ctx.countCons*cellCon)+(ctx.countProd*cellProd))
	for _, m := range ctx.meta {
		if m.Dir == model.CONSUMER_DIRECTION {
			baseIdx := cCnt * cellCon
			cCnt += 1
			for i := 0; i < cellCon; i++ {
				lineData[baseIdx+i] = excelize.Cell{Value: value(m, i), StyleID: style(m, i)}
			}
		} else if m.Dir == model.PRODUCER_DIRECTION {
			baseIdx := (ctx.countCons * cellCon) + (pCnt * cellProd)
			pCnt += 1
			for i := 0; i < cellProd; i++ {
				lineData[baseIdx+i] = excelize.Cell{Value: value(m, i), StyleID: style(m, i)}
			}
		}
	}
	return lineData
}
