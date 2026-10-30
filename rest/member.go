package rest

import (
	"errors"
	"net/http"
	"strings"

	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/ownmeters"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

// A member token reads per-meter energy only of its own metering points; the backend names them
// (energystore 261025-ca1). Whole-community totals stay readable.

// memberMeters is set by NewRestServer from the config key services.backend-api.
var memberMeters *ownmeters.Resolver

const errMeterNotPermitted = "metering point not permitted"

func initMemberMeters() {
	url := viper.GetString("services.backend-api")
	if url == "" {
		glog.Warning("services.backend-api is not set: per-meter requests of members are refused (503)")
	}
	memberMeters = ownmeters.New(url)
}

// ownMeters returns the caller's own metering points if the caller is a member, nil otherwise. ok is
// false when the response is written (503: the backend could not name them).
func ownMeters(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) (own map[string]bool, ok bool) {
	if !claims.IsMember() {
		return nil, true
	}
	own, err := memberMeters.Meters(r.Context(), r.Header.Get("Authorization"), claims.Subject, tenant)
	if err != nil {
		if !errors.Is(err, ownmeters.ErrNotConfigured) {
			glog.Errorf("own metering points of a member: %v tenant=%s", err, tenant)
		}
		respondWithError(w, http.StatusServiceUnavailable, "metering points of the member not available")
		return nil, false
	}
	return own, true
}

// allowMeters answers 403 (same answer for unknown and foreign meters) or 503 when a member names a
// metering point that is not its own; for every other caller it allows all. own is as in ownMeters.
func allowMeters(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string, meters []string) (own map[string]bool, ok bool) {
	own, ok = ownMeters(w, r, claims, tenant)
	if !ok || own == nil {
		return own, ok
	}
	for _, m := range meters {
		if !own[strings.ToUpper(m)] {
			respondWithError(w, http.StatusForbidden, errMeterNotPermitted)
			return nil, false
		}
	}
	return own, true
}

// denyMember answers 403 to a member (writes, imports, exports of the whole community).
func denyMember(w http.ResponseWriter, claims *middleware.PlatformClaims) bool {
	if claims.IsMember() {
		respondWithError(w, http.StatusForbidden, "not permitted for members")
		return true
	}
	return false
}

// noMembers refuses members before h: the Basic-auth API has no bearer token to ask the backend with.
func noMembers(h middleware.JWTHandlerFunc) middleware.JWTHandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) {
		if denyMember(w, claims) {
			return
		}
		h(w, r, claims, tenant)
	}
}
