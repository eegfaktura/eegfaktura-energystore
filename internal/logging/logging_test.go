package logging_test

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/logging"
	"at.ourproject/energystore/internal/testsupport/logcapture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	capture = logcapture.Capture
	decode  = logcapture.Decode
)

func TestJSONLineHasTheCommonFields(t *testing.T) {
	out := capture(t)
	logging.Log(t.Context(), 0, slog.LevelInfo, "hello", logging.ErrorAttrs(errors.New("boom"))...)

	lines := out.Lines()
	require.Len(t, lines, 1)
	m := decode(t, lines[0])
	assert.Regexp(t, regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$`), m["@timestamp"])
	assert.Equal(t, "info", m["log.level"])
	assert.Equal(t, "hello", m["message"])
	assert.Equal(t, logging.ServiceName, m["service.name"])
	assert.Equal(t, "dev", m["service.version"], "without SERVICE_VERSION")
	assert.Equal(t, "*errors.errorString", m["error.type"])
	assert.Equal(t, "boom", m["error.message"])
	for _, k := range []string{"time", "level", "msg"} {
		assert.NotContains(t, m, k, "slog's own key %q is renamed", k)
	}
}

func TestUnknownFieldsAreDropped(t *testing.T) {
	out := capture(t, "SERVICE_VERSION=1.8.2")
	slog.Default().With("user.email", "a@b.example").Info("x", "iban", "AT61", "client.id", "app")
	m := decode(t, out.Lines()[0])
	assert.Equal(t, "app", m["client.id"])
	assert.Equal(t, "1.8.2", m["service.version"])
	assert.NotContains(t, m, "iban")
	assert.NotContains(t, m, "user.email")
}

// tenant.id and the request ids come only from the request context, never from a caller: a caller
// could write them twice or bypass the tenant rule.
func TestRequestFieldsOnlyFromTheContext(t *testing.T) {
	out := capture(t)
	ids := []any{"tenant.id", "not a tenant", "request.id", "r", "trace.id", "t", "span.id", "s"}
	slog.Info("outside", ids...)
	slog.Default().With(ids...).Info("with")
	logging.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.SetCaller(r.Context(), "te100001", "client")
		slog.InfoContext(r.Context(), "inside", ids...)
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))

	lines := out.Lines()
	require.Len(t, lines, 4, "two lines outside, one inside, the access line")
	for _, line := range lines[:2] {
		m := decode(t, line)
		for _, k := range []string{"tenant.id", "request.id", "trace.id", "span.id"} {
			assert.NotContains(t, m, k, line)
		}
	}
	inside := decode(t, lines[2])
	assert.Equal(t, "TE100001", inside["tenant.id"], "the context tenant, upper-cased")
	assert.NotEqual(t, "r", inside["request.id"])
	assert.NotContains(t, inside, "trace.id")
	for _, k := range []string{"tenant.id", "request.id"} {
		assert.Equal(t, 1, strings.Count(lines[2], `"`+k+`"`), "%s written once", k)
	}
}

// audit.operator (decision 2026-10-30) appears on the raw-data deletion events only.
func TestAuditOperatorOnlyOnAuditEvents(t *testing.T) {
	out := capture(t)
	logging.Event(t.Context(), slog.LevelWarn, "rawdata.delete.done", "done", logging.AuditOperator("ops.person"))
	logging.Event(t.Context(), slog.LevelError, "rawdata.delete.failed", "failed", logging.AuditOperator("ops.person"))
	logging.Event(t.Context(), slog.LevelInfo, "http.access", "access", logging.AuditOperator("ops.person"))
	logging.Log(t.Context(), 0, slog.LevelInfo, "no event", logging.AuditOperator("ops.person"))
	slog.Default().With(logging.AuditOperator("ops.person")).Info("with", "event.action", "rawdata.delete.done")
	lines := out.Lines()
	require.Len(t, lines, 5)
	for i, line := range lines {
		m := decode(t, line)
		if i < 2 {
			assert.Equal(t, "ops.person", m["audit.operator"], line)
			assert.Equal(t, "logging/logging_test.go", m["log.origin.file.name"])
		} else {
			assert.NotContains(t, line, "ops.person", "audit.operator only on rawdata.delete.*")
		}
	}
}

func TestTextFormat(t *testing.T) {
	out := capture(t, "LOG_FORMAT=text")
	logging.Log(t.Context(), 0, slog.LevelWarn, "careful")
	line := out.Lines()[0]
	assert.Contains(t, line, "log.level=warn")
	assert.Contains(t, line, "message=careful")
	assert.Contains(t, line, "service.name="+logging.ServiceName)
}

func TestLevels(t *testing.T) {
	cases := []struct {
		env     string
		written []string
	}{
		{"", []string{"info", "warn", "error"}},
		{"LOG_LEVEL=trace", []string{"trace", "debug", "info", "warn", "error"}},
		{"LOG_LEVEL=DEBUG", []string{"debug", "info", "warn", "error"}},
		{"LOG_LEVEL=warn", []string{"warn", "error"}},
		{"LOG_LEVEL=warning", []string{"warn", "error"}},
		{"LOG_LEVEL=error", []string{"error"}},
	}
	for _, c := range cases {
		out := capture(t, c.env)
		for _, l := range []slog.Level{logging.LevelTrace, slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
			logging.Log(t.Context(), 0, l, "x")
		}
		var got []string
		for _, m := range out.Records(t) {
			got = append(got, m["log.level"].(string))
		}
		assert.Equal(t, c.written, got, c.env)
	}
}

func TestUnknownSettingsAreRefused(t *testing.T) {
	logcapture.Restore(t)
	for _, env := range []string{"LOG_LEVEL=verbose", "LOG_FORMAT=xml", "LOG_LEVEL_REST=loud"} {
		assert.Error(t, logging.Configure([]string{env}, &logcapture.Buffer{}), env)
	}
}

func TestPackageOverride(t *testing.T) {
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:]) // a pc in this package (logging_test)
	pc := pcs[0]

	out := capture(t, "LOG_LEVEL=warn", "LOG_LEVEL_LOGGING_TEST=debug")
	assert.True(t, logging.Enabled(slog.LevelDebug, 0), "this package is lowered to debug")
	assert.False(t, logging.Enabled(logging.LevelTrace, 0), "but not to trace")
	logging.Log(t.Context(), pc, slog.LevelDebug, "from logging_test")
	logging.Log(t.Context(), 0, slog.LevelInfo, "from nowhere")
	lines := out.Lines()
	require.Len(t, lines, 1)
	m := decode(t, lines[0])
	assert.Equal(t, "from logging_test", m["message"])
	assert.Equal(t, "logging/logging_test.go", m["log.origin.file.name"])

	out = capture(t, "LOG_LEVEL=warn", "LOG_LEVEL_REST=debug")
	logging.Log(t.Context(), pc, slog.LevelDebug, "from logging_test")
	assert.Empty(t, out.Lines(), "an override for another package changes nothing here")
}

func TestStandardLogGoesThroughTheModule(t *testing.T) {
	out := capture(t)
	log.Printf("std %d", 1)
	m := decode(t, out.Lines()[0])
	assert.Equal(t, "std 1", m["message"])
	assert.Equal(t, "info", m["log.level"])
	assert.Equal(t, logging.ServiceName, m["service.name"])
}

// An empty LOG_LEVEL_<PKG> is no override (it would pin the package to info).
func TestEmptyPackageOverrideIsIgnored(t *testing.T) {
	capture(t, "LOG_LEVEL=debug", "LOG_LEVEL_LOGGING_TEST= ")
	assert.True(t, logging.Enabled(slog.LevelDebug, 0), "the global level applies, not info")
}

func TestStartLine(t *testing.T) {
	out := capture(t, "SERVICE_VERSION=1.9.0", "DEPLOYMENT_ENVIRONMENT=pilot", "LOG_LEVEL=error",
		"LOG_LEVEL_MQTTCLIENT=debug", "LOG_LEVEL_EBOW=trace", "MQTT_PASSWORD=s3cret")
	logging.Started()

	lines := out.Lines()
	require.Len(t, lines, 1, "written at every LOG_LEVEL")
	m := decode(t, lines[0])
	assert.Equal(t, "service.start", m["event.action"])
	assert.Equal(t, "1.9.0", m["service.version"])
	assert.Equal(t, "error", m["log.effective_level"], "one plain level, in all services")
	assert.Equal(t, "service start", m["message"], "the same in all services, no overrides named")
	assert.Equal(t, "pilot", m["deployment.environment"])
	assert.ElementsMatch(t, []string{"@timestamp", "log.level", "message", "service.name", "service.version",
		"deployment.environment", "event.action", "log.effective_level"}, keys(m), "nothing else, so no secret")
	assert.NotContains(t, lines[0], "s3cret")
}

func TestStartLineEffectiveLevel(t *testing.T) {
	for env, want := range map[string]string{"": "info", "LOG_LEVEL=warning": "warn", "LOG_LEVEL=TRACE": "trace"} {
		out := capture(t, env)
		logging.Started()
		m := decode(t, out.Lines()[0])
		assert.Equal(t, want, m["log.effective_level"], env)
		assert.Equal(t, "info", m["log.level"], env)
		assert.Equal(t, "service start", m["message"], env)
	}
}

func keys(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestDeploymentEnvironmentOnEveryLine(t *testing.T) {
	out := capture(t, "DEPLOYMENT_ENVIRONMENT= prod ")
	logging.Log(t.Context(), 0, slog.LevelInfo, "direct")
	slog.Info("through slog")
	log.Print("through the standard logger")
	for _, m := range out.Records(t) {
		assert.Equal(t, "prod", m["deployment.environment"], m["message"])
	}
	require.Len(t, out.Lines(), 3)
}

func TestDeploymentEnvironmentOmittedWhenUnset(t *testing.T) {
	for _, env := range [][]string{nil, {"DEPLOYMENT_ENVIRONMENT="}, {"DEPLOYMENT_ENVIRONMENT=  "}} {
		out := capture(t, env...)
		logging.Log(t.Context(), 0, slog.LevelInfo, "x")
		assert.NotContains(t, decode(t, out.Lines()[0]), "deployment.environment", env)
	}
}

// The configuration in use before a test is back afterwards, without reading the environment again.
func TestCaptureRestoresTheConfiguration(t *testing.T) {
	outer := capture(t, "LOG_LEVEL=info")
	t.Run("inner", func(t *testing.T) {
		inner := capture(t, "LOG_LEVEL=error")
		slog.Info("dropped at error")
		assert.Empty(t, inner.Lines())
	})
	slog.Info("outer")
	log.Print("std")
	rs := outer.Records(t)
	require.Len(t, rs, 2, "slog and the standard logger are back on the outer configuration")
	assert.Equal(t, "outer", rs[0]["message"])
	assert.Equal(t, "std", rs[1]["message"])
}
