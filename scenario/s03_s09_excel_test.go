package scenario_test

import (
	"fmt"
	"testing"
	"time"

	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sheet = "ConsumptionDataReport"

func uploadExcel(t *testing.T, rows []excelRow, direction string, withMeterCode bool) error {
	t.Helper()
	f := excelWorkbook(t, direction, withMeterCode, rows)
	return excel.ImportFile(tenant, ecId, "upload.xlsx", sheet, workbookBytes(t, f))
}

func storedLine(t *testing.T, id string) *model.RawSourceLine {
	t.Helper()
	db, err := ebow.OpenStorage(tenant, ecId)
	require.NoError(t, err)
	defer db.Close()
	l := &model.RawSourceLine{Id: id}
	require.NoError(t, db.GetLine(l), id)
	return l
}

// dayRows is one normal day of Excel rows with G.01 = cons(i), G.02 = share(i), G.03 = cover(i).
func dayRows(day time.Time) []excelRow {
	var rows []excelRow
	for i, ts := range testsupport.QuarterHours(day) {
		rows = append(rows, excelRow{ts: ts.Format("02.01.2006 15:04:05"),
			g1: fmt.Sprint(cons(i)), g2: fmt.Sprint(share(i)), g3: fmt.Sprint(cover(i))})
	}
	return rows
}

// S9 (base) — a valid EDA report imports every slot with its values and the meta record.
func TestS09_ExcelImport(t *testing.T) {
	community(t)
	require.NoError(t, uploadExcel(t, dayRows(june1), "CONSUMPTION", true))

	c := raw(t, consumer, june1, june1)
	require.Len(t, c, 96)
	for i, d := range c {
		testsupport.InDelta(t, cons(i), d.Value[0], "G.01 slot %d", i)
		testsupport.InDelta(t, share(i), d.Value[1], "G.02 slot %d", i)
		testsupport.InDelta(t, cover(i), d.Value[2], "G.03 slot %d", i)
	}
}

// S9 — an unknown "Energy direction" must be refused with an error, not a panic (known-errors #27).
func TestS09_ExcelUnknownDirection(t *testing.T) {
	t.Skip("known-errors #27")
	community(t)
	assert.NotPanics(t, func() {
		assert.Error(t, uploadExcel(t, dayRows(june1), "SIDEWAYS", true))
	})
}

// S9 — a decimal comma ("1,5") must not become 0 silently (known-errors #28, F13).
func TestS09_ExcelDecimalComma(t *testing.T) {
	t.Skip("known-errors #28")
	community(t)
	rows := []excelRow{{ts: "01.06.2026 00:00:00", g1: "1,5", g2: "0", g3: "0"}}
	err := uploadExcel(t, rows, "CONSUMPTION", true)
	if err == nil {
		testsupport.InDelta(t, 1.5, storedLine(t, "CP/2026/06/01/00/00/00").Consumers[0])
	}
}

// S9 — a sheet without the Metercode header must be an error, not an import of zero meters
// (known-errors #28, F13).
func TestS09_ExcelWithoutMeterCodeHeader(t *testing.T) {
	t.Skip("known-errors #28")
	community(t)
	assert.Error(t, uploadExcel(t, dayRows(june1), "CONSUMPTION", false))
}

// S3 (Excel) — a TF column replaces the base value; it is not added to it.
func TestS03_ExcelTFReplacesBase(t *testing.T) {
	community(t)
	rows := []excelRow{{ts: "01.06.2026 00:00:00", g1: "1", g2: "0", g3: "0", g1TF: "0.8"}}
	require.NoError(t, uploadExcel(t, rows, "CONSUMPTION", true))
	testsupport.InDelta(t, 0.8, storedLine(t, "CP/2026/06/01/00/00/00").Consumers[0])
}

// S3 (Excel) — the autumn hour appears twice; the one stored key must hold ext1 + ext2, not
// ext1 + base2 + ext2 (known-errors #29, F14).
func TestS03_ExcelAutumnHourWithTF(t *testing.T) {
	t.Skip("known-errors #29")
	community(t)
	rows := []excelRow{
		{ts: "25.10.2026 02:00:00", g1: "2", g2: "0", g3: "0", g1TF: "2"},
		{ts: "25.10.2026 02:00:00", g1: "3", g2: "0", g3: "0", g1TF: "3"},
	}
	require.NoError(t, uploadExcel(t, rows, "CONSUMPTION", true))
	testsupport.InDelta(t, 5, storedLine(t, "CP/2026/10/25/02/00/00").Consumers[0])
}
