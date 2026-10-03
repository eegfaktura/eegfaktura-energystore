package rest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	protobuf "at.ourproject/energystore/protoc"
	"at.ourproject/energystore/store"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func appHeader(roles ...string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + apitest.AppToken([]string{tenant}, roles...), "tenant": tenant}
}

func apiHeader() map[string]string {
	return map[string]string{"Authorization": apitest.Basic(apiUser, apiPass), "X-Tenant": tenant}
}

// dirs lists the <tenant>/<ecId> directories under the temporary persistence path.
func dirs(t *testing.T) []string {
	t.Helper()
	root := viper.GetString("persistence.path")
	var out []string
	tenants, _ := os.ReadDir(root)
	for _, tn := range tenants {
		ecs, _ := os.ReadDir(filepath.Join(root, tn.Name()))
		for _, ec := range ecs {
			out = append(out, tn.Name()+"/"+ec.Name())
		}
	}
	sort.Strings(out)
	return out
}

func TestRawDataReturnsTheStoredDay(t *testing.T) {
	seed(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/raw", Header: appHeader(),
		Body: fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, ms(day), ms(day.Add(23*time.Hour+45*time.Minute)))})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var resp map[string]*store.RawDataResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Contains(t, resp, meter)
	data := resp[meter].Data
	require.Len(t, data, 96)
	assert.Equal(t, ms(day), data[0].Ts)
	testsupport.InDelta(t, 0.96, data[95].Value[0])
	testsupport.InDelta(t, 0.48, data[95].Value[1])
}

// Invalid ecIds are refused before they reach the file system (rowdata.go idPattern); the status
// is 400 or 500 depending on the route. A path with ".." never reaches a handler (mux redirects).
func TestInvalidEcIdCreatesNoDirectory(t *testing.T) {
	seed(t)
	h := apitest.Router()
	before := dirs(t)
	for _, bad := range []string{"a-b", "a.b", "a%20b"} {
		for _, path := range []string{"/eeg/v2/" + bad + "/meta", "/eeg/" + bad + "/lastRecordDate"} {
			rec := apitest.Do(h, apitest.Request{Method: "GET", Path: path, Header: appHeader()})
			assert.Contains(t, []int{400, 404, 500}, rec.Code, "%s: %s", path, rec.Body.String())
		}
		rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/eeg/v2/" + bad + "/raw", Header: appHeader(),
			Body: fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, ms(day), ms(day.Add(time.Hour)))})
		assert.Contains(t, []int{400, 500}, rec.Code, "raw %s", bad)
	}
	assert.Equal(t, before, dirs(t))
}

// A valid but unknown ecId creates a Badger directory on every read (known-errors #36, F21).
func TestUnknownEcIdCreatesNoDirectory(t *testing.T) {
	t.Skip("known-errors #36")
	seed(t)
	before := dirs(t)
	apitest.Do(apitest.Router(), apitest.Request{Method: "GET", Path: "/eeg/v2/RC999999/meta", Header: appHeader()})
	assert.Equal(t, before, dirs(t), "a read of an unknown community must not create its store")
}

func TestInvalidJSONIs400(t *testing.T) {
	seed(t)
	h := apitest.Router()
	for _, rt := range routes() {
		if rt.method != "POST" || rt.body == "" || rt.guard == gql {
			continue
		}
		header := appHeader("superuser")
		if rt.guard == api {
			header = apiHeader()
		}
		rec := apitest.Do(h, apitest.Request{Method: "POST", Path: rt.path, Header: header, Body: `{"start": "not json`})
		assert.Equal(t, 400, rec.Code, "%s", rt.name())
	}
}

// GET load-curve-report and combined-report: an unparsable start is overwritten by the parse of
// end and becomes 1970; no report checks start < end (known-errors #37, F22).
func TestGetReportsRejectInvalidQuery(t *testing.T) {
	t.Skip("known-errors #37")
	seed(t)
	h := apitest.Router()
	for _, p := range []string{"load-curve-report", "combined-report"} {
		for _, q := range []string{"start=abc&end=1780264800000", fmt.Sprintf("start=%d&end=%d", ms(day.Add(time.Hour)), ms(day))} {
			rec := apitest.Do(h, apitest.Request{Method: "GET", Path: "/eeg/v2/" + ecId + "/" + p + "?" + q, Header: appHeader()})
			assert.Equal(t, 400, rec.Code, "%s?%s", p, q)
		}
	}
}

func TestRawDataDelete(t *testing.T) {
	seed(t)
	h := apitest.Router()
	body := fmt.Sprintf(`{"meteringPoint":"%s","start":%d,"end":%d,"dryRun":false}`, meter, ms(day), ms(day.Add(time.Hour)))

	rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/rawdata/delete", Header: appHeader(), Body: body})
	assert.Equal(t, 403, rec.Code)
	assert.Contains(t, rec.Body.String(), "superuser role required")

	rec = apitest.Do(h, apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/rawdata/delete", Header: appHeader("superuser"),
		Body: fmt.Sprintf(`{"meteringPoint":"%s","start":%d,"end":%d}`, meter, ms(day.Add(time.Hour)), ms(day))})
	assert.Equal(t, 400, rec.Code, "start after end")

	rec = apitest.Do(h, apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/rawdata/delete", Header: appHeader("superuser"), Body: body})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var resp struct {
		Affected int     `json:"affectedTimesteps"`
		Sum      float64 `json:"sumKwh"`
		Deleted  bool    `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Deleted)
	assert.Greater(t, resp.Affected, 0)

	rec = apitest.Do(h, apitest.Request{Method: "POST", Path: "/eeg/v2/" + ecId + "/raw", Header: appHeader(),
		Body: fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, ms(day), ms(day.Add(23*time.Hour+45*time.Minute)))})
	require.Equal(t, 200, rec.Code)
	var raw map[string]*store.RawDataResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	assert.Equal(t, 0.0, raw[meter].Data[0].Value[0], "deleted slot")
	testsupport.InDelta(t, 0.96, raw[meter].Data[95].Value[0], "neighbour outside the range")
}

// lastRecordDate without data writes 404 and then a 200 body (known-errors #17, F2).
func TestLastRecordDateWithoutData(t *testing.T) {
	t.Skip("known-errors #17")
	testsupport.UseTempPersistence(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "GET", Path: "/eeg/RC100009/lastRecordDate", Header: appHeader()})
	assert.Equal(t, 404, rec.Code)
	assert.JSONEq(t, `{"error":"No entry found"}`, rec.Body.String(), "exactly one body")
}

func TestLastRecordDate(t *testing.T) {
	seed(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "GET", Path: "/eeg/" + ecId + "/lastRecordDate", Header: appHeader()})
	require.Equal(t, 200, rec.Code)
	assert.JSONEq(t, `{"periodEnd":"01.06.2026 23:45:00"}`, rec.Body.String())
}

// The monthly export by mail always hands nil participants to the export and panics
// (known-errors #53); expected: one mail with the workbook to the token's address.
func TestExcelExportByMail(t *testing.T) {
	t.Skip("known-errors #53")
	fakes := seed(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/eeg/" + ecId + "/excel/export/2026/6", Header: appHeader()})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, 1, fakes.MailCount())
	assert.Equal(t, "member@example.org", fakes.Mails[0].Recipient)
	assert.NotEmpty(t, fakes.Mails[0].Content)
}

func TestExcelReportDownload(t *testing.T) {
	seed(t)
	rt := routes()[12]
	require.Equal(t, "/eeg/{ecid}/excel/report/download", rt.template)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: rt.path, Header: appHeader(), Body: rt.body})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-type"))
	assert.Equal(t, "TE100001-Energy-Report-20260601_20260601", rec.Header().Get("filename"))
	f, err := excelize.OpenReader(rec.Body)
	require.NoError(t, err)
	assert.Equal(t, []string{"Summary", "Energiedaten"}, f.GetSheetList())
}

func TestQueryRawDataUsesMasterDataWithoutCps(t *testing.T) {
	fakes := seed(t)
	fakes.Meters = []*protobuf.MeteringPoint{{MeteringPointId: meter}}
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/query/rawdata", Header: apiHeader(),
		Body: fmt.Sprintf(`{"ecId":"%s","start":%d,"end":%d}`, ecId, ms(day), ms(day.Add(time.Hour)))})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var resp map[string]*store.RawDataResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []string{meter}, keys(resp), "only the community's metering points")
	require.Len(t, fakes.MeterRequests, 1)
	assert.Equal(t, tenant, fakes.MeterRequests[0].Tenant)
}

func TestQueryRawDataCSV(t *testing.T) {
	seed(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: "/query/rawdata", Header: apiHeader(),
		Body: fmt.Sprintf(`{"cps":[{"meteringPoint":"%s"}],"ecId":"%s","start":%d,"end":%d,"format":"csv"}`, meter, ecId, ms(day), ms(day.Add(15*time.Minute)))})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, "text/csv", rec.Header().Get("Content-Type"))
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	// the query engine widens the range to whole local days (Engine.Query): 96 slots for 00:00–00:15
	// after TrimSpace: two header lines (the empty first record is gone), then the slots
	require.Equal(t, 2+96, len(lines))
	assert.Equal(t, ","+meter+","+meter+","+meter, lines[0])
	assert.Equal(t, "time,CONSUMPTION,CONSUMPTION,CONSUMPTION", lines[1])
	assert.Equal(t, "2026-06-01 00:00:00,0.010000,0.005000,0.002500", lines[2])
}

func keys(m map[string]*store.RawDataResult) []string {
	var k []string
	for name := range m {
		k = append(k, name)
	}
	sort.Strings(k)
	return k
}

// Basic credentials are decoded with URLEncoding and split at every ':' (known-errors #39, F24):
// standard base64 with '+' or '/' and a password containing ':' are refused today.
func TestBasicCredentialsStandardEncoding(t *testing.T) {
	t.Skip("known-errors #39")
	seed(t)
	h := apitest.Router()
	for _, pass := range []string{"a~~?", "pa??w0rd", "sec:ret?>"} {
		fakeoidc.SetUser(apiUser, pass, map[string]any{"tenant": []string{tenant}})
		auth := apitest.Basic(apiUser, pass)
		require.True(t, strings.ContainsAny(auth[len("Basic "):], "+/") || strings.Contains(pass, ":"))
		rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/query/" + ecId + "/metadata",
			Header: map[string]string{"Authorization": auth, "X-Tenant": tenant}})
		assert.Equal(t, 200, rec.Code, "password %q", pass)
	}
}

// Every /query request makes a password grant at Keycloak, also for repeated credentials
// (known-errors #39: no cache). Recorded here as today's behaviour of the guard.
func TestEveryApiRequestMakesAGrant(t *testing.T) {
	seed(t)
	h := apitest.Router()
	for range 3 {
		apitest.Do(h, apitest.Request{Method: "POST", Path: "/query/" + ecId + "/metadata", Header: apiHeader()})
	}
	assert.Equal(t, 3, fakeoidc.Grants())
}

// A 401 carries the verifier's error text to the client (known-errors #40, F25).
func TestUnauthorizedBodyHasNoVerifierDetails(t *testing.T) {
	t.Skip("known-errors #40")
	seed(t)
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "GET", Path: "/eeg/v2/" + ecId + "/meta",
		Header: map[string]string{"Authorization": "Bearer not-a-jwt", "tenant": tenant}})
	assert.Equal(t, 401, rec.Code)
	assert.NotContains(t, rec.Body.String(), "oidc")
}
