package ebow

import (
	"fmt"
	"os"
	"testing"

	"at.ourproject/energystore/internal/testsupport/tz"
	"github.com/spf13/viper"
)

// TestMain fixes the time zone and points the pool tests at a temporary persistence path
// (before M0 they wrote store/ebow/te*, known-errors #7). The pool tests share the
// process-wide connectionPool and depend on their order, so the directory is per binary.
func TestMain(m *testing.M) {
	tz.SetLocal()
	dir, err := os.MkdirTemp("", "ebow-pool-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	viper.Set("persistence.path", dir)
	code := m.Run()
	ClosePool()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
