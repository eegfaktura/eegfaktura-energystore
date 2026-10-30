package logging

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accessFields is every key an access line may carry. A new key needs a decision against the
// never-log list (no user id, e-mail, name, user agent, IP, query value, body, token).
var accessFields = map[string]bool{
	"@timestamp": true, "log.level": true, "message": true, "service.name": true, "service.version": true,
	"log.origin.file.name": true, "log.origin.file.line": true,
	"event.action": true, "event.outcome": true, "event.duration": true,
	"http.request.method": true, "http.route": true, "http.response.status_code": true,
	"graphql.operation.name": true, "tenant.id": true, "client.id": true,
	"request.id": true, "trace.id": true, "span.id": true,
}

// testRouter mimics the service: a mux route behind a token check that calls SetCaller.
func testRouter(status int) http.Handler {
	r := mux.NewRouter()
	r.Use(MuxRoute)
	r.HandleFunc("/eeg/{ecid}/thing", func(w http.ResponseWriter, r *http.Request) {
		if status < 400 {
			SetCaller(r.Context(), "TE100001", "at.ourproject.vfeeg.app")
		}
		w.WriteHeader(status)
	})
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {})
	return AccessLog(r)
}

func sensitiveRequest() *http.Request {
	req := httptest.NewRequest("POST", "/eeg/RC100001/thing?meter=AT0030000000000000000000000000001", strings.NewReader(`{"iban":"AT611904300234573201"}`))
	req.Header.Set("Authorization", "Bearer eyJ.secret.token")
	req.Header.Set("User-Agent", "Mozilla/5.0 (secret agent)")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.RemoteAddr = "198.51.100.9:4711"
	return req
}

func TestAccessLine(t *testing.T) {
	out := capture(t)
	req := sensitiveRequest()
	req.Header.Set("X-Request-Id", "req-1")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	testRouter(200).ServeHTTP(rec, req)

	assert.Equal(t, "req-1", rec.Header().Get("X-Request-Id"))
	lines := out.lines()
	require.Len(t, lines, 1)
	m := decode(t, lines[0])
	assert.Equal(t, "http.access", m["event.action"])
	assert.Equal(t, "success", m["event.outcome"])
	assert.Equal(t, "POST", m["http.request.method"])
	assert.Equal(t, "/eeg/{ecid}/thing", m["http.route"])
	assert.EqualValues(t, 200, m["http.response.status_code"])
	assert.IsType(t, float64(0), m["event.duration"], "nanoseconds, an integer")
	assert.Equal(t, "req-1", m["request.id"])
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", m["trace.id"])
	assert.Equal(t, "00f067aa0ba902b7", m["span.id"])
	assert.Equal(t, "TE100001", m["tenant.id"])
	assert.Equal(t, "at.ourproject.vfeeg.app", m["client.id"])
}

// TestAccessLineNeverLogs guards the never-log list and "no user tracking" (DSGVO, 2026-10-29).
func TestAccessLineNeverLogs(t *testing.T) {
	out := capture(t)
	testRouter(200).ServeHTTP(httptest.NewRecorder(), sensitiveRequest())
	line := out.lines()[0]
	for k := range decode(t, line) {
		assert.True(t, accessFields[k], "unexpected field %q in the access line", k)
	}
	for _, s := range []string{"RC100001", "AT0030000000000000000000000000001", "AT611904300234573201",
		"secret", "Mozilla", "203.0.113.7", "198.51.100.9", "meter="} {
		assert.NotContains(t, line, s)
	}
}

func TestAccessLineRefusedAndUnmatched(t *testing.T) {
	out := capture(t)
	h := testRouter(403)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/eeg/RC1/thing", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/nowhere/RC1", nil))
	lines := out.lines()
	require.Len(t, lines, 2)
	refused := decode(t, lines[0])
	assert.Equal(t, "refused", refused["event.outcome"])
	assert.NotContains(t, refused, "tenant.id", "no tenant before the token check passed")
	unmatched := decode(t, lines[1])
	assert.NotContains(t, unmatched, "http.route")
	assert.Equal(t, "GET - Not Found", unmatched["message"])
	assert.Equal(t, "failure", unmatched["event.outcome"])
	assert.NotContains(t, lines[1], "nowhere")
}

func TestAccessLineRequestID(t *testing.T) {
	out := capture(t)
	h := testRouter(200)
	for _, id := range []string{"", "has space", strings.Repeat("a", 129), "x\ny"} {
		req := httptest.NewRequest("GET", "/eeg/RC1/thing", nil)
		if id != "" {
			req.Header.Set("X-Request-Id", id)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		assert.Regexp(t, `^[0-9a-f]{32}$`, rec.Header().Get("X-Request-Id"), "%q is replaced", id)
	}
	for _, line := range out.lines() {
		assert.Regexp(t, `^[0-9a-f]{12}4[0-9a-f]{3}[89ab][0-9a-f]{15}$`, decode(t, line)["request.id"], "uuid4")
	}
	keep := strings.Repeat("a", 128)
	req := httptest.NewRequest("GET", "/eeg/RC1/thing", nil)
	req.Header.Set("X-Request-Id", keep)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, keep, rec.Header().Get("X-Request-Id"))
}

func TestTraceparentValidation(t *testing.T) {
	ok := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	_, _, valid := parseTraceparent(ok)
	assert.True(t, valid)
	for _, bad := range []string{
		"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
	} {
		_, _, valid := parseTraceparent(bad)
		assert.False(t, valid, bad)
	}
}

func TestProbesAreNotLogged(t *testing.T) {
	out := capture(t)
	testRouter(200).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/healthz", nil))
	assert.Equal(t, []string{""}, out.lines())
}

func TestPropagate(t *testing.T) {
	capture(t)
	var got http.Header
	r := mux.NewRouter()
	r.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) {
		got = http.Header{}
		Propagate(r.Context(), got)
	})
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-Id", "req-2")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	AccessLog(r).ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "req-2", got.Get("X-Request-Id"))
	// a child span: same trace id and flags, a new parent id
	out := got.Get("traceparent")
	assert.Regexp(t, `^00-4bf92f3577b34da6a3ce929d0e0e4736-[0-9a-f]{16}-01$`, out)
	assert.NotEqual(t, "00f067aa0ba902b7", out[36:52], "not the incoming span id")
	_, _, valid := parseTraceparent(out)
	assert.True(t, valid)

	// without an incoming traceparent: only X-Request-Id
	req = httptest.NewRequest("GET", "/x", nil)
	AccessLog(r).ServeHTTP(httptest.NewRecorder(), req)
	assert.Regexp(t, `^[0-9a-f]{32}$`, got.Get("X-Request-Id"))
	assert.Empty(t, got.Get("traceparent"))

	h := http.Header{}
	Propagate(t.Context(), h) // outside a request: nothing
	assert.Empty(t, h)
}

// The GraphQL websocket transport hijacks the connection; the access wrapper must pass Hijack on.
func TestAccessLogKeepsHijack(t *testing.T) {
	capture(t)
	var hijacked bool
	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		require.True(t, ok, "statusWriter implements http.Hijacker")
		conn, _, err := hj.Hijack()
		require.NoError(t, err)
		hijacked = true
		_ = conn.Close()
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/x")
	if err == nil {
		_ = res.Body.Close()
	}
	assert.True(t, hijacked)

	// a writer that cannot hijack reports an error instead of panicking
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder()}
	_, _, err = sw.Hijack()
	assert.Error(t, err)
}

// A resolver panic is one JSON line through the module (not a plain stack on stderr); the client
// gets a generic error.
func TestGQLRecover(t *testing.T) {
	out := capture(t)
	err := GQLRecover(t.Context(), "nil map")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal system error")
	m := decode(t, out.lines()[0])
	assert.Equal(t, "error", m["log.level"])
	assert.Equal(t, "graphql.panic", m["event.action"])
	assert.Equal(t, "string", m["error.type"])
	assert.Equal(t, "nil map", m["error.message"])
	assert.Contains(t, m["error.stack_trace"], "GQLRecover")
}
