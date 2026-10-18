package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBillingCheckRequestValidate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli()
	endOfYear := time.Date(2026, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli()
	cp := BillingCheckCp{MeteringPoint: "AT003", Direction: CONSUMER_DIRECTION}

	ok := BillingCheckRequest{Start: start, End: endOfYear, Cps: []BillingCheckCp{cp}}
	assert.NoError(t, ok.Validate(), "one year is a valid billing period")
	assert.NoError(t, (&BillingCheckRequest{Start: start, End: start}).Validate(), "no metering points is valid")

	for name, req := range map[string]BillingCheckRequest{
		"start missing":     {End: endOfYear},
		"end before start":  {Start: endOfYear, End: start},
		"longer than year":  {Start: start, End: start + 400*24*3600*1000},
		"huge end":          {Start: start, End: 1 << 62},
		"metering point":    {Start: start, End: start, Cps: []BillingCheckCp{{Direction: CONSUMER_DIRECTION}}},
		"invalid direction": {Start: start, End: start, Cps: []BillingCheckCp{{MeteringPoint: "AT003", Direction: "IN"}}},
		"too many":          {Start: start, End: start, Cps: make([]BillingCheckCp, billingCheckMaxCps+1)},
	} {
		assert.Error(t, req.Validate(), name)
	}
}
