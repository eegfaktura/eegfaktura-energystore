// Package testsupport holds shared helpers of the test suite (ported from the branch
// improve-test-environment): Badger stores under t.TempDir(), CR_MSG payloads and a fake MQTT client.
// Test code only; no production package imports it.
package testsupport

import (
	"testing"

	"github.com/spf13/viper"
)

// UseTempPersistence points viper's persistence.path at a fresh t.TempDir() and restores the old
// path when the test ends. The test must return every store it opened (the pool closes a store
// when its last handle comes back, and opens it again under the then current path). ebow.ClosePool
// is not called: on main it shuts the process-wide pool down for good.
func UseTempPersistence(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	old := viper.Get("persistence.path")
	viper.Set("persistence.path", dir)
	t.Cleanup(func() { viper.Set("persistence.path", old) })
	return dir
}
