package ebow

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #31: a record that cannot be decoded is an error (not the end of the data), and Close releases
// iterator and transaction also after that error.

type decRow struct {
	Id     string `bow:"key"`
	Values []float64
}

type badDecRow struct {
	Id     string `bow:"key"`
	Values string
}

func openDecodeDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir())
	require.NoError(t, err)
	b := db.Bucket("rows")
	require.NoError(t, b.Put(decRow{Id: "a/1", Values: []float64{1}}))
	require.NoError(t, b.Put(badDecRow{Id: "a/2", Values: "garbage"}))
	require.NoError(t, b.Put(decRow{Id: "a/3", Values: []float64{3}}))
	return db
}

func TestRangeUndecodableRowIsAnErrorAndCloseReleases(t *testing.T) {
	db := openDecodeDB(t)
	r := db.Bucket("rows").Range("a/", "a/9")

	var row decRow
	require.True(t, r.Next(&row))
	assert.Equal(t, "a/1", row.Id)
	assert.False(t, r.Next(&row), "the undecodable row ends the loop")
	require.Error(t, r.Err())
	assert.True(t, errors.Is(r.Err(), ErrUndecodableRow))
	assert.Contains(t, r.Err().Error(), "a/2")

	r.Close()
	assert.True(t, r.closed, "Close released iterator and transaction after the decode error")
	r.Close() // a second Close is a no-op
	require.NoError(t, db.Close())
}

func TestIterUndecodableRowIsAnErrorAndCloseReleases(t *testing.T) {
	db := openDecodeDB(t)
	it := db.Bucket("rows").Iter()

	var row decRow
	require.True(t, it.Next(&row))
	assert.False(t, it.Next(&row))
	assert.True(t, errors.Is(it.Err(), ErrUndecodableRow))

	it.Close()
	assert.True(t, it.closed, "Close released iterator and transaction after the decode error")
	require.NoError(t, db.Close())
}

// When Open fails after Badger was opened (here: an undecodable meta record), Badger is closed
// again, so the directory lock is released and the store can be opened (and repaired) later.
func TestOpenReleasesBadgerOnMetaError(t *testing.T) {
	dir := t.TempDir()
	bdb, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.NoError(t, err)
	require.NoError(t, bdb.Update(func(txn *badger.Txn) error { return txn.Set(metaKey, []byte("garbage")) }))
	require.NoError(t, bdb.Close())

	_, err = Open(dir)
	require.Error(t, err)

	again, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.NoError(t, err, "directory lock released after the failed Open")
	require.NoError(t, again.Close())
}

// Iterators of a failed bucket carry the bucket's error and have no Badger iterator or
// transaction: Close must not panic (review of #85).
func TestErrorIteratorsCloseWithoutPanic(t *testing.T) {
	bucketErr := errors.New("bucket failed")
	b := &Bucket{err: bucketErr}

	r := b.Range("a/", "a/9")
	assert.NotPanics(t, r.Close)
	assert.Same(t, bucketErr, r.Err())

	p := b.Prefix("a/")
	assert.NotPanics(t, p.Close)
	assert.Same(t, bucketErr, p.Err())

	it := b.Iter()
	assert.NotPanics(t, it.Close)
	assert.Same(t, bucketErr, it.Err())
}

// Only a value the codec cannot decode is ErrUndecodableRow; an error of the result type is
// passed through unchanged.
func TestNonCodecErrorIsNotUndecodableRow(t *testing.T) {
	db := openDecodeDB(t)
	defer func() { require.NoError(t, db.Close()) }()
	r := db.Bucket("rows").Range("a/", "a/9")
	defer r.Close()

	var notAStruct int
	assert.False(t, r.Next(&notAStruct))
	require.Error(t, r.Err())
	assert.False(t, errors.Is(r.Err(), ErrUndecodableRow), "got %v", r.Err())
	assert.True(t, errors.Is(r.Err(), ErrReadRecord), "got %v", r.Err())
	assert.Contains(t, r.Err().Error(), "read record a/1")
}

// A read error may carry local file paths: API clients get only the generic text (#31).
func TestClientMessageHidesReadCause(t *testing.T) {
	err := fmt.Errorf("%w: read record CP/2026/06/01/12/00/00: %w", ErrReadRecord,
		errors.New("open /opt/energy/rawdata/te100001/RC100001/000001.vlog: no such file"))
	assert.Equal(t, "stored record cannot be read", ClientMessage(err))

	undecodable := fmt.Errorf("%w (key CP/x)", ErrUndecodableRow)
	assert.Equal(t, undecodable.Error(), ClientMessage(undecodable))
}
