package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// accessLine returns the http.access line of the request with X-Request-Id id.
func (b *lockedBuffer) accessLine(t *testing.T, id string) (string, map[string]any) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, line := range strings.Split(b.buf.String(), "\n") {
		m := map[string]any{}
		if json.Unmarshal([]byte(line), &m) == nil && m["event.action"] == "http.access" && m["request.id"] == id {
			return line, m
		}
	}
	t.Fatalf("no access line for %s", id)
	return "", nil
}

// TestAccessLineThroughTheRouter: the access line of the real routes (server.go wraps the router in
// logging.AccessLog) carries the route template, the checked tenant and the Keycloak client, for the
// bearer REST routes, GraphQL and the Basic-auth /query API, and nothing about the person.
func TestAccessLineThroughTheRouter(t *testing.T) {
	seed(t)
	out := &lockedBuffer{}
	require.NoError(t, logging.Configure([]string{"LOG_LEVEL=info"}, out))
	t.Cleanup(func() { _ = logging.Configure(os.Environ(), os.Stdout) })
	h := logging.AccessLog(apitest.Router())

	token := fakeoidc.Token(map[string]any{
		"tenant": []string{tenant}, "realm_access": map[string]any{"roles": []string{}},
		"access_groups": []string{"/EEG_ADMIN"}, "azp": "at.ourproject.vfeeg.app",
		"email": "person@example.org", "preferred_username": "person.name",
	})
	cases := []struct {
		name, id string
		req      apitest.Request
		status   int
		route    string
		client   string
		op       string
	}{
		{"rest bearer", "t-rest", apitest.Request{Method: "GET", Path: "/eeg/" + ecId + "/lastRecordDate",
			Header: map[string]string{"Authorization": "Bearer " + token, "tenant": strings.ToLower(tenant)}},
			200, "/eeg/{ecid}/lastRecordDate", "at.ourproject.vfeeg.app", ""},
		{"graphql", "t-gql", apitest.Request{Method: "POST", Path: "/query",
			Header: map[string]string{"Authorization": "Bearer " + token, "tenant": tenant},
			Body:   fmt.Sprintf(`{"operationName":"Last","query":"query Last { lastEnergyDate(tenant: \"%s\", ecId: \"%s\") }"}`, tenant, ecId)},
			200, "/query", "at.ourproject.vfeeg.app", "Last"},
		{"basic api", "t-api", apitest.Request{Method: "POST", Path: "/query/" + ecId + "/metadata",
			Header: map[string]string{"Authorization": apitest.Basic(apiUser, apiPass), "X-Tenant": tenant}},
			200, "/query/{ecid}/metadata", fakeoidc.APIClientID, ""},
	}
	for _, c := range cases {
		c.req.Header["X-Request-Id"] = c.id
		rec := apitest.Do(h, c.req)
		require.Equal(t, c.status, rec.Code, c.name)
		line, m := out.accessLine(t, c.id)
		assert.Equal(t, c.route, m["http.route"], c.name)
		assert.Equal(t, "success", m["event.outcome"], c.name)
		assert.Equal(t, tenant, m["tenant.id"], c.name)
		assert.Equal(t, c.client, m["client.id"], c.name)
		if c.op != "" {
			assert.Equal(t, c.op, m["graphql.operation.name"], c.name)
		}
		for _, s := range []string{"person@example.org", "person.name", apiUser, apiPass, token, ecId} {
			assert.NotContains(t, line, s, c.name)
		}
	}

	// refused: no tenant (it was not confirmed), outcome refused
	rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/query/" + ecId + "/metadata",
		Header: map[string]string{"Authorization": apitest.Basic(apiUser, "wrong"), "X-Tenant": tenant, "X-Request-Id": "t-refused"}})
	require.Equal(t, 403, rec.Code)
	line, m := out.accessLine(t, "t-refused")
	assert.Equal(t, "refused", m["event.outcome"])
	assert.NotContains(t, m, "tenant.id")
	assert.NotContains(t, line, apiUser)
}
