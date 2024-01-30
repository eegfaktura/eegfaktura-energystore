package middleware

import (
	"encoding/json"
	"errors"
	"fmt"
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
