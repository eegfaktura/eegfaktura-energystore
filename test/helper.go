package test

import (
	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/require"
	"testing"
)

func ImportTestContent(t *testing.T, file, sheet string, db *ebow.BowStorage) (yearSet []int) {
	excelFile, err := excel.OpenExceFile(file)
	require.NoError(t, err)
	defer excelFile.Close()

	err = excel.ImportExcelEnergyFileNew(excelFile, sheet, db)
	require.NoError(t, err)

	return
}
