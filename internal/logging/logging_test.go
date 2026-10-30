package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is safe for the concurrent writes of a running service.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Split(strings.TrimSpace(b.buf.String()), "\n")
}

// capture configures the module with env, writing into a buffer; the default is restored after the test.
func capture(t *testing.T, env ...string) *syncBuffer {
	t.Helper()
	out := &syncBuffer{}
	require.NoError(t, Configure(env, out))
	t.Cleanup(func() { _ = Configure(os.Environ(), os.Stdout) })
	return out
}

func decode(t *testing.T, line string) map[string]any {
	t.Helper()
	m := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(line), &m), line)
	return m
}

func TestJSONLineHasTheCommonFields(t *testing.T) {
	out := capture(t)
	Log(t.Context(), 0, slog.LevelInfo, "hello", ErrorAttrs(errors.New("boom"))...)

	lines := out.lines()
	require.Len(t, lines, 1)
	m := decode(t, lines[0])
	assert.Regexp(t, regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$`), m["@timestamp"])
	assert.Equal(t, "info", m["log.level"])
	assert.Equal(t, "hello", m["message"])
	assert.Equal(t, ServiceName, m["service.name"])
	assert.Equal(t, Version, m["service.version"])
	assert.Equal(t, "*errors.errorString", m["error.type"])
	assert.Equal(t, "boom", m["error.message"])
	for _, k := range []string{"time", "level", "msg"} {
		assert.NotContains(t, m, k, "slog's own key %q is renamed", k)
	}
}

func TestUnknownFieldsAreDropped(t *testing.T) {
	out := capture(t, "SERVICE_VERSION=1.8.2")
	slog.Default().With("user.email", "a@b.example").Info("x", "iban", "AT61", "tenant.id", "TE100001")
	m := decode(t, out.lines()[0])
	assert.Equal(t, "TE100001", m["tenant.id"])
	assert.Equal(t, "1.8.2", m["service.version"])
	assert.NotContains(t, m, "iban")
	assert.NotContains(t, m, "user.email")
}

// audit.operator (decision 2026-10-30) appears on the raw-data deletion events only.
func TestAuditOperatorOnlyOnAuditEvents(t *testing.T) {
	out := capture(t)
	Event(t.Context(), slog.LevelWarn, "rawdata.delete.done", "done", AuditOperator("ops.person"))
	Event(t.Context(), slog.LevelError, "rawdata.delete.failed", "failed", AuditOperator("ops.person"))
	Event(t.Context(), slog.LevelInfo, "http.access", "access", AuditOperator("ops.person"))
	Log(t.Context(), 0, slog.LevelInfo, "no event", AuditOperator("ops.person"))
	slog.Default().With(AuditOperator("ops.person")).Info("with", "event.action", "rawdata.delete.done")
	lines := out.lines()
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
	Log(t.Context(), 0, slog.LevelWarn, "careful")
	line := out.lines()[0]
	assert.Contains(t, line, "log.level=warn")
	assert.Contains(t, line, "message=careful")
	assert.Contains(t, line, "service.name="+ServiceName)
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
		{"LOG_LEVEL=error", []string{"error"}},
	}
	for _, c := range cases {
		out := capture(t, c.env)
		for _, l := range []slog.Level{LevelTrace, slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
			Log(t.Context(), 0, l, "x")
		}
		var got []string
		for _, line := range out.lines() {
			if line != "" {
				got = append(got, decode(t, line)["log.level"].(string))
			}
		}
		assert.Equal(t, c.written, got, c.env)
	}
}

func TestUnknownSettingsAreRefused(t *testing.T) {
	t.Cleanup(func() { _ = Configure(os.Environ(), os.Stdout) })
	for _, env := range []string{"LOG_LEVEL=verbose", "LOG_FORMAT=xml", "LOG_LEVEL_REST=loud"} {
		assert.Error(t, Configure([]string{env}, &syncBuffer{}), env)
	}
}

func TestPackageOverride(t *testing.T) {
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:]) // a pc in this package
	pc := pcs[0]

	out := capture(t, "LOG_LEVEL=warn", "LOG_LEVEL_LOGGING=debug")
	assert.True(t, Enabled(pc, slog.LevelDebug), "this package is lowered to debug")
	assert.False(t, Enabled(0, slog.LevelDebug), "without a caller the global level applies")
	Log(t.Context(), pc, slog.LevelDebug, "from logging")
	Log(t.Context(), 0, slog.LevelInfo, "from nowhere")
	lines := out.lines()
	require.Len(t, lines, 1)
	m := decode(t, lines[0])
	assert.Equal(t, "from logging", m["message"])
	assert.Equal(t, "logging/logging_test.go", m["log.origin.file.name"])

	out = capture(t, "LOG_LEVEL=warn", "LOG_LEVEL_REST=debug")
	Log(t.Context(), pc, slog.LevelDebug, "from logging")
	assert.Equal(t, []string{""}, out.lines(), "an override for another package changes nothing here")
}

func TestStandardLogGoesThroughTheModule(t *testing.T) {
	out := capture(t)
	log.Printf("std %d", 1)
	m := decode(t, out.lines()[0])
	assert.Equal(t, "std 1", m["message"])
	assert.Equal(t, "info", m["log.level"])
	assert.Equal(t, ServiceName, m["service.name"])
}
