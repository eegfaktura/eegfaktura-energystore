package utils

import (
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport/tz"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Row ids are local wall-clock time in Europe/Vienna (AGENTS.md section 8). These tests pin the
// conversion in both directions over DST, year and leap-day boundaries (concept T2).
func TestRowIdWallClockInVienna(t *testing.T) {
	v := tz.Vienna
	tests := []struct {
		name string
		ts   time.Time
		want string
	}{
		{"spring: last slot before the jump", time.Date(2026, 3, 29, 1, 45, 0, 0, v), "CP/2026/03/29/01/45/00"},
		{"spring: 15 minutes later is 03:00", time.Date(2026, 3, 29, 1, 45, 0, 0, v).Add(15 * time.Minute), "CP/2026/03/29/03/00/00"},
		{"autumn: first 02:00 (CEST)", time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC), "CP/2026/10/25/02/00/00"},
		{"autumn: second 02:00 (CET) has the same id", time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC), "CP/2026/10/25/02/00/00"},
		{"year end", time.Date(2026, 12, 31, 23, 45, 0, 0, v), "CP/2026/12/31/23/45/00"},
		{"new year", time.Date(2026, 12, 31, 23, 45, 0, 0, v).Add(15 * time.Minute), "CP/2027/01/01/00/00/00"},
		{"leap day", time.Date(2028, 2, 29, 12, 15, 0, 0, v), "CP/2028/02/29/12/15/00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertUnixTimeToRowId("CP/", tt.ts.In(time.Local))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			back, err := ConvertRowIdToTime("CP", got)
			require.NoError(t, err)
			assert.Equal(t, tt.ts.In(v).Format("2006-01-02 15:04"), back.Format("2006-01-02 15:04"), "wall clock round trip")
		})
	}
}

func TestConvertRowIdToTimeRejectsGarbage(t *testing.T) {
	_, err := ConvertRowIdToTime("CP", "CP/2026/xx/01/00/00/00")
	assert.Error(t, err)
}

func TestConvertTimeToRowIdAndString(t *testing.T) {
	id, err := ConvertTimeToRowId("CP/", "29.02.2028 23:45:00")
	require.NoError(t, err)
	assert.Equal(t, "CP/2028/02/29/23/45/00", id)

	_, err = ConvertTimeToRowId("CP/", "garbage")
	assert.Error(t, err)

	s, ts, err := ConvertRowIdToTimeString("CP", "CP/2026/10/25/02/15/00", tz.Vienna)
	require.NoError(t, err)
	assert.Equal(t, "25.10.2026 02:15:00", s)
	assert.Equal(t, 2, ts.Hour())

	assert.Equal(t, "31.12.2026 23:45:00", ConvertTimeToStringExcel(time.Date(2026, 12, 31, 23, 45, 59, 0, tz.Vienna)))
	assert.Equal(t, "2028-02-29", ConvertDate(time.Date(2028, 2, 29, 0, 0, 0, 0, tz.Vienna)))
	assert.Equal(t, time.Date(2026, 3, 29, 0, 0, 0, 0, time.Local), TruncateToDay(time.Date(2026, 3, 29, 23, 45, 0, 0, time.Local)))
}

func TestCheckTime(t *testing.T) {
	a := time.Date(2026, 6, 1, 0, 0, 0, 0, tz.Vienna)
	b := a.Add(15 * time.Minute)
	c := a.Add(30 * time.Minute)
	assert.True(t, CheckTime(nil, &a))
	assert.True(t, CheckTime(&a, &b))
	assert.False(t, CheckTime(&a, &c), "a gap of one slot")
}

func TestPeriodToStartEndTime(t *testing.T) {
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	tests := []struct {
		code       string
		year, seg  int
		start, end time.Time
	}{
		{"YM", 2028, 2, date(2028, 2, 1), date(2028, 2, 29)},
		{"YM", 2026, 12, date(2026, 12, 1), date(2026, 12, 31)},
		{"YQ", 2026, 1, date(2026, 1, 1), date(2026, 3, 31)},
		{"YQ", 2026, 4, date(2026, 10, 1), date(2026, 12, 31)},
		{"YH", 2026, 2, date(2026, 7, 1), date(2026, 12, 31)},
		{"Y", 2026, 0, date(2026, 1, 1), date(2026, 12, 31)},
	}
	for _, tt := range tests {
		s, e, err := PeriodToStartEndTime(tt.year, tt.seg, tt.code)
		require.NoError(t, err, "%s %d/%d", tt.code, tt.year, tt.seg)
		assert.Equal(t, tt.start, s, "%s %d start", tt.code, tt.seg)
		assert.Equal(t, tt.end, e, "%s %d end", tt.code, tt.seg)
	}
	for _, bad := range []struct {
		code string
		seg  int
	}{{"YM", 0}, {"YM", 13}, {"YQ", 5}, {"YH", 3}, {"Y", 1}, {"X", 1}} {
		_, _, err := PeriodToStartEndTime(2026, bad.seg, bad.code)
		assert.Error(t, err, "%s %d", bad.code, bad.seg)
	}
}

func TestIsLineDateOutOfRange(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, tz.Vienna)
	until := time.Date(2026, 6, 30, 0, 0, 0, 0, tz.Vienna)
	r := [2]int64{from.UnixMilli(), until.UnixMilli()}
	assert.True(t, IsLineDateOutOfRange(from.Add(-15*time.Minute), r))
	assert.False(t, IsLineDateOutOfRange(from, r))
	assert.False(t, IsLineDateOutOfRange(until.Add(23*time.Hour+45*time.Minute), r), "the until day is included up to 23:59")
	assert.True(t, IsLineDateOutOfRange(until.Add(24*time.Hour), r))
}

func TestGetMonthDurationAcrossYear(t *testing.T) {
	y, months := GetMonthDuration(time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), time.Date(2027, 2, 14, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, 2026, y)
	assert.Equal(t, 2, months, "15.11. to 14.02. is two full months")
}

func TestExamineDirection(t *testing.T) {
	assert.Equal(t, model.PRODUCER_DIRECTION, ExamineDirection([]model.MqttEnergyData{{MeterCode: model.CODE_CON}, {MeterCode: model.CODE_PLUS}}))
	assert.Equal(t, model.PRODUCER_DIRECTION, ExamineDirection([]model.MqttEnergyData{{MeterCode: model.CODE_GEN_TF}}))
	assert.Equal(t, model.CONSUMER_DIRECTION, ExamineDirection([]model.MqttEnergyData{{MeterCode: model.CODE_CON}, {MeterCode: model.CODE_SHARE}}))
	assert.Equal(t, model.CONSUMER_DIRECTION, ExamineDirection(nil))
}

func TestSliceHelpers(t *testing.T) {
	assert.Equal(t, []float64{1, 0, 3}, Insert([]float64{1}, 2, 3), "grows the slice")
	assert.Equal(t, []float64{5, 2}, Insert([]float64{1, 2}, 0, 5))
	assert.Equal(t, []int{0, 7}, InsertInt(nil, 1, 7))
	assert.Equal(t, 0, GetInt([]int{1}, 3))
	assert.Equal(t, 0, GetInt([]int{1}, -1))
	assert.Equal(t, 1, GetInt([]int{1}, 0))
	assert.Equal(t, 1.235, RoundToFixed(1.23456, 3))
	assert.Equal(t, []int{4, 4}, InitSlice(4, make([]int, 2)))

	c, p := CountConsumerProducer([]*model.CounterPointMeta{{Dir: model.CONSUMER_DIRECTION}, {Dir: model.PRODUCER_DIRECTION}, {Dir: model.CONSUMER_DIRECTION}})
	assert.Equal(t, 2, c)
	assert.Equal(t, 1, p)

	cm, pm := ConvertLineToMatrix(&model.RawSourceLine{Consumers: []float64{1, 2, 3, 4, 5, 6}, Producers: []float64{7, 8}})
	assert.Equal(t, 2, cm.Rows)
	assert.Equal(t, []float64{4, 5, 6}, cm.GetRow(1))
	assert.Equal(t, 1, pm.Rows)
	assert.Equal(t, []float64{7, 8}, pm.GetRow(0))
}

// DecodeMeterCode maps the meter codes of a CR_MSG to the position inside a metering point's slot
// group: consumers G.01/G.02/G.03 at +0/+1/+2, producers G.01/P.01 at +0/+1; TF codes like the base.
func TestDecodeMeterCode(t *testing.T) {
	tests := []struct {
		code      model.MeterCodeValue
		typ, name string
		delta     int
	}{
		{model.CODE_GEN, "GEN", "G.01", 0}, {model.CODE_GEN_TF, "GEN", "G.01", 0},
		{model.CODE_PLUS, "PLUS", "P.01", 1}, {model.CODE_PLUS_TF, "PLUS", "P.01", 1},
		{model.CODE_CON, "CON", "G.01", 0}, {model.CODE_CON_TF, "CON", "G.01", 0},
		{model.CODE_SHARE, "SHARE", "G.02", 1},
		{model.CODE_COVER, "COVER", "G.03", 2}, {model.CODE_COVER_TF, "COVER", "G.03", 2},
	}
	for _, tt := range tests {
		m := DecodeMeterCode(tt.code, 4)
		require.NotNil(t, m, tt.code)
		assert.Equal(t, model.MeterCodeMeta{Type: tt.typ, Code: tt.name, SourceInData: 4, SourceDelta: tt.delta}, *m, tt.code)
	}
	assert.Nil(t, DecodeMeterCode("1-1:9.9.9 X.99", 0), "unknown code")
}

func TestCastQoVStringToInt(t *testing.T) {
	for in, want := range map[string]int{"L1": 1, "l2": 2, "L3": 3, "": 0, "L0": 0, "X": 0} {
		assert.Equal(t, want, CastQoVStringToInt(in), in)
	}
}

func TestStringSliceHelpers(t *testing.T) {
	vs := []string{"a", "bb", "ccc"}
	long := func(s string) bool { return len(s) > 1 }
	assert.Equal(t, 1, Index(vs, "bb"))
	assert.Equal(t, -1, Index(vs, "x"))
	assert.True(t, Include(vs, "a"))
	assert.False(t, Include(nil, "a"))
	assert.True(t, Any(vs, long))
	assert.False(t, All(vs, long))
	assert.True(t, All(nil, long))
	assert.Equal(t, []string{"bb", "ccc"}, Filter(vs, long))
	assert.Equal(t, []string{"A", "BB", "CCC"}, Map(vs, strings.ToUpper))
	assert.Equal(t, 6.5, Sum([]float64{1, 2.5, 3}))
	assert.True(t, GetBool([]bool{true}, 0))
	assert.False(t, GetBool([]bool{true}, 1))
}
