package ebow

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
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

	objA := connectionPool.pool[poolKey(tenantA, ecId)]
	objB := connectionPool.pool[poolKey(tenantB, ecId)]
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

// Put of a nil object for a key the pool has never seen does nothing. The key is new per run: on
// a key that exists, Put(nil) dereferences nil (test-only path, known-errors #54), so the old fixed
// key made the test fail with -count > 1.
func TestPutEmptyDbObj(t *testing.T) {
	putEmptyRuns++
	ecId := fmt.Sprintf("PUTEMPTY%d", putEmptyRuns)
	connectionPool.Put(testRc, ecId, nil)

	assert.Nil(t, connectionPool.pool[poolKey(testRc, ecId)])
}

var putEmptyRuns int

func TestOpenObject(t *testing.T) {
	db := connectionPool.Get(testRc, testEcId)
	assert.NotNil(t, db)

	dbObj := connectionPool.pool[poolKey(testRc, testEcId)]
	assert.Equal(t, len(dbObj.pool), 19)

	connectionPool.Put(testRc, testEcId, db)
	assert.Nil(t, dbObj.db)
	assert.Equal(t, len(dbObj.pool), 20)

	fmt.Printf("%+v\n", dbObj)
}

func TestOpenMaxObject(t *testing.T) {
	t.Skip("known-errors #20") // enabled again with the fix (M4c)
	var db [21]*DbObject
	wg := NewCountedWait(20)

	go func() {
		for i := 0; i < 21; i++ {
			db[i] = connectionPool.Get(testRc, testEcId)
			assert.NotNil(t, db[i])
			assert.NotNil(t, db[i].Db)
			wg.Done()
		}
	}()

	wg.Wait()
	dbObj := connectionPool.pool[poolKey(testRc, testEcId)]
	assert.Equal(t, len(dbObj.pool), 0)
	assert.Nil(t, db[20])

	for i := 0; i < 20; i++ {
		connectionPool.Put(testRc, testEcId, db[i])
		assert.Nil(t, db[i].Db)
	}
	assert.Equal(t, len(dbObj.pool), 19)

	time.Sleep(500 * time.Microsecond)
	assert.NotNil(t, db[20].Db)

	connectionPool.Put(testRc, testEcId, db[20])
	assert.Nil(t, db[20].Db)
	assert.Equal(t, len(dbObj.pool), 20)
}
