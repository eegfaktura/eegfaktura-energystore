package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestGetMonthDuration(t *testing.T) {
	startTime := time.Date(2022, time.April, 18, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2022, time.May, 30, 23, 45, 0, 0, time.UTC)

	y, d := GetMonthDuration(startTime, endTime)

	assert.Equal(t, y, 2022)
	assert.Equal(t, d, 1)

}

func TestGetMonthDurationDec(t *testing.T) {
	startTime := time.Date(2022, time.December, 31, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2022, time.December, 31, 23, 45, 0, 0, time.UTC)

	y, d := GetMonthDuration(startTime, endTime)

	assert.Equal(t, y, 2022)
	assert.Equal(t, d, 0)
}

func TestParseTime(t *testing.T) {
	expectedTime := time.Date(2022, time.April, 18, 0, 0, 0, 0, time.Local)
	strTime := "18.04.2022 00:00:00"
	d, err := ParseTime(strTime, time.Now().UnixMilli())
	assert.NoError(t, err)

	fmt.Printf("Actual-Time: %v\n", d)
	fmt.Printf("Expected-Time: %v\n", expectedTime)

	assert.Equal(t, d, expectedTime)
}

func TestConvertUnixTimeToRowId(t *testing.T) {
	rowId, err := ConvertUnixTimeToRowId("CP/", time.UnixMilli(1688680800000).UTC())
	require.NoError(t, err)

	fmt.Printf("RowID: %v\n", rowId)
}
