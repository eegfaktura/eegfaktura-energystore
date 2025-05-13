package ebow

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"path/filepath"
	"strings"
	"sync"
)

type ebowLogger struct {
	level glog.Level
}

func (el ebowLogger) Infof(format string, args ...interface{}) {
	glog.V(el.level).Infof(format, args...)
}

func (el ebowLogger) Warningf(format string, args ...interface{}) {
	glog.Warningf(format, args...)
}

func (el ebowLogger) Errorf(format string, args ...interface{}) {
	glog.Errorf(format, args...)
}

func (el ebowLogger) Debugf(format string, args ...interface{}) {
	glog.V(el.level+1).Infof(format, args...)
}

type DbObject struct {
	Db *DB
}

type DbPoolObject struct {
	pool chan *DbObject
	mu   sync.Mutex
	//cl   sync.Mutex

	db     *DB
	ecId   string
	tenant string
}

func newDbPoolObject(size int, ecId, tenant string) *DbPoolObject {
	pool := make(chan *DbObject, size)
	for i := 0; i < size; i++ {
		pool <- &DbObject{Db: nil}
	}
	return &DbPoolObject{pool: pool, ecId: ecId, tenant: strings.ToLower(tenant)}
}

func (dpo *DbPoolObject) Get() *DbObject {
	dpo.mu.Lock()
	defer dpo.mu.Unlock()

	glog.V(4).Infof("B:dpo.Get(): Pool-Size %d of %d tenant=%s", len(dpo.pool), cap(dpo.pool), dpo.tenant)

	select {
	case dbObject := <-dpo.pool:
		if dpo.db == nil {
			var err error
			dpo.db, err = dpo.OpenStorage()
			if err != nil {
				glog.Errorf("%v tenant=%s", err, dpo.tenant)
				return nil
			}
		}
		glog.V(4).Infof("E:dpo.Get(): Pool-Size %d of %d tenant=%s", len(dpo.pool), cap(dpo.pool), dpo.tenant)
		dbObject.Db = dpo.db
		return dbObject
	}
}

func (dpo *DbPoolObject) Put(obj *DbObject) {
	obj.Db = nil
	if len(dpo.pool) == cap(dpo.pool) {
		glog.Warningf("Needless object release! tenant=%s", dpo.tenant)
		return
	}
	glog.V(4).Infof("B:dpo.Put(): Pool-Size %d of %d tenant=%s", len(dpo.pool), cap(dpo.pool), dpo.tenant)
	select {
	case dpo.pool <- obj:
		if len(dpo.pool) == cap(dpo.pool) {
			dpo.mu.Lock()
			defer dpo.mu.Unlock()

			dpo.close()
			glog.V(4).Infof("DB connection %s closed ... Object Pool max (%d) tenant=%s", dpo.ecId, len(dpo.pool), dpo.tenant)
		}
	}
	glog.V(4).Infof("E:dpo.Put(): Pool-Size %d of %d tenant=%s", len(dpo.pool), cap(dpo.pool), dpo.tenant)
}

func (dpo *DbPoolObject) close() {
	//dpo.cl.Lock()
	//defer dpo.cl.Unlock()

	if dpo.db != nil {
		_ = dpo.db.Close()
		dpo.db = nil
	}
}

func (dpo *DbPoolObject) OpenStorage() (*DB, error) {
	basePath := viper.GetString("persistence.path")
	path := filepath.Join(basePath, dpo.tenant, dpo.ecId)

	badgerOpts := badger.DefaultOptions(path)
	badgerOpts.Logger = ebowLogger{4}
	badgerOpts.BlockCacheSize = 512 << 20

	db, err := Open(path, SetBadgerOptions(badgerOpts))
	if err != nil {
		return nil, err
	}
	return db, nil
}

type Pool struct {
	pool     map[string]*DbPoolObject
	poolSize int
	nextID   int
	mutex    sync.Mutex
	mutexPut sync.Mutex
}

func NewPool(size int) *Pool {
	return &Pool{poolSize: size, pool: make(map[string]*DbPoolObject)}
}

func (p *Pool) Put(ecId string, e *DbObject) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if poolObj, ok := p.pool[ecId]; ok {
		poolObj.Put(e)
	}
}

func (p *Pool) Get(tenant, ecId string) *DbObject {
	p.mutexPut.Lock()
	defer p.mutexPut.Unlock()

	poolObj, ok := p.pool[ecId]
	if !ok {
		poolObj = newDbPoolObject(p.poolSize, ecId, tenant)
		p.pool[ecId] = poolObj
	}

	return poolObj.Get()
}

func (p *Pool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, poolObj := range p.pool {
		poolObj.close()
	}
}
