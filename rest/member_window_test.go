package rest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// energystore 261025-ca1: a member reads a metering point only for the time it was its own (the
// participation window the backend reports).

func TestMemberOutsideWindowIs403(t *testing.T) {
	// the stored day is 01.06.2026
	for _, window := range [][2]string{{`"2026-06-02"`, "null"}, {`"2026-01-01"`, `"2026-05-31"`}} {
		b := startBackendWindow(t, window[0], window[1])
		h := memberRouter(t, b.URL)
		for name, rt := range perMeter(meter) {
			rec := send(h, memberAuth, "POST", rt[0], rt[1])
			assert.Equal(t, 403, rec.Code, "%s %v", name, window)
			assert.JSONEq(t, `{"error":"metering point not permitted"}`, rec.Body.String(), "%s %v", name, window)
		}
	}
}

func TestMemberPartlyOutsideWindowIsClamped(t *testing.T) {
	b := startBackendWindow(t, `"2026-06-01"`, `"2026-06-01"`)
	h := memberRouter(t, b.URL)
	from, until := ms(day.AddDate(0, 0, -5)), ms(day.AddDate(0, 0, 5))
	for name, rt := range map[string][2]string{
		"report": {"/eeg/v2/" + ecId + "/report", `{"reportInterval":{"year":2026,"segment":6,"type":"YM"},"participants":[{"participantId":"P1","meters":[{"meterId":"` +
			meter + `","meterDir":"CONSUMPTION","from":0,"until":` + fmt.Sprint(until) + `}]}]}`},
		"raw": {"/eeg/v2/" + ecId + "/raw", fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, from, until)},
		"billing-check": {"/eeg/v2/" + ecId + "/billing-check", fmt.Sprintf(`{"start":%d,"end":%d,"cps":[{"meteringPoint":"%s","direction":"CONSUMPTION","activeSince":%d,"inactiveSince":0}]}`,
			from, until, meter, from)},
		"excel report": {"/eeg/" + ecId + "/excel/report/download", fmt.Sprintf(`{"start":%d,"end":%d,"communityId":"%s","cps":[{"meteringPoint":"%s","direction":"CONSUMPTION","activeSince":%d,"inactiveSince":%d}]}`,
			from, until, ecId, meter, from, until)},
	} {
		rec := send(h, memberAuth, "POST", rt[0], rt[1])
		assert.Equal(t, 200, rec.Code, "%s: %s", name, truncate(rec.Body.String()))
	}
	rec := send(h, memberAuth, "POST", "/eeg/v2/"+ecId+"/raw", fmt.Sprintf(`{"meters":["%s"],"start":%d,"end":%d}`, meter, from, until))
	assert.Contains(t, rec.Body.String(), meter, "the own day is in the clamped range")
}

// An open end (inactiveSince 0) stays 0 for a member whose participation has no end: the billing check
// answers the member exactly as an admin.
func TestMemberBillingCheckOpenEndAsAdmin(t *testing.T) {
	b := startBackend(t) // active since 01.01.2026, no end
	h := memberRouter(t, b.URL)
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	body := fmt.Sprintf(`{"start":%d,"end":%d,"cps":[{"meteringPoint":"%s","direction":"CONSUMPTION","activeSince":%d,"inactiveSince":0}]}`,
		start, end, meter, start)
	path := "/eeg/v2/" + ecId + "/billing-check"
	member := send(h, memberAuth, "POST", path, body)
	admin := send(h, groupToken("/EEG_ADMIN"), "POST", path, body)
	assert.Equal(t, 200, member.Code, member.Body.String())
	assert.JSONEq(t, admin.Body.String(), member.Body.String())
}

// The backend matches the member by the token's email: a member token without one gets 503 without a
// backend call.
func TestMemberWithoutEmailIs503(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	noEmail := "Bearer " + fakeoidc.Token(map[string]any{"tenant": []string{tenant}, "access_groups": []string{"/EEG_USER"},
		"sub": "member-2", "realm_access": map[string]any{"roles": []string{}}})
	for name, rt := range perMeter(meter) {
		assert.Equal(t, 503, send(h, noEmail, "POST", rt[0], rt[1]).Code, name)
	}
	assert.Equal(t, int32(0), b.calls.Load())
}

func TestMemberNamingNoMeterAsksNoBackend(t *testing.T) {
	h := memberRouter(t, "") // not configured: any backend lookup would answer 503
	start, end := ms(day), ms(day.Add(23*time.Hour+45*time.Minute))
	for name, rt := range map[string][2]string{
		"report": {"/eeg/v2/" + ecId + "/report", `{"reportInterval":{"year":2026,"segment":6,"type":"YM"},"participants":[]}`},
		"raw":    {"/eeg/v2/" + ecId + "/raw", fmt.Sprintf(`{"meters":[],"start":%d,"end":%d}`, start, end)},
	} {
		rec := send(h, memberAuth, "POST", rt[0], rt[1])
		assert.Equal(t, 200, rec.Code, "%s: %s", name, truncate(rec.Body.String()))
		assert.NotContains(t, rec.Body.String(), otherMeter, name)
	}
}

func TestMemberMalformedRequestIs400(t *testing.T) {
	h := memberRouter(t, "") // validation comes before the backend lookup
	tw := `{"key":"a","from":"08:00","to":"10:00"}`
	rec := send(h, memberAuth, "POST", "/eeg/v2/"+ecId+"/report", `{"reportInterval":{"year":2026,"segment":6,"type":"YM"},"participants":[{"participantId":"P1","meters":[{"meterId":"`+
		meter+`","meterDir":"CONSUMPTION","from":0,"until":0,"timeWindows":[`+tw+`,`+tw+`,`+tw+`]}]}]}`)
	assert.Equal(t, 400, rec.Code, "report: %s", rec.Body.String())
	rec = send(h, memberAuth, "POST", "/eeg/v2/"+ecId+"/billing-check", fmt.Sprintf(`{"start":0,"end":0,"cps":[{"meteringPoint":"%s","direction":"CONSUMPTION"}]}`, meter))
	assert.Equal(t, 400, rec.Code, "billing-check: %s", rec.Body.String())
}

// A community without any energy data (no meta record) answers a member's report like an admin's: an
// empty report, no 403 and no backend-dependent failure.
func TestMemberCommunityWithoutData(t *testing.T) {
	b := startBackend(t)
	testsupport.UseTempPersistence(t)
	fakeoidc.ResetUsers()
	apitest.StartFakes(t)
	viper.Set("services.backend-api", b.URL)
	t.Cleanup(func() { viper.Set("services.backend-api", "") })
	h := apitest.Router()
	rt := perMeter(meter)["report"]
	member := send(h, memberAuth, "POST", rt[0], rt[1])
	admin := send(h, groupToken("/EEG_ADMIN"), "POST", rt[0], rt[1])
	assert.Equal(t, admin.Code, member.Code, "member %s admin %s", member.Body.String(), admin.Body.String())
	assert.Equal(t, 200, member.Code, member.Body.String())
}

// Decided 2026-10-30: without activeSince only status ACTIVE/INACTIVE starts at registeredSince (not a
// placeholder before 2000); INIT, NEW or an empty status has no window.
func TestMemberMeterWithoutActiveSince(t *testing.T) {
	// the stored day is 01.06.2026; active/inactive are JSON values
	for _, c := range []struct {
		status, registered, active, inactive string
		want                                 int
	}{
		{"ACTIVE", "2025-01-01", "null", "null", 200},
		{"INACTIVE", "2025-01-01", "null", `"2026-12-31"`, 200},
		{"INACTIVE", "2025-01-01", "null", "null", 403},                 // INACTIVE without an end
		{"INACTIVE", "2025-01-01", `"2026-01-01"`, `"2999-12-31"`, 403}, // archived-like
		{"ACTIVE", "0001-01-01", "null", "null", 403},
		{"ACTIVE", "1999-12-31", "null", "null", 403},         // registeredSince before 2000: placeholder
		{"ACTIVE", "2024-03-01", `"1970-01-01"`, "null", 200}, // activeSince placeholder: registeredSince
		{"ACTIVE", "2026-07-01", `"1970-01-01"`, "null", 403}, // ... which starts after the stored day
		{"INIT", "2025-01-01", "null", "null", 403},
		{"NEW", "2025-01-01", "null", "null", 403},
		{"", "2025-01-01", "null", "null", 403},
	} {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(w, `[{"meters":[{"meteringPoint":"%s","status":"%s","registeredSince":"%s",`+
				`"participantState":{"activeSince":%s,"inactiveSince":%s}}]}]`, meter, c.status, c.registered, c.active, c.inactive)
		}))
		t.Cleanup(backend.Close)
		h := memberRouter(t, backend.URL)
		rt := perMeter(meter)["raw"]
		assert.Equal(t, c.want, send(h, memberAuth, "POST", rt[0], rt[1]).Code, "%+v", c)
	}
}

// A member is looked up only with a verified email (energystore 261025-ca1): email_verified false or
// missing → 403 on per-meter routes, without a backend call; totals and admins are unaffected.
func TestMemberWithUnverifiedEmailIs403(t *testing.T) {
	b := startBackend(t)
	h := memberRouter(t, b.URL)
	for name, verified := range map[string]any{"false": false, "missing": nil} {
		claims := map[string]any{"tenant": []string{tenant}, "access_groups": []string{"/EEG_USER"}, "sub": "member-3",
			"email": "member@example.org", "realm_access": map[string]any{"roles": []string{}}}
		if verified != nil {
			claims["email_verified"] = verified
		}
		auth := "Bearer " + fakeoidc.Token(claims)
		for route, rt := range perMeter(meter) {
			rec := send(h, auth, "POST", rt[0], rt[1])
			assert.Equal(t, 403, rec.Code, "%s %s", name, route)
		}
		rec := send(h, auth, "POST", "/eeg/v2/"+ecId+"/summary", `{"year":2026,"segment":6,"type":"YM"}`)
		assert.Equal(t, 200, rec.Code, "%s: totals unaffected", name)
	}
	adminUnverified := "Bearer " + fakeoidc.Token(map[string]any{"tenant": []string{tenant}, "access_groups": []string{"/EEG_ADMIN"},
		"sub": "admin-1", "email": "admin@example.org", "email_verified": false})
	for route, rt := range perMeter(otherMeter) {
		assert.Equal(t, 200, send(h, adminUnverified, "POST", rt[0], rt[1]).Code, "admin %s", route)
	}
	assert.Equal(t, int32(0), b.calls.Load())
}
