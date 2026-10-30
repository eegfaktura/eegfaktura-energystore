package excel

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// skippedRowWorkbook has rows the importer cannot handle (not a header, not a date) in A3 to A5;
// A3 has a style of its own; rows 1 and 2 are empty.
func skippedRowWorkbook(t *testing.T) (*excelize.File, int) {
	t.Helper()
	f := excelize.NewFile()
	t.Cleanup(func() { _ = f.Close() })
	require.NoError(t, f.SetCellValue("Sheet1", "A3", "unknown row"))
	require.NoError(t, f.SetCellValue("Sheet1", "B3", 1.5))
	require.NoError(t, f.SetCellValue("Sheet1", "A4", "uploaded content 1"))
	require.NoError(t, f.SetCellValue("Sheet1", "A5", "uploaded content 2"))
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

// Skipped rows give one warning per sheet with the count, the first skipped row and the style of
// its first cell (261030-mm2: GetCellStyle got the cell's value instead of its reference and the
// wrong error was checked, so the style was always 0), and no cell content.
func TestSkippedRowNamesTheCellStyle_New(t *testing.T) {
	f, style := skippedRowWorkbook(t)
	out := captureLog(t)

	_ = ImportExcelEnergyFileNew(f, "Sheet1", setupMock()) // no energy data: error expected

	assertSkippedSummary(t, out.String(), style)
}

func TestSkippedRowNamesTheCellStyle(t *testing.T) {
	f, style := skippedRowWorkbook(t)
	db, err := ebow.OpenStorageTest("skippedrow", "ecid", t.TempDir())
	require.NoError(t, err)
	defer db.CloseTestDriver()
	out := captureLog(t)

	_, _ = ImportExcelEnergyFile(f, "Sheet1", db)

	assertSkippedSummary(t, out.String(), style)
}

func assertSkippedSummary(t *testing.T, log string, style int) {
	t.Helper()
	assert.Equal(t, 1, strings.Count(log, "Could not handle row format"), "one line per sheet")
	assert.Contains(t, log, fmt.Sprintf("Could not handle row format, 3 rows skipped; first skipped row 3 (cell style %d)", style))
	assert.NotContains(t, log, "Error get cell format")
	assert.NotContains(t, log, "unknown row")
	assert.NotContains(t, log, "uploaded content")
}
