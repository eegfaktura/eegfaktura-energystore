package calculation

import (
	"at.ourproject/energystore/store/ebow"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCalcHourSum(t *testing.T) {
	db, err := ebow.OpenStorageTest("dashboard", "ecid", "../../../rawdata")
	require.Nil(t, err)
	defer db.CloseTestDriver()

	rCons, rProd := CalcHourSum(db, "2021/04/18")

	fmt.Printf("Hour 12: Consumed - %+v\n", rCons[12])
	fmt.Printf("Hour 12: TotalProduced - %+v\n", rProd[12])
}
