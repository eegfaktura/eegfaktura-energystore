package rest

import (
	"errors"
	"net/http"
	"strings"

	"at.ourproject/energystore/internal/logging/glog"
	"at.ourproject/energystore/middleware"
	"at.ourproject/energystore/ownmeters"
	"at.ourproject/energystore/store"
	"github.com/gorilla/mux"
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

// ownMeters returns the caller's own metering points in the requested community if the caller is a
// member, nil otherwise. ok is false when the response is written (503: the backend could not name them).
func ownMeters(w http.ResponseWriter, r *http.Request, claims *middleware.PlatformClaims, tenant string) (own ownmeters.Meters, ok bool) {
	if !claims.IsMember() {
		return nil, true
	}
	if strings.TrimSpace(claims.Email) == "" {
		// the backend finds the member by the token's email; without one there is nothing to match
		glog.Warningf("member token without email claim: own metering points not available tenant=%s", tenant)
		respondWithError(w, http.StatusServiceUnavailable, "metering points of the member not available")
		return nil, false
	}
	all, err := memberMeters.Meters(r.Context(), r.Header.Get("Authorization"), claims.Subject, tenant, claims.Tenants)
	if err == nil {
		own, err = inCommunity(all, tenant, mux.Vars(r)["ecid"])
	}
	if err != nil {
		if refuseTenant(w, err) {
			return nil, false
		}
		if !errors.Is(err, ownmeters.ErrNotConfigured) {
			glog.Errorf("own metering points of a member: %v tenant=%s", err, tenant)
		}
		respondWithError(w, http.StatusServiceUnavailable, "metering points of the member not available")
		return nil, false
	}
	return own, true
}

// inCommunity keeps the meters that the requested community (tenant and ecid, already checked against
// the token) has in this store: the backend lookup spans every tenant of the token (GEA), so a meter of
// another community is never readable here. A community without any meta record (QueryMetaData then
// answers an empty map, no error) has no energy to read: its set stays as it is, so the member gets the
// same empty report as an admin instead of 403.
func inCommunity(all ownmeters.Meters, tenant, ecid string) (ownmeters.Meters, error) {
	meta, err := store.QueryMetaData(tenant, ecid)
	if err != nil {
		return nil, err
	}
	if len(meta) == 0 {
		return all, nil
	}
	own := ownmeters.Meters{}
	for name := range meta {
		if w, ok := all[strings.ToUpper(name)]; ok {
			own[strings.ToUpper(name)] = w
		}
	}
	return own, nil
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
