package middleware

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang/glog"
)

const (
	BASIC_SCHEMA  string = "Basic "
	BEARER_SCHEMA string = "Bearer "
)

var (
	kcConfig map[string]*keycloakConfig
	verifier *oidc.IDTokenVerifier
	// VerifyTokenClaims pluggable verification function used by middlewares and tests.
	// InitKeycloak should set this to the real verifier at startup.
	// Tests override it to avoid network calls.
	VerifyTokenClaims func(ctx context.Context, token string) (*PlatformClaims, error)

	ErrNoAuthorization      = errors.New("authorization header missing")
	ErrInvalidAuthSchema    = errors.New("invalid authorization schema")
	ErrInvalidBasicEncoding = errors.New("invalid basic auth encoding")
	ErrInvalidBasicFormat   = errors.New("invalid basic auth format")
)

type keycloakConfig struct {
	ClientId  string `json:"resource"`
	Secret    string `json:"secret,omitempty"`
	Realm     string `json:"realm"`
	Host      string `json:"auth-server-url"`
	Internal  bool   `json:"issuer-internal,omitempty"`
	IssuerUrl string `json:"issuer-url,omitempty"`
}

func init() {
	var err error
	kcConfig, err = readKeycloakConfig()
	if err != nil {
		glog.Errorf("Init Keycloak: %v", err)
		panic(err)
	}

	if _, ok := kcConfig["api"]; !ok {
		glog.Errorf("Init Keycloak: %v", errors.New("no client-id 'at.ourproject.vfeeg.api' available"))
		panic(err)
	}

	clientIDApi := kcConfig["api"].ClientId
	clientSecretApi := kcConfig["api"].Secret
	issuerUrl := kcConfig["api"].IssuerUrl

	realmApi := kcConfig["api"].Realm
	host := strings.TrimRight(kcConfig["api"].Host, "/")

	c := &http.Client{Timeout: 10 * time.Second}
	kcClientAPI, err = NewKeycloakClient(fmt.Sprintf("%s/realms/%s", host, realmApi), clientIDApi, clientSecretApi, issuerUrl, c)
	if err != nil {
		panic(err)
	}

	/**
	set up jwt token verifier
	*/
	clientIDApp := kcConfig["app"].ClientId
	realmApp := kcConfig["app"].Realm
	hostApp := strings.TrimRight(kcConfig["app"].Host, "/")

	ctx := context.Background()
	if kcConfig["app"].Internal {
		internalHost := kcConfig["app"].IssuerUrl
		if internalHost == "" {
			panic("issuerUrl is required")
		}
		// External issuer (MUST match the token's "iss")
		u, err := url.Parse(hostApp)
		if err != nil {
			panic(err)
		}
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// addr looks like "auth.example.com:443"
				if strings.HasPrefix(addr, u.Host) {
					// Replace with internal Docker hostname
					addr = internalHost
				}
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, network, addr)
			},
		}

		// Custom HTTP client using the resolver
		httpClient := &http.Client{Transport: transport, Timeout: 10 * time.Second}

		// Inject client into OIDC context
		ctx = oidc.ClientContext(ctx, httpClient)
	}

	providerUriApp := fmt.Sprintf("%s/realms/%s", hostApp, realmApp)
	provider, err := oidc.NewProvider(ctx, providerUriApp)
	if err != nil {
		glog.Errorf("%v", err)
		panic(err)
	}
	verifier = provider.Verifier(&oidc.Config{ClientID: clientIDApp, SkipClientIDCheck: true})
}

func readKeycloakConfig() (map[string]*keycloakConfig, error) {
	kcPath, ok := os.LookupEnv("KEYCLOAK_CONFIG")
	if !ok {
		kcPath = "./keycloak.json"
	}
	kcConfigFile, err := os.Open(kcPath)
	if err != nil {
		return nil, err
	}
	defer kcConfigFile.Close()

	payload, err := io.ReadAll(kcConfigFile)
	if err != nil {
		return nil, err
	}

	kcConfig := map[string]*keycloakConfig{}
	err = json.Unmarshal(payload, &kcConfig)
	return kcConfig, err
}

// verifyAndExtractClaims tries the test-hook VerifyTokenClaims (if present) and
// falls back to the actual OIDC verifier otherwise. It always returns a populated
// PlatformClaims or an error.
func verifyAndExtractClaims(ctx context.Context, token string) (*PlatformClaims, error) {
	// If a test hook / custom verifier is provided, use it.
	if VerifyTokenClaims != nil {
		return VerifyTokenClaims(ctx, token)
	}

	// Fallback to OIDC verifier
	if verifier == nil {
		return nil, fmt.Errorf("oidc verifier not initialized")
	}
	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, err
	}
	claims := &PlatformClaims{}
	if err := idToken.Claims(claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// ParseBearerTokenFromHeader extracts the Bearer token from the Authorization header.
// Returns a non-empty token or an error describing the problem.
func parseBearerTokenFromHeader(r *http.Request) (string, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return "", ErrNoAuthorization
	}
	if !strings.HasPrefix(auth, BEARER_SCHEMA) {
		return "", ErrInvalidAuthSchema
	}
	token := strings.TrimSpace(auth[len(BEARER_SCHEMA):])
	if token == "" {
		return "", ErrInvalidAuthSchema
	}
	return token, nil
}

func GQLProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtToken, err := parseBearerTokenFromHeader(r)
		if err != nil {
			glog.Errorf("No Access_token in request or invalid Authorization: %v\n", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		claims, err := verifyAndExtractClaims(context.Background(), jwtToken)
		if err != nil {
			glog.Errorf("%v", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}

		tenant := getTenant(r)
		if len(tenant) == 0 {
			glog.Warning("Unauthorized access without tenant")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		superuser := hasRole(claims.RealmAccess.Roles, "superuser")
		if !superuser {
			if contains(claims.Tenants, tenant) == false {
				glog.Warningf("Unauthorized access with tenant %s", tenant)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func verifyRequest(handler JWTHandlerFunc) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		jwtToken, err := parseBearerTokenFromHeader(r)
		if err != nil {
			glog.Errorf("No Access_token in request or invalid Authorization: %v\n", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		claims, err := verifyAndExtractClaims(context.Background(), jwtToken)
		if err != nil {
			glog.Errorf("%v", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(err.Error()))
			return
		}

		tenant := getTenant(r)
		if len(tenant) == 0 {
			glog.Warning("Unauthorized access without tenant")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		superuser := hasRole(claims.RealmAccess.Roles, "superuser")
		if !superuser {
			if contains(claims.Tenants, tenant) == false {
				glog.Warningf("Unauthorized access with tenant %s", tenant)
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		handler(w, r, claims, strings.ToUpper(tenant))
	}
}

func hasRole(roles []string, role string) bool {
	return slices.Contains(roles, role)
}

func getTenant(r *http.Request) string {
	tenant := r.Header.Get("tenant")
	if len(tenant) == 0 {
		tenant = r.Header.Get("X-Tenant")
	}
	return tenant
}
