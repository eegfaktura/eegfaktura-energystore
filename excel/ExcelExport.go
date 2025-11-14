package excel

import (
	"bytes"
	"errors"
	"time"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/golang/glog"
	"github.com/xuri/excelize/v2"
)

type ExportContext struct {
	communityId string
	cps         []*ParticipantCp
	producers   []*ParticipantCp
	consumers   []*ParticipantCp
	orderedCps  []*ParticipantCp
	qovLogArray []model.RawSourceLine
}

//func createExportContext(cps *ExportParticipantEnergy) (*ExportContext, error) {
//
//	metaRangeConsumer := map[int]periodRange{}
//	metaRangeProducer := map[int]periodRange{}
//
//	var _cps []*ParticipantCp
//	var producers []*ParticipantCp
//	var consumers []*ParticipantCp
//
//	for i, _ := range cps.Cps {
//		if cps.Cps[i].Direction == model.PRODUCER_DIRECTION {
//			producers = append(producers, &cps.Cps[i])
//		} else {
//			consumers = append(consumers, &cps.Cps[i])
//		}
//		if _, ok := metaMap[cps.Cps[i].MeteringPoint]; ok {
//			cps.Cps[i].QoV = true
//		}
//		_cps = append(_cps, &cps.Cps[i])
//	}
//
//	return &ExportContext{
//		cps:         _cps,
//		orderedCps:  append(consumers, producers...),
//		communityId: cps.CommunityId,
//		consumers:   consumers,
//		producers:   producers,
//	}, nil
//}

func createExportContext(cps *ExportParticipantEnergy) (*RunnerContext, error) {

	var _cps []*ParticipantCp
	var producers []*ParticipantCp
	var consumers []*ParticipantCp

	for i, _ := range cps.Cps {
		if cps.Cps[i].Direction == model.PRODUCER_DIRECTION {
			producers = append(producers, &cps.Cps[i])
		} else {
			consumers = append(consumers, &cps.Cps[i])
		}
		//if _, ok := metaMap[cps.Cps[i].MeteringPoint]; ok {
		//	cps.Cps[i].QoV = true
		//}
		_cps = append(_cps, &cps.Cps[i])
	}

	return &RunnerContext{
		cps:         _cps,
		orderedCps:  append(consumers, producers...),
		communityId: cps.CommunityId,
		consumers:   consumers,
		producers:   producers,
	}, nil
}

type ExcelExport struct {
	exportCtx *RunnerContext
	sheets    []Sheet
}

func NewExcelExport(sheets []Sheet) *ExcelExport {
	return &ExcelExport{sheets: sheets}
}

func (ee *ExcelExport) HandleStart(ctx *store.EngineContext) error {
	for _, sheet := range ee.sheets {
		if err := sheet.initSheet(ee.exportCtx); err != nil {
			return err
		}
	}
	return nil
}

func (ee *ExcelExport) HandleLine(ctx *store.EngineContext, line *model.RawSourceLine) error {
	for _, sheet := range ee.sheets {
		if err := sheet.handleLine(ee.exportCtx, line); err != nil {
			return err
		}
	}
	return nil
}

func (ee *ExcelExport) HandleEnd(ctx *store.EngineContext) error {
	for _, sheet := range ee.sheets {
		if err := sheet.closeSheet(ee.exportCtx); err != nil {
			return err
		}
	}
	return nil
}

func (ee *ExcelExport) Export(tenant, ecid string, start, end time.Time, cps *ExportParticipantEnergy, f *excelize.File) error {

	var err error
	ee.exportCtx, err = createExportContext(cps)
	if err != nil {
		return err
	}

	sm := time.Now()

	e := &store.Engine{Consumer: ee}
	if err := e.Query(tenant, ecid, start, end); err != nil && !errors.Is(err, ebow.ErrNoRows) {
		return err
	}

	if ee.exportCtx.qovLogArray != nil && len(ee.exportCtx.qovLogArray) > 0 {
		if err = generateLogDataSheet(ee.exportCtx, f); err != nil {
			glog.Infof("LOG: %+v\n", err)
		}
	}

	glog.V(5).Infof("Export Energy Data took %v (%s)", time.Since(sm).Seconds(), cps.CommunityId)

	return nil
}

func ExportToExcel(tenant, ecid string, start, end time.Time, cps *ExportParticipantEnergy) (*bytes.Buffer, error) {

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			glog.Errorf("tenant=%s err: %v", tenant, err)
		}
	}()

	runner := NewExcelExport([]Sheet{
		&SummarySheet{name: "Summary", excel: f},
		&EnergySheet{name: "Energiedaten", excel: f},
	})

	if err := runner.Export(tenant, ecid, start, end, cps, f); err != nil {
		return nil, err
	}

	_ = f.DeleteSheet("Sheet1")
	return f.WriteToBuffer()
}
