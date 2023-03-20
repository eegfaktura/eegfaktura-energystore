package test

import (
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/store"
	"github.com/stretchr/testify/require"
	"testing"
)

func ImportTestContent(t *testing.T, file, sheet string, db *store.BowStorage) (yearSet []int) {
	excelFile, err := excel.OpenExceFile(file)
	require.NoError(t, err)
	defer excelFile.Close()

	yearSet, err = excel.ImportExcelEnergyFile(excelFile, sheet, db)
	require.NoError(t, err)

	return
}
