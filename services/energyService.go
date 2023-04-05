package services

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"fmt"
	"time"
)

func GetLastEnergyEntry(tenant string) (string, error) {
	var err error
	var meta *model.RawSourceMeta

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return "", err
	}
	defer func() { db.Close() }()

	meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%s", "0"))
	if err != nil {
		return "", err
	}

	endDate := time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
	for _, mcp := range meta.CounterPoints {
		dcp := utils.StringToTime(mcp.PeriodEnd)
		if dcp.After(endDate) {
			endDate = dcp
		}
	}
	return utils.DateToString(endDate), nil
}
