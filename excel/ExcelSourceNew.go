package excel

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"
	"github.com/golang/glog"
	"github.com/xuri/excelize/v2"
	"math"
	"strings"
	"time"
)

func calcRawDataMatrixLen(a []float64, step int) int {
	l := len(a) - 1
	if l < 1 {
		return 1
	}
	return int(math.Ceil(float64(l) / float64(step)))
}

func ImportExcelEnergyFileNew(f *excelize.File, sheet string, db *store.BowStorage) error {

	exp := "DD.MM.YYYY HH:MM:SS"
	style, err := f.NewStyle(&excelize.Style{CustomNumFmt: &exp})
	err = f.SetCellStyle("Sheet1", "A12", "A15", style)

	rows, err := f.Rows(sheet)
	if err != nil {
		glog.Error(err)
		return err
	}
	glog.Infof("Rows Error: %+v", rows.Error())

	var rIdx int = 1
	var rawDatas []*model.RawSourceLine = []*model.RawSourceLine{}

	var excelHeader excelHeader
	excelHeaderInitialized := false

	var excelCpMeta map[int]*excelCounterPointMeta
	var updatedCpMeta []*model.CounterPointMeta
	var yearSet map[int]bool = make(map[int]bool)

	t := time.Now()
	totalRowCols := 0
	for rows.Next() {
		totalRowCols = totalRowCols + 1
		if cols, err := rows.Columns(excelize.Options{RawCellValue: true}); err == nil && len(cols) > 0 {
			switch cols[0] {
			case "MeteringpointID":
				excelHeader.meteringPointId = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.meteringPointId[i] = c
				}
			case "Spaltensumme", "Metering Interval", "Name", "MeteringReason", "Number of Metering Intervals":
				continue
			case "Energy direction":
				excelHeader.energyDirection = make(map[int]model.MeterDirection, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.energyDirection[i] = model.MeterDirection(c)
				}
			case "Period end":
				excelHeader.periodEnd = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.periodEnd[i] = excelDateToString(c)
				}
			case "Period start":
				excelHeader.periodStart = make(map[int]string, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.periodStart[i] = excelDateToString(c)
				}
			case "Metercode":
				excelHeader.meterCode = make(map[int]MeterCodeType, len(cols)-1)
				for i, c := range cols[1:] {
					excelHeader.meterCode[i] = returnMeterCode(strings.ToUpper(c))
				}
			default:
				if isDate(cols[0]) {
					d, m, y, hh, mm, ss := getExcelDate(cols[0])
					yearSet[y] = true
					if !excelHeaderInitialized {
						excelCpMeta, updatedCpMeta, err = buildMatrixMetaStruct(db, excelHeader)
						excelHeaderInitialized = true
					}

					//
					// Insert G1 values
					//
					rawData := &model.RawSourceLine{Consumers: []float64{}, Producers: []float64{}}
					rawData.Id = fmt.Sprintf("CP/%d/%.2d/%.2d/%.2d/%.2d/%.2d", y, m, d, hh, mm, ss)
					_ = db.GetLine(rawData)

					consumerMatrix := model.MakeMatrix(rawData.Consumers, calcRawDataMatrixLen(rawData.Consumers, 3), 3)
					producerMatrix := model.MakeMatrix(rawData.Producers, calcRawDataMatrixLen(rawData.Producers, 2), 2)
					for i := 0; i < len(excelCpMeta); i++ {
						v := excelCpMeta[i]
						switch v.Dir {
						case "CONSUMPTION":
							consumerMatrix.SetElm(v.SourceIdx, 0, returnMeterValue(cols, v.Idx))
							consumerMatrix.SetElm(v.SourceIdx, 1, returnMeterValue(cols, v.IdxG2))
							consumerMatrix.SetElm(v.SourceIdx, 2, returnMeterValue(cols, v.IdxG3))
							v.Count += 1
						case "GENERATION":
							producerMatrix.SetElm(v.SourceIdx, 0, returnMeterValue(cols, v.Idx))
							producerMatrix.SetElm(v.SourceIdx, 1, returnMeterValue(cols, v.IdxG2))
							v.Count += 1
						}
					}
					rawDatas = append(rawDatas, &model.RawSourceLine{Id: rawData.Id, Consumers: consumerMatrix.Elements, Producers: producerMatrix.Elements})
					rIdx += 1
				} else {
					s, e := f.GetCellStyle(sheet, cols[0])
					if err != nil {
						glog.Errorf("Error get cell format %+v", e)
					}
					glog.V(3).Infof("Could not handle row format (%d). Cols %+v <%v>", s, cols, cols[0])
				}
			}
		}
	}
	glog.Infof("Time taken via read file: %v (%d Rows)", time.Since(t), totalRowCols)
	if err := db.SetLines(rawDatas); err != nil {
		return err
	}

	glog.V(3).Infof("Import <%d> G1 lines", len(rawDatas))

	rawMeta := &model.RawSourceMeta{Id: fmt.Sprintf("cpmeta/%d", 0), CounterPoints: updatedCpMeta, NumberOfMetering: rIdx}
	err = db.SetMeta(rawMeta)

	glog.V(3).Infof("Update Metadata: %+v", rawMeta)

	if err != nil {
		glog.Error(err.Error())
		return err
	}
	glog.V(3).Infof("Time taken via write batch: %v", time.Since(t))

	years := []int{}
	for k, _ := range yearSet {
		years = append(years, k)
	}
	return nil
}
