package rest_test

import (
	"fmt"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"github.com/stretchr/testify/assert"
)

// POST billing-check (#73) refuses a request that fails BillingCheckRequest.Validate with 400,
// before the store is read.
func TestBillingCheckRejectsInvalidRequest(t *testing.T) {
	seed(t)
	h := apitest.Router()
	start, end := ms(day), ms(day.Add(23*time.Hour))
	for name, body := range map[string]string{
		"end before start":       fmt.Sprintf(`{"start":%d,"end":%d,"cps":[]}`, end, start),
		"no start":               fmt.Sprintf(`{"end":%d,"cps":[]}`, end),
		"period over 367 days":   fmt.Sprintf(`{"start":%d,"end":%d,"cps":[]}`, start, ms(day.AddDate(1, 0, 3))),
		"metering point missing": fmt.Sprintf(`{"start":%d,"end":%d,"cps":[{"direction":"CONSUMPTION"}]}`, start, end),
	} {
		rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/billing-check", Header: appHeader(), Body: body})
		assert.Equal(t, 400, rec.Code, "%s: %s", name, rec.Body.String())
	}
}

// An ecId the store refuses (rowdata.go idPattern) passes the request checks and fails in the
// store: 500 with a generic message, and no directory is created.
func TestBillingCheckInvalidEcIdIs500(t *testing.T) {
	seed(t)
	before := dirs(t)
	body := fmt.Sprintf(`{"start":%d,"end":%d,"cps":[]}`, ms(day), ms(day.Add(23*time.Hour)))
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/eeg/v2/a-b/billing-check", Header: appHeader(), Body: body})
	assert.Equal(t, 500, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "billing check failed")
	assert.Equal(t, before, dirs(t))
}
