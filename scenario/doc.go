// Package scenario holds storage, import and report scenarios across packages (ported in part from
// the branch improve-test-environment, docs/improve-test-environment/m03-storage-scenarios.md). It
// has test files only: data is imported through the production MQTT importer into a store under
// t.TempDir() and read back the way the callers read it.
package scenario
