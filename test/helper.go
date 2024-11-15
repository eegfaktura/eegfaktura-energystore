package test

import (
	"testing"

	"github.com/eegfaktura/eegfaktura-energystore/excel"
	"github.com/eegfaktura/eegfaktura-energystore/store"
	"github.com/stretchr/testify/require"
)

func ImportTestContent(t *testing.T, file, sheet string, db *store.BowStorage) (yearSet []int) {
	excelFile, err := excel.OpenExceFile(file)
	require.NoError(t, err)
	defer excelFile.Close()

	err = excel.ImportExcelEnergyFileNew(excelFile, sheet, db)
	require.NoError(t, err)

	return
}
