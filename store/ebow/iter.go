package ebow

import (
	"fmt"
	"runtime"

	"github.com/dgraph-io/badger/v4"
	"github.com/golang/glog"
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
	ik := item.Key()
	err := item.Value(func(v []byte) error {
		var err error
		if it.resultType == nil {
			it.resultType, err = newStructType(result, true)
			if err != nil {
				return err
			}
		}
		err = it.bucket.db.codec.Unmarshal(v, result)
		if err != nil {
			return err
		}
		err = it.resultType.value(result).setKey(ik[bucketIdSize:])
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// #31: the caller sees false like at the end of the data and must check Err().
		glog.Errorf("ebow: cannot decode record %s: %v", ik[bucketIdSize:], err)
		it.err = fmt.Errorf("%w (key %s)", ErrUndecodableRow, ik[bucketIdSize:])
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
	if it.closed {
		return
	}
	it.closed = true
	it.it.Close()
	it.txn.Discard()
}
