package logging_test

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/logging"
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

// testRouter mimics the service: a mux route behind a token check that calls logging.SetCaller.
func testRouter(status int) http.Handler {
	r := mux.NewRouter()
	r.Use(logging.MuxRoute)
	r.HandleFunc("/eeg/{ecid}/thing", func(w http.ResponseWriter, r *http.Request) {
		if status < 400 {
			logging.SetCaller(r.Context(), "TE100001", "at.ourproject.vfeeg.app")
		}
		w.WriteHeader(status)
	})
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {})
	return logging.AccessLog(r)
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
	lines := out.Lines()
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
	line := out.Lines()[0]
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
	lines := out.Lines()
	require.Len(t, lines, 2)
	refused := decode(t, lines[0])
	assert.Equal(t, "refused", refused["event.outcome"])
	assert.Equal(t, "warn", refused["log.level"], "401/403 at warn, as in the backend")
	assert.Equal(t, "GET /eeg/{ecid}/thing 403", refused["message"])
	assert.NotContains(t, refused, "tenant.id", "no tenant before the token check passed")
	unmatched := decode(t, lines[1])
	assert.NotContains(t, unmatched, "http.route")
	assert.Equal(t, "GET - 404", unmatched["message"])
	assert.Equal(t, "failure", unmatched["event.outcome"])
	assert.Equal(t, "info", unmatched["log.level"])
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
	for _, line := range out.Lines() {
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
	traceID := func(tp string) any {
		out := capture(t)
		req := httptest.NewRequest("GET", "/x", nil)
		req.Header.Set("traceparent", tp)
		logging.AccessLog(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(httptest.NewRecorder(), req)
		return decode(t, out.Lines()[0])["trace.id"]
	}
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", traceID("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"))
	for _, bad := range []string{
		"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7",
	} {
		assert.Nil(t, traceID(bad), bad)
	}
}

func TestProbesAreNotLogged(t *testing.T) {
	out := capture(t)
	testRouter(200).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/healthz", nil))
	assert.Empty(t, out.Lines())
}

func TestPropagate(t *testing.T) {
	capture(t)
	var got http.Header
	r := mux.NewRouter()
	r.HandleFunc("/x", func(w http.ResponseWriter, r *http.Request) {
		got = http.Header{}
		logging.Propagate(r.Context(), got)
	})
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-Id", "req-2")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	logging.AccessLog(r).ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "req-2", got.Get("X-Request-Id"))
	// a child span: same trace id and flags, a new parent id
	out := got.Get("traceparent")
	assert.Regexp(t, `^00-4bf92f3577b34da6a3ce929d0e0e4736-[0-9a-f]{16}-01$`, out)
	assert.NotEqual(t, "00f067aa0ba902b7", out[36:52], "not the incoming span id")
	assert.NotEqual(t, "0000000000000000", out[36:52], "a valid parent id")

	// without an incoming traceparent: only X-Request-Id
	req = httptest.NewRequest("GET", "/x", nil)
	logging.AccessLog(r).ServeHTTP(httptest.NewRecorder(), req)
	assert.Regexp(t, `^[0-9a-f]{32}$`, got.Get("X-Request-Id"))
	assert.Empty(t, got.Get("traceparent"))

	h := http.Header{}
	logging.Propagate(t.Context(), h) // outside a request: nothing
	assert.Empty(t, h)
}

// The GraphQL websocket transport hijacks the connection; the access wrapper must pass Hijack on.
func TestAccessLogKeepsHijack(t *testing.T) {
	capture(t)
	// the handler runs on the server's goroutine: report back over a channel, no require there
	hijackErr := make(chan error, 1)
	h := logging.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			hijackErr <- errors.New("statusWriter does not implement http.Hijacker")
			return
		}
		conn, _, err := hj.Hijack()
		if err == nil {
			_ = conn.Close()
		}
		hijackErr <- err
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	res, err := http.Get(srv.URL + "/x")
	if err == nil {
		_ = res.Body.Close()
	}
	assert.NoError(t, <-hijackErr, "the handler hijacked the connection")

	// a writer that cannot hijack reports an error instead of panicking
	logging.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, err := w.(http.Hijacker).Hijack()
		hijackErr <- err
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	assert.Error(t, <-hijackErr)
}

// A resolver panic is one JSON line through the module (not a plain stack on stderr); the client
// gets a generic error.
func TestGQLRecover(t *testing.T) {
	out := capture(t)
	err := logging.GQLRecover(t.Context(), "nil map")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal system error")
	m := decode(t, out.Lines()[0])
	assert.Equal(t, "error", m["log.level"])
	assert.Equal(t, "graphql.panic", m["event.action"])
	assert.Equal(t, "string", m["error.type"])
	assert.Equal(t, "nil map", m["error.message"])
	assert.Contains(t, m["error.stack_trace"], "logging.GQLRecover")
}

// A handler panic: one error line with error.* (no remote address), an access line with 500, and
// the client gets 500.
func TestAccessLineHandlerPanic(t *testing.T) {
	out := capture(t)
	r := mux.NewRouter()
	r.Use(logging.MuxRoute)
	r.HandleFunc("/eeg/{ecid}/thing", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	req := httptest.NewRequest("GET", "/eeg/RC1/thing", nil)
	req.RemoteAddr = "198.51.100.9:4711"
	rec := httptest.NewRecorder()
	logging.AccessLog(r).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	lines := out.Lines()
	require.Len(t, lines, 2)
	e := decode(t, lines[0])
	assert.Equal(t, "error", e["log.level"])
	assert.Equal(t, "http.panic", e["event.action"])
	assert.Equal(t, "boom", e["error.message"])
	assert.Contains(t, e["error.stack_trace"], "TestAccessLineHandlerPanic")
	a := decode(t, lines[1])
	assert.Equal(t, e["request.id"], a["request.id"])
	assert.EqualValues(t, 500, a["http.response.status_code"])
	assert.Equal(t, "failure", a["event.outcome"])
	assert.Equal(t, "GET /eeg/{ecid}/thing 500", a["message"])
	assert.NotContains(t, lines[0]+lines[1], "198.51.100.9")
}

// After the response has begun, the connection is aborted with http.ErrAbortHandler.
func TestAccessLineHandlerPanicAfterWrite(t *testing.T) {
	out := capture(t)
	h := logging.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic("late")
	}))
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	})
	lines := out.Lines()
	require.Len(t, lines, 2)
	assert.Equal(t, "late", decode(t, lines[0])["error.message"])
	assert.EqualValues(t, 500, decode(t, lines[1])["http.response.status_code"])
}

// Every line of a request carries its request.id, and tenant.id once the token check confirmed it;
// the access line gets tenant.id the same way, so it is written once.
func TestLinesOfARequestCarryTheIds(t *testing.T) {
	out := capture(t)
	h := logging.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.Log(r.Context(), 0, slog.LevelWarn, "before the check")
		logging.SetCaller(r.Context(), "TE100001", "at.ourproject.vfeeg.app")
		logging.Log(r.Context(), 0, slog.LevelError, "after the check")
	}))
	req := httptest.NewRequest("GET", "/eeg/RC100001/thing", nil)
	req.Header.Set("X-Request-Id", "req-7")
	h.ServeHTTP(httptest.NewRecorder(), req)

	lines := out.Lines()
	require.Len(t, lines, 3)
	before, after, access := decode(t, lines[0]), decode(t, lines[1]), decode(t, lines[2])
	assert.Equal(t, "req-7", before["request.id"])
	assert.NotContains(t, before, "tenant.id", "not confirmed yet")
	assert.Equal(t, "req-7", after["request.id"])
	assert.Equal(t, "TE100001", after["tenant.id"])
	assert.Equal(t, "TE100001", access["tenant.id"])
	assert.Equal(t, 1, strings.Count(lines[2], `"tenant.id"`), "no duplicate key")
}

// A line outside a request has neither.
func TestLineOutsideARequestHasNoIds(t *testing.T) {
	out := capture(t)
	logging.Log(t.Context(), 0, slog.LevelError, "mqtt")
	m := decode(t, out.Lines()[0])
	assert.NotContains(t, m, "request.id")
	assert.NotContains(t, m, "tenant.id")
}

// tenant.id only for a value of the agreed form, upper-cased; the client stays either way.
func TestSetCallerValidatesTheTenant(t *testing.T) {
	for in, want := range map[string]string{
		"RC100123": "RC100123", "rc100123": "RC100123", "GC123456-001": "GC123456-001", "gc123456_002": "GC123456_002",
		strings.Repeat("A", 64): strings.Repeat("A", 64),
		strings.Repeat("A", 65): "", "": "", "-RC1": "", "_RC1": "", "RC 1": "", "RC1\n": "", `RC1"x`: "", "RC.1": "", "RC/1": "",
	} {
		out := capture(t)
		h := logging.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { logging.SetCaller(r.Context(), in, "client") }))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
		m := decode(t, out.Lines()[0])
		assert.Equal(t, "client", m["client.id"], in)
		if want == "" {
			assert.NotContains(t, m, "tenant.id", in)
		} else {
			assert.Equal(t, want, m["tenant.id"], in)
		}
	}
}
