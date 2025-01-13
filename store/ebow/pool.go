package ebow

import (
	"fmt"
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
	glog.V(el.level).Infof(format, args...)
}

type DbObject struct {
	Db *DB
}

type DbPoolObject struct {
	pool chan *DbObject
	mu   sync.Mutex
	cl   sync.Mutex

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

	select {
	case dbObject := <-dpo.pool:
		if dpo.db == nil {
			var err error
			dpo.db, err = dpo.OpenStorage()
			if err != nil {
				return nil
			}
		}
		dbObject.Db = dpo.db
		return dbObject
	}
}

func (dpo *DbPoolObject) Put(obj *DbObject) {
	dpo.mu.Lock()
	defer dpo.mu.Unlock()

	obj.Db = nil

	select {
	case dpo.pool <- obj:
		if len(dpo.pool) == cap(dpo.pool) {
			dpo.Close()
			glog.V(3).Infof("DB connection %s closed ... Object Pool max (%d)", dpo.ecId, len(dpo.pool))
		}
	}
}

func (dpo *DbPoolObject) Close() {
	dpo.cl.Lock()
	defer dpo.cl.Unlock()

	if dpo.db != nil {
		_ = dpo.db.Close()
		dpo.db = nil
	}
}

func (dpo *DbPoolObject) OpenStorage() (*DB, error) {
	basePath := viper.GetString("persistence.path")
	path := filepath.Join(fmt.Sprintf("%s/%s", basePath, dpo.tenant), dpo.ecId)

	badgerOpts := badger.DefaultOptions(path)
	badgerOpts.Logger = ebowLogger{5}
	badgerOpts.BlockCacheSize = 1024 << 20

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
	mutex    sync.RWMutex
}

func NewPool(size int) *Pool {
	return &Pool{poolSize: size, pool: make(map[string]*DbPoolObject)}
}

func (p *Pool) Put(ecId string, e *DbObject) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	p.pool[ecId].Put(e)
}

func (p *Pool) Get(tenant, ecId string) *DbObject {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	poolObj, ok := p.pool[ecId]
	if !ok {
		poolObj = newDbPoolObject(p.poolSize, ecId, tenant)
		p.pool[ecId] = poolObj
	}

	return poolObj.Get()
}
