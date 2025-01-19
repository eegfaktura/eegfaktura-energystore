package ebow

import (
	"fmt"
	"path/filepath"
)

func OpenStorageTest(tenant, ecId string, basedir string) (*BowStorage, error) {
	unlock := turns.lock(tenant)
	db, err := Open(filepath.Join(fmt.Sprintf("%s/%s", basedir, tenant), ecId))
	if err != nil {
		unlock()
		return nil, err
	}
	return &BowStorage{db, nil, tenant, ecId, unlock}, nil
}

func (b *BowStorage) CloseTestDriver() {
	_ = b.db.Close()
	b.unlock()
	return
}
