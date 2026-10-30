package rest

import (
	"errors"
	"net/http"

	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/ownmeters"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

// A member token (every bearer token without admin, owner or superuser) reads per-meter energy only of
// its own metering points and only for the time they were its own; the backend names them (energystore
// 261025-ca1). Whole-community totals stay readable.

// memberMeters is set by NewRestServer from the config key services.backend-api; nil counts as not
// configured.
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
func ownMeters(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) (own ownmeters.Meters, ok bool) {
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
// metering point that is not its own; for every other caller it allows all and returns nil. A member
// that names no meter gets an empty set without asking the backend.
func allowMeters(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string, meters []string) (own ownmeters.Meters, ok bool) {
	if !claims.IsMember() {
		return nil, true
	}
	if len(meters) == 0 {
		return ownmeters.Meters{}, true
	}
	if own, ok = ownMeters(w, r, claims, tenant); !ok {
		return nil, false
	}
	for _, m := range meters {
		if !own.Owns(m) {
			forbidMeter(w)
			return nil, false
		}
	}
	return own, true
}

// forbidMeter is the answer for a foreign or unknown metering point, and for a time outside the
// member's windows.
func forbidMeter(w http.ResponseWriter) {
	respondWithError(w, http.StatusForbidden, errMeterNotPermitted)
}

// clampDays limits the day window *from/*until of meter to the member's windows (own nil: no limit);
// false when the response is written (403).
func clampDays(w http.ResponseWriter, own ownmeters.Meters, meter string, from, until *int64) bool {
	if own == nil {
		return true
	}
	f, u, ok := own.ClampDays(meter, *from, *until)
	if !ok {
		forbidMeter(w)
		return false
	}
	*from, *until = f, u
	return true
}

// denyMember answers 403 to a member (writes, imports, exports of the whole community).
func denyMember(w http.ResponseWriter, claims *middleware.PlatformClaims) bool {
	if claims.IsMember() {
		respondWithError(w, http.StatusForbidden, "not permitted for members")
		return true
	}
	return false
}
