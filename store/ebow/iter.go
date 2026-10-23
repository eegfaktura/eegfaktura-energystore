package ebow

import (
	"runtime"

	"github.com/dgraph-io/badger/v4"
)

type Iter struct {
	bucket     *Bucket
	prefix     []byte
	txn        *badger.Txn
	it         *badger.Iterator
	resultType *structType
	advanced   bool
	closed     bool
	err        error
}

func newIter(bucket *Bucket, prefix []byte) *Iter {
	prefix = bucket.internalKey(prefix)
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = runtime.GOMAXPROCS(-1)
	txn := bucket.db.db.NewTransaction(false)
	it := txn.NewIterator(opts)
	it.Seek(prefix)
	return &Iter{
		bucket: bucket,
		txn:    txn,
		it:     it,
		prefix: prefix,
	}
}

func (it *Iter) Next(result interface{}) bool {
	if it.err != nil {
		return false
	}
	if it.closed {
		return false
	}
	if it.advanced {
		it.it.Next()
	}
	if !it.it.ValidForPrefix(it.prefix) {
		it.Close()
		return false
	}
	item := it.it.Item()
	// #31: on an error the caller sees false like at the end of the data and must check Err().
	if err := decodeItem(it.bucket, &it.resultType, item, result); err != nil {
		it.err = err
		return false
	}

	if !it.advanced {
		it.advanced = true
	}
	return true
}

// Err returns the error, if any, that was encountered during iteration.
// Err may be called after an explicit or implicit Close.
func (it *Iter) Err() error {
	return it.err
}

// Close closes the Iter. If Next is called and returns false and there are no
// further results, Iter is closed automatically and it will suffice to check the
// result of Err.
func (it *Iter) Close() {
	// Release iterator and transaction also after a decode error (#31); only a second Close is a no-op.
	// An iterator built with an error (Bucket.Iter/Prefix/Range on a failed bucket) has neither.
	if it.closed || it.it == nil || it.txn == nil {
		return
	}
	it.closed = true
	it.it.Close()
	it.txn.Discard()
}
