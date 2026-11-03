// Package logcapture captures the lines of internal/logging in a test. It is a package of its own
// (not testsupport), so the tests of internal/logging and its glog shim can import it: testsupport
// imports the store, which logs through glog.
package logcapture

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"at.ourproject/energystore/internal/logging"
)

// Buffer holds the written lines; it is safe for the concurrent writes of a running service.
type Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// String is everything written so far.
func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Lines are the lines written so far, without empty ones.
func (b *Buffer) Lines() []string {
	var lines []string
	for _, l := range strings.Split(b.String(), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// Records are the lines decoded as JSON objects; a line that is not one fails the test.
func (b *Buffer) Records(t testing.TB) []map[string]any {
	t.Helper()
	var rs []map[string]any
	for _, l := range b.Lines() {
		rs = append(rs, Decode(t, l))
	}
	return rs
}

// Decode decodes one JSON line; a line that is not a JSON object fails the test.
func Decode(t testing.TB, line string) map[string]any {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("not a JSON line: %v: %s", err, line)
	}
	return m
}

// Capture configures internal/logging with env (KEY=VALUE entries, as os.Environ) writing into a
// new Buffer. The configuration in use before is put back when the test ends; the developer's
// environment is not read. An invalid env fails the test.
func Capture(t testing.TB, env ...string) *Buffer {
	t.Helper()
	Restore(t)
	out := &Buffer{}
	if err := logging.Configure(env, out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Restore puts the logging configuration in use now back when the test ends.
func Restore(t testing.TB) { t.Cleanup(logging.Snapshot()) }
