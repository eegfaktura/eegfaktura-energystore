// Package logging is the one place every log line of energystore goes through (logging concept,
// W-261008-ca2): one slog handler that writes one JSON object per line (or key=value text) on
// stdout with the common ECS fields. The glog-shaped package internal/logging/glog and the
// standard library's log package both end up here.
//
// Settings (read from the environment when the package is initialised, so the init() of other
// packages already logs through it):
//
//	LOG_FORMAT        json (default) | text
//	LOG_LEVEL         trace | debug | info (default) | warn | error
//	LOG_LEVEL_<PKG>   the same levels for one package, <PKG> = last element of the import path in
//	                  upper case, e.g. LOG_LEVEL_MQTTCLIENT=debug, LOG_LEVEL_EBOW=trace
//	SERVICE_VERSION   service.version (set by the image build), default dev
//
// An unknown value is reported by Check; main refuses to start on it.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

// ServiceName is service.name in every line.
const ServiceName = "eegfaktura-energystore"

// Version is service.version in every line: SERVICE_VERSION, set by the image build (Dockerfile
// ARG/ENV from the release workflow), default "dev".
var Version = "dev"

// LevelTrace is below slog's debug: payload shapes, only locally.
const LevelTrace = slog.Level(-8)

var (
	startErr error
	cur      *handler
)

func init() {
	startErr = Configure(os.Environ(), os.Stdout)
}

// Check returns the configuration error found at start (unknown LOG_FORMAT or LOG_LEVEL*).
func Check() error { return startErr }

// Configure builds the handler from env (KEY=VALUE entries, as os.Environ) and makes it the default
// for slog and the standard log package. On an invalid setting it still installs a handler (json,
// info), so the error can be logged, and returns the error.
func Configure(env []string, out io.Writer) error {
	var errs []string
	vars := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		vars[k] = v
	}
	Version = "dev"
	if v := strings.TrimSpace(vars["SERVICE_VERSION"]); v != "" {
		Version = v
	}
	format := strings.ToLower(strings.TrimSpace(vars["LOG_FORMAT"]))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "text" {
		errs = append(errs, fmt.Sprintf("LOG_FORMAT=%q: want json or text", format))
		format = "json"
	}
	global, err := parseLevel(vars["LOG_LEVEL"])
	if err != nil {
		errs = append(errs, "LOG_LEVEL: "+err.Error())
		global = slog.LevelInfo
	}
	overrides := map[string]slog.Level{}
	for name, value := range vars {
		pkg, ok := strings.CutPrefix(name, "LOG_LEVEL_")
		if !ok || pkg == "" || strings.TrimSpace(value) == "" { // empty: no override
			continue
		}
		l, err := parseLevel(value)
		if err != nil {
			errs = append(errs, name+": "+err.Error())
			continue
		}
		overrides[strings.ToLower(pkg)] = l
	}

	opts := &slog.HandlerOptions{Level: LevelTrace, ReplaceAttr: replaceAttr}
	var inner slog.Handler
	if format == "json" {
		inner = slog.NewJSONHandler(out, opts)
	} else {
		inner = slog.NewTextHandler(out, opts)
	}
	inner = inner.WithAttrs([]slog.Attr{
		slog.String("service.name", ServiceName),
		slog.String("service.version", Version),
	})
	lowest := global
	for _, l := range overrides {
		lowest = min(lowest, l)
	}
	cur = &handler{inner: inner, global: global, min: lowest, overrides: overrides}
	slog.SetDefault(slog.New(cur))
	if len(errs) > 0 {
		return fmt.Errorf("logging: %s", strings.Join(errs, "; "))
	}
	return nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("%q: want trace, debug, info, warn or error", s)
}

func levelName(l slog.Level) string {
	switch {
	case l < slog.LevelDebug:
		return "trace"
	case l < slog.LevelInfo:
		return "debug"
	case l < slog.LevelWarn:
		return "info"
	case l < slog.LevelError:
		return "warn"
	}
	return "error"
}

// replaceAttr renames slog's built-in keys to the ECS names.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		return slog.String("@timestamp", a.Value.Time().UTC().Format("2006-01-02T15:04:05.000Z"))
	case slog.LevelKey:
		l, _ := a.Value.Any().(slog.Level)
		return slog.String("log.level", levelName(l))
	case slog.MessageKey:
		a.Key = "message"
	}
	return a
}

// Enabled reports whether a line at level l from the caller pc would be written (pc 0: the
// global level only).
func Enabled(pc uintptr, l slog.Level) bool { return cur.levelFor(pc) <= l }

// EnabledAnywhere reports whether level l reaches the global level or any package override; it
// needs no caller pc.
func EnabledAnywhere(l slog.Level) bool { return l >= cur.min }

// HasOverrides reports whether a LOG_LEVEL_<PKG> override is set (only then the caller's package
// matters).
func HasOverrides() bool { return len(cur.overrides) > 0 }

// handler applies LOG_LEVEL and the per-package overrides, and adds log.origin and the request
// ids carried in the context.
type handler struct {
	inner     slog.Handler
	global    slog.Level
	min       slog.Level // the lowest of global and the overrides
	overrides map[string]slog.Level
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.min }

// allowed are the only fields a line may carry besides @timestamp, log.level, message and
// service.* (never-log rule: an unknown attribute is dropped, not written).
var allowed = map[string]bool{
	"log.origin.file.name": true, "log.origin.file.line": true, "log.logger": true,
	"event.action": true, "event.outcome": true, "event.duration": true,
	"http.request.method": true, "http.route": true, "http.response.status_code": true,
	"graphql.operation.name": true, "tenant.id": true, "client.id": true,
	"request.id": true, "trace.id": true, "span.id": true,
	"error.type": true, "error.message": true, "error.stack_trace": true,
}

// auditOperator names the person who ran a destructive ops tool. Documented exception to "no persons
// in logs" (decision 2026-10-30): the superuser-only raw-data deletion keeps the operator for
// accountability. The field is written only on the events in auditEvents and dropped everywhere else.
const auditOperator = "audit.operator"

var auditEvents = map[string]bool{"rawdata.delete.done": true, "rawdata.delete.failed": true}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level < h.levelFor(r.PC) {
		return nil
	}
	if r.NumAttrs() > 0 {
		audit := false
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "event.action" && auditEvents[a.Value.String()] {
				audit = true
			}
			return true
		})
		kept := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		r.Attrs(func(a slog.Attr) bool {
			if allowed[a.Key] || (audit && a.Key == auditOperator) {
				kept.AddAttrs(a)
			}
			return true
		})
		r = kept
	}
	if r.PC != 0 {
		f, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		if f.File != "" {
			r.AddAttrs(slog.String("log.origin.file.name", path.Base(path.Dir(f.File))+"/"+path.Base(f.File)),
				slog.Int("log.origin.file.line", f.Line))
		}
	}
	if ids := idsFrom(ctx); ids != nil {
		r.AddAttrs(slog.String("request.id", ids.requestID))
		if ids.traceID != "" {
			r.AddAttrs(slog.String("trace.id", ids.traceID), slog.String("span.id", ids.spanID))
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *handler) WithAttrs(as []slog.Attr) slog.Handler {
	c := *h
	var kept []slog.Attr
	for _, a := range as {
		if allowed[a.Key] {
			kept = append(kept, a)
		}
	}
	c.inner = h.inner.WithAttrs(kept)
	return &c
}

func (h *handler) WithGroup(name string) slog.Handler {
	c := *h
	c.inner = h.inner.WithGroup(name)
	return &c
}

// levelFor is the level that applies to the package of pc.
func (h *handler) levelFor(pc uintptr) slog.Level {
	if len(h.overrides) == 0 || pc == 0 {
		return h.global
	}
	if l, ok := h.overrides[packageOf(pc)]; ok {
		return l
	}
	return h.global
}

// packageOf is the last element of the import path of the function at pc, in lower case
// ("at.ourproject/energystore/store/ebow.(*DB).Get" → "ebow").
func packageOf(pc uintptr) string {
	f, _ := runtime.CallersFrames([]uintptr{pc}).Next()
	name := f.Function
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.Index(name, "."); i >= 0 {
		name = name[:i]
	}
	return strings.ToLower(name)
}

// Log writes one line through the module with the caller pc (0: none).
func Log(ctx context.Context, pc uintptr, l slog.Level, msg string, attrs ...slog.Attr) {
	if !cur.Enabled(ctx, l) {
		return
	}
	r := slog.NewRecord(time.Now(), l, msg, pc)
	r.AddAttrs(attrs...)
	_ = cur.Handle(ctx, r)
}

// Event writes one line with event.action = action and the caller as log.origin.
func Event(ctx context.Context, l slog.Level, action, msg string, attrs ...slog.Attr) {
	var pcs [1]uintptr
	runtime.Callers(2, pcs[:]) // Callers, Event
	Log(ctx, pcs[0], l, msg, append([]slog.Attr{slog.String("event.action", action)}, attrs...)...)
}

// AuditOperator is the audit.operator field; it is written only on the events allowed for it.
func AuditOperator(name string) slog.Attr { return slog.String(auditOperator, name) }

// ErrorAttrs are error.type and error.message of err.
func ErrorAttrs(err error) []slog.Attr {
	return []slog.Attr{slog.String("error.type", fmt.Sprintf("%T", err)), slog.String("error.message", err.Error())}
}
