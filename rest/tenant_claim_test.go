package rest_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/internal/testsupport/fakeoidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The `tenant` claim arrives in several shapes depending on the Keycloak mapper (known-errors #14):
// a flat array, a nested array (multivalued JSON mapper over one JSON-array attribute value) or a
// plain string (single-valued mapper). Every shape is accepted; the comparison with the tenant header
// stays case-insensitive and a foreign tenant stays refused.
var tenantClaimShapes = []struct {
	name  string
	claim any
	ok    bool
}{
	{"flat array", []string{tenant}, true},
	{"nested array", [][]string{{tenant}}, true},
	{"nested array, several tenants, lower case", [][]string{{foreign, "te100001"}}, true},
	{"nested arrays, one per value", [][]string{{foreign}, {tenant}}, true},
	{"string", tenant, true},
	{"string, lower case", "te100001", true},
	{"foreign flat array", []string{foreign}, false},
	{"foreign nested array", [][]string{{foreign}}, false},
	{"foreign string", foreign, false},
	{"empty array", []string{}, false},
	{"empty string", "", false},
	{"null", nil, false},
}

func TestTenantClaimShapesApp(t *testing.T) {
	seed(t)
	h := apitest.Router()
	for _, tc := range tenantClaimShapes {
		token := fakeoidc.Token(map[string]any{"tenant": tc.claim, "realm_access": map[string]any{"roles": []string{}}})
		rec := apitest.Do(h, apitest.Request{Method: "GET", Path: "/eeg/" + ecId + "/lastRecordDate",
			Header: map[string]string{"Authorization": "Bearer " + token, "tenant": tenant}})
		if tc.ok {
			assert.Equal(t, 200, rec.Code, tc.name)
		} else {
			assert.Equal(t, 403, rec.Code, tc.name)
		}
	}
}

func TestTenantClaimShapesGraphQL(t *testing.T) {
	seed(t)
	h := apitest.Router()
	for _, tc := range tenantClaimShapes {
		token := fakeoidc.Token(map[string]any{"tenant": tc.claim, "realm_access": map[string]any{"roles": []string{}}})
		rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/query",
			Header: map[string]string{"Authorization": "Bearer " + token, "tenant": tenant},
			Body:   fmt.Sprintf(`{"query":"{ lastEnergyDate(tenant: \"%s\", ecId: \"%s\") }"}`, tenant, ecId)})
		if !tc.ok {
			assert.Equal(t, 401, rec.Code, tc.name)
			continue
		}
		require.Equal(t, 200, rec.Code, tc.name)
		var resp gqlResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp), tc.name)
		assert.Empty(t, resp.Errors, tc.name)
	}
}

func TestTenantClaimShapesApi(t *testing.T) {
	seed(t)
	h := apitest.Router()
	for _, tc := range tenantClaimShapes {
		fakeoidc.SetUser(apiUser, apiPass, map[string]any{"tenant": tc.claim})
		rec := apitest.Do(h, apitest.Request{Method: "POST", Path: "/query/" + ecId + "/metadata", Header: apiHeader()})
		if tc.ok {
			assert.Equal(t, 200, rec.Code, tc.name)
		} else {
			assert.Equal(t, 403, rec.Code, tc.name)
		}
	}
}

// A claim of another type cannot be read: the token is refused as before.
func TestTenantClaimOfAnotherTypeIsRefused(t *testing.T) {
	seed(t)
	h := apitest.Router()
	for _, claim := range []any{42, map[string]any{"id": tenant}, []any{tenant, 1}, [][][]string{{{tenant}}}} {
		token := fakeoidc.Token(map[string]any{"tenant": claim, "realm_access": map[string]any{"roles": []string{}}})
		rec := apitest.Do(h, apitest.Request{Method: "GET", Path: "/eeg/" + ecId + "/lastRecordDate",
			Header: map[string]string{"Authorization": "Bearer " + token, "tenant": tenant}})
		assert.Contains(t, []int{401, 403}, rec.Code, "%v", claim)
		assert.NotEqual(t, 200, rec.Code, "%v", claim)
	}
}
