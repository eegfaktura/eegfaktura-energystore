package ebow

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	testEcId = "ECID123456TEST"
	testRc   = "TE999999"
)

type CountedWait struct {
	wait  chan struct{}
	limit int
}

func NewCountedWait(limit int) *CountedWait {
	return &CountedWait{
		wait:  make(chan struct{}, limit),
		limit: limit,
	}
}

func (cwg *CountedWait) Done() {
	cwg.wait <- struct{}{}
}

func (cwg *CountedWait) Wait() {
	count := 0
	for count < cwg.limit {
		<-cwg.wait
		count += 1
	}
}

// TestTenantIsolation sichert ab, dass zwei Mandanten mit DERSELBEN ecId getrennte
// Pool-Objekte bekommen. Vorher war der Pool allein nach ecId geschluesselt, der zweite
// Mandant bekam die DB des ersten (unter dessen Verzeichnis) zurueck.
func TestTenantIsolation(t *testing.T) {
	const ecId = "SHAREDECID01"
	const tenantA = "TE000011"
	const tenantB = "TE000022"

	dbA := connectionPool.Get(tenantA, ecId)
	assert.NotNil(t, dbA)
	dbB := connectionPool.Get(tenantB, ecId)
	assert.NotNil(t, dbB)

	objA, _ := connectionPool.lookup(tenantA, ecId)
	objB, _ := connectionPool.lookup(tenantB, ecId)
	assert.NotNil(t, objA)
	assert.NotNil(t, objB)
	// Getrennte Pool-Objekte, jeweils unter dem eigenen Mandanten.
	assert.NotSame(t, objA, objB)
	// Pool-Objekt haelt den Mandanten klein, wie der Dateipfad basePath/<tenant>/<ecId>.
	assert.Equal(t, strings.ToLower(tenantA), objA.tenant)
	assert.Equal(t, strings.ToLower(tenantB), objB.tenant)
	assert.NotEqual(t, poolKey(tenantA, ecId), poolKey(tenantB, ecId))

	connectionPool.Put(tenantA, ecId, dbA)
	connectionPool.Put(tenantB, ecId, dbB)
}

// TestOpenStorageRejectsInvalidIds sichert ab, dass Werte mit Pfad-Metazeichen abgelehnt
// werden, bevor sie in filepath.Join geraten.
func TestOpenStorageRejectsInvalidIds(t *testing.T) {
	bad := []struct{ tenant, ecId string }{
		{"TE100200", "../../etc"},
		{"TE100200", "a/b"},
		{"..", "AT00999900000TC100200000000000002"},
		{"TE100200", ""},
		{"", "AT00999900000TC100200000000000002"},
	}
	for _, c := range bad {
		s, err := OpenStorage(c.tenant, c.ecId)
		assert.Error(t, err, "tenant=%q ecId=%q sollte abgelehnt werden", c.tenant, c.ecId)
		assert.Nil(t, s)
	}
}

func TestPutEmptyDbObj(t *testing.T) {
	connectionPool.Put("TE999998", testEcId, nil)

	_, ok := connectionPool.lookup("TE999998", testEcId)
	assert.False(t, ok)
}

func TestOpenObject(t *testing.T) {
	vlogGCTestBase(t)
	db := connectionPool.Get(testRc, testEcId)
	assert.NotNil(t, db)

	dbObj, _ := connectionPool.lookup(testRc, testEcId)
	assert.Equal(t, 19, dbObj.available())

	connectionPool.Put(testRc, testEcId, db)
	assert.Nil(t, db.Db)
	assert.Equal(t, 20, dbObj.available())
	dbObj.mu.Lock()
	assert.Nil(t, dbObj.db, "letzte Rueckgabe schliesst die DB")
	dbObj.mu.Unlock()
}

// TestOpenMaxObject: das 21. Get wartet, bis ein Handle zurueckkommt.
func TestOpenMaxObject(t *testing.T) {
	vlogGCTestBase(t)
	var db [20]*DbObject
	for i := range db {
		db[i] = connectionPool.Get(testRc, testEcId)
		assert.NotNil(t, db[i].Db)
	}
	dbObj, _ := connectionPool.lookup(testRc, testEcId)
	assert.Equal(t, 0, dbObj.available())

	got := make(chan *DbObject)
	go func() { got <- connectionPool.Get(testRc, testEcId) }()

	select {
	case <-got:
		t.Fatal("Get darf bei ausgeschoepftem Pool nicht zurueckkommen")
	case <-time.After(100 * time.Millisecond):
	}

	connectionPool.Put(testRc, testEcId, db[0])
	var extra *DbObject
	select {
	case extra = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("wartendes Get bekam das zurueckgegebene Handle nicht")
	}
	assert.NotNil(t, extra.Db)
	assert.False(t, extra.Db.Badger().IsClosed())

	for i := 1; i < 20; i++ {
		connectionPool.Put(testRc, testEcId, db[i])
	}
	connectionPool.Put(testRc, testEcId, extra)
	assert.Equal(t, 20, dbObj.available())
}

// TestPutTwiceIsIgnored: ein zweites Put desselben Handles zaehlt nicht doppelt.
func TestPutTwiceIsIgnored(t *testing.T) {
	vlogGCTestBase(t)
	a := connectionPool.Get(testRc, testEcId)
	b := connectionPool.Get(testRc, testEcId)
	connectionPool.Put(testRc, testEcId, a)
	connectionPool.Put(testRc, testEcId, a)

	dbObj, _ := connectionPool.lookup(testRc, testEcId)
	assert.Equal(t, 19, dbObj.available())
	assert.False(t, b.Db.Badger().IsClosed(), "doppeltes Put darf die DB eines anderen Nutzers nicht schliessen")
	connectionPool.Put(testRc, testEcId, b)
}

// TestPoolConcurrentGetPut stellt das Muster GC gegen Import nach: mehrere Goroutinen holen und
// geben Handles derselben DB und anderer Mandanten zurueck, die DB wird dabei laufend geschlossen
// und wieder geoeffnet. Kein ausgegebenes Handle darf auf eine geschlossene DB zeigen; mit -race
// darf es keine Data Race geben (frueher: Map ohne gemeinsame Sperre, Schliessen ausserhalb von mu).
func TestPoolConcurrentGetPut(t *testing.T) {
	vlogGCTestBase(t)
	tenants := []string{"TE000101", "TE000102", "TE000103"}
	const workers = 8
	const rounds = 40

	var wg sync.WaitGroup
	errs := make(chan string, workers*rounds)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			tenant := tenants[w%len(tenants)]
			for r := 0; r < rounds; r++ {
				obj := connectionPool.Get(tenant, testEcId)
				if obj == nil || obj.Db == nil {
					errs <- "Get lieferte kein Handle"
					continue
				}
				if obj.Db.Badger().IsClosed() {
					errs <- "Get lieferte eine geschlossene DB"
				}
				connectionPool.Put(tenant, testEcId, obj)
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	for _, tenant := range tenants {
		dbObj, ok := connectionPool.lookup(tenant, testEcId)
		if assert.True(t, ok) {
			assert.Equal(t, 20, dbObj.available())
		}
	}
}
