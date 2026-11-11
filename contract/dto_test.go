package contract_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/excel"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenant   = "TE100001"
	ecId     = "RC100001"
	consumer = "AT0030000000000000000000000000001"
	producer = "AT0030000000000000000000030000001"
)

var day = time.Date(2026, 6, 1, 0, 0, 0, 0, testsupport.Vienna)

// sample builds a JSON value for a Kotlin type: data classes recursively by their fields.
func sample(classes map[string][]kotlinField, typ string) interface{} {
	typ = strings.TrimSuffix(strings.TrimSpace(typ), "?")
	switch {
	case typ == "Long" || typ == "Int":
		return 1
	case typ == "Double":
		return 1.5
	case typ == "Boolean":
		return true
	case typ == "String":
		return "x"
	case strings.HasPrefix(typ, "List<"):
		return []interface{}{sample(classes, typ[5:len(typ)-1])}
	}
	obj := map[string]interface{}{}
	for _, f := range classes[typ] {
		obj[f.JSON] = sample(classes, f.Type)
	}
	return obj
}

// decodeStrict decodes JSON into v and fails on fields v does not know (a field the caller sends
// that the energystore would drop).
func decodeStrict(v interface{}, data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// TestCallerRequestContract: v3's request classes (parsed from EnergyStoreWire.kt) decode into the
// handlers' request types without an unknown field.
func TestCallerRequestContract(t *testing.T) {
	classes, at := kotlinClasses(t, "testdata/v3/EnergyStoreWire.kt")
	for _, tc := range []struct {
		class  string
		target func() interface{}
	}{
		{"EsReportRequest", func() interface{} { return &model.ReportRequest{} }},
		{"EsSummaryRequest", func() interface{} { return &model.EnergyReportRequest{} }},
		{"EsRawRequest", func() interface{} { return &model.RawDataRequest{} }},
		{"EsRangeRequest", func() interface{} { return &struct{ Start, End int64 }{} }},
		{"EsDeleteRequest", func() interface{} {
			return &struct {
				MeteringPoint string `json:"meteringPoint"`
				Start, End    int64
				DryRun        bool `json:"dryRun"`
			}{}
		}},
	} {
		require.Contains(t, classes, tc.class)
		body, err := json.Marshal(sample(classes, tc.class))
		require.NoError(t, err)
		assert.NoError(t, decodeStrict(tc.target(), body), "eegfaktura-v3 EnergyStoreWire.kt:%d %s: %s", at[tc.class], tc.class, body)
	}

	// eegfaktura-web: the bodies as energy.service.ts builds them (lines checked against the copy).
	src := lines(t, "testdata/web/energy.service.ts")
	for _, tc := range []struct {
		line   int
		keys   []string
		body   string
		target interface{}
	}{
		{31, []string{"reportInterval", "type", "year", "segment", "participants"}, `{"reportInterval":{"type":"YM","year":2026,"segment":6},"participants":[]}`, &model.ReportRequest{}},
		{131, []string{"type", "year", "segment"}, `{"type":"YM","year":2026,"segment":6}`, &model.EnergyReportRequest{}},
		{146, []string{"meters", "start", "end"}, `{"meters":["x"],"start":1,"end":2}`, &model.RawDataRequest{}},
	} {
		for _, k := range tc.keys {
			assert.Contains(t, src[tc.line-1], k, "eegfaktura-web energy.service.ts:%d no longer sends %q", tc.line, k)
		}
		assert.NoError(t, decodeStrict(tc.target, []byte(tc.body)), "eegfaktura-web energy.service.ts:%d", tc.line)
	}
	// ExcelReportRequest (web src/models/reports.model.ts): start, end, communityId, cps[meteringPoint,
	// direction, name, activeSince, inactiveSince].
	assert.NoError(t, decodeStrict(&excel.ExportParticipantEnergy{}, []byte(
		`{"start":1,"end":2,"communityId":"x","cps":[{"meteringPoint":"x","direction":"CONSUMPTION","name":"n","activeSince":1,"inactiveSince":2}]}`)))
}

// seedCommunity stores one consumer and one producer for one day and starts the gRPC fakes.
func seedCommunity(t *testing.T) {
	t.Helper()
	db := testsupport.TempStore(t, tenant, ecId)
	require.NoError(t, db.SetMeta(testsupport.CpMeta(
		testsupport.Consumer(consumer, 0, "01.06.2026 00:00:00", "01.06.2026 23:45:00"),
		testsupport.Producer(producer, 0, "01.06.2026 00:00:00", "01.06.2026 23:45:00"))))
	for i, ts := range testsupport.QuarterHours(day) {
		v := float64(i+1) / 100
		require.NoError(t, db.SetLine(testsupport.RawLine(testsupport.RowId(ts), []float64{v, v / 2, v / 4}, []float64{v * 2, v})))
	}
	apitest.StartFakes(t)
	fakeoidc.ResetUsers()
}

func post(t *testing.T, path, body string, roles ...string) []byte {
	t.Helper()
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: path, Body: body,
		Header: map[string]string{"Authorization": "Bearer " + apitest.AppToken([]string{tenant}, roles...), "X-Tenant": tenant}})
	require.Equal(t, 200, rec.Code, "%s: %s", path, rec.Body.String())
	return rec.Body.Bytes()
}

func get(t *testing.T, path string) []byte {
	t.Helper()
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "GET", Path: path,
		Header: map[string]string{"Authorization": "Bearer " + apitest.AppToken([]string{tenant}), "X-Tenant": tenant}})
	require.Equal(t, 200, rec.Code, "%s: %s", path, rec.Body.String())
	return rec.Body.Bytes()
}

func ms(t time.Time) int64 { return t.UnixMilli() }

func at(v interface{}, path ...interface{}) interface{} {
	for _, p := range path {
		switch k := p.(type) {
		case string:
			v = v.(map[string]interface{})[k]
		case int:
			v = v.([]interface{})[k]
		}
	}
	return v
}

// TestCallerResponseContract: every field v3 declares for a response (EnergyStoreWire.kt) is a key of
// the energystore's JSON; a removed or renamed field names the class and line.
func TestCallerResponseContract(t *testing.T) {
	seedCommunity(t)
	classes, line := kotlinClasses(t, "testdata/v3/EnergyStoreWire.kt")
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	v2 := "/eeg/v2/" + ecId

	var report, raw, summary, intraday, meta, deleted, lastRecord interface{}
	require.NoError(t, json.Unmarshal(post(t, v2+"/report", fmt.Sprintf(`{"reportInterval":{"type":"YM","year":2026,"segment":6},"participants":[{"participantId":"P","meters":[{"meterId":"%s","meterDir":"CONSUMPTION","from":%d,"until":%d}]}]}`, consumer, start, end)), &report))
	require.NoError(t, json.Unmarshal(post(t, v2+"/raw", fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, consumer, start, end)), &raw))
	require.NoError(t, json.Unmarshal(post(t, v2+"/summary", `{"type":"YM","year":2026,"segment":6}`), &summary))
	require.NoError(t, json.Unmarshal(post(t, v2+"/intra-day-report", fmt.Sprintf(`{"start":%d,"end":%d}`, start, end)), &intraday))
	require.NoError(t, json.Unmarshal(get(t, v2+"/meta"), &meta))
	require.NoError(t, json.Unmarshal(get(t, "/eeg/"+ecId+"/lastRecordDate"), &lastRecord))
	require.NoError(t, json.Unmarshal(post(t, v2+"/rawdata/delete", fmt.Sprintf(`{"meteringPoint":"%s","start":%d,"end":%d,"dryRun":true}`, consumer, start, end), "superuser"), &deleted))

	for _, tc := range []struct {
		class string
		json  interface{}
	}{
		{"EsReportResponse", report},
		{"EsParticipantReport", at(report, "participantReports", 0)},
		{"EsMeterReport", at(report, "participantReports", 0, "meters", 0)},
		{"EsReport", at(report, "participantReports", 0, "meters", 0, "report")},
		{"EsRecord", at(report, "participantReports", 0, "meters", 0, "report", "summary")},
		{"EsIntermediate", at(report, "participantReports", 0, "meters", 0, "report", "intermediate")},
		{"EsCounterPointMeta", at(report, "meta", 0)},
		{"EsRawMeter", at(raw, consumer)},
		{"EsRawRow", at(raw, consumer, "data", 0)},
		{"EsReportData", at(summary, 0)},
		{"EsReportData", at(intraday, 0)},
		{"EsMetaPeriod", at(meta, consumer)},
		{"EsDeleteResponse", deleted},
	} {
		obj, ok := tc.json.(map[string]interface{})
		require.True(t, ok, "%s: not a JSON object: %v", tc.class, tc.json)
		require.Contains(t, classes, tc.class)
		for _, f := range classes[tc.class] {
			assert.Contains(t, obj, f.JSON, "eegfaktura-v3 EnergyStoreWire.kt:%d %s.%s is not in the response", line[tc.class], tc.class, f.Name)
		}
	}
	assert.Contains(t, lastRecord, "periodEnd", "eegfaktura-v3 EnergyStoreClient.kt reads periodEnd of lastRecordDate")
}
