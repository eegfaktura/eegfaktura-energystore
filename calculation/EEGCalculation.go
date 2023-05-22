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
			Consumed:      cm.Elements,
			Allocated:     am.Elements,
			Produced:      pm.Elements,
			Distributed:   dm.Elements,
			Shared:        sm.Elements,
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
		Consumed:      report_cm.Elements,
		Allocated:     report_am.Elements,
		Produced:      report_pm.Elements,
		Distributed:   report_dm.Elements,
		Shared:        report_sm.Elements,
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
