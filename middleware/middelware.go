package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

type PlatformClaims struct {
	Tenants     []string `json:"tenant"`
	Username    string   `json:"preferred_username"`
	Email       string   `json:"email"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	// AccessGroups are the Keycloak groups of the user (full path, e.g. "/EEG_ADMIN"), as in the backend.
	// They are not per community (api 261025-ca3).
	AccessGroups []string `json:"access_groups"`
	// EmailVerified (standard `email` scope): a member is looked up by its email only when it is verified;
	// missing counts as false (energystore 261025-ca1).
	EmailVerified bool `json:"email_verified"`
	// RegisteredClaims (not the deprecated StandardClaims) so `aud` parses as
	// either a string or an array — RFC 7519 allows both, and Keycloak emits an
	// array when a token carries more than one audience (e.g. the admin client).
	// StandardClaims models `aud` as a plain string and fails to unmarshal an
	// array, which 401'd every multi-audience token. exp is still validated.
	jwt.RegisteredClaims
}

// UnmarshalJSON reads the `tenant` claim with tenantClaim, `access_groups` with looseStrings and
// `email_verified` with looseBool; every other claim as before.
func (c *PlatformClaims) UnmarshalJSON(b []byte) error {
	type plain PlatformClaims // without this method
	aux := struct {
		*plain
		Tenants       tenantClaim  `json:"tenant"`
		AccessGroups  looseStrings `json:"access_groups"`
		EmailVerified looseBool    `json:"email_verified"`
	}{plain: (*plain)(c)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	c.Tenants = aux.Tenants
	c.AccessGroups = aux.AccessGroups
	c.EmailVerified = bool(aux.EmailVerified)
	return nil
}

// looseBool reads a JSON bool or the strings "true"/"false"; any other value is false and never an error,
// so an odd mapper cannot refuse the whole token (energystore 261025-ca1).
type looseBool bool

func (lb *looseBool) UnmarshalJSON(b []byte) error {
	var v bool
	if err := json.Unmarshal(b, &v); err == nil {
		*lb = looseBool(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*lb = looseBool(strings.EqualFold(s, "true"))
		return nil
	}
	*lb = false
	return nil
}

// looseStrings reads an array of strings or a single string (a one-element list); any other value is an
// empty list (no group: the token counts as a member) and never an error.
type looseStrings []string

func (ls *looseStrings) UnmarshalJSON(b []byte) error {
	var list []string
	if err := json.Unmarshal(b, &list); err == nil {
		*ls = list
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil && s != "" {
		*ls = looseStrings{s}
		return nil
	}
	*ls = nil
	return nil
}

// tenantClaim reads the `tenant` claim in the shapes the Keycloak mappers produce (known-errors #14):
// a string (single-valued mapper), an array of strings, or an array of such arrays (multivalued JSON
// mapper over an attribute value that holds a JSON array). Empty values are dropped; any other shape
// is an error, so the token is refused. A string that starts with "[" is the JSON-array text admin-backend
// stores in the attribute (String mapper, e.g. the `.api` client): its strings are the tenants; if it is
// not a JSON array of strings, it yields no tenant.
type tenantClaim []string

func (tc *tenantClaim) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		if !strings.HasPrefix(one, "[") {
			*tc = appendTenant(nil, one)
			return nil
		}
		var inner []string
		if err := json.Unmarshal([]byte(one), &inner); err != nil {
			inner = nil // not a JSON array of strings (json may have filled part of it): no tenant
		}
		var out []string
		for _, s := range inner {
			out = appendTenant(out, s)
		}
		*tc = out
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(b, &items); err != nil {
		return fmt.Errorf("tenant claim: %w", err)
	}
	var out []string
	for _, item := range items {
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			out = appendTenant(out, s)
			continue
		}
		var inner []string
		if err := json.Unmarshal(item, &inner); err != nil {
			return fmt.Errorf("tenant claim: %w", err)
		}
		for _, s := range inner {
			out = appendTenant(out, s)
		}
	}
	*tc = out
	return nil
}

func appendTenant(tenants []string, t string) []string {
	if t == "" {
		return tenants
	}
	return append(tenants, t)
}

// IsMember: every token without the group /EEG_ADMIN or /EEG_OWNER and without the realm role superuser —
// /EEG_USER and a token without any group alike. A member reads per-meter energy only of its own metering
// points (energystore 261025-ca1). Applied to bearer tokens only; the Basic-auth /query/* API does not use it.
func (c *PlatformClaims) IsMember() bool {
	return !slices.Contains(c.AccessGroups, "/EEG_ADMIN") &&
		!slices.Contains(c.AccessGroups, "/EEG_OWNER") &&
		!slices.Contains(c.RealmAccess.Roles, "superuser")
}

type SecHandlerFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, vars map[string]string, claims *PlatformClaims) error

type Middleware interface {
	WrapHandler(handler SecHandlerFunc) http.HandlerFunc
}

type JWTHandlerFunc func(http.ResponseWriter, *http.Request, *PlatformClaims, string)

type JWTWrapperFunc func(handlerFunc JWTHandlerFunc) http.HandlerFunc

type AccessTokenGenJWT struct {
	PublicKey *rsa.PublicKey
}

func (c *AccessTokenGenJWT) ExtractClaims(accesstoken string) (*PlatformClaims, error) {
	token, err := jwt.ParseWithClaims(accesstoken, &PlatformClaims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, errors.New(fmt.Sprintf("parseWithClaims different algorithms used - %v", t.Method.Alg()))
		}
		return c.PublicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't ParseWithClaims in parseToken %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token not valid in parseToken")
	}

	return token.Claims.(*PlatformClaims), nil

}
