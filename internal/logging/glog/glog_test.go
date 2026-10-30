package glog

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func capture(t *testing.T, env ...string) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	require.NoError(t, logging.Configure(env, out))
	t.Cleanup(func() { _ = logging.Configure(os.Environ(), os.Stdout) })
	return out
}

func records(t *testing.T, out *bytes.Buffer) []map[string]any {
	t.Helper()
	var rs []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		m := map[string]any{}
		require.NoError(t, json.Unmarshal([]byte(line), &m), line)
		rs = append(rs, m)
	}
	return rs
}

func TestLevelsOfTheGlogCalls(t *testing.T) {
	out := capture(t, "LOG_LEVEL=trace")
	Info("i")
	Infof("if %d\n", 1)
	Warning("w")
	Warningf("wf")
	Error("e")
	Errorf("ef")
	V(1).Info("v1")
	V(3).Infof("v3")
	V(4).Info("v4")
	V(6).Infof("v6")

	want := [][2]string{{"info", "i"}, {"info", "if 1"}, {"warn", "w"}, {"warn", "wf"}, {"error", "e"},
		{"error", "ef"}, {"debug", "v1"}, {"debug", "v3"}, {"trace", "v4"}, {"trace", "v6"}}
	rs := records(t, out)
	require.Len(t, rs, len(want))
	for i, w := range want {
		assert.Equal(t, w[0], rs[i]["log.level"], w[1])
		assert.Equal(t, w[1], rs[i]["message"], "message without the trailing newline")
		assert.Equal(t, "glog/glog_test.go", rs[i]["log.origin.file.name"], "origin is the caller, not the shim")
	}
}

func TestVerbosityFollowsLogLevel(t *testing.T) {
	out := capture(t) // info
	assert.False(t, V(1).Enabled())
	V(3).Infof("hidden")
	Info("shown")
	require.Len(t, records(t, out), 1)

	capture(t, "LOG_LEVEL=debug")
	assert.True(t, V(3).Enabled())
	assert.False(t, V(4).Enabled(), "V(4) and above are trace")

	capture(t, "LOG_LEVEL=info", "LOG_LEVEL_GLOG=trace")
	assert.True(t, V(5).Enabled(), "the package override applies to the caller's package")
}

func TestErrorArgumentBecomesErrorFields(t *testing.T) {
	out := capture(t)
	Error(errors.New("disk full"))
	Errorf("open: %v", os.ErrNotExist)
	rs := records(t, out)
	require.Len(t, rs, 2)
	assert.Equal(t, "disk full", rs[0]["error.message"])
	assert.Equal(t, "*errors.errorString", rs[0]["error.type"])
	assert.Equal(t, "file does not exist", rs[1]["error.message"])
}

func TestFatalAndExit(t *testing.T) {
	out := capture(t)
	var codes []int
	exit = func(c int) { codes = append(codes, c) }
	t.Cleanup(func() { exit = os.Exit })
	Fatal("f")
	Fatalf("ff %d", 1)
	Exitf("ef")
	assert.Equal(t, []int{255, 255, 1}, codes)
	for _, r := range records(t, out) {
		assert.Equal(t, "error", r["log.level"])
	}
}

// The deployment still passes the glog flags (gitops: -logtostderr=true -stderrthreshold=INFO -v=3);
// they must not stop the start.
func TestFormerGlogFlagsAreAccepted(t *testing.T) {
	fs := flag.CommandLine
	require.NoError(t, fs.Parse([]string{"-logtostderr", "-alsologtostderr=false", "-stderrthreshold=INFO", "-v=3",
		"-vmodule=x=2", "-log_dir=/tmp"}))
	assert.ElementsMatch(t, []string{"v=3", "vmodule=x=2"}, IgnoredFlags())
}
