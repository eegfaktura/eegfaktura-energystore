package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"
)

//const StorageException = errors.New()

func EnergyDashboard(tenant, function string, year, month int) (*model.EegEnergy, error) {
	var err error

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return nil, err
	}
	defer func() { db.Close() }()

	var eegModel *model.EegEnergy
	var results []*model.EnergyReport
	var report *model.EnergyReport

	calcF := GetCalcFunc(function)
	if calcF == nil {
		calcF = CalculateEEG
	}

	if month > 0 {
		results, report, err = CalculateWeeklyReport(db, year, month, calcF)
	} else {
		results, report, err = CalculateYearlyReport(db, year, calcF)
	}

	if err != nil {
		return nil, err
	}

	eegModel = &model.EegEnergy{}
	eegModel.Results = append(eegModel.Results, results...)
	eegModel.Report = report

	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%d", 0)); err != nil {
		return nil, err
	} else {
		//metaMap := map[int]*model.CounterPointMeta{}
		for _, m := range meta.CounterPoints {
			fmt.Printf("Meta: %+v\n", m)
			if m.Dir == "CONSUMPTION" || m.Dir == "GENERATION" {
				eegModel.Meta = append(eegModel.Meta, m)
			} else {
				fmt.Printf("Omitted Meta: %+v\n", m)
			}
		}
	}

	return eegModel, nil
}
