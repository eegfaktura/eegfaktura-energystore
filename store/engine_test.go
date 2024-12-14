package store

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_CacheTime(t *testing.T) {
	now := time.Now()
	testCacheT := CacheTime{now}
	tsFunc := func(dir int, ct CacheTime) CacheTime {
		return CacheTime{ct.AddDate(0, 0, dir*1)}
	}
	println(testCacheT.Format("2006-01-02 15:04:05"))
	assert.Equal(t, now, testCacheT.Time)
	newTs := testCacheT.AddTs(tsFunc)
	println(newTs.Format("2006-01-02 15:04:05"))
	assert.Equal(t, now.AddDate(0, 0, 1), newTs.Time)
}
