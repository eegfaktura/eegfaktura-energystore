package excel

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// skippedRowWorkbook has one row the importer cannot handle (not a header, not a date) in A3, with
// a style of its own; rows 1 and 2 are empty.
func skippedRowWorkbook(t *testing.T) (*excelize.File, int) {
	t.Helper()
	f := excelize.NewFile()
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.SetCellValue("Sheet1", "A3", "unknown row"))
	require.NoError(t, f.SetCellValue("Sheet1", "B3", 1.5))
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Italic: true}})
	require.NoError(t, err)
	require.NoError(t, f.SetCellStyle("Sheet1", "A3", "A3", style))
	return f, style
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	require.NoError(t, logging.Configure([]string{"LOG_LEVEL=info"}, out))
	t.Cleanup(func() { _ = logging.Configure(os.Environ(), os.Stdout) })
	return out
}

// The warning about a skipped row names the style of the row's first cell. GetCellStyle got the
// cell's value instead of its reference and the wrong error was checked, so the style was always 0
// and the lookup error was lost.
func TestSkippedRowNamesTheCellStyle_New(t *testing.T) {
	f, style := skippedRowWorkbook(t)
	out := captureLog(t)

	_ = ImportExcelEnergyFileNew(f, "Sheet1", setupMock()) // no energy data: error expected

	assert.Contains(t, out.String(), fmt.Sprintf("Could not handle row format (%d), row skipped: 2 columns", style))
	assert.NotContains(t, out.String(), "Error get cell format")
}

func TestSkippedRowNamesTheCellStyle(t *testing.T) {
	f, style := skippedRowWorkbook(t)
	db, err := ebow.OpenStorageTest("skippedrow", "ecid", t.TempDir())
	require.NoError(t, err)
	defer db.CloseTestDriver()
	out := captureLog(t)

	_, _ = ImportExcelEnergyFile(f, "Sheet1", db)

	assert.Contains(t, out.String(), fmt.Sprintf("Could not handle row format (%d), row skipped: 2 columns", style))
	assert.NotContains(t, out.String(), "Error get cell format")
}
