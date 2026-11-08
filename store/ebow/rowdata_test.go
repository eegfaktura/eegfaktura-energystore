package ebow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An ecId may carry dashes inside (261016-ca1); everything that could leave the directory stays refused.
func TestOpenStorageEcIdWithDash(t *testing.T) {
	s, err := OpenStorage("TE100300", "AT00300000000GC90001000000000-001")
	require.NoError(t, err)
	s.Close()

	for _, ecId := range []string{"-abc", "abc-", "a--b", "a/-b", "a-/b", "..-a", "a-..", "a-.b"} {
		s, err := OpenStorage("TE100300", ecId)
		assert.Error(t, err, "ecId=%q", ecId)
		assert.Nil(t, s, "ecId=%q", ecId)
	}
}
