package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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
	ClientId string `json:"resource"`
	Secret   string `json:"secret"`
	Realm    string `json:"realm"`
	Host     string `json:"auth-server-url"`
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

	realmApi := kcConfig["api"].Realm
	host := strings.TrimRight(kcConfig["api"].Host, "/")

	c := &http.Client{Timeout: time.Duration(1) * time.Second}
	kcClientAPI, err = NewKeycloakClient(fmt.Sprintf("%s/realms/%s", host, realmApi), clientIDApi, clientSecretApi, c)
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
	providerUriApp := fmt.Sprintf("%s/realms/%s", hostApp, realmApp)
	println(providerUriApp)
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
		if contains(claims.Tenants, tenant) == false {
			logrus.WithField("tenant", tenant).Warnf("Unauthorized access with tenant %s", tenant)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		handler(w, r, &claims, strings.ToUpper(tenant))
	}
}
