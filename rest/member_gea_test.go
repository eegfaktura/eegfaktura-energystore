package rest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GEA (energystore 261025-ca1): the web addresses energystore with the RC number (here tenant), the
// backend keeps the member's participant under the GEA tenant GC123456-001. The lookup asks the backend
// for every tenant of the token and keeps only the meters the requested community has.
func TestMemberOfGeaCommunity(t *testing.T) {
	const unstored = "AT0030000000000000000000000000099" // the member's, but not in this community
	var asked []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.Header.Get("tenant"))
		if r.Header.Get("tenant") != "GC123456-001" {
			_, _ = fmt.Fprint(w, `[]`)
			return
		}
		_, _ = fmt.Fprintf(w, `[{"meters":[`+
			`{"meteringPoint":"%s","participantState":{"activeSince":"2026-01-01","inactiveSince":null}},`+
			`{"meteringPoint":"%s","participantState":{"activeSince":"2026-01-01","inactiveSince":null}}]}]`, meter, unstored)
	}))
	t.Cleanup(backend.Close)
	h := memberRouter(t, backend.URL)
	gea := "Bearer " + fakeoidc.Token(map[string]any{
		"tenant": []string{tenant, "GC123456-001"}, "access_groups": []string{"/EEG_USER"}, "sub": "gea-member",
		"email": "member@example.org", "email_verified": true, "realm_access": map[string]any{"roles": []string{}},
	})

	for name, rt := range perMeter(meter) {
		rec := send(h, gea, "POST", rt[0], rt[1])
		assert.Equal(t, 200, rec.Code, "%s: %s", name, truncate(rec.Body.String()))
	}
	require.ElementsMatch(t, []string{tenant, "GC123456-001"}, asked, "one call per tenant, then cached")
	for _, cp := range []string{otherMeter, unstored} {
		rt := perMeter(cp)["raw"]
		assert.Equal(t, 403, send(h, gea, "POST", rt[0], rt[1]).Code, cp)
	}
}
