package ebow

import (
	"path/filepath"
	"testing"

	"at.ourproject/energystore/model"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A GEA sub-community reads and writes the store of its stem: eda-xp publishes its CR_MSG under the
// RC number, so that is where its energy is. No second directory comes up (261008-ca3).
func TestOpenStorageSubCommunityTenantUsesTheStemStore(t *testing.T) {
	const ecId = "AT00300000000GC100403000000000001"

	base, err := OpenStorage("GC100403", ecId)
	require.NoError(t, err)
	defer base.Close()
	require.NoError(t, base.SetMeta(&model.RawSourceMeta{Id: "cpmeta/0"}))

	sub, err := OpenStorage("GC100403-001", ecId)
	require.NoError(t, err)
	defer sub.Close()

	_, err = sub.GetMeta("cpmeta/0")
	assert.NoError(t, err, "the sub-community sees the stem's data")
	assert.Equal(t, "GC100403", sub.GetTenant())

	root := viper.GetString("persistence.path")
	assert.DirExists(t, filepath.Join(root, "gc100403", ecId))
	assert.NoDirExists(t, filepath.Join(root, "gc100403-001"))
}

func TestOpenStorageRefusesOtherTenantSuffixes(t *testing.T) {
	for _, tenant := range []string{
		"GC100403-", "GC100403-1", "GC100403-0001", "GC100403-00a", "GC100403-001-001",
		"-001", "../x-001", "GC1004030-001", "GC100403001",
	} {
		s, err := OpenStorage(tenant, "ECID0001")
		assert.Error(t, err, "tenant=%q", tenant)
		assert.Nil(t, s, "tenant=%q", tenant)
	}
}
