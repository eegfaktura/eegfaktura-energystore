package rest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	tenant  = "TE100001"
	foreign = "TE100002"
	ecId    = "RC100001"
	meter   = "AT0030000000000000000000000000001"
	apiUser = "api-user"
	apiPass = "api-secret"
)

var day = time.Date(2026, 6, 1, 0, 0, 0, 0, testsupport.Vienna)

// seed stores one consumer with one day of quarter-hour values for tenant/ecId in a temp store
// and starts the gRPC fakes, so the success cases of every route return data.
func seed(t *testing.T) *apitest.Fakes {
	t.Helper()
	db := testsupport.TempStore(t, tenant, ecId)
	require.NoError(t, db.SetMeta(testsupport.CpMeta(testsupport.Consumer(meter, 0, "01.06.2026 00:00:00", "01.06.2026 23:45:00"))))
	for i, ts := range testsupport.QuarterHours(day) {
		v := float64(i+1) / 100
		require.NoError(t, db.SetLine(testsupport.RawLine(testsupport.RowId(ts), []float64{v, v / 2, v / 4}, nil)))
	}
	fakeoidc.ResetUsers()
	fakeoidc.SetUser(apiUser, apiPass, map[string]any{"tenant": []string{tenant}})
	return apitest.StartFakes(t)
}

type guard int

const (
	app guard = iota // middleware.ProtectApp
	api              // middleware.ProtectApi (Basic credentials, password grant)
	gql              // middleware.GQLProtect
)

// route is one entry point of the matrix: method, path template, the path used in the test and a
// request body that the handler accepts.
type route struct {
	method, template, path string
	guard                  guard
	body                   string
	contentType            string
	ok                     int    // status of the success case
	ownStatus              int    // status for the own tenant without a role, if not ok
	defect                 string // known-errors row that makes the success case fail today
}

func ms(t time.Time) int64 { return t.UnixMilli() }

// routes lists all 17 entry points (13 ProtectApp, 2 ProtectApi, 2 GraphQL fields), counted on
// 2026-10-02 (m02). TestRouteWalkGuard fails when the router has a route this list lacks.
func routes() []route {
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	window := fmt.Sprintf(`{"start":%d,"end":%d}`, start, end)
	report := `{"reportInterval":{"year":2026,"segment":6,"type":"YM"},"participants":[{"participantId":"P1","meters":[{"meterId":"` +
		meter + `","meterDir":"CONSUMPTION","from":` + fmt.Sprint(start) + `,"until":` + fmt.Sprint(end) + `}]}]}`
	q := fmt.Sprintf("start=%d&end=%d", start, end)
	v2 := "/eeg/v2/" + ecId
	return []route{
		{method: "POST", template: "/eeg/v2/{ecid}/report", path: v2 + "/report", body: report, ok: 200},
		{method: "GET", template: "/eeg/v2/{ecid}/meta", path: v2 + "/meta", ok: 200},
		{method: "POST", template: "/eeg/v2/{ecid}/raw", path: v2 + "/raw", body: fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, start, end), ok: 200},
		{method: "POST", template: "/eeg/v2/{ecid}/rawdata/delete", path: v2 + "/rawdata/delete",
			body: fmt.Sprintf(`{"meteringPoint":"%s","start":%d,"end":%d,"dryRun":true}`, meter, start, end), ok: 200, ownStatus: 403},
		{method: "POST", template: "/eeg/v2/{ecid}/intra-day-report", path: v2 + "/intra-day-report", body: window, ok: 200},
		{method: "POST", template: "/eeg/v2/{ecid}/load-curve-report", path: v2 + "/load-curve-report", body: window, ok: 200},
		{method: "GET", template: "/eeg/v2/{ecid}/load-curve-report", path: v2 + "/load-curve-report?" + q, ok: 200},
		{method: "POST", template: "/eeg/v2/{ecid}/combined-report", path: v2 + "/combined-report", body: fmt.Sprintf(`{"reports":[],"start":%d,"end":%d}`, start, end), ok: 200},
		{method: "GET", template: "/eeg/v2/{ecid}/combined-report", path: v2 + "/combined-report?" + q, ok: 200},
		{method: "POST", template: "/eeg/v2/{ecid}/summary", path: v2 + "/summary", body: `{"year":2026,"segment":6,"type":"YM"}`, ok: 200},
		{method: "GET", template: "/eeg/{ecid}/lastRecordDate", path: "/eeg/" + ecId + "/lastRecordDate", ok: 200},
		{method: "POST", template: "/eeg/{ecid}/excel/export/{year}/{month}", path: "/eeg/" + ecId + "/excel/export/2026/6", ok: 200, defect: "#53"},
		{method: "POST", template: "/eeg/{ecid}/excel/report/download", path: "/eeg/" + ecId + "/excel/report/download",
			body: fmt.Sprintf(`{"start":%d,"end":%d,"communityId":"%s","cps":[{"meteringPoint":"%s","direction":"CONSUMPTION","activeSince":%d,"inactiveSince":%d}]}`,
				start, end, ecId, meter, start, end), ok: 200},
		{method: "POST", template: "/query/rawdata", path: "/query/rawdata", guard: api,
			body: fmt.Sprintf(`{"cps":[{"meteringPoint":"%s"}],"ecId":"%s","start":%d,"end":%d}`, meter, ecId, start, end), ok: 200},
		{method: "POST", template: "/query/{ecid}/metadata", path: "/query/" + ecId + "/metadata", guard: api, ok: 200},
		{method: "POST", template: "/query", path: "/query", guard: gql,
			body: fmt.Sprintf(`{"query":"{ lastEnergyDate(tenant: \"%s\", ecId: \"%s\") }"}`, tenant, ecId), ok: 200},
		{method: "POST", template: "/query", path: "/query", guard: gql, contentType: "multipart",
			body: "singleUpload", ok: 200},
	}
}

// call sends the route's success request with the given authorization and tenant headers.
func call(t *testing.T, h http.Handler, rt route, auth, tenantHeader string) *httptest.ResponseRecorder {
	t.Helper()
	req := apitest.Request{Method: rt.method, Path: rt.path, Body: rt.body, Header: map[string]string{}}
	if rt.contentType == "multipart" {
		req.RawBody, req.ContentType = uploadBody(t, tenantOr(tenantHeader))
		req.Body = ""
	}
	if auth != "" {
		req.Header["Authorization"] = auth
	}
	if tenantHeader != "" {
		if rt.guard == api {
			req.Header["X-Tenant"] = tenantHeader
		} else {
			req.Header["tenant"] = tenantHeader
		}
	}
	return apitest.Do(h, req)
}

func (rt route) own() int {
	if rt.ownStatus != 0 {
		return rt.ownStatus
	}
	return rt.ok
}

func tenantOr(s string) string {
	if s == "" {
		return tenant
	}
	return s
}

func (rt route) name() string {
	n := rt.method + " " + rt.template
	if rt.contentType == "multipart" {
		n += " singleUpload"
	} else if rt.guard == gql {
		n += " lastEnergyDate"
	}
	return n
}

// TestAuthMatrix applies the guard cases of m02 to every entry point. The expected status codes
// are what the three guards answer today (read in middleware/authentication.go and
// api_authentication.go); the differences between the guards are recorded in m02, not changed.
func TestAuthMatrix(t *testing.T) {
	seed(t)
	h := apitest.Router()
	own := "Bearer " + apitest.AppToken([]string{tenant})
	other := "Bearer " + apitest.AppToken([]string{foreign})
	super := "Bearer " + apitest.AppToken([]string{foreign}, "superuser")
	expired := "Bearer " + fakeoidc.Token(map[string]any{"tenant": []string{tenant}, "exp": time.Now().Add(-time.Hour).Unix()})

	for _, rt := range routes() {
		t.Run(rt.name(), func(t *testing.T) {
			type c struct {
				name, auth, tenant string
				app, api, gql      int
			}
			cases := []c{
				{"no Authorization", "", tenant, 403, 403, 403},
				{"wrong scheme", "Token abc", tenant, 403, 400, 403},
				{"invalid token", "Bearer not-a-jwt", tenant, 401, 0, 401},
				{"expired token", expired, tenant, 401, 0, 401},
				{"failed password grant", apitest.Basic(apiUser, "wrong"), tenant, 0, 403, 0},
				{"foreign tenant in the header", other, tenant, 403, 0, 401},
				{"no tenant header", own, "", 403, 0, 403},
				{"api: foreign tenant header", apitest.Basic(apiUser, apiPass), foreign, 0, 403, 0},
				{"api: no X-Tenant", apitest.Basic(apiUser, apiPass), "", 0, 403, 0},
				{"own tenant", own, tenant, rt.own(), 0, rt.ok},
				{"own tenant, lower case header", own, strings.ToLower(tenant), rt.own(), 0, rt.ok},
				{"superuser with a foreign tenant", super, tenant, rt.ok, 0, rt.ok},
				{"api: own tenant", apitest.Basic(apiUser, apiPass), tenant, 0, rt.ok, 0},
				{"api: Bearer instead of Basic", own, tenant, 0, 400, 0},
			}
			for _, tc := range cases {
				want := map[guard]int{app: tc.app, api: tc.api, gql: tc.gql}[rt.guard]
				if want == 0 {
					continue // case does not apply to this guard
				}
				rec := call(t, h, rt, tc.auth, tc.tenant)
				if rt.defect != "" && want == rt.ok {
					// the guard let the request through; the handler's answer is the defect test of rt.defect
					assert.NotContains(t, []int{400, 401, 403}, rec.Code, "%s (known-errors %s): body %s", tc.name, rt.defect, truncate(rec.Body.String()))
					continue
				}
				assert.Equal(t, want, rec.Code, "%s: body %s", tc.name, truncate(rec.Body.String()))
			}
		})
	}
}

func truncate(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// TestRouteWalkGuard fails when the router has a route that routes() does not list: a new
// entry point without the matrix is red (m02).
func TestRouteWalkGuard(t *testing.T) {
	listed := map[string]bool{}
	for _, rt := range routes() {
		listed[rt.method+" "+rt.template] = true
	}
	var unlisted []string
	seen := map[string]bool{}
	err := apitest.Router().Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, err := route.GetMethods()
		if err != nil {
			methods = []string{"POST"} // /query (GraphQL) has no method matcher; the tests use POST
		}
		for _, m := range methods {
			if seen[m+" "+tpl] {
				continue // "/query" is the GraphQL handler and the prefix of the /query/* subrouter
			}
			seen[m+" "+tpl] = true
			if !listed[m+" "+tpl] {
				unlisted = append(unlisted, m+" "+tpl)
			}
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(unlisted)
	assert.Empty(t, unlisted, "routes without the auth matrix")
	assert.Len(t, seen, 16, "15 REST routes and /query (two GraphQL fields)")
}
