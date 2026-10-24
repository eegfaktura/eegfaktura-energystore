package excel

import (
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReturnFloat(t *testing.T) {
	for in, want := range map[string]float64{"": 0, "1.5": 1.5, "0": 0, "0.000077": 0.000077, "12": 12} {
		assert.Equal(t, want, returnFloat(in), "returnFloat(%q)", in)
	}
	assert.Equal(t, 3, returnInt("3"))
	assert.Equal(t, 0, returnInt("x"))
}

// A cell the import cannot read becomes 0 without an error (known-errors #28, F13). EDA exports
// with a decimal comma ("1,5") lose the value silently; the expected value is 1.5.
func TestReturnFloatDecimalComma(t *testing.T) {
	t.Skip("known-errors #28")
	assert.Equal(t, 1.5, returnFloat("1,5"))
}

func TestReturnMeterValue(t *testing.T) {
	cols := []string{"01.06.2026 00:00:00", "1.25", "2.5"}
	assert.Equal(t, 1.25, returnMeterValue(cols, 0), "the value is one column right of the meter index")
	assert.Equal(t, 2.5, returnMeterValue(cols, 1))
	assert.Equal(t, 0.0, returnMeterValue(cols, 2), "beyond the row")
	assert.Equal(t, 0.0, returnMeterValue(cols, -1), "no column")
}

func TestReturnMeterCode(t *testing.T) {
	tests := map[string]MeterCodeType{
		"Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]": Total,
		"Anteil gemeinschaftliche Erzeugung [KWH]":                         Share,
		"Eigendeckung gemeinschaftliche Erzeugung [KWH]":                   Coverage,
		"Gesamt/Überschusserzeugung, Gemeinschaftsüberschuss [KWH]":        Profit,
		"Gesamte gemeinschaftliche Erzeugung [KWH]":                        Total,
		"Restüberschuss bei EG und je ZP [KWH]":                            ProfitTF,
		"irgendwas":                                                        Bad,
	}
	for header, want := range tests {
		assert.Equal(t, want, returnMeterCode(strings.ToUpper(header)), header)
	}
	assert.Equal(t, "G.01", convertExcelMeterCode(Total))
	assert.Equal(t, "G.02", convertExcelMeterCode(Profit))
	assert.Equal(t, "", convertExcelMeterCode(Bad))
}

func TestExcelDates(t *testing.T) {
	assert.True(t, isDate("01.06.2026 00:15:00"))
	assert.True(t, isDate("46174.25"), "an Excel serial number")
	assert.False(t, isDate(""))
	assert.False(t, isDate("Metercode"))

	assert.Equal(t, time.Date(2026, 6, 1, 0, 15, 0, 0, time.Local), parseExcelDate("01.06.2026 00:15:00"))
	// Excel serial 46174.25 = 2026-06-01 06:00 (days since 1899-12-30, read as UTC wall time)
	assert.Equal(t, time.Date(2026, 6, 1, 6, 0, 0, 0, time.UTC), parseExcelDate("46174.25"))
	d, m, y, hh, mm, ss := getExcelDate("01.06.2026 00:14:59")
	assert.Equal(t, []int{1, 6, 2026, 0, 15, 0}, []int{d, m, y, hh, mm, ss}, "rounded to the quarter hour")
}

// A new metering point whose "Energy direction" is missing or unknown is never added to the stored
// meta and is dereferenced as nil (known-errors #27, F12): the import must refuse it with an error.
func TestBuildMatrixMetaStructMissingDirection(t *testing.T) {
	t.Skip("known-errors #27")
	for _, dir := range []model.MeterDirection{"", "SIDEWAYS"} {
		t.Run(string(dir), func(t *testing.T) { // one store per subtest: OpenStorageTest locks the tenant
			db, _ := testsupport.TestDriverStore(t, "te100001", "ecid")
			header := excelHeader{
				meteringPointId: map[int]string{0: "AT0030000000000000000000000000001"},
				energyDirection: map[int]model.MeterDirection{0: dir},
				periodStart:     map[int]string{0: "01.06.2026 00:00:00"},
				periodEnd:       map[int]string{0: "30.06.2026 23:45:00"},
				meterCode:       map[int]MeterCodeType{0: Total},
			}
			assert.NotPanics(t, func() {
				_, _, err := buildMatrixMetaStruct(db, header)
				assert.Error(t, err, "direction %q", dir)
			})
		})
	}
}

func TestBuildMatrixMetaStructRejectsBadPeriod(t *testing.T) {
	db, _ := testsupport.TestDriverStore(t, "te100001", "ecid")
	header := excelHeader{
		meteringPointId: map[int]string{0: "AT0030000000000000000000000000001"},
		energyDirection: map[int]model.MeterDirection{0: model.CONSUMER_DIRECTION},
		periodStart:     map[int]string{0: "01.06.2026 00:00:00"},
		periodEnd:       map[int]string{0: "end of june"},
		meterCode:       map[int]MeterCodeType{0: Total},
	}
	_, _, err := buildMatrixMetaStruct(db, header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Period End")
}
