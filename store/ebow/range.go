package ebow

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
)

type IRange interface {
	Next(result interface{}) bool
	Close()
	Err() error
}

type Range struct {
	until      []byte
	bucket     *Bucket
	prefix     []byte
	txn        *badger.Txn
	it         *badger.Iterator
	resultType *structType
	advanced   bool
	closed     bool
	err        error
}

func newRange(bucket *Bucket, prefix, until []byte) *Range {
	prefix = bucket.internalKey(prefix)
	until = bucket.internalKey(until)
	opts := badger.DefaultIteratorOptions
	//opts.PrefetchSize = runtime.GOMAXPROCS(-1)
	opts.PrefetchSize = 200
	txn := bucket.db.db.NewTransaction(false)
	it := txn.NewIterator(opts)
	it.Seek(prefix)
	return &Range{
		until:  until,
		bucket: bucket,
		txn:    txn,
		it:     it,
		prefix: prefix,
	}
}

func (ra *Range) Next(result interface{}) bool {
	if ra.err != nil {
		return false
	}
	if ra.closed {
		return false
	}
	if ra.advanced {
		ra.it.Next()
	}
	if !ra.it.Valid() {
		ra.Close()
		return false
	}
	item := ra.it.Item()
	ik := item.Key()
	if len(ik) < len(ra.until) {
		return false
	}
	if bytes.Compare(ra.until, ik[:len(ra.until)]) < 0 {
		ra.Close()
		return false
	}

	// #31: on an error the caller sees false like at the end of the data and must check Err().
	if err := decodeItem(ra.bucket, &ra.resultType, item, result); err != nil {
		ra.err = err
		return false
	}

	if !ra.advanced {
		ra.advanced = true
	}
	return true
}

func (ra *Range) Err() error {
	return ra.err
}

// Close closes the Iter. If Next is called and returns false and there are no
// further results, Iter is closed automatically and it will suffice to check the
// result of Err.
func (ra *Range) Close() {
	// Release iterator and transaction also after a decode error (#31); only a second Close is a no-op.
	// An iterator built with an error (Bucket.Iter/Prefix/Range on a failed bucket) has neither.
	if ra.closed || ra.it == nil || ra.txn == nil {
		return
	}
	ra.closed = true
	ra.it.Close()
	ra.txn.Discard()
}
