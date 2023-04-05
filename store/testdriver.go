package store

import (
	"at.ourproject/energystore/store/ebow"
	"fmt"
)

func OpenStorageTest(tenant string, basedir string) (*BowStorage, error) {
	unlock := turns.lock(tenant)
	db, err := ebow.Open(fmt.Sprintf("%s/%s", basedir, tenant))
	if err != nil {
		unlock()
		return nil, err
	}
	return &BowStorage{db, unlock}, nil
}
