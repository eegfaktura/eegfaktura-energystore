package rest_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// energystore 261025-ca1: a member (every bearer token without admin, owner or superuser) reads per-meter
// energy only of its own metering points; the backend (GET /participant with the member's token) names them.

const otherMeter = "AT0030000000000000000000000000002" // stored, belongs to another member

// fakeBackend answers GET /participant with one participant owning meter and counts the calls.
type fakeBackend struct {
	*httptest.Server
	calls         atomic.Int32
	authorization atomic.Value
	tenant        atomic.Value
}

func startBackend(t *testing.T) *fakeBackend {
	return startBackendWindow(t, `"2026-01-01"`, "null")
}

// startBackendWindow: meter belongs to the member from activeSince until inactiveSince (JSON values).
func startBackendWindow(t *testing.T, activeSince, inactiveSince string) *fakeBackend {
	t.Helper()
	b := &fakeBackend{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.calls.Add(1)
		b.authorization.Store(r.Header.Get("Authorization"))
		b.tenant.Store(r.Header.Get("tenant"))
		if r.Method != http.MethodGet || r.URL.Path != "/participant" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `[{"id":"P1","firstname":"A","meters":[{"meteringPoint":"%s","direction":"CONSUMPTION",`+
			`"registeredSince":"2025-01-01","participantState":{"activeSince":%s,"inactiveSince":%s}}]}]`, meter, activeSince, inactiveSince)
	}))
	t.Cleanup(b.Close)
	return b
}

// memberRouter seeds two stored consumers (meter, otherMeter) and returns the router with
// services.backend-api set to url.
func memberRouter(t *testing.T, url string) http.Handler {
	t.Helper()
	db := testsupport.TempStore(t, tenant, ecId)
	require.NoError(t, db.SetMeta(testsupport.CpMeta(
		testsupport.Consumer(meter, 0, "01.06.2026 00:00:00", "01.06.2026 23:45:00"),
		testsupport.Consumer(otherMeter, 1, "01.06.2026 00:00:00", "01.06.2026 23:45:00"))))
	for i, ts := range testsupport.QuarterHours(day) {
		v := float64(i+1) / 100
		require.NoError(t, db.SetLine(testsupport.RawLine(testsupport.RowId(ts), []float64{v, v / 2, v / 4, v, v / 2, v / 4}, nil)))
	}
	fakeoidc.ResetUsers()
	apitest.StartFakes(t)
	viper.Set("services.backend-api", url)
	t.Cleanup(func() { viper.Set("services.backend-api", "") })
	return apitest.Router()
}

func groupToken(groups ...string) string {
	return "Bearer " + fakeoidc.Token(map[string]any{
		"tenant": []string{tenant}, "access_groups": groups, "sub": "member-1",
		"email": "member@example.org", "realm_access": map[string]any{"roles": []string{}},
	})
}

var memberAuth = groupToken("/EEG_USER")

func send(h http.Handler, auth, method, path, body string) *httptest.ResponseRecorder {
	return apitest.Do(h, apitest.Request{Method: method, Path: path, Body: body,
		Header: map[string]string{"Authorization": auth, "tenant": tenant}})
}

// perMeter lists the per-meter routes with a body that names cp.
func perMeter(cp string) map[string][2]string {
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	v2 := "/eeg/v2/" + ecId
	return map[string][2]string{
		"report": {v2 + "/report", `{"reportInterval":{"year":2026,"segment":6,"type":"YM"},"participants":[{"participantId":"P1","meters":[{"meterId":"` +
			cp + `","meterDir":"CONSUMPTION","from":` + fmt.Sprint(start) + `,"until":` + fmt.Sprint(end) + `}]}]}`},
		"raw": {v2 + "/raw", fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, cp, start, end)},
		"billing-check": {v2 + "/billing-check", fmt.Sprintf(`{"start":%d,"end":%d,"cps":[{"meteringPoint":"%s","direction":"CONSUMPTION","activeSince":%d,"inactiveSince":%d}]}`,
			start, end, cp, start, end)},
		"excel report": {"/eeg/" + ecId + "/excel/report/download", fmt.Sprintf(`{"start":%d,"end":%d,"communityId":"%s","cps":[{"meteringPoint":"%s","direction":"CONSUMPTION","activeSince":%d,"inactiveSince":%d}]}`,
			start, end, ecId, cp, start, end)},
	}
}

func TestMemberReadsOwnMeters(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	for name, rt := range perMeter(meter) {
		rec := send(h, memberAuth, "POST", rt[0], rt[1])
		assert.Equal(t, 200, rec.Code, "%s: %s", name, truncate(rec.Body.String()))
	}
	assert.Equal(t, int32(1), b.calls.Load(), "one backend call, the others from the cache")
	assert.Equal(t, memberAuth, b.authorization.Load(), "the member's token is forwarded")
	assert.Equal(t, tenant, b.tenant.Load())
}

func TestMemberForeignAndUnknownMeterAre403(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	for _, cp := range []string{otherMeter, "AT0030000000000000000000000099999"} {
		for name, rt := range perMeter(cp) {
			rec := send(h, memberAuth, "POST", rt[0], rt[1])
			assert.Equal(t, 403, rec.Code, "%s %s", name, cp)
			assert.JSONEq(t, `{"error":"metering point not permitted"}`, rec.Body.String(), "%s %s", name, cp)
		}
	}
}

func TestMemberSeesOwnMetaOnly(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)

	rec := send(h, memberAuth, "GET", "/eeg/v2/"+ecId+"/meta", "")
	require.Equal(t, 200, rec.Code)
	var meta map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &meta))
	assert.Contains(t, meta, meter)
	assert.NotContains(t, meta, otherMeter)

	rec = send(h, memberAuth, "POST", perMeter(meter)["report"][0], perMeter(meter)["report"][1])
	require.Equal(t, 200, rec.Code)
	var report struct {
		Meta []struct {
			Name string `json:"name"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &report))
	require.Len(t, report.Meta, 1)
	assert.Equal(t, meter, report.Meta[0].Name)
}

func TestMemberTotalsUnchanged(t *testing.T) {
	// no backend configured: the totals do not ask it
	h := memberRouter(t, "")
	for _, rt := range routes() {
		switch rt.template {
		case "/eeg/v2/{ecid}/summary", "/eeg/v2/{ecid}/combined-report", "/eeg/v2/{ecid}/intra-day-report",
			"/eeg/v2/{ecid}/load-curve-report", "/eeg/{ecid}/lastRecordDate":
			rec := call(t, h, rt, memberAuth, tenant)
			assert.Equal(t, rt.ok, rec.Code, "%s: %s", rt.name(), truncate(rec.Body.String()))
		}
	}
}

func TestAdminOwnerUnchanged(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	super := "Bearer " + fakeoidc.Token(map[string]any{"tenant": []string{foreign}, "sub": "super-1",
		"realm_access": map[string]any{"roles": []string{"superuser"}}})
	for _, auth := range []string{groupToken("/EEG_ADMIN"), groupToken("/EEG_USER", "/EEG_ADMIN"),
		groupToken("/EEG_OWNER"), groupToken("/EEG_USER", "/EEG_OWNER"), super} {
		for name, rt := range perMeter(otherMeter) {
			rec := send(h, auth, "POST", rt[0], rt[1])
			assert.Equal(t, 200, rec.Code, "%s: %s", name, truncate(rec.Body.String()))
		}
		rec := send(h, auth, "GET", "/eeg/v2/"+ecId+"/meta", "")
		assert.Contains(t, rec.Body.String(), otherMeter)
	}
	assert.Equal(t, int32(0), b.calls.Load(), "the backend is asked for members only")
}

func TestTokenWithoutGroupIsRestricted(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	noGroup := groupToken()
	for name, rt := range perMeter(otherMeter) {
		assert.Equal(t, 403, send(h, noGroup, "POST", rt[0], rt[1]).Code, name)
	}
	for name, rt := range perMeter(meter) {
		assert.Equal(t, 200, send(h, noGroup, "POST", rt[0], rt[1]).Code, name)
	}
	assert.Equal(t, 403, send(h, noGroup, "POST", "/eeg/"+ecId+"/excel/export/2026/6", "").Code)
}

func TestMemberBackendDownIs503(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	b.Close()
	for name, rt := range perMeter(meter) {
		assert.Equal(t, 503, send(h, memberAuth, "POST", rt[0], rt[1]).Code, name)
	}
	assert.Equal(t, 503, send(h, memberAuth, "GET", "/eeg/v2/"+ecId+"/meta", "").Code)
}

func TestMemberBackendErrorIs503(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(backend.Close)
	h := memberRouter(t, backend.URL)
	rt := perMeter(meter)["raw"]
	assert.Equal(t, 503, send(h, memberAuth, "POST", rt[0], rt[1]).Code)
}

func TestMemberWithoutBackendConfigIs503(t *testing.T) {
	h := memberRouter(t, "")
	for name, rt := range perMeter(meter) {
		assert.Equal(t, 503, send(h, memberAuth, "POST", rt[0], rt[1]).Code, name)
	}
}

func TestMemberWritesAndCommunityExportsAre403(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)

	rec := send(h, memberAuth, "POST", "/eeg/"+ecId+"/excel/export/2026/6", "")
	assert.Equal(t, 403, rec.Code, "export of every metering point by mail")

	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	rec = send(h, memberAuth, "POST", "/eeg/v2/"+ecId+"/rawdata/delete",
		fmt.Sprintf(`{"meteringPoint":"%s","start":%d,"end":%d,"dryRun":true}`, meter, start, end))
	assert.Equal(t, 403, rec.Code, "raw data delete")

	body, ct := uploadBody(t, tenant)
	rec = apitest.Do(h, apitest.Request{Method: "POST", Path: "/query", RawBody: body, ContentType: ct,
		Header: map[string]string{"Authorization": memberAuth, "tenant": tenant}})
	var resp gqlResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Errors, "singleUpload")
	assert.Equal(t, "not permitted for members", resp.Errors[0].Message)
	assert.Equal(t, int32(0), b.calls.Load())
}

// The Basic-auth /query API keeps its behaviour for users without a group or with /EEG_USER: it is not
// restricted and does not ask the backend.
func TestBasicApiUnrestricted(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	for user, claims := range map[string]map[string]any{
		"api-nogroup": {"tenant": []string{tenant}},
		"api-member":  {"tenant": []string{tenant}, "access_groups": []string{"/EEG_USER"}},
	} {
		fakeoidc.SetUser(user, apiPass, claims)
		for _, path := range []string{"/query/rawdata", "/query/" + ecId + "/metadata"} {
			rec := apitest.Do(h, apitest.Request{Method: "POST", Path: path,
				Body:   fmt.Sprintf(`{"cps":[{"meteringPoint":"%s"}],"ecId":"%s","start":%d,"end":%d}`, otherMeter, ecId, start, end),
				Header: map[string]string{"Authorization": apitest.Basic(user, apiPass), "X-Tenant": tenant}})
			assert.Equal(t, 200, rec.Code, "%s %s", user, path)
			assert.Contains(t, rec.Body.String(), otherMeter, "%s %s", user, path)
		}
	}
	assert.Equal(t, int32(0), b.calls.Load())
}
