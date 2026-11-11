package ebow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// writeMeta's deferred Commit overwrites the error of Set (known-errors #45, F30): on a read-only
// store the write of the meta record fails, but writeMeta returns nil.
func TestWriteMetaReturnsSetError(t *testing.T) {
	t.Skip("known-errors #45")
	db := OpenTestDB(t)
	db.Put("arrows", Arrow{Id: "1"})
	db.Close()

	ro := db.OpenAgain(SetReadOnly(true))
	defer ro.Close()
	assert.Error(t, ro.DB().writeMeta(nil), "the meta record cannot be written to a read-only store")
}
