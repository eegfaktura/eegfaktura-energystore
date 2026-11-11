package rest_test

import (
	"os"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport/fakeoidc"
)

// TestMain: the fake Keycloak was started by fakeoidc's init() before middleware's init()
// (see the fakeoidc package comment). fakeoidc also set time.Local to Europe/Vienna before its
// server goroutines started; writing it again here would race with them.
func TestMain(m *testing.M) {
	code := m.Run()
	fakeoidc.Close()
	os.Exit(code)
}

// TestInitOrder guards the order fakeoidc → middleware: middleware's init() read the generated
// config, otherwise it would have panicked on ./keycloak.json before any test ran.
func TestInitOrder(t *testing.T) {
	if !fakeoidc.Started() {
		t.Fatal("fakeoidc did not start")
	}
	if got := os.Getenv("KEYCLOAK_CONFIG"); got != fakeoidc.ConfigPath() {
		t.Fatalf("KEYCLOAK_CONFIG = %q, want the fake config %q", got, fakeoidc.ConfigPath())
	}
	if time.Local.String() != "Europe/Vienna" {
		t.Fatalf("time.Local = %s, want Europe/Vienna", time.Local)
	}
}
