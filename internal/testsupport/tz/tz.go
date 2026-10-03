// Package tz fixes the time zone of a test binary to Europe/Vienna, the zone the row ids of
// production are written in (AGENTS.md section 4). It has no dependencies inside the module,
// so in-package tests of model and store/ebow can use it without an import cycle.
package tz

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// Vienna is Europe/Vienna. Nil when the zone database is missing; TestVienna then fails.
var Vienna, viennaErr = time.LoadLocation("Europe/Vienna")

// The DST days of 2026 in Vienna: the spring day has 92 quarter hours, the autumn day 100.
var (
	SpringForward2026 = time.Date(2026, time.March, 29, 0, 0, 0, 0, mustVienna())
	FallBack2026      = time.Date(2026, time.October, 25, 0, 0, 0, 0, mustVienna())
)

func mustVienna() *time.Location {
	if Vienna == nil {
		return time.UTC
	}
	return Vienna
}

// Err returns the error of loading Europe/Vienna, nil when the zone database is present.
func Err() error { return viennaErr }

// SetLocal makes Europe/Vienna the process-wide time.Local. Called from TestMain before any test.
func SetLocal() {
	if viennaErr != nil {
		fmt.Fprintf(os.Stderr, "tz: Europe/Vienna not available: %v\n", viennaErr)
		os.Exit(2)
	}
	time.Local = Vienna
}

// Main is a TestMain body: fix the zone, run the tests.
func Main(m *testing.M) {
	SetLocal()
	os.Exit(m.Run())
}
