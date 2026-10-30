// Package glog has the call shape of github.com/golang/glog, as far as energystore uses it, and
// writes through internal/logging: call sites only change their import. Levels:
//
//	Info*                 info
//	Warning*              warn
//	Error*                error
//	Fatal*, Exit*         error, then the process exits (255 / 1, as glog)
//	V(1..3).Info*         debug
//	V(4+).Info*           trace
//
// The old glog flags (-v, -vmodule, -logtostderr, -stderrthreshold, …) are still accepted so the
// existing deployment arguments do not stop the start, but they change nothing: the level is
// LOG_LEVEL (see IgnoredFlags).
package glog

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"at.ourproject/energystore/internal/logging"
)

// Level is the verbosity of V.
type Level int32

// Verbose is returned by V; its Info methods log only when Enabled.
type Verbose struct {
	enabled bool
	level   slog.Level
}

const ignoredUsage = "ignored (former glog flag); the log level is set with LOG_LEVEL"

func init() {
	for _, name := range []string{"v", "vmodule", "log_backtrace_at", "stderrthreshold", "log_dir", "log_link", "logbuflevel"} {
		flag.String(name, "", ignoredUsage)
	}
	for _, name := range []string{"logtostderr", "alsologtostderr"} { // given without a value
		flag.Var(&ignoredBool{}, name, ignoredUsage)
	}
}

// IgnoredFlags lists the former glog flags that change behaviour in glog but not here and were
// set on the command line (after flag.Parse); main warns about them.
func IgnoredFlags() []string {
	var set []string
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == "v" || f.Name == "vmodule" {
			set = append(set, f.Name+"="+f.Value.String())
		}
	})
	return set
}

type ignoredBool struct{ v string }

func (b *ignoredBool) String() string     { return b.v }
func (b *ignoredBool) Set(s string) error { b.v = s; return nil }
func (b *ignoredBool) IsBoolFlag() bool   { return true }

func vLevel(l Level) slog.Level {
	if l >= 4 {
		return logging.LevelTrace
	}
	return slog.LevelDebug
}

// V reports whether verbosity level l is on for the calling package. The caller's stack is read
// only when a per-package override could change the answer (V is called on hot paths).
func V(l Level) Verbose {
	lv := vLevel(l)
	if !logging.EnabledAnywhere(lv) {
		return Verbose{level: lv}
	}
	if !logging.HasOverrides() {
		return Verbose{enabled: true, level: lv}
	}
	return Verbose{enabled: logging.Enabled(callerPC(), lv), level: lv}
}

// Enabled replaces glog's `if glog.V(n) {`.
func (v Verbose) Enabled() bool { return v.enabled }

func (v Verbose) Info(args ...any) {
	if v.enabled {
		emit(v.level, fmt.Sprint(args...), args)
	}
}

func (v Verbose) Infof(format string, args ...any) {
	if v.enabled {
		emit(v.level, fmt.Sprintf(format, args...), args)
	}
}

func Info(args ...any)                    { emit(slog.LevelInfo, fmt.Sprint(args...), args) }
func Infof(format string, args ...any)    { emit(slog.LevelInfo, fmt.Sprintf(format, args...), args) }
func Warning(args ...any)                 { emit(slog.LevelWarn, fmt.Sprint(args...), args) }
func Warningf(format string, args ...any) { emit(slog.LevelWarn, fmt.Sprintf(format, args...), args) }
func Error(args ...any)                   { emit(slog.LevelError, fmt.Sprint(args...), args) }
func Errorf(format string, args ...any)   { emit(slog.LevelError, fmt.Sprintf(format, args...), args) }

// Fatal logs at error and exits with 255, as glog (without glog's goroutine dump).
func Fatal(args ...any) { emit(slog.LevelError, fmt.Sprint(args...), args); exit(255) }

// Fatalf logs at error and exits with 255.
func Fatalf(format string, args ...any) {
	emit(slog.LevelError, fmt.Sprintf(format, args...), args)
	exit(255)
}

// Exitf logs at error and exits with 1, as glog.
func Exitf(format string, args ...any) {
	emit(slog.LevelError, fmt.Sprintf(format, args...), args)
	exit(1)
}

var exit = os.Exit

// callerPC is the pc of the function that called the exported glog function.
func callerPC() uintptr {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // Callers, callerPC, the glog function
	return pcs[0]
}

// emit writes msg; the first error among args becomes error.type / error.message.
func emit(l slog.Level, msg string, args []any) {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) // Callers, emit, the glog function
	var attrs []slog.Attr
	for _, a := range args {
		if err, ok := a.(error); ok && err != nil {
			attrs = logging.ErrorAttrs(err)
			break
		}
	}
	logging.Log(context.Background(), pcs[0], l, strings.TrimRight(msg, "\n"), attrs...)
}
