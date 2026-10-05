package ebow

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/golang/glog"
	"github.com/spf13/viper"
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

// DbPoolObject verwaltet die geoeffnete Badger-DB einer (tenant, ecId) und zaehlt, wie viele
// Handles gerade ausgegeben sind. Ausgeben, Zuruecknehmen und Schliessen entscheiden alle unter
// mu: frueher prueften Put und Get den Fuellstand eines Channels ohne gemeinsame Sperre, sodass
// Put die DB schliessen konnte, waehrend ein Get sie gerade ausgegeben hatte -- der Aufrufer
// schrieb dann in eine geschlossene DB.
type DbPoolObject struct {
	mu    sync.Mutex
	freed *sync.Cond // signalisiert, dass ein Handle zurueckgegeben wurde
	size  int        // hoechstens so viele Handles gleichzeitig
	inUse int

	db     *DB
	ecId   string
	tenant string
}

func newDbPoolObject(size int, ecId, tenant string) *DbPoolObject {
	dpo := &DbPoolObject{size: size, ecId: ecId, tenant: strings.ToLower(tenant)}
	dpo.freed = sync.NewCond(&dpo.mu)
	return dpo
}

// Get gibt ein Handle auf die DB aus und oeffnet sie beim ersten Bedarf. Sind alle size Handles
// vergeben, wartet Get, bis eines zurueckkommt. Schlaegt das Oeffnen fehl, kommt nil zurueck.
func (dpo *DbPoolObject) Get() *DbObject {
	dpo.mu.Lock()
	defer dpo.mu.Unlock()

	for dpo.inUse >= dpo.size {
		dpo.freed.Wait()
	}
	if dpo.db == nil {
		db, err := dpo.OpenStorage()
		if err != nil {
			glog.Errorf("%v tenant=%s", err, dpo.tenant)
			return nil
		}
		dpo.db = db
	}
	dpo.inUse++
	glog.V(4).Infof("dpo.Get(): %d of %d in use tenant=%s", dpo.inUse, dpo.size, dpo.tenant)
	return &DbObject{Db: dpo.db}
}

// Put nimmt ein Handle zurueck. Ist danach keines mehr ausgegeben, wird die DB geschlossen.
// Ein schon zurueckgegebenes Handle (Db == nil) wird ignoriert.
func (dpo *DbPoolObject) Put(obj *DbObject) {
	dpo.mu.Lock()
	defer dpo.mu.Unlock()

	if obj == nil || obj.Db == nil || dpo.inUse == 0 {
		glog.Warningf("Needless object release! tenant=%s", dpo.tenant)
		return
	}
	obj.Db = nil
	dpo.inUse--
	if dpo.inUse == 0 {
		dpo.close()
		glog.V(4).Infof("DB connection %s closed, no handle in use tenant=%s", dpo.ecId, dpo.tenant)
	}
	dpo.freed.Signal()
}

// available liefert die Zahl der noch freien Handles (fuer Tests).
func (dpo *DbPoolObject) available() int {
	dpo.mu.Lock()
	defer dpo.mu.Unlock()
	return dpo.size - dpo.inUse
}

// close schliesst die DB. Aufrufer halten dpo.mu.
func (dpo *DbPoolObject) close() {
	if dpo.db != nil {
		_ = dpo.db.Close()
		dpo.db = nil
	}
}

func (dpo *DbPoolObject) OpenStorage() (*DB, error) {
	basePath := viper.GetString("persistence.path")
	path := filepath.Join(basePath, dpo.tenant, dpo.ecId)

	badgerOpts := badger.DefaultOptions(path).
		WithLogger(ebowLogger{4}).
		//WithCompression(options.None).
		WithMemTableSize(32 << 20). // 32 MB write buffer (OK)
		WithNumMemtables(1).
		WithBlockCacheSize(64 << 20). // cap block cache at 64 MB/DB (Badger default is 256 MB)
		WithIndexCacheSize(16 << 20). // bound index/bloom cache at 16 MB/DB
		WithNumLevelZeroTables(4). // allow a few in-memory tables
		WithNumLevelZeroTablesStall(8).
		////WithValueLogFileSize(128 << 20). // reasonable log file size
		//WithValueThreshold(8 << 20). // store values >1MB in vlog
		WithValueThreshold(128 << 10). // 64 KB
		WithNumCompactors(2). // fewer background threads = less RAM
		////WithCompactL0OnClose(false).     // skip final compaction to save time
		WithMetricsEnabled(false)

	// original Options: 01-11-2025
	//badgerOpts.Logger = ebowLogger{4}
	//badgerOpts.BlockCacheSize = 512 << 20

	db, err := Open(path, SetBadgerOptions(badgerOpts))
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Pool haelt je (tenant, ecId) ein DbPoolObject. mu schuetzt nur die Map; das Warten auf ein
// freies Handle passiert ausserhalb, damit ein ausgelasteter Mandant die anderen nicht aufhaelt.
type Pool struct {
	pool     map[string]*DbPoolObject
	poolSize int
	mu       sync.Mutex
}

func NewPool(size int) *Pool {
	return &Pool{poolSize: size, pool: make(map[string]*DbPoolObject)}
}

// poolKey schluesselt den Verbindungs-Pool nach Mandant UND ecId.
//
// Frueher war der Pool allein nach ecId geschluesselt, der Mandant ging nur beim ersten
// Anlegen ein. Ein Pool-Objekt oeffnet seine DB aber unter basePath/<tenant>/<ecId> mit dem
// Mandanten des ersten Aufrufers. Rief danach ein anderer Mandant mit derselben ecId auf,
// bekam er die DB des ersten zurueck. Der Mandant gehoert deshalb in den Schluessel.
// tenant wird klein geschrieben, wie beim Pfadaufbau in newDbPoolObject.
func poolKey(tenant, ecId string) string {
	return strings.ToLower(tenant) + "/" + ecId
}

func (p *Pool) lookup(tenant, ecId string) (*DbPoolObject, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	obj, ok := p.pool[poolKey(tenant, ecId)]
	return obj, ok
}

func (p *Pool) Put(tenant, ecId string, e *DbObject) {
	if poolObj, ok := p.lookup(tenant, ecId); ok {
		poolObj.Put(e)
	}
}

func (p *Pool) Get(tenant, ecId string) *DbObject {
	p.mu.Lock()
	key := poolKey(tenant, ecId)
	poolObj, ok := p.pool[key]
	if !ok {
		poolObj = newDbPoolObject(p.poolSize, ecId, tenant)
		p.pool[key] = poolObj
	}
	p.mu.Unlock()

	return poolObj.Get()
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, poolObj := range p.pool {
		poolObj.mu.Lock()
		poolObj.close()
		poolObj.mu.Unlock()
	}
}
