package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/eegfaktura/eegfaktura-energystore/model"
	"github.com/eegfaktura/eegfaktura-energystore/store"
	"github.com/stretchr/testify/require"
)

func TestTe100110(t *testing.T) {
	db, err := store.OpenStorageTest("te100110", "ecIdTest", "../../../rawdata/converted")
	require.NoError(t, err)
	defer func() {
		db.Close()
	}()

	start := time.UnixMilli(1691704800000)
	end := time.UnixMilli(1691791200000)

	fmt.Printf("START %s, END %s\n", start.String(), end.String())

	//iter := db.GetLinePrefix(fmt.Sprintf("%s/%d/%.2d/", "CP", 2023, 9))
	//iter := db.GetLinePrefix("CP/")
	iter := db.GetLineRange("CP", fmt.Sprintf("%.4d/%.2d/%.2d/", 2023, 8, 3), fmt.Sprintf("%.4d/%.2d/%.2d/", 2023, 9, 5))
	defer iter.Close()

	var _line model.RawSourceLine
	for iter.Next(&_line) {
		fmt.Printf("LINE: %+v\n", _line)
	}
}
