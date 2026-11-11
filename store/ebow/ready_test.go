package ebow

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProbe swaps readyProbe and readyTimeout for one test.
func fakeProbe(t *testing.T, probe func(string) error, timeout time.Duration) {
	t.Helper()
	oldProbe, oldTimeout := readyProbe, readyTimeout
	readyProbe, readyTimeout = probe, timeout
	t.Cleanup(func() {
		readyProbe, readyTimeout = oldProbe, oldTimeout
		probeHanging = nil
	})
}

// usePath points persistence.path at path for one test (other tests of the package leave it changed).
func usePath(t *testing.T, path string) {
	t.Helper()
	old := viper.Get("persistence.path")
	viper.Set("persistence.path", path)
	t.Cleanup(func() { viper.Set("persistence.path", old) })
}

func TestReadyWritesAndRemovesProbeFile(t *testing.T) {
	dir := t.TempDir()
	usePath(t, dir)
	require.NoError(t, Ready())
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".readyz-", "probe file left behind")
	}
}

func TestReadyNotWritable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	usePath(t, file) // not a directory: no file can be created in it
	assert.Error(t, Ready())
}

func TestEnsurePersistencePathCreatesDir(t *testing.T) {
	usePath(t, filepath.Join(t.TempDir(), "opt", "rawdata"))
	require.Error(t, Ready(), "fresh volume without the directory")
	require.NoError(t, EnsurePersistencePath())
	assert.NoError(t, Ready())
}

// A hanging check times out, is not started a second time while it hangs, and its late result is
// discarded: the next probe runs a fresh check.
func TestReadyHangingCheck(t *testing.T) {
	usePath(t, t.TempDir())
	release := make(chan struct{})
	var calls atomic.Int32
	fakeProbe(t, func(string) error {
		if calls.Add(1) == 1 {
			<-release
			return errors.New("late result")
		}
		return nil
	}, 50*time.Millisecond)

	assert.EqualError(t, Ready(), "persistence path check timed out")
	assert.EqualError(t, Ready(), "persistence path check still hanging")
	assert.Equal(t, int32(1), calls.Load(), "hanging check started twice")

	close(release)
	assert.Eventually(t, func() bool { return Ready() == nil }, time.Second, 10*time.Millisecond,
		"the late result must be discarded and a fresh check run")
	assert.Equal(t, int32(2), calls.Load())
}
