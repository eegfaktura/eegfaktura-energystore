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
	"github.com/sirupsen/logrus"
)

const (
	BASIC_SCHEMA  string = "Basic "
	BEARER_SCHEMA string = "Bearer "
)

var (
	kcConfig map[string]*keycloakConfig
	verifier *oidc.IDTokenVerifier
)

type keycloakConfig struct {
	ClientId  string `json:"resource"`
	Secret    string `json:"secret,omitempty"`
	Realm     string `json:"realm"`
	Host      string `json:"auth-server-url"`
	Internal  bool   `json:"auth-server-url-internal,omitempty"`
	IssuerUrl string `json:"issuer_url,omitempty"`
}

func init() {
	var err error
	kcConfig, err = readKeycloakConfig()
	if err != nil {
		logrus.WithField("error", "Init Keycloak").Errorf("E: %v", err)
		panic(err)
	}

	if _, ok := kcConfig["api"]; !ok {
		logrus.WithField("error", "Init Keycloak").Errorf("E: %v", errors.New("no client-id 'at.ourproject.vfeeg.api' available"))
		panic(err)
	}

	clientIDApi := kcConfig["api"].ClientId
	clientSecretApi := kcConfig["api"].Secret
	issuerUrl := kcConfig["api"].IssuerUrl

	realmApi := kcConfig["api"].Realm
	host := strings.TrimRight(kcConfig["api"].Host, "/")

	c := &http.Client{Timeout: time.Duration(1) * time.Second}
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
		if issuerUrl == "" {
			panic("issuerUrl is required")
		}
		// External issuer (MUST match the token's "iss")
		u, err := url.Parse(hostApp)
		if err != nil {
			panic(err)
		}
		internalHost := issuerUrl // Custom transport that rewrites DNS lookups
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
		logrus.Errorf("E: %v", err)
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

func verifyRequest(handler JWTHandlerFunc) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		jwtToken := r.Header.Get("Authorization")
		if len(jwtToken) == 0 {
			logrus.WithField("error", "JWT-Token").Printf("No Access_token in request!\n")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if strings.HasPrefix(jwtToken, BEARER_SCHEMA) {
			jwtToken = jwtToken[len(BEARER_SCHEMA):]
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		idToken, err := verifier.Verify(context.Background(), jwtToken)
		if err != nil {
			logrus.WithField("error", "JWT-Token").Errorf("%v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		claims := PlatformClaims{}
		if err := idToken.Claims(&claims); err != nil {
			logrus.WithField("error", "Claims").Errorf("%v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		tenant := r.Header.Get("X-Tenant")
		superuser := hasRole(claims.RealmAccess.Roles, "superuser")
		if !superuser {
			if contains(claims.Tenants, tenant) == false {
				logrus.WithField("tenant", tenant).Warnf("Unauthorized access with tenant %s", tenant)
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		handler(w, r, &claims, strings.ToUpper(tenant))
	}
}

func hasRole(roles []string, role string) bool {
	return slices.Contains(roles, role)
}
