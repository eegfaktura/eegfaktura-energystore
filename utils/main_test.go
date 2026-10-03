package utils

import (
	"testing"

	"at.ourproject/energystore/internal/testsupport/tz"
)

// TestMain fixes time.Local to Europe/Vienna, the zone of the stored row ids (known-errors #9).
func TestMain(m *testing.M) { tz.Main(m) }
