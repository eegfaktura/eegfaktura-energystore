package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The custom decoding of `tenant` (known-errors #14) keeps every other claim.
func TestPlatformClaimsKeepsTheOtherClaims(t *testing.T) {
	var c PlatformClaims
	require.NoError(t, json.Unmarshal([]byte(`{"tenant":[["RC1","RC2"]],"preferred_username":"u","email":"u@example.org",
		"realm_access":{"roles":["superuser"]},"exp":1900000000,"aud":["a","b"],"iss":"https://kc/realms/r","sub":"s"}`), &c))
	assert.Equal(t, []string{"RC1", "RC2"}, c.Tenants)
	assert.Equal(t, "u", c.Username)
	assert.Equal(t, "u@example.org", c.Email)
	assert.Equal(t, []string{"superuser"}, c.RealmAccess.Roles)
	require.NotNil(t, c.ExpiresAt)
	assert.Equal(t, int64(1900000000), c.ExpiresAt.Unix())
	assert.Equal(t, []string{"a", "b"}, []string(c.Audience))
	assert.Equal(t, "https://kc/realms/r", c.Issuer)
	assert.Equal(t, "s", c.Subject)
}

func TestTenantClaimShapes(t *testing.T) {
	for in, want := range map[string][]string{
		`"RC1"`:                {"RC1"},
		`["RC1","RC2"]`:        {"RC1", "RC2"},
		`[["RC1","RC2"]]`:      {"RC1", "RC2"},
		`[["RC1"],["RC2"]]`:    {"RC1", "RC2"},
		`["RC1",["RC2"]]`:      {"RC1", "RC2"},
		`["","RC1",[""],null]`: {"RC1"},
		`""`:                   nil,
		// a string holding a JSON array of strings (String mapper over admin-backend's attribute text)
		`"[\"RC1\",\"\",\"RC2\"]"`: {"RC1", "RC2"},
		`"[\"RC1\""`:               nil, // malformed: matches nothing
		`"[[\"RC1\"]]"`:            nil,
		`"[1]"`:                    nil,
		`"[\"RC1\",1]"`:            nil, // partly a string array: nothing
		`null`:                     nil,
		`[]`:                       nil,
	} {
		var c PlatformClaims
		require.NoError(t, json.Unmarshal([]byte(`{"tenant":`+in+`}`), &c), in)
		assert.Equal(t, want, c.Tenants, in)
	}
	for _, in := range []string{`42`, `{"id":"RC1"}`, `["RC1",1]`, `[[["RC1"]]]`, `[[1]]`} {
		var c PlatformClaims
		assert.Error(t, json.Unmarshal([]byte(`{"tenant":`+in+`}`), &c), in)
	}
}

// The comparison with the tenant header stays case-insensitive and needs a non-empty match.
func TestContainsTenant(t *testing.T) {
	assert.True(t, contains([]string{"RC1"}, "rc1"))
	assert.False(t, contains([]string{"RC1"}, "RC2"))
	var c PlatformClaims
	require.NoError(t, json.Unmarshal([]byte(`{"tenant":[""]}`), &c))
	assert.False(t, contains(c.Tenants, ""), "an empty claim value never matches an empty header")
}
