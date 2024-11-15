package store

import (
	"fmt"
	"path/filepath"

	"github.com/eegfaktura/eegfaktura-energystore/store/ebow"
)

func OpenStorageTest(tenant, ecId string, basedir string) (*BowStorage, error) {
	unlock := turns.lock(tenant)
	db, err := ebow.Open(filepath.Join(fmt.Sprintf("%s/%s", basedir, tenant), ecId))
	if err != nil {
		unlock()
		return nil, err
	}
	return &BowStorage{db, unlock}, nil
}
