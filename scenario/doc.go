// Package scenario holds the storage, import and report scenarios S1 – S12 of the test concept
// (milestone M3). It has test files only: data is
// imported through the production paths (the MQTT importer, the Excel importer) into a store under
// t.TempDir() and read back the way the callers read it. Expected values come from oracle tables
// computed in the test files, never from the code's own output.
package scenario
