package services

import (
	"testing"

	"at.ourproject/energystore/internal/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetLastEnergyEntry answers lastRecordDate and the GraphQL lastEnergyDate: the latest PeriodEnd
// of all metering points of the community (the empty table of a GoLand skeleton before M3, #8).
func TestGetLastEnergyEntry(t *testing.T) {
	t.Run("latest period end, legacy four-digit seconds read", func(t *testing.T) {
		db := testsupport.TempStore(t, "TE100001", "RC100001")
		require.NoError(t, db.SetMeta(testsupport.CpMeta(
			testsupport.Consumer("AT001", 0, "01.01.2026 00:00:00", "31.05.2026 23:45:00"),
			testsupport.Consumer("AT002", 1, "01.01.2026 00:00:00", "30.06.2026 23:45:0000"),
			testsupport.Producer("AT003", 0, "01.01.2026 00:00:00", "15.06.2026 12:00:00"))))
		got, err := GetLastEnergyEntry("TE100001", "RC100001")
		require.NoError(t, err)
		assert.Equal(t, "30.06.2026 23:45:00", got)
	})

	t.Run("a store without meta record", func(t *testing.T) {
		testsupport.TempStore(t, "TE100001", "RC100002")
		_, err := GetLastEnergyEntry("TE100001", "RC100002")
		assert.Error(t, err)
	})

	t.Run("invalid community id", func(t *testing.T) {
		testsupport.UseTempPersistence(t)
		_, err := GetLastEnergyEntry("TE100001", "../x")
		assert.Error(t, err)
	})
}
