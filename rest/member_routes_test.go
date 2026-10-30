package rest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/stretchr/testify/assert"
)

// memberClass is how an entry point treats a member token (energystore 261025-ca1).
type memberClass int

const (
	perMeterOwn    memberClass = iota + 1 // the member's own metering points only (403 for others, meta filtered)
	communityTotal                        // whole-community totals without meter ids: unchanged
	adminOnly                             // writes, imports, exports of every meter: 403 for members
)

// memberClasses classifies every entry point of routes() (which TestRouteWalkGuard keeps complete): a new
// route without a class fails TestEveryRouteIsClassifiedForMembers.
var memberClasses = map[string]memberClass{
	"POST /eeg/v2/{ecid}/report":                   perMeterOwn,
	"GET /eeg/v2/{ecid}/meta":                      perMeterOwn,
	"POST /eeg/v2/{ecid}/raw":                      perMeterOwn,
	"POST /eeg/v2/{ecid}/billing-check":            perMeterOwn,
	"POST /eeg/{ecid}/excel/report/download":       perMeterOwn,
	"POST /eeg/v2/{ecid}/intra-day-report":         communityTotal,
	"POST /eeg/v2/{ecid}/load-curve-report":        communityTotal,
	"GET /eeg/v2/{ecid}/load-curve-report":         communityTotal,
	"POST /eeg/v2/{ecid}/combined-report":          communityTotal,
	"GET /eeg/v2/{ecid}/combined-report":           communityTotal,
	"POST /eeg/v2/{ecid}/summary":                  communityTotal,
	"GET /eeg/{ecid}/lastRecordDate":               communityTotal,
	"POST /query lastEnergyDate":                   communityTotal,
	"POST /eeg/v2/{ecid}/rawdata/delete":           adminOnly,
	"POST /eeg/{ecid}/excel/export/{year}/{month}": adminOnly,
	"POST /query singleUpload":                     adminOnly,
	"POST /query/rawdata":                          adminOnly,
	"POST /query/{ecid}/metadata":                  adminOnly,
}

func TestEveryRouteIsClassifiedForMembers(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	fakeoidc.SetUser("api-member", apiPass, map[string]any{"tenant": []string{tenant}, "access_groups": []string{"/EEG_USER"}})

	for _, rt := range routes() {
		class, ok := memberClasses[rt.name()]
		if !ok {
			t.Errorf("%s: no member class - classify it in memberClasses", rt.name())
			continue
		}
		switch class {
		case perMeterOwn:
			foreign := rt
			foreign.path = strings.ReplaceAll(rt.path, meter, otherMeter)
			foreign.body = strings.ReplaceAll(rt.body, meter, otherMeter)
			rec := call(t, h, foreign, memberAuth, tenant)
			if rt.body == "" { // names no meter: the answer lists the member's own only
				assert.Equal(t, 200, rec.Code, rt.name())
				assert.NotContains(t, rec.Body.String(), otherMeter, rt.name())
				continue
			}
			assert.Equal(t, 403, rec.Code, "%s: %s", rt.name(), truncate(rec.Body.String()))
		case communityTotal:
			rec := call(t, h, rt, memberAuth, tenant)
			assert.Equal(t, rt.ok, rec.Code, "%s: %s", rt.name(), truncate(rec.Body.String()))
		case adminOnly:
			auth := memberAuth
			if rt.guard == api {
				auth = apitest.Basic("api-member", apiPass)
			}
			rec := call(t, h, rt, auth, tenant)
			if rt.guard == gql {
				var resp gqlResponse
				_ = json.Unmarshal(rec.Body.Bytes(), &resp)
				assert.NotEmpty(t, resp.Errors, rt.name())
				continue
			}
			assert.Equal(t, 403, rec.Code, "%s: %s", rt.name(), truncate(rec.Body.String()))
		}
	}
	assert.Len(t, memberClasses, len(routes()), "a class for a route that no longer exists")
}
