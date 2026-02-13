package ebow

import (
	"fmt"
	"github.com/stretchr/testify/assert"
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

func TestPutEmptyDbObj(t *testing.T) {
	connectionPool.Put(testEcId, nil)

	assert.Nil(t, connectionPool.pool[testEcId])
}

func TestOpenObject(t *testing.T) {
	db := connectionPool.Get(testRc, testEcId)
	assert.NotNil(t, db)

	dbObj := connectionPool.pool[testEcId]
	assert.Equal(t, len(dbObj.pool), 19)

	connectionPool.Put(testEcId, db)
	assert.Nil(t, dbObj.db)
	assert.Equal(t, len(dbObj.pool), 20)

	fmt.Printf("%+v\n", dbObj)
}

func TestOpenMaxObject(t *testing.T) {
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
	dbObj := connectionPool.pool[testEcId]
	assert.Equal(t, len(dbObj.pool), 0)
	assert.Nil(t, db[20])

	for i := 0; i < 20; i++ {
		connectionPool.Put(testEcId, db[i])
		assert.Nil(t, db[i].Db)
	}
	assert.Equal(t, len(dbObj.pool), 19)

	time.Sleep(500 * time.Microsecond)
	assert.NotNil(t, db[20].Db)

	connectionPool.Put(testEcId, db[20])
	assert.Nil(t, db[20].Db)
	assert.Equal(t, len(dbObj.pool), 20)
}
