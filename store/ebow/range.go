package ebow

import (
	"bytes"
	"github.com/dgraph-io/badger/v3"
	"runtime"
)

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
	opts.PrefetchSize = runtime.GOMAXPROCS(-1)
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
	if bytes.Compare(ra.until, ik[:len(ra.until)]) < 0 {
		ra.Close()
		return false
	}

	err := item.Value(func(v []byte) error {
		var err error
		if ra.resultType == nil {
			ra.resultType, err = newStructType(result, true)
			if err != nil {
				return err
			}
		}
		err = ra.bucket.db.codec.Unmarshal(v, result)
		if err != nil {
			return err
		}
		err = ra.resultType.value(result).setKey(ik[bucketIdSize:])
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
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
	if ra.err != nil {
		return
	}
	ra.closed = true
	ra.it.Close()
	ra.txn.Discard()
}
