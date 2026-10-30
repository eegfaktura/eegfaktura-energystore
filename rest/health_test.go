package rest_test

import (
	"path/filepath"
	"sync"
	"testing"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/apitest"
	"at.ourproject/energystore/store/ebow"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// publicRoutes are the only entry points without a token (Kubernetes probes); TestRouteWalkGuard
// accepts them outside the auth matrix of routes().
var publicRoutes = []string{"GET /healthz", "HEAD /healthz", "GET /readyz", "HEAD /readyz"}

func probe(t *testing.T, path string) (int, string, string) {
	t.Helper()
	rec := apitest.Do(apitest.Router(), apitest.Request{Method: "GET", Path: path})
	return rec.Code, rec.Body.String(), rec.Header().Get("Content-Type")
}

func TestHealthzWithoutToken(t *testing.T) {
	code, body, ct := probe(t, "/healthz")
	assert.Equal(t, 200, code)
	assert.Equal(t, "ok", body)
	assert.Equal(t, "text/plain; charset=utf-8", ct)
}

func TestHealthzIgnoresStorage(t *testing.T) {
	testsupport.UseTempPersistence(t)
	viper.Set("persistence.path", filepath.Join(t.TempDir(), "missing"))
	code, body, _ := probe(t, "/healthz")
	assert.Equal(t, 200, code, "liveness must not depend on the store")
	assert.Equal(t, "ok", body)
}

func TestReadyzWithStore(t *testing.T) {
	testsupport.UseTempPersistence(t)
	code, body, _ := probe(t, "/readyz")
	assert.Equal(t, 200, code)
	assert.Equal(t, "ok", body)
}

// Concurrent probes share one check (ebow.Ready); every caller still gets the result.
func TestReadyzConcurrent(t *testing.T) {
	testsupport.UseTempPersistence(t)
	h := apitest.Router()
	var wg sync.WaitGroup
	codes := make([]int, 20)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = apitest.Do(h, apitest.Request{Method: "GET", Path: "/readyz"}).Code
		}(i)
	}
	wg.Wait()
	for _, c := range codes {
		assert.Equal(t, 200, c)
	}
}

func TestReadyzMissingPersistencePath(t *testing.T) {
	testsupport.UseTempPersistence(t)
	missing := filepath.Join(t.TempDir(), "missing")
	viper.Set("persistence.path", missing)
	code, body, ct := probe(t, "/readyz")
	assert.Equal(t, 503, code)
	assert.Equal(t, "unavailable", body, "no reason or path in the body")
	assert.Equal(t, "text/plain; charset=utf-8", ct)
}

func TestReadyzClosedPool(t *testing.T) {
	testsupport.UseTempPersistence(t) // its cleanup replaces the closed pool with a fresh one
	ebow.ClosePool()
	code, body, _ := probe(t, "/readyz")
	assert.Equal(t, 503, code)
	assert.Equal(t, "unavailable", body)
}

func TestProbesHead(t *testing.T) {
	testsupport.UseTempPersistence(t)
	for _, p := range []string{"/healthz", "/readyz"} {
		rec := apitest.Do(apitest.Router(), apitest.Request{Method: "HEAD", Path: p})
		assert.Equal(t, 200, rec.Code, p)
	}
}

func TestProbesOnlyGetAndHead(t *testing.T) {
	for _, p := range []string{"/healthz", "/readyz"} {
		rec := apitest.Do(apitest.Router(), apitest.Request{Method: "POST", Path: p})
		assert.Equal(t, 405, rec.Code, p)
	}
}

// The probes match their exact paths only, no prefix.
func TestProbesExactPath(t *testing.T) {
	code, _, _ := probe(t, "/healthz/x")
	assert.Equal(t, 404, code)
}
