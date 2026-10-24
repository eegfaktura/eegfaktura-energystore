package rest_test

import (
	"encoding/json"
	"testing"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A community without any imported energy data has no meta record cpmeta/0. The report answered
// 500 {"error":"Record doesn't exist"}; it is an empty report (web#191 mapped the text to one).
func TestReportWithoutAnyData(t *testing.T) {
	testsupport.UseTempPersistence(t)
	var body string
	for _, rt := range routes() {
		if rt.template == "/eeg/v2/{ecid}/report" {
			body = rt.body
		}
	}
	require.NotEmpty(t, body, "matrix row of the report")
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/eeg/v2/RC100009/report", Header: appHeader(), Body: body})
	require.Equal(t, 200, rec.Code, rec.Body.String())

	var resp model.ReportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "YM/2026/06", resp.Id)
	assert.NotNil(t, resp.Meta, "meta is an empty list, not null")
	assert.Empty(t, resp.Meta)
	assert.Zero(t, resp.TotalProduction)
	assert.Zero(t, resp.TotalConsumption)
	require.Len(t, resp.ParticipantReports, 1)
	assert.Equal(t, "P1", resp.ParticipantReports[0].ParticipantId)
}
