// Package fakeoidc is an in-process fake Keycloak for the HTTP tests (M2, the fallback of
// open-points ES-13 without a production change, ES-24).
//
// middleware's package init() reads KEYCLOAK_CONFIG and runs two OIDC discoveries at start
// (known-errors #12). Package initialisation follows the Go spec (Go 1.21+): packages sorted by
// import path, each step initialises the first package whose imports are all initialised. This
// package sorts before "at.ourproject/energystore/middleware" and imports only standard-library
// packages that middleware imports itself, so its init() always runs first when a test binary
// imports it (a blank import is enough): it starts a loopback server with discovery, JWKS and a
// password-grant token endpoint and writes a keycloak config without secrets. Do not add imports
// outside the standard library here; TestInitOrder in package rest guards the order.
package fakeoidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	Realm       = "EEGFaktura"
	AppClientID = "at.ourproject.vfeeg.app"
	APIClientID = "at.ourproject.vfeeg.api"
	keyID       = "fake-key"
)

var (
	key        *rsa.PrivateKey
	listener   net.Listener
	baseURL    string
	configPath string
	started    bool

	mu     sync.Mutex
	users  = map[string]user{}
	grants int
)

type user struct {
	password string
	claims   map[string]any
}

func init() {
	if err := start(); err != nil {
		panic(fmt.Sprintf("fakeoidc: %v", err))
	}
}

func start() error {
	// time.Local is fixed here, before any goroutine exists: the server goroutines read it, so a
	// later write in TestMain would be a data race. Same zone as internal/testsupport/tz.
	vienna, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		return err
	}
	time.Local = vienna
	if key, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		return err
	}
	if listener, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
		return err
	}
	baseURL = "http://" + listener.Addr().String()
	go func() { _ = http.Serve(listener, http.HandlerFunc(serve)) }()

	cfg := map[string]map[string]string{
		"app": {"resource": AppClientID, "realm": Realm, "auth-server-url": baseURL},
		"api": {"resource": APIClientID, "realm": Realm, "auth-server-url": baseURL},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "fakeoidc-")
	if err != nil {
		return err
	}
	configPath = filepath.Join(dir, "keycloak.json")
	if err = os.WriteFile(configPath, b, 0o600); err != nil {
		return err
	}
	started = true
	return os.Setenv("KEYCLOAK_CONFIG", configPath)
}

// Started reports whether init() ran (it panics otherwise).
func Started() bool { return started }

// Issuer is the issuer URL of the realm, as go-oidc checks it.
func Issuer() string { return baseURL + "/realms/" + Realm }

// ConfigPath is the absolute path of the generated keycloak config (KEYCLOAK_CONFIG).
func ConfigPath() string { return configPath }

// Close stops the server and removes the generated config. Call it at the end of TestMain.
func Close() {
	_ = listener.Close()
	_ = os.RemoveAll(filepath.Dir(configPath))
}

// SetUser registers Basic credentials for the password grant of ProtectApi; claims go into the
// ID token (e.g. "tenant": []string{"TE100001"}).
func SetUser(username, password string, claims map[string]any) {
	mu.Lock()
	defer mu.Unlock()
	users[username] = user{password: password, claims: claims}
}

// ResetUsers removes every user and the grant counter.
func ResetUsers() {
	mu.Lock()
	defer mu.Unlock()
	users = map[string]user{}
	grants = 0
}

// Grants is the number of password grants the token endpoint answered (successful or not).
func Grants() int {
	mu.Lock()
	defer mu.Unlock()
	return grants
}

// Token signs an ID/access token of the app realm with the given claims; iss, aud, iat and exp
// (in one hour) are set unless the claims override them.
func Token(claims map[string]any) string {
	full := map[string]any{
		"iss": Issuer(), "aud": AppClientID, "sub": "test-user",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	for k, v := range claims {
		full[k] = v
	}
	return sign(full)
}

func sign(claims map[string]any) string {
	enc := base64.RawURLEncoding
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": keyID})
	payload, _ := json.Marshal(claims)
	signingInput := enc.EncodeToString(header) + "." + enc.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		panic(err)
	}
	return signingInput + "." + enc.EncodeToString(sig)
}

func serve(w http.ResponseWriter, r *http.Request) {
	prefix := "/realms/" + Realm
	switch r.URL.Path {
	case prefix + "/.well-known/openid-configuration":
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":                                Issuer(),
			"authorization_endpoint":                Issuer() + "/protocol/openid-connect/auth",
			"token_endpoint":                        Issuer() + "/protocol/openid-connect/token",
			"jwks_uri":                              Issuer() + "/protocol/openid-connect/certs",
			"userinfo_endpoint":                     Issuer() + "/protocol/openid-connect/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	case prefix + "/protocol/openid-connect/certs":
		enc := base64.RawURLEncoding
		writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": keyID,
			"n": enc.EncodeToString(key.N.Bytes()),
			"e": enc.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	case prefix + "/protocol/openid-connect/token":
		tokenEndpoint(w, r)
	default:
		http.NotFound(w, r)
	}
}

func tokenEndpoint(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "password" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
		return
	}
	mu.Lock()
	grants++
	u, ok := users[r.PostForm.Get("username")]
	mu.Unlock()
	if !ok || u.password != r.PostForm.Get("password") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_grant"})
		return
	}
	claims := map[string]any{
		"iss": Issuer(), "aud": APIClientID, "sub": r.PostForm.Get("username"),
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		"preferred_username": strings.ToLower(r.PostForm.Get("username")),
	}
	for k, v := range u.claims {
		claims[k] = v
	}
	writeJSON(w, http.StatusOK, map[string]string{"id_token": sign(claims), "access_token": sign(claims), "token_type": "Bearer"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
