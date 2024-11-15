package calculation

import (
	"fmt"
	"testing"

	"github.com/eegfaktura/eegfaktura-energystore/store"
	"github.com/stretchr/testify/require"
)

func TestCalcHourSum(t *testing.T) {
	db, err := store.OpenStorageTest("dashboard", "ecid", "../../../rawdata")
	require.Nil(t, err)
	defer db.Close()

	rCons, rProd := CalcHourSum(db, "2021/04/18")

	fmt.Printf("Hour 12: Consumed - %+v\n", rCons[12])
	fmt.Printf("Hour 12: TotalProduced - %+v\n", rProd[12])
}
