package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"fmt"
	"time"
)

/*
Calculate allocation for.
Return:

	Allocation, Consumption, Produced, Distributed, Share

	Allocation: Energy value allocated for consumer
	Consumption: Energy value consumed by consumer
	Produced: Energy value Produced by generator
	Share: produced value divided to consumers.
*/
type CalcHandler func(*store.BowStorage, string) (*model.Matrix, *model.Matrix, *model.Matrix, *model.Matrix, *model.Matrix, float64)
type AllocationHandler func(*model.RawSourceLine) (*model.Matrix, *model.Matrix, *model.Matrix)

type calcResults struct {
	rAlloc *model.Matrix
	rCons  *model.Matrix
	rProd  *model.Matrix
	rDist  *model.Matrix
	rShar  *model.Matrix
	pSum   float64
}

func CalculateEEG(db *store.BowStorage, period string) (*model.Matrix, *model.Matrix, *model.Matrix, *model.Matrix, *model.Matrix, float64) {
	metaMap, err := store.GetConsumerMetaMap(db)
	if err != nil {
		return nil, nil, nil, nil, nil, 0
	}
	iter := db.GetLinePrefix(fmt.Sprintf("CP-G.01/%s", period))
	defer iter.Close()

	var _line model.RawSourceLine

	var rAlloc *model.Matrix
	var rCons *model.Matrix
	var rProd *model.Matrix
	var rDist *model.Matrix
	var rShar *model.Matrix
	var pSum float64 = 0.0
	defaultConsumerLen := len(metaMap)

	for iter.Next(&_line) {
		//line := transformConsumer(&_line)
		line := _line.Copy(defaultConsumerLen)
		m, s, p := AllocDynamic2(&line)

		if rCons == nil {
			rCons = model.MakeMatrix(line.Consumers, len(line.Consumers), 1)
		} else {
			rCons.Add(model.MakeMatrix(line.Consumers, len(line.Consumers), 1))
		}

		if rProd == nil {
			rProd = model.MakeMatrix(line.Producers, len(line.Producers), 1)
		} else {
			rProd.Add(model.MakeMatrix(line.Producers, len(line.Producers), 1))
		}

		if rAlloc == nil {
			rAlloc = model.MakeMatrix(m.Elements, m.CountRows(), m.CountCols())
		} else {
			rAlloc.Add(m)
		}

		if rDist == nil {
			rDist = model.MakeMatrix(p.Elements, p.CountRows(), p.CountCols())
		} else {
			rDist.Add(p)
		}

		if rShar == nil {
			rShar = model.MakeMatrix(s.Elements, s.CountRows(), s.CountCols())
		} else {
			rShar.Add(s)
		}
		pSum += utils.Sum(line.Producers)
	}
	return rAlloc, rCons, rProd, rDist, rShar, pSum
}

func appendResults(line *model.RawSourceLine, allocFunc AllocationHandler, results *calcResults) error {

	m, s, p := allocFunc(line)

	if results.rCons == nil {
		results.rCons = model.NewCopiedMatrixFromElements(line.Consumers, len(line.Consumers), 1)
	} else {
		results.rCons.Add(model.MakeMatrix(line.Consumers, len(line.Consumers), 1))
	}

	if results.rProd == nil {
		results.rProd = model.NewCopiedMatrixFromElements(line.Producers, len(line.Producers), 1)
	} else {
		results.rProd.Add(model.MakeMatrix(line.Producers, len(line.Producers), 1))
	}

	if results.rAlloc == nil {
		results.rAlloc = model.NewCopiedMatrixFromElements(m.Elements, m.CountRows(), m.CountCols())
	} else {
		results.rAlloc.Add(m)
	}

	if results.rDist == nil {
		results.rDist = model.NewCopiedMatrixFromElements(p.Elements, p.CountRows(), p.CountCols())
	} else {
		results.rDist.Add(p)
	}

	if results.rShar == nil {
		results.rShar = model.NewCopiedMatrixFromElements(s.Elements, s.CountRows(), s.CountCols())
	} else {
		results.rShar.Add(s)
	}
	results.pSum += utils.Sum(line.Producers)

	return nil

}

func ensureMatrix(matrix *model.Matrix, defaultLen int) *model.Matrix {
	if matrix == nil {
		return model.NewMatrix(defaultLen, 1)
	}
	return matrix
}

func sumIntermediate(
	intermediate calcResults,
	results *calcResults) error {
	if results.rCons == nil {
		results.rCons = model.NewCopiedMatrixFromElements(intermediate.rCons.Elements, len(intermediate.rCons.Elements), 1)
	} else {
		results.rCons.Add(intermediate.rCons)
	}

	if results.rProd == nil {
		results.rProd = model.NewCopiedMatrixFromElements(intermediate.rProd.Elements, intermediate.rProd.Rows, 1)
	} else {
		results.rProd.Add(intermediate.rProd)
	}

	if results.rAlloc == nil {
		results.rAlloc = model.NewCopiedMatrixFromElements(intermediate.rAlloc.Elements, intermediate.rAlloc.CountRows(), intermediate.rAlloc.CountCols())
	} else {
		results.rAlloc.Add(intermediate.rAlloc)
	}

	if results.rDist == nil {
		results.rDist = model.NewCopiedMatrixFromElements(intermediate.rDist.Elements, intermediate.rDist.CountRows(), intermediate.rDist.CountCols())
	} else {
		results.rDist.Add(intermediate.rDist)
	}

	if results.rShar == nil {
		results.rShar = model.NewCopiedMatrixFromElements(intermediate.rShar.Elements, intermediate.rShar.CountRows(), intermediate.rShar.CountCols())
	} else {
		results.rShar.Add(intermediate.rShar)
	}
	results.pSum += intermediate.pSum

	return nil
}

func calculateReport(iter *ebow.Iter,
	metaInfo *model.CounterPointMetaInfo,
	allocFunc AllocationHandler,
	reportId string,
	switchIntermediate func(rowId string) bool,
	intermediateId func() string) ([]*model.EnergyReport, *model.EnergyReport, error) {

	results := calcResults{}
	intermediate := calcResults{}
	defaultConsumerLen := metaInfo.ConsumerCount

	dayReports := []*model.EnergyReport{}

	var err error
	var _line model.RawSourceLine
	for iter.Next(&_line) {
		line := _line.Copy(defaultConsumerLen)

		if switchIntermediate(line.Id) {
			dayReports = append(dayReports, &model.EnergyReport{
				Id:            intermediateId(),
				Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
				Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
				Produced:      intermediate.rProd.RoundToFixed(6).Elements,
				Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
				Shared:        intermediate.rShar.RoundToFixed(6).Elements,
				TotalProduced: intermediate.pSum,
			})

			if err = sumIntermediate(intermediate, &results); err != nil {
				return []*model.EnergyReport{}, nil, err
			}

			intermediate = calcResults{}

		}

		//lineTime, err := utils.ConvertRowIdToTime(rowPrefix, line.Id)
		//if err != nil {
		//	return []*model.EnergyReport{}, nil, err
		//}
		//
		//if lineTime.Day() > cDay {
		//	dayReports = append(dayReports, &model.EnergyReport{
		//		Id:            fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, cDay),
		//		Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
		//		Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
		//		Produced:      intermediate.rProd.RoundToFixed(6).Elements,
		//		Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
		//		Shared:        intermediate.rShar.RoundToFixed(6).Elements,
		//		TotalProduced: intermediate.pSum,
		//	})
		//	cDay = lineTime.Day()
		//
		//	if err = sumIntermediate(intermediate, &results); err != nil {
		//		return []*model.EnergyReport{}, nil, err
		//	}
		//
		//	intermediate = calcResults{}
		//}

		if err := appendResults(&line, allocFunc, &intermediate); err != nil {
			return []*model.EnergyReport{}, nil, err
		}

	}

	dayReports = append(dayReports, &model.EnergyReport{
		Id:            intermediateId(),
		Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
		Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
		Produced:      intermediate.rProd.RoundToFixed(6).Elements,
		Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
		Shared:        intermediate.rShar.RoundToFixed(6).Elements,
		TotalProduced: intermediate.pSum,
	})

	if err = sumIntermediate(intermediate, &results); err != nil {
		return []*model.EnergyReport{}, nil, err
	}

	return dayReports, &model.EnergyReport{
		Id:            reportId,
		Consumed:      ensureMatrix(results.rCons, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
		Allocated:     ensureMatrix(results.rAlloc, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
		Produced:      ensureMatrix(results.rProd, metaInfo.ProducerCount).RoundToFixed(6).Elements,
		Distributed:   ensureMatrix(results.rDist, metaInfo.ProducerCount).RoundToFixed(6).Elements,
		Shared:        ensureMatrix(results.rShar, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
		TotalProduced: results.pSum}, nil
}

func CalculateMonthlyPeriod(db *store.BowStorage, allocFunc AllocationHandler, year, month int) ([]*model.EnergyReport, *model.EnergyReport, error) {
	rowPrefix := "CP-G.01"
	_, metaInfo, err := store.GetMetaInfo(db)
	if err != nil {
		return []*model.EnergyReport{}, nil, err
	}
	iter := db.GetLinePrefix(fmt.Sprintf("%s/%d/%.2d/", rowPrefix, year, month))
	defer iter.Close()

	cDay := 1
	return calculateReport(iter, metaInfo, allocFunc, fmt.Sprintf("MRP/%d/%.2d", year, month),
		func(rowId string) bool {
			lineTime, _ := utils.ConvertRowIdToTime(rowPrefix, rowId)
			shouldSwitch := lineTime.Day() > cDay
			cDay = lineTime.Day()
			return shouldSwitch
		},
		func() string {
			return fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, cDay-1)
		},
	)

	//results := calcResults{}
	//intermediate := calcResults{}
	//defaultConsumerLen := metaInfo.ConsumerCount
	//
	//dayReports := []*model.EnergyReport{}
	//
	//var _line model.RawSourceLine
	//cDay := 1
	//for iter.Next(&_line) {
	//	line := _line.Copy(defaultConsumerLen)
	//
	//	lineTime, err := utils.ConvertRowIdToTime(rowPrefix, line.Id)
	//	if err != nil {
	//		return []*model.EnergyReport{}, nil, err
	//	}
	//
	//	if lineTime.Day() > cDay {
	//		dayReports = append(dayReports, &model.EnergyReport{
	//			Id:            fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, cDay),
	//			Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
	//			Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
	//			Produced:      intermediate.rProd.RoundToFixed(6).Elements,
	//			Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
	//			Shared:        intermediate.rShar.RoundToFixed(6).Elements,
	//			TotalProduced: intermediate.pSum,
	//		})
	//		cDay = lineTime.Day()
	//
	//		if err = sumIntermediate(intermediate, &results); err != nil {
	//			return []*model.EnergyReport{}, nil, err
	//		}
	//
	//		intermediate = calcResults{}
	//	}
	//
	//	if err := appendResults(&line, allocFunc, &intermediate); err != nil {
	//		return []*model.EnergyReport{}, nil, err
	//	}
	//
	//}
	//
	//dayReports = append(dayReports, &model.EnergyReport{
	//	Id:            fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, cDay),
	//	Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
	//	Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
	//	Produced:      intermediate.rProd.RoundToFixed(6).Elements,
	//	Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
	//	Shared:        intermediate.rShar.RoundToFixed(6).Elements,
	//	TotalProduced: intermediate.pSum,
	//})
	//
	//if err = sumIntermediate(intermediate, &results); err != nil {
	//	return []*model.EnergyReport{}, nil, err
	//}
	//
	//return dayReports, &model.EnergyReport{
	//	Id:            fmt.Sprintf("MRP/%d/%.2d", year, month),
	//	Consumed:      ensureMatrix(results.rCons, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
	//	Allocated:     ensureMatrix(results.rAlloc, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
	//	Produced:      ensureMatrix(results.rProd, metaInfo.ProducerCount).RoundToFixed(6).Elements,
	//	Distributed:   ensureMatrix(results.rDist, metaInfo.ProducerCount).RoundToFixed(6).Elements,
	//	Shared:        ensureMatrix(results.rShar, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
	//	TotalProduced: results.pSum}, nil
}

//func CalculateMonthlyPeriod(db *store.BowStorage, allocFunc AllocationHandler, year, month int) ([]*model.EnergyReport, *model.EnergyReport, error) {
//	rowPrefix := "CP-G.01"
//	_, metaInfo, err := store.GetMetaInfo(db)
//	if err != nil {
//		return []*model.EnergyReport{}, nil, err
//	}
//	iter := db.GetLinePrefix(fmt.Sprintf("%s/%d/%.2d/", rowPrefix, year, month))
//	defer iter.Close()
//
//	results := calcResults{}
//	intermediate := calcResults{}
//	defaultConsumerLen := metaInfo.ConsumerCount
//
//	dayReports := []*model.EnergyReport{}
//
//	var _line model.RawSourceLine
//	cDay := 1
//	for iter.Next(&_line) {
//		line := _line.Copy(defaultConsumerLen)
//
//		lineTime, err := utils.ConvertRowIdToTime(rowPrefix, line.Id)
//		if err != nil {
//			return []*model.EnergyReport{}, nil, err
//		}
//
//		if lineTime.Day() > cDay {
//			dayReports = append(dayReports, &model.EnergyReport{
//				Id:            fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, cDay),
//				Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
//				Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
//				Produced:      intermediate.rProd.RoundToFixed(6).Elements,
//				Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
//				Shared:        intermediate.rShar.RoundToFixed(6).Elements,
//				TotalProduced: intermediate.pSum,
//			})
//			cDay = lineTime.Day()
//
//			if err = sumIntermediate(intermediate, &results); err != nil {
//				return []*model.EnergyReport{}, nil, err
//			}
//
//			intermediate = calcResults{}
//		}
//
//		if err := appendResults(&line, allocFunc, &intermediate); err != nil {
//			return []*model.EnergyReport{}, nil, err
//		}
//
//	}
//
//	dayReports = append(dayReports, &model.EnergyReport{
//		Id:            fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, cDay),
//		Consumed:      intermediate.rCons.RoundToFixed(6).Elements,
//		Allocated:     intermediate.rAlloc.RoundToFixed(6).Elements,
//		Produced:      intermediate.rProd.RoundToFixed(6).Elements,
//		Distributed:   intermediate.rDist.RoundToFixed(6).Elements,
//		Shared:        intermediate.rShar.RoundToFixed(6).Elements,
//		TotalProduced: intermediate.pSum,
//	})
//
//	if err = sumIntermediate(intermediate, &results); err != nil {
//		return []*model.EnergyReport{}, nil, err
//	}
//
//	return dayReports, &model.EnergyReport{
//		Id:            fmt.Sprintf("MRP/%d/%.2d", year, month),
//		Consumed:      ensureMatrix(results.rCons, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
//		Allocated:     ensureMatrix(results.rAlloc, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
//		Produced:      ensureMatrix(results.rProd, metaInfo.ProducerCount).RoundToFixed(6).Elements,
//		Distributed:   ensureMatrix(results.rDist, metaInfo.ProducerCount).RoundToFixed(6).Elements,
//		Shared:        ensureMatrix(results.rShar, metaInfo.ConsumerCount).RoundToFixed(6).Elements,
//		TotalProduced: results.pSum}, nil
//}

func CalculateMonthlyDash(db *store.BowStorage, year string, calc CalcHandler) error {
	mounth := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

	metaMap, err := store.GetConsumerMetaMap(db)
	if err != nil {
		return nil
	}
	var annual_am, annual_cm, annual_pm, annual_dm, annual_sm *model.Matrix = &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}
	var annual_ps float64 = 0.0

	defaultMatrix := model.NewMatrix(len(metaMap), 1)
	verifyResult := func(matrix *model.Matrix) *model.Matrix {
		if matrix == nil {
			return defaultMatrix
		} else {
			return matrix
		}
	}
	for _, m := range mounth {
		am, cm, pm, dm, sm, ps := calc(db, fmt.Sprintf("%s/%.2d/", year, m))
		am = verifyResult(am)
		cm = verifyResult(cm)
		pm = verifyResult(pm)
		dm = verifyResult(dm)
		sm = verifyResult(sm)

		if err := db.SetReport(
			&model.EnergyReport{
				Id:            fmt.Sprintf("MRP/%s/%.2d", year, m),
				Consumed:      cm.Elements,
				Allocated:     am.Elements,
				Produced:      pm.Elements,
				Distributed:   dm.Elements,
				Shared:        sm.Elements,
				TotalProduced: ps}); err != nil {
			return err
		}

		_ = annual_am.Add(am)
		_ = annual_cm.Add(cm)
		_ = annual_pm.Add(pm)
		_ = annual_dm.Add(dm)
		_ = annual_sm.Add(sm)

		annual_ps += ps
		fmt.Printf("AllocMonth (%d) %f - %+v\n", m, ps, am)
	}

	//fmt.Printf("Annual Alloc-Report: %+v\n", annual_am)

	if err := db.SetReport(
		&model.EnergyReport{
			Id:            fmt.Sprintf("YRP/%s", year),
			Consumed:      annual_cm.Elements,
			Allocated:     annual_am.Elements,
			Produced:      annual_pm.Elements,
			Distributed:   annual_dm.Elements,
			Shared:        annual_sm.Elements,
			TotalProduced: annual_ps}); err != nil {
		return err
	}

	return nil
}

func CalculateWeeklyReport(db *store.BowStorage, year, month int, calc CalcHandler) ([]*model.EnergyReport, *model.EnergyReport, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC)
	return CalculateReport(db, start, end, calc)
}

func CalculateReport(db *store.BowStorage, start, end time.Time, calc CalcHandler) ([]*model.EnergyReport, *model.EnergyReport, error) {
	var report_am, report_cm, report_pm, report_dm, report_sm *model.Matrix = &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}
	var report_ps float64 = 0.0

	metaMap, err := store.GetConsumerMetaMap(db)
	if err != nil {
		return nil, nil, err
	}
	defaultMatrix := model.NewMatrix(len(metaMap), 1)
	verifyResult := func(matrix *model.Matrix) *model.Matrix {
		if matrix == nil {
			return defaultMatrix
		} else {
			return matrix
		}
	}
	dayReports := []*model.EnergyReport{}

	for d := start; d.After(end) == false; d = d.AddDate(0, 0, 1) {
		year, month, day := d.Year(), d.Month(), d.Day()
		am, cm, pm, dm, sm, ps := calc(db, fmt.Sprintf("%d/%.2d/%.2d", year, month, day))

		am = verifyResult(am)
		cm = verifyResult(cm)
		pm = verifyResult(pm)
		dm = verifyResult(dm)
		sm = verifyResult(sm)

		dayReports = append(dayReports, &model.EnergyReport{
			Id:            fmt.Sprintf("WRP/%d/%.2d/%.2d", year, month, day),
			Consumed:      cm.RoundToFixed(6).Elements,
			Allocated:     am.RoundToFixed(6).Elements,
			Produced:      pm.RoundToFixed(6).Elements,
			Distributed:   dm.RoundToFixed(6).Elements,
			Shared:        sm.RoundToFixed(6).Elements,
			TotalProduced: ps,
		})

		_ = report_am.Add(am)
		_ = report_cm.Add(cm)
		_ = report_pm.Add(pm)
		_ = report_dm.Add(dm)
		_ = report_sm.Add(sm)

		report_ps += ps
	}

	return dayReports, &model.EnergyReport{
		Id:            fmt.Sprintf("MRP/%d/%.2d", start.Year(), start.Month()),
		Consumed:      report_cm.RoundToFixed(6).Elements,
		Allocated:     report_am.RoundToFixed(6).Elements,
		Produced:      report_pm.RoundToFixed(6).Elements,
		Distributed:   report_dm.RoundToFixed(6).Elements,
		Shared:        report_sm.RoundToFixed(6).Elements,
		TotalProduced: report_ps}, nil
}

func CalculateYearlyReport(db *store.BowStorage, year int, calc CalcHandler) ([]*model.EnergyReport, *model.EnergyReport, error) {
	monthReports := []*model.EnergyReport{}
	resAnnual := &model.EnergyReport{}

	var line model.EnergyReport = model.EnergyReport{}

	iter := db.GetLinePrefix(fmt.Sprintf("MRP/%d/", year))
	for iter.Next(&line) {
		monthReports = append(monthReports, &model.EnergyReport{
			Id:            line.Id,
			Consumed:      line.Consumed,
			Allocated:     line.Allocated,
			Produced:      line.Produced,
			Distributed:   line.Distributed,
			TotalProduced: line.TotalProduced,
		})
		line = model.EnergyReport{}
	}

	if annual, err := db.GetReport(fmt.Sprintf("YRP/%d", year)); err != nil {
		if err != ebow.ErrNotFound {
			return nil, nil, err
		}
	} else {
		resAnnual = annual
	}
	return monthReports, resAnnual, nil
}

func transformConsumer(line *model.RawSourceLine) *model.RawSourceLine {
	result := &model.RawSourceLine{Id: line.Id, Consumers: []float64{}, Producers: line.Producers}
	for i := 0; i < len(line.Consumers); i += 3 {
		result.Consumers = append(result.Consumers, line.Consumers[i])
	}
	return result
}

func CalculatePeriodReport(db *store.BowStorage, from, to time.Time, calc CalcHandler) (*model.EnergyReport, map[string]*model.CounterPointMeta, error) {

	var report_am, report_cm, report_pm, report_dm, report_sm *model.Matrix = &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}
	var report_ps float64 = 0

	year, months := utils.GetMonthDuration(from, to)
	verifyResult := func(matrix *model.Matrix, defaultMatrix *model.Matrix) *model.Matrix {
		if matrix == nil {
			return defaultMatrix
		} else {
			return matrix
		}
	}

	periodFrom := int(from.Month())
	totalMonth := months + periodFrom
	var periodTo int
	var metaMap map[string]*model.CounterPointMeta
	for {
		if totalMonth > 12 {
			periodTo = 12
		} else {
			periodTo = totalMonth
		}

		cm, am, pm, dm, sm, prodSum, meta, err := CalculatePeriodWithinYearReport(db, year, periodFrom, periodTo, calc)
		if err != nil {
			return nil, nil, err
		}

		if len(meta) > 0 {

			defaultMatrix := model.NewMatrix(len(metaMap), 1)

			am = verifyResult(am, defaultMatrix)
			cm = verifyResult(cm, defaultMatrix)
			pm = verifyResult(pm, defaultMatrix)
			dm = verifyResult(dm, defaultMatrix)
			sm = verifyResult(sm, defaultMatrix)

			if am.Rows > 0 && am.Cols == defaultMatrix.Cols {
				_ = report_am.Add(am)
			}
			if cm.Rows > 0 && cm.Cols == defaultMatrix.Cols {
				_ = report_cm.Add(cm)
			}
			if pm.Rows > 0 && pm.Cols == defaultMatrix.Cols {
				_ = report_pm.Add(pm)
			}
			if dm.Rows > 0 && dm.Cols == defaultMatrix.Cols {
				_ = report_dm.Add(dm)
			}
			if sm.Rows > 0 && sm.Cols == defaultMatrix.Cols {
				_ = report_sm.Add(sm)
			}

			report_ps += prodSum
			metaMap = meta
		}

		totalMonth -= 12
		if totalMonth <= 0 {
			break
		}

		year += 1
		periodFrom = 1
	}

	return &model.EnergyReport{
		Id:            fmt.Sprintf("PRP/%d/%.2d", year, int(from.Month())),
		Consumed:      report_cm.Elements,
		Allocated:     report_am.Elements,
		Distributed:   report_dm.Elements,
		TotalProduced: report_ps}, metaMap, nil
}

func CalculatePeriodWithinYearReport(db *store.BowStorage, year, from, to int, calc CalcHandler) (*model.Matrix, *model.Matrix, *model.Matrix, *model.Matrix, *model.Matrix, float64, map[string]*model.CounterPointMeta, error) {
	var report_am, report_cm, report_pm, report_dm, report_sm *model.Matrix = &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}, &model.Matrix{}
	var report_ps float64 = 0.0

	metaMap, err := store.GetConsumerMetaMap(db)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, nil, err
	}
	defaultMatrix := model.NewMatrix(len(metaMap), 1)
	verifyResult := func(matrix *model.Matrix) *model.Matrix {
		if matrix == nil {
			return defaultMatrix
		} else {
			return matrix
		}
	}

	for m := from; m <= to; m++ {
		am, cm, pm, dm, sm, ps := calc(db, fmt.Sprintf("%d/%.2d/", year, m))

		am = verifyResult(am)
		cm = verifyResult(cm)
		pm = verifyResult(pm)
		dm = verifyResult(dm)
		sm = verifyResult(sm)

		_ = report_am.Add(am)
		_ = report_cm.Add(cm)
		_ = report_pm.Add(pm)
		_ = report_dm.Add(dm)
		_ = report_sm.Add(sm)

		report_ps += ps
	}

	return report_cm, report_am, report_pm, report_dm, report_sm, report_ps, metaMap, nil
}
