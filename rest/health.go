package rest

import (
	"net/http"

	"at.ourproject/energystore/store/ebow"
	"github.com/golang/glog"
)

// healthz is the liveness probe: no dependency checks, so a storage problem never restarts the pod.
// Unauthenticated, like readyz (the documented exception to "every endpoint requires a token").
func healthz(w http.ResponseWriter, _ *http.Request) {
	respondPlain(w, http.StatusOK, "ok")
}

// readyz is the readiness probe: 200 while the Badger store can serve requests (ebow.Ready), else
// 503. The reason goes to the log only, never into the body.
func readyz(w http.ResponseWriter, _ *http.Request) {
	if err := ebow.Ready(); err != nil {
		glog.Warningf("readyz: not ready: %v", err)
		respondPlain(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	respondPlain(w, http.StatusOK, "ok")
}

func respondPlain(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(body))
}
