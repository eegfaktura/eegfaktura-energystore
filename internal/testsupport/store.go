// Package testsupport holds the shared helpers of the test suite (concept T3, T7): Badger
// stores under t.TempDir(), builders for raw lines, the meta record and CR_MSG payloads, and a
// float tolerance for kWh. Test code only; no production package imports it.
package testsupport

import (
	"testing"

	"at.ourproject/energystore/internal/testsupport/tz"
	"at.ourproject/energystore/store/ebow"
	"github.com/spf13/viper"
)

// Vienna is Europe/Vienna (see package tz).
var Vienna = tz.Vienna

// UseTempPersistence points viper's persistence.path at a fresh t.TempDir(). The cleanup closes
// the process-wide ebow pool and replaces it with a fresh one (it keeps stores open across tests,
// and a closed pool stays closed since upstream #66 — see pool.go), then restores the old path;
// t.TempDir()'s own cleanup was registered before and therefore runs last.
func UseTempPersistence(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()
	old := viper.Get("persistence.path")
	viper.Set("persistence.path", dir)
	t.Cleanup(func() {
		resetPool()
		viper.Set("persistence.path", old)
	})
	return dir
}

// TempStore opens tenant/ecId through the production pool (ebow.OpenStorage) under a fresh
// temporary persistence path and closes it when the test ends.
func TempStore(t testing.TB, tenant, ecId string) *ebow.BowStorage {
	t.Helper()
	UseTempPersistence(t)
	db, err := ebow.OpenStorage(tenant, ecId)
	if err != nil {
		t.Fatalf("open store %s/%s: %v", tenant, ecId, err)
	}
	t.Cleanup(db.Close)
	return db
}

// TestDriverStore opens tenant/ecId with ebow.OpenStorageTest (no pool) under t.TempDir() and
// closes it when the test ends. Returns the store and its base directory.
func TestDriverStore(t testing.TB, tenant, ecId string) (*ebow.BowStorage, string) {
	t.Helper()
	dir := t.TempDir()
	db, err := ebow.OpenStorageTest(tenant, ecId, dir)
	if err != nil {
		t.Fatalf("open test store %s/%s: %v", tenant, ecId, err)
	}
	t.Cleanup(db.CloseTestDriver)
	return db, dir
}
