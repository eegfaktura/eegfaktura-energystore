package middleware

// Signing-algorithm tests for the real go-oidc verifiers (platform#191,
// analog backend#99). The package init() reads KEYCLOAK_CONFIG and runs the
// OIDC discovery for the "app" and "api" clients. Go initializes every
// package-level variable before any init() runs, so tvIssuer below starts a
// local OIDC provider (discovery + JWKS from httptest) and points
// KEYCLOAK_CONFIG at it first; init() then builds both verifiers exactly as
// in production. No live Keycloak is needed.
//
// The provider advertises the same id_token algorithms as Keycloak 26; the
// JWKS holds an RSA key and an EC key, so RS512/PS256/ES256 tokens are valid
// apart from the algorithm.

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	tvRealm = "test"
	tvKid   = "test-kid"
	tvKidEC = "test-kid-ec"
	tvAud   = "at.ourproject.vfeeg.app"
)

var (
	tvKey    *rsa.PrivateKey
	tvECKey  *ecdsa.PrivateKey
	tvCfgDir string
	tvIssuer = tvStartProvider()
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// tvStartProvider runs during package variable initialization, i.e. before
// init(). It panics on error, as init() would.
func tvStartProvider() string {
	var err error
	if tvKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		panic(err)
	}
	if tvECKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux) // lives for the whole test binary
	issuer := srv.URL + "/realms/" + tvRealm
	mux.HandleFunc("/realms/"+tvRealm+"/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 issuer,
			"jwks_uri":               issuer + "/protocol/openid-connect/certs",
			"token_endpoint":         issuer + "/protocol/openid-connect/token",
			"authorization_endpoint": issuer + "/protocol/openid-connect/auth",
			// as Keycloak 26 advertises them
			"id_token_signing_alg_values_supported": []string{
				"PS384", "RS384", "EdDSA", "ES384", "HS256", "HS512", "ES256", "RS256",
				"HS384", "ES512", "PS256", "PS512", "RS512",
			},
		})
	})
	mux.HandleFunc("/realms/"+tvRealm+"/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
		pub := tvKey.PublicKey
		ec := tvECKey.PublicKey
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": tvKid, "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": b64(pub.N.Bytes()),
			"e": b64(big.NewInt(int64(pub.E)).Bytes()),
		}, {
			"kid": tvKidEC, "kty": "EC", "alg": "ES256", "use": "sig", "crv": "P-256",
			"x": b64(ec.X.FillBytes(make([]byte, 32))),
			"y": b64(ec.Y.FillBytes(make([]byte, 32))),
		}}})
	})

	client := map[string]string{"realm": tvRealm, "auth-server-url": srv.URL + "/", "resource": tvAud}
	cfg, err := json.Marshal(map[string]any{"app": client, "api": client})
	if err != nil {
		panic(err)
	}
	if tvCfgDir, err = os.MkdirTemp("", "es-kc-test"); err != nil {
		panic(err)
	}
	path := filepath.Join(tvCfgDir, "keycloak.json")
	if err = os.WriteFile(path, cfg, 0o600); err != nil {
		panic(err)
	}
	if err = os.Setenv("KEYCLOAK_CONFIG", path); err != nil {
		panic(err)
	}
	return issuer
}

func TestMain(m *testing.M) {
	code := m.Run()
	_ = os.RemoveAll(tvCfgDir)
	os.Exit(code)
}

// tvInput builds the signing input (header.payload) of a valid token.
func tvInput(t *testing.T, alg, kid string) string {
	t.Helper()
	h, err := json.Marshal(map[string]string{"alg": alg, "typ": "JWT", "kid": kid})
	require.NoError(t, err)
	now := time.Now()
	p, err := json.Marshal(map[string]any{
		"iss": tvIssuer, "aud": tvAud, "sub": "user-1",
		"iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		"tenant": []string{"RC100001"},
	})
	require.NoError(t, err)
	return b64(h) + "." + b64(p)
}

// tvSign builds a compact JWS with alg; the kid follows the key type.
func tvSign(t *testing.T, alg string, key crypto.Signer) string {
	t.Helper()
	kid := tvKid
	if _, ok := key.(*ecdsa.PrivateKey); ok {
		kid = tvKidEC
	}
	input := tvInput(t, alg, kid)
	d256 := sha256.Sum256([]byte(input))
	var sig []byte
	var err error
	switch alg {
	case "RS256":
		sig, err = key.Sign(rand.Reader, d256[:], crypto.SHA256)
	case "RS512":
		d := sha512.Sum512([]byte(input))
		sig, err = key.Sign(rand.Reader, d[:], crypto.SHA512)
	case "PS256":
		sig, err = key.Sign(rand.Reader, d256[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256})
	case "ES256":
		// JWS wants r||s, not the ASN.1 form of crypto.Signer.
		ecKey, ok := key.(*ecdsa.PrivateKey)
		require.True(t, ok, "ES256 needs an EC key")
		var r, s *big.Int
		r, s, err = ecdsa.Sign(rand.Reader, ecKey, d256[:])
		if err == nil {
			sig = append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
		}
	default:
		t.Fatalf("unsupported alg %s", alg)
	}
	require.NoError(t, err)
	return input + "." + b64(sig)
}

// tvSignHS256 signs with HS256 and the given secret (key confusion: the
// realm's public RSA key as HMAC secret).
func tvSignHS256(t *testing.T, secret []byte) string {
	t.Helper()
	input := tvInput(t, "HS256", tvKid)
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(input))
	return input + "." + b64(m.Sum(nil))
}

// Only RS256 is accepted (CLAUDE.md, Authentication): both go-oidc verifiers
// are restricted via SupportedSigningAlgs, although the discovery advertises
// more algorithms. The error must be go-jose's algorithm check, so a broken
// fixture cannot make a case pass. Trade-off: the match is on go-jose's error
// text, so a go-jose upgrade that rewords it fails this test (fix the
// constant then); it does not let a token through.
func TestTokenValidation_OnlyRS256Accepted(t *testing.T) {
	require.Nil(t, VerifyTokenClaims, "test hook must be unset, the real verifier is under test")
	const algErr = "unexpected signature algorithm"
	cases := []struct {
		alg    string
		key    crypto.Signer
		accept bool
	}{
		{"RS256", tvKey, true},
		{"RS512", tvKey, false},
		{"PS256", tvKey, false},
		{"ES256", tvECKey, false},
	}
	for _, tc := range cases {
		t.Run(tc.alg, func(t *testing.T) {
			token := tvSign(t, tc.alg, tc.key)
			// app verifier (GQLProtect, verifyRequest)
			claims, err := verifyAndExtractClaims(context.Background(), token)
			// api verifier (ProtectApi, id token of the password grant)
			_, errAPI := kcClientAPI.VerifyToken(token)
			if tc.accept {
				require.NoError(t, err)
				require.NoError(t, errAPI)
				require.Equal(t, []string{"RC100001"}, claims.Tenants)
			} else {
				require.ErrorContains(t, err, algErr)
				require.ErrorContains(t, errAPI, algErr)
			}
		})
	}
}

// HS256 with the realm's public key as HMAC secret (key confusion) and
// alg=none are rejected by both verifiers. Only an error is asserted here:
// go-oidc/go-jose reject these at different stages (unsupported algorithm,
// no matching key, malformed JWS) and the message differs between versions.
func TestTokenValidation_HS256AndNoneRejected(t *testing.T) {
	der, err := x509.MarshalPKIXPublicKey(&tvKey.PublicKey)
	require.NoError(t, err)
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	tokens := map[string]string{
		"HS256 public key PEM": tvSignHS256(t, pemKey),
		"HS256 public key DER": tvSignHS256(t, der),
		"none":                 tvInput(t, "none", tvKid) + ".",
	}
	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			_, err := verifyAndExtractClaims(context.Background(), token)
			require.Error(t, err)
			_, err = kcClientAPI.VerifyToken(token)
			require.Error(t, err)
		})
	}
}
