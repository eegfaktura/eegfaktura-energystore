package services

import (
	"fmt"
	"time"

	"github.com/eegfaktura/eegfaktura-energystore/model"
	"github.com/eegfaktura/eegfaktura-energystore/store"
	"github.com/eegfaktura/eegfaktura-energystore/utils"
)

func GetLastEnergyEntry(tenant, ecid string) (string, error) {
	var err error
	var meta *model.RawSourceMeta

	db, err := store.OpenStorage(tenant, ecid)
	if err != nil {
		return "", err
	}
	defer func() { db.Close() }()

	meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%s", "0"))
	if err != nil {
		return "", err
	}

	endDate := time.Date(0, 0, 0, 0, 0, 0, 0, time.Local)
	for _, mcp := range meta.CounterPoints {
		dcp := utils.StringToTime(mcp.PeriodEnd)
		if dcp.After(endDate) {
			endDate = dcp
		}
	}
	return utils.DateToString(endDate), nil
}
