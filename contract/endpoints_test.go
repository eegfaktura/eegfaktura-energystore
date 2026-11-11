package contract_test

import (
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallerEndpointContract: every call of the inventory (method and path template) has a route;
// a renamed path on either side names the caller's file and line.
func TestCallerEndpointContract(t *testing.T) {
	web, v3 := webCalls(t), v3Calls(t)
	// Inventory counted 2026-10-03 (m06): web 9 calls (7 REST, /query twice), v3 8 calls.
	assert.Len(t, web, 9, "eegfaktura-web calls in energy.service.ts")
	assert.Len(t, v3, 8, "eegfaktura-v3 calls in EnergyStoreClient.kt")

	router := apitest.Router()
	for _, c := range append(web, v3...) {
		req := httptest.NewRequest(c.method, strings.ReplaceAll(c.path, "{ecid}", "RC100001"), nil)
		var m mux.RouteMatch
		ok := router.Match(req, &m)
		assert.True(t, ok && m.MatchErr == nil, "no route for %s", c)
		if ok && m.MatchErr == nil {
			tpl, _ := m.Route.GetPathTemplate()
			assert.Equal(t, c.path, tpl, "%s matches another template", c)
		}
	}
}

var (
	gqlField  = regexp.MustCompile(`(\w+)\(([^)]*)\)`)
	gqlArg    = regexp.MustCompile(`(\w+)\s*:`)
	webGQLUse = regexp.MustCompile(`(lastEnergyDate|singleUpload)\(([^)]*)\)`)
)

// TestGraphQLContract: the two GraphQL fields the web sends exist with the same argument names.
func TestGraphQLContract(t *testing.T) {
	schema, err := os.ReadFile("../graph/schema.graphqls")
	require.NoError(t, err)
	args := map[string][]string{}
	for _, m := range gqlField.FindAllStringSubmatch(string(schema), -1) {
		for _, a := range gqlArg.FindAllStringSubmatch(m[2], -1) {
			args[m[1]] = append(args[m[1]], a[1])
		}
	}
	src := lines(t, "testdata/web/graphql-query.ts")
	found := 0
	for i, l := range src {
		m := webGQLUse.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		found++
		var sent []string
		for _, a := range gqlArg.FindAllStringSubmatch(m[2], -1) {
			sent = append(sent, a[1])
		}
		sort.Strings(sent)
		want := append([]string{}, args[m[1]]...)
		sort.Strings(want)
		assert.Equal(t, want, sent, "eegfaktura-web src/service/graphql-query.ts:%d %s", i+1, m[1])
	}
	assert.Equal(t, 2, found, "lastEnergyDate and singleUpload")
}
