package middleware

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/golang/glog"
)

var (
	kcClientAPI *KeycloakClient
)

func ProtectApi(handler JWTHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		basicAuth := r.Header.Get("Authorization")
		if len(basicAuth) == 0 {
			glog.Info("Authorization: No Access_token in request!")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		if strings.HasPrefix(basicAuth, BASIC_SCHEMA) {
			basicAuth = basicAuth[len(BASIC_SCHEMA):]
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		uDec, _ := base64.URLEncoding.DecodeString(basicAuth)
		creds := strings.Split(string(uDec), ":")

		if len(creds) < 2 {
			glog.Info("No User:Password in request!")
			w.WriteHeader(http.StatusForbidden)
			return
		}

		idToken, err := kcClientAPI.AuthenticateUserWithPassword(creds[0], creds[1])
		if err != nil {
			glog.Errorf("%v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		claims := PlatformClaims{}
		if err := idToken.Claims(&claims); err != nil {
			glog.Errorf("%v", err)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		tenant := r.Header.Get("X-Tenant")
		if contains(claims.Tenants, tenant) == false {
			glog.Errorf("Unauthorized access with tenant %s", tenant)
			w.WriteHeader(http.StatusForbidden)
			return
		}

		handler(w, r, &claims, strings.ToUpper(tenant))
	}
}
