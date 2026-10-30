package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// energystore 261025-ca1: email_verified and access_groups are read tolerantly; an odd shape never refuses
// the whole token.
func TestEmailVerifiedShapes(t *testing.T) {
	for raw, want := range map[string]bool{
		`true`: true, `false`: false, `"true"`: true, `"TRUE"`: true, `"false"`: false,
		`"yes"`: false, `1`: false, `null`: false, `{"x":1}`: false, `[true]`: false,
	} {
		var c PlatformClaims
		require.NoError(t, json.Unmarshal([]byte(`{"tenant":["TE100001"],"email_verified":`+raw+`}`), &c), raw)
		assert.Equal(t, want, c.EmailVerified, raw)
		assert.Equal(t, []string{"TE100001"}, c.Tenants, raw)
	}
	var c PlatformClaims
	require.NoError(t, json.Unmarshal([]byte(`{"tenant":["TE100001"]}`), &c))
	assert.False(t, c.EmailVerified, "missing")
}

func TestAccessGroupsShapes(t *testing.T) {
	for raw, want := range map[string][]string{
		`["/EEG_ADMIN","/EEG_USER"]`: {"/EEG_ADMIN", "/EEG_USER"},
		`"/EEG_ADMIN"`:               {"/EEG_ADMIN"},
		`""`:                         nil,
		`null`:                       nil,
		`42`:                         nil,
		`{"a":1}`:                    nil,
	} {
		var c PlatformClaims
		require.NoError(t, json.Unmarshal([]byte(`{"tenant":["TE100001"],"access_groups":`+raw+`}`), &c), raw)
		assert.Equal(t, want, c.AccessGroups, raw)
	}
	var c PlatformClaims
	require.NoError(t, json.Unmarshal([]byte(`{"access_groups":"/EEG_OWNER"}`), &c))
	assert.False(t, c.IsMember(), "a single group string counts")
}
