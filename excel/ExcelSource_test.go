package excel

import (
	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

//var (
//	db *store.BowStorage
//)

//func TestMain(m *testing.M) {
//
//	setUp()
//
//	retCode := m.Run()
//
//	setDown()
//
//	os.Exit(retCode)
//}
//
//func setUp() {
//	println("Setup TEST!!!!!!!!!!!")
//	var err error
//	db, err = store.OpenStorageTest("excelsource", "../../../rawdata/test")
//	if err != nil {
//		panic(err)
//	}
//}
//
//func setDown() {
//	_ = db.Close()
//	os.RemoveAll("../../../rawdata/test/excelsource")
//
//}

func TestImportExcelEnergyFile(t *testing.T) {
	db, err := store.OpenStorageTest("dashboard", "../test/rawdata")
	require.Nil(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("../test/rawdata/dashboard")
	}()

	excelFile, err := OpenExceFile("../test/221220 Daten VIERE 04-10 bis 18-12.xlsx")
	require.NoError(t, err)
	defer excelFile.Close()

	yearSet, err := ImportExcelEnergyFile(excelFile, "Energiedaten", db)
	require.NoError(t, err)
	for _, k := range yearSet {
		err = calculation.CalculateMonthlyDash(db, fmt.Sprintf("%d", k), calculation.CalculateEEG)
		require.NoError(t, err)
	}

}

func TestBuildMatixMetaStruct(t *testing.T) {

	db, err := store.OpenStorageTest("excelsource", "../test/rawdata")
	require.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("../test/rawdata/excelsource")
	}()

	t.Run("Initiate Meta Struckt", func(t *testing.T) {
		header := excelHeader{
			meteringPointId: map[int]string{
				0:  "AT0030000000000000000000000000001",
				1:  "AT0030000000000000000000000000001",
				2:  "AT0030000000000000000000000000001",
				3:  "AT0030000000000000000000000000002",
				4:  "AT0030000000000000000000000000002",
				5:  "AT0030000000000000000000000000002",
				6:  "AT0030000000000000000000000000003",
				7:  "AT0030000000000000000000000000003",
				8:  "AT0030000000000000000000000000003",
				9:  "AT0030000000000000000000000000004",
				10: "AT0030000000000000000000000000005",
				11: "AT0030000000000000000000000000006",
				12: "AT0030000000000000000000000000007",
				13: "AT0030000000000000000000000000008",
				14: "AT0030000000000000000000000000009",
				15: "AT0030000000000000000000000000010",
				16: "AT0030000000000000000000000000011",
				17: "AT0030000000000000000000000000012",
				18: "AT0030000000000000000000000000013",
				19: "TOTAL",
				20: "TOTAL",
				21: "TOTAL",
			},
			energyDirection: map[int]string{
				0:  "CONSUMPTION",
				1:  "CONSUMPTION",
				2:  "CONSUMPTION",
				3:  "CONSUMPTION",
				4:  "CONSUMPTION",
				5:  "CONSUMPTION",
				6:  "CONSUMPTION",
				7:  "CONSUMPTION",
				8:  "CONSUMPTION",
				9:  "CONSUMPTION",
				10: "CONSUMPTION",
				11: "CONSUMPTION",
				12: "CONSUMPTION",
				13: "CONSUMPTION",
				14: "CONSUMPTION",
				15: "CONSUMPTION",
				16: "CONSUMPTION",
				17: "CONSUMPTION",
				18: "GENERATION",
			},
			meterCode: map[int]MeterCodeType{
				0:  Total,
				1:  Share,
				2:  Coverage,
				3:  Total,
				4:  Share,
				5:  Coverage,
				6:  Total,
				7:  Share,
				8:  Coverage,
				9:  Total,
				10: Total,
				11: Total,
				12: Total,
				13: Total,
				14: Total,
				15: Total,
				16: Total,
				17: Total,
				18: Total,
				19: Total,
				20: Total,
				21: Total,
			},
			periodStart: map[int]string{
				0:  "24.10.2022 00:00:00",
				1:  "24.10.2022 00:00:00",
				2:  "24.10.2022 00:00:00",
				3:  "24.10.2022 00:00:00",
				4:  "24.10.2022 00:00:00",
				5:  "24.10.2022 00:00:00",
				6:  "24.10.2022 00:00:00",
				7:  "24.10.2022 00:00:00",
				8:  "24.10.2022 00:00:00",
				9:  "24.10.2022 00:00:00",
				10: "24.10.2022 00:00:00",
				11: "24.10.2022 00:00:00",
				12: "24.10.2022 00:00:00",
				13: "24.10.2022 00:00:00",
				14: "24.10.2022 00:00:00",
				15: "24.10.2022 00:00:00",
				16: "24.10.2022 00:00:00",
				17: "24.10.2022 00:00:00",
				18: "24.10.2022 00:00:00",
			},
			periodEnd: map[int]string{
				0:  "24.10.2022 00:00:00",
				1:  "24.10.2022 00:00:00",
				2:  "24.10.2022 00:00:00",
				3:  "24.10.2022 00:00:00",
				4:  "24.10.2022 00:00:00",
				5:  "24.10.2022 00:00:00",
				6:  "24.10.2022 00:00:00",
				7:  "24.10.2022 00:00:00",
				8:  "24.10.2022 00:00:00",
				9:  "24.10.2022 00:00:00",
				10: "24.10.2022 00:00:00",
				11: "24.10.2022 00:00:00",
				12: "24.10.2022 00:00:00",
				13: "24.10.2022 00:00:00",
				14: "24.10.2022 00:00:00",
				15: "24.10.2022 00:00:00",
				16: "24.10.2022 00:00:00",
				17: "24.10.2022 00:00:00",
				18: "24.10.2022 00:00:00",
			},
		}

		excelCpMeta, cpMeta, err := buildMatrixMetaStruct(db, header)
		require.NoError(t, err)
		require.NotNil(t, cpMeta)
		require.Equal(t, 13, len(cpMeta))
		require.NotNil(t, excelCpMeta)
		require.Equal(t, 13, len(excelCpMeta))

		cConsuption := 0
		cGeneration := 0

		//expectedIdx := []int{0, 3, 6, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}
		//expectedSourceIdx := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 0}
		currentIdx := []int{}
		currentSourceIdx := []int{}
		for j := 0; j < len(cpMeta); j++ {
			v := excelCpMeta[j]
			if v.Dir == "CONSUMPTION" {
				cConsuption += 1
			} else if v.Dir == "GENERATION" {
				cGeneration += 1
			}
			currentIdx = append(currentIdx, v.Idx)
			currentSourceIdx = append(currentSourceIdx, v.SourceIdx)
			fmt.Printf("Meta: %+v (%+v)\n", v, excelCpMeta[j])
		}

		require.Equal(t, 1, cGeneration)
		require.Equal(t, 12, cConsuption)

		//sort.Ints(currentIdx)
		//require.ElementsMatch(t, expectedIdx, currentIdx)
		//require.ElementsMatch(t, expectedSourceIdx, currentSourceIdx)

		rawMeta := &model.RawSourceMeta{Id: fmt.Sprintf("cpmeta/%d", 0), CounterPoints: cpMeta, NumberOfMetering: 13}
		err = db.SetMeta(rawMeta)
	})

	t.Run("Initiate Meta Struckt", func(t *testing.T) {

		header := excelHeader{
			meteringPointId: map[int]string{
				0:  "AT0030000000000000000000000000015", // 0
				1:  "AT0030000000000000000000000000015",
				2:  "AT0030000000000000000000000000015",
				3:  "AT0030000000000000000000000000014", // 3
				4:  "AT0030000000000000000000000000014",
				5:  "AT0030000000000000000000000000014",
				6:  "AT0030000000000000000000000000002", // 6
				7:  "AT0030000000000000000000000000002",
				8:  "AT0030000000000000000000000000002",
				9:  "AT0030000000000000000000000000004", // 9
				10: "AT0030000000000000000000000000005",
				11: "AT0030000000000000000000000000006",
				12: "AT0030000000000000000000000000007",
				13: "AT0030000000000000000000000000008",
				14: "AT0030000000000000000000000000009",
				15: "AT0030000000000000000000000000010",
				16: "AT0030000000000000000000000000011",
				17: "AT0030000000000000000000000000012",
				18: "AT0030000000000000000000000000013",
				19: "AT0030000000000000000000000000013",
				20: "TOTAL",
				21: "TOTAL",
				22: "TOTAL",
			},
			energyDirection: map[int]string{
				0:  "CONSUMPTION",
				1:  "CONSUMPTION",
				2:  "CONSUMPTION",
				3:  "CONSUMPTION",
				4:  "CONSUMPTION",
				5:  "CONSUMPTION",
				6:  "CONSUMPTION",
				7:  "CONSUMPTION",
				8:  "CONSUMPTION",
				9:  "CONSUMPTION",
				10: "CONSUMPTION",
				11: "CONSUMPTION",
				12: "CONSUMPTION",
				13: "CONSUMPTION",
				14: "CONSUMPTION",
				15: "CONSUMPTION",
				16: "CONSUMPTION",
				17: "CONSUMPTION",
				18: "GENERATION",
				19: "GENERATION",
			},
			meterCode: map[int]MeterCodeType{
				0:  Total,
				1:  Share,
				2:  Coverage,
				3:  Total,
				4:  Share,
				5:  Coverage,
				6:  Total,
				7:  Share,
				8:  Coverage,
				9:  Total,
				10: Total,
				11: Total,
				12: Total,
				13: Total,
				14: Total,
				15: Total,
				16: Total,
				17: Total,
				18: Bad,
				19: Total,
			},
			periodStart: map[int]string{
				0:  "25.10.2022 00:00:00",
				1:  "25.10.2022 00:00:00",
				2:  "25.10.2022 00:00:00",
				3:  "25.10.2022 00:00:00",
				4:  "25.10.2022 00:00:00",
				5:  "25.10.2022 00:00:00",
				6:  "24.10.2022 00:00:00",
				7:  "24.10.2022 00:00:00",
				8:  "24.10.2022 00:00:00",
				9:  "24.10.2022 00:00:00",
				10: "24.10.2022 00:00:00",
				11: "24.10.2022 00:00:00",
				12: "24.10.2022 00:00:00",
				13: "24.10.2022 00:00:00",
				14: "24.10.2022 00:00:00",
				15: "24.10.2022 00:00:00",
				16: "24.10.2022 00:00:00",
				17: "24.10.2022 00:00:00",
				18: "",
				19: "24.10.2022 00:00:00",
			},
			periodEnd: map[int]string{
				0:  "25.10.2022 00:00:00",
				1:  "25.10.2022 00:00:00",
				2:  "25.10.2022 00:00:00",
				3:  "25.10.2022 00:00:00",
				4:  "25.10.2022 00:00:00",
				5:  "25.10.2022 00:00:00",
				6:  "24.10.2022 00:00:00",
				7:  "24.10.2022 00:00:00",
				8:  "24.10.2022 00:00:00",
				9:  "24.10.2022 00:00:00",
				10: "24.10.2022 00:00:00",
				11: "25.10.2022 00:00:00",
				12: "24.10.2022 00:00:00",
				13: "24.10.2022 00:00:00",
				14: "25.10.2022 00:00:00",
				15: "24.10.2022 00:00:00",
				16: "24.10.2022 00:00:00",
				17: "24.10.2022 00:00:00",
				18: "24.10.2022 00:00:00",
			},
		}

		excelCpMeta, cpMeta, err := buildMatrixMetaStruct(db, header)
		require.NoError(t, err)
		require.NotNil(t, cpMeta)
		require.Equal(t, 15, len(cpMeta))
		require.NotNil(t, excelCpMeta)
		require.Equal(t, 13, len(excelCpMeta))

		cConsuption := 0
		cGeneration := 0
		var currentIdx []int
		currentSourceIdx := []int{}
		consumptionIdx := []int{}
		productionIdx := []int{}
		for j := 0; j < len(excelCpMeta); j++ {
			v := excelCpMeta[j]
			if v.Dir == "CONSUMPTION" {
				cConsuption += 1
				consumptionIdx = append(consumptionIdx, v.Idx)
			} else if v.Dir == "GENERATION" {
				cGeneration += 1
				productionIdx = append(productionIdx, v.Idx)
			}
			currentIdx = append(currentIdx, v.Idx)
			currentSourceIdx = append(currentSourceIdx, v.SourceIdx)
			fmt.Printf("Meta: %+v\n", v)
		}

		require.Equal(t, 1, cGeneration)
		require.Equal(t, 12, cConsuption)

		fmt.Printf("currentIdx: (%+v)\n", currentIdx)
		fmt.Printf("currentSourceIdx: (%+v)\n", currentSourceIdx)
		fmt.Printf("productionSourceIdx: (%+v)\n", productionIdx)

		expectedIdx := []int{0, 3, 6, 9, 10, 11, 12, 13, 14, 15, 16, 17, 19}
		require.ElementsMatch(t, expectedIdx, currentIdx)
		expectedSourceIdx := []int{12, 13, 1, 3, 4, 5, 6, 7, 8, 9, 10, 11, 0}
		require.ElementsMatch(t, expectedSourceIdx, currentSourceIdx)
	})

	t.Run("Initiate Metadata with 32 consumers and 1 producer", func(t *testing.T) {
		header := excelHeader{
			meteringPointId: map[int]string{
				0:  "AT003000000000000000000Zaehlpkt01",
				1:  "AT003000000000000000000Zaehlpkt01",
				2:  "AT003000000000000000000Zaehlpkt01",
				3:  "AT003000000000000000000Zaehlpkt02",
				4:  "AT003000000000000000000Zaehlpkt02",
				5:  "AT003000000000000000000Zaehlpkt02",
				6:  "AT003000000000000000000Zaehlpkt03",
				7:  "AT003000000000000000000Zaehlpkt03",
				8:  "AT003000000000000000000Zaehlpkt03",
				9:  "AT003000000000000000000Zaehlpkt04",
				10: "AT003000000000000000000Zaehlpkt04",
				11: "AT003000000000000000000Zaehlpkt04",
				12: "AT003000000000000000000Zaehlpkt05",
				13: "AT003000000000000000000Zaehlpkt05",
				14: "AT003000000000000000000Zaehlpkt05",
				15: "AT003000000000000000000Zaehlpkt06",
				16: "AT003000000000000000000Zaehlpkt06",
				17: "AT003000000000000000000Zaehlpkt06",
				18: "AT003000000000000000000Zaehlpkt07",
				19: "AT003000000000000000000Zaehlpkt07",
				20: "AT003000000000000000000Zaehlpkt07",
				21: "AT003000000000000000000Zaehlpkt08",
				22: "AT003000000000000000000Zaehlpkt08",
				23: "AT003000000000000000000Zaehlpkt08",
				24: "AT003000000000000000000Zaehlpkt09",
				25: "AT003000000000000000000Zaehlpkt09",
				26: "AT003000000000000000000Zaehlpkt09",
				27: "AT003000000000000000000Zaehlpkt10",
				28: "AT003000000000000000000Zaehlpkt10",
				29: "AT003000000000000000000Zaehlpkt10",
				30: "AT003000000000000000000Zaehlpkt11",
				31: "AT003000000000000000000Zaehlpkt11",
				32: "AT003000000000000000000Zaehlpkt11",
				33: "AT003000000000000000000Zaehlpkt12",
				34: "AT003000000000000000000Zaehlpkt12",
				35: "AT003000000000000000000Zaehlpkt12",
				36: "AT003000000000000000000Zaehlpkt13",
				37: "AT003000000000000000000Zaehlpkt13",
				38: "AT003000000000000000000Zaehlpkt13",
				39: "AT003000000000000000000Zaehlpkt14",
				40: "AT003000000000000000000Zaehlpkt14",
				41: "AT003000000000000000000Zaehlpkt14",
				42: "AT003000000000000000000Zaehlpkt15",
				43: "AT003000000000000000000Zaehlpkt15",
				44: "AT003000000000000000000Zaehlpkt15",
				45: "AT003000000000000000000Zaehlpkt16",
				46: "AT003000000000000000000Zaehlpkt16",
				47: "AT003000000000000000000Zaehlpkt16",
				48: "AT003000000000000000000Zaehlpkt17",
				49: "AT003000000000000000000Zaehlpkt17",
				50: "AT003000000000000000000Zaehlpkt17",
				51: "AT003000000000000000000Zaehlpkt18",
				52: "AT003000000000000000000Zaehlpkt18",
				53: "AT003000000000000000000Zaehlpkt18",
				54: "AT003000000000000000000Zaehlpkt19",
				55: "AT003000000000000000000Zaehlpkt19",
				56: "AT003000000000000000000Zaehlpkt19",
				57: "AT003000000000000000000Zaehlpkt20",
				58: "AT003000000000000000000Zaehlpkt20",
				59: "AT003000000000000000000Zaehlpkt20",
				60: "AT003000000000000000000Zaehlpkt21",
				61: "AT003000000000000000000Zaehlpkt21",
				62: "AT003000000000000000000Zaehlpkt21",
				63: "AT003000000000000000000Zaehlpkt22",
				64: "AT003000000000000000000Zaehlpkt22",
				65: "AT003000000000000000000Zaehlpkt22",
				66: "AT003000000000000000000Zaehlpkt23",
				67: "AT003000000000000000000Zaehlpkt23",
				68: "AT003000000000000000000Zaehlpkt23",
				69: "AT003000000000000000000Zaehlpkt24",
				70: "AT003000000000000000000Zaehlpkt24",
				71: "AT003000000000000000000Zaehlpkt24",
				72: "AT003000000000000000000Zaehlpkt25",
				73: "AT003000000000000000000Zaehlpkt25",
				74: "AT003000000000000000000Zaehlpkt25",
				75: "AT003000000000000000000Zaehlpkt26",
				76: "AT003000000000000000000Zaehlpkt26",
				77: "AT003000000000000000000Zaehlpkt26",
				78: "AT003000000000000000000Zaehlpkt27",
				79: "AT003000000000000000000Zaehlpkt27",
				80: "AT003000000000000000000Zaehlpkt27",
				81: "AT003000000000000000000Zaehlpkt28",
				82: "AT003000000000000000000Zaehlpkt28",
				83: "AT003000000000000000000Zaehlpkt28",
				84: "AT003000000000000000000Zaehlpkt29",
				85: "AT003000000000000000000Zaehlpkt29",
				86: "AT003000000000000000000Zaehlpkt29",
				87: "AT003000000000000000000Zaehlpkt30",
				88: "AT003000000000000000000Zaehlpkt30",
				89: "AT003000000000000000000Zaehlpkt30",
				90: "AT003000000000000000000Zaehlpkt31",
				91: "AT003000000000000000000Zaehlpkt31",
				92: "AT003000000000000000000Zaehlpkt31",
				93: "AT003000000000000000000Zaehlpkt32",
				94: "AT003000000000000000000Zaehlpkt32",
				95: "AT003000000000000000000Zaehlpkt32",
				96: "AT00300000000000000000000Erzeuger",
				97: "TOTAL",
				98: "TOTAL",
				99: "TOTAL",
			},
			energyDirection: map[int]string{
				0: "CONSUMPTION", 1: "CONSUMPTION",
				2: "CONSUMPTION", 3: "CONSUMPTION", 4: "CONSUMPTION", 5: "CONSUMPTION", 6: "CONSUMPTION", 7: "CONSUMPTION",
				8: "CONSUMPTION", 9: "CONSUMPTION", 10: "CONSUMPTION", 11: "CONSUMPTION", 12: "CONSUMPTION", 13: "CONSUMPTION",
				14: "CONSUMPTION", 15: "CONSUMPTION", 16: "CONSUMPTION", 17: "CONSUMPTION", 18: "CONSUMPTION", 19: "CONSUMPTION",
				20: "CONSUMPTION", 21: "CONSUMPTION", 22: "CONSUMPTION", 23: "CONSUMPTION", 24: "CONSUMPTION", 25: "CONSUMPTION",
				26: "CONSUMPTION", 27: "CONSUMPTION", 28: "CONSUMPTION", 29: "CONSUMPTION", 30: "CONSUMPTION", 31: "CONSUMPTION",
				32: "CONSUMPTION", 33: "CONSUMPTION", 34: "CONSUMPTION", 35: "CONSUMPTION", 36: "CONSUMPTION", 37: "CONSUMPTION",
				38: "CONSUMPTION", 39: "CONSUMPTION", 40: "CONSUMPTION", 41: "CONSUMPTION", 42: "CONSUMPTION", 43: "CONSUMPTION",
				44: "CONSUMPTION", 45: "CONSUMPTION", 46: "CONSUMPTION", 47: "CONSUMPTION", 48: "CONSUMPTION", 49: "CONSUMPTION",
				50: "CONSUMPTION", 51: "CONSUMPTION", 52: "CONSUMPTION", 53: "CONSUMPTION", 54: "CONSUMPTION", 55: "CONSUMPTION",
				56: "CONSUMPTION", 57: "CONSUMPTION", 58: "CONSUMPTION", 59: "CONSUMPTION", 60: "CONSUMPTION", 61: "CONSUMPTION",
				62: "CONSUMPTION", 63: "CONSUMPTION", 64: "CONSUMPTION", 65: "CONSUMPTION", 66: "CONSUMPTION", 67: "CONSUMPTION",
				68: "CONSUMPTION", 69: "CONSUMPTION", 70: "CONSUMPTION", 71: "CONSUMPTION", 72: "CONSUMPTION", 73: "CONSUMPTION",
				74: "CONSUMPTION", 75: "CONSUMPTION", 76: "CONSUMPTION", 77: "CONSUMPTION", 78: "CONSUMPTION", 79: "CONSUMPTION",
				80: "CONSUMPTION", 81: "CONSUMPTION", 82: "CONSUMPTION", 83: "CONSUMPTION", 84: "CONSUMPTION", 85: "CONSUMPTION",
				86: "CONSUMPTION", 87: "CONSUMPTION", 88: "CONSUMPTION", 89: "CONSUMPTION", 90: "CONSUMPTION", 91: "CONSUMPTION",
				92: "CONSUMPTION", 93: "CONSUMPTION", 94: "CONSUMPTION", 95: "CONSUMPTION", 96: "GENERATION",
			},
			meterCode: map[int]MeterCodeType{
				0:  Total,
				1:  Share,
				2:  Coverage,
				3:  Total,
				4:  Share,
				5:  Coverage,
				6:  Total,
				7:  Share,
				8:  Coverage,
				9:  Total,
				10: Share,
				11: Coverage,
				12: Total,
				13: Share,
				14: Coverage,
				15: Total,
				16: Share,
				17: Coverage,
				18: Total,
				19: Share,
				20: Coverage,
				21: Total,
				22: Share,
				23: Coverage,
				24: Total,
				25: Share,
				26: Coverage,
				27: Total,
				28: Share,
				29: Coverage,
				30: Total,
				31: Share,
				32: Coverage,
				33: Total,
				34: Share,
				35: Coverage,
				36: Total,
				37: Share,
				38: Coverage,
				39: Total,
				40: Share,
				41: Coverage,
				42: Total,
				43: Share,
				44: Coverage,
				45: Total,
				46: Share,
				47: Coverage,
				48: Total,
				49: Share,
				50: Coverage,
				51: Total,
				52: Share,
				53: Coverage,
				54: Total,
				55: Share,
				56: Coverage,
				57: Total,
				58: Share,
				59: Coverage,
				60: Total,
				61: Share,
				62: Coverage,
				63: Total,
				64: Share,
				65: Coverage,
				66: Total,
				67: Share,
				68: Coverage,
				69: Total,
				70: Share,
				71: Coverage,
				72: Total,
				73: Share,
				74: Coverage,
				75: Total,
				76: Share,
				77: Coverage,
				78: Total,
				79: Share,
				80: Coverage,
				81: Total,
				82: Share,
				83: Coverage,
				84: Total,
				85: Share,
				86: Coverage,
				87: Total,
				88: Share,
				89: Coverage,
				90: Total,
				91: Share,
				92: Coverage,
				93: Total,
				94: Share,
				95: Coverage,
				96: Total,
			},
			periodStart: map[int]string{
				0: "01.01.2021 00:00:00", 1: "01.01.2021 00:00:00", 2: "01.01.2021 00:00:00", 3: "01.01.2021 00:00:00", 4: "01.01.2021 00:00:00", 5: "01.01.2021 00:00:00", 6: "01.01.2021 00:00:00", 7: "01.01.2021 00:00:00", 8: "01.01.2021 00:00:00", 9: "01.01.2021 00:00:00", 10: "01.01.2021 00:00:00", 11: "01.01.2021 00:00:00", 12: "01.01.2021 00:00:00", 13: "01.01.2021 00:00:00", 14: "01.01.2021 00:00:00", 15: "01.01.2021 00:00:00", 16: "01.01.2021 00:00:00", 17: "01.01.2021 00:00:00", 18: "01.01.2021 00:00:00", 19: "01.01.2021 00:00:00", 20: "01.01.2021 00:00:00", 21: "01.01.2021 00:00:00", 22: "01.01.2021 00:00:00", 23: "01.01.2021 00:00:00", 24: "01.01.2021 00:00:00", 25: "01.01.2021 00:00:00", 26: "01.01.2021 00:00:00", 27: "01.01.2021 00:00:00", 28: "01.01.2021 00:00:00", 29: "01.01.2021 00:00:00", 30: "01.01.2021 00:00:00", 31: "01.01.2021 00:00:00", 32: "01.01.2021 00:00:00", 33: "01.01.2021 00:00:00", 34: "01.01.2021 00:00:00", 35: "01.01.2021 00:00:00", 36: "01.01.2021 00:00:00", 37: "01.01.2021 00:00:00", 38: "01.01.2021 00:00:00", 39: "01.01.2021 00:00:00", 40: "01.01.2021 00:00:00", 41: "01.01.2021 00:00:00", 42: "01.01.2021 00:00:00", 43: "01.01.2021 00:00:00", 44: "01.01.2021 00:00:00", 45: "01.01.2021 00:00:00", 46: "01.01.2021 00:00:00", 47: "01.01.2021 00:00:00", 48: "01.01.2021 00:00:00", 49: "01.01.2021 00:00:00", 50: "01.01.2021 00:00:00", 51: "01.01.2021 00:00:00", 52: "01.01.2021 00:00:00", 53: "01.01.2021 00:00:00", 54: "01.01.2021 00:00:00", 55: "01.01.2021 00:00:00", 56: "01.01.2021 00:00:00", 57: "01.01.2021 00:00:00", 58: "01.01.2021 00:00:00", 59: "01.01.2021 00:00:00", 60: "01.01.2021 00:00:00", 61: "01.01.2021 00:00:00", 62: "01.01.2021 00:00:00", 63: "01.01.2021 00:00:00", 64: "01.01.2021 00:00:00", 65: "01.01.2021 00:00:00", 66: "01.01.2021 00:00:00", 67: "01.01.2021 00:00:00", 68: "01.01.2021 00:00:00", 69: "01.01.2021 00:00:00", 70: "01.01.2021 00:00:00", 71: "01.01.2021 00:00:00", 72: "01.01.2021 00:00:00", 73: "01.01.2021 00:00:00", 74: "01.01.2021 00:00:00", 75: "01.01.2021 00:00:00", 76: "01.01.2021 00:00:00", 77: "01.01.2021 00:00:00", 78: "01.01.2021 00:00:00", 79: "01.01.2021 00:00:00", 80: "01.01.2021 00:00:00", 81: "01.01.2021 00:00:00", 82: "01.01.2021 00:00:00", 83: "01.01.2021 00:00:00", 84: "01.01.2021 00:00:00", 85: "01.01.2021 00:00:00", 86: "01.01.2021 00:00:00", 87: "01.01.2021 00:00:00", 88: "01.01.2021 00:00:00", 89: "01.01.2021 00:00:00", 90: "01.01.2021 00:00:00", 91: "01.01.2021 00:00:00", 92: "01.01.2021 00:00:00", 93: "01.01.2021 00:00:00", 94: "01.01.2021 00:00:00", 95: "01.01.2021 00:00:00", 96: "01.01.2021 00:00:00", 97: "01.01.2021 00:00:00", 98: "01.01.2021 00:00:00", 99: "01.01.2021 00:00:00",
			},
			periodEnd: map[int]string{
				0: "31.12.2021 23:59:00", 1: "31.12.2021 23:59:00", 2: "31.12.2021 23:59:00", 3: "31.12.2021 23:59:00", 4: "31.12.2021 23:59:00", 5: "31.12.2021 23:59:00", 6: "31.12.2021 23:59:00", 7: "31.12.2021 23:59:00", 8: "31.12.2021 23:59:00", 9: "01.08.2021 23:59:00", 10: "01.08.2021 23:59:00", 11: "01.08.2021 23:59:00", 12: "31.12.2021 23:59:00", 13: "31.12.2021 23:59:00", 14: "31.12.2021 23:59:00", 15: "31.12.2021 23:59:00", 16: "31.12.2021 23:59:00", 17: "31.12.2021 23:59:00", 18: "31.12.2021 23:59:00", 19: "31.12.2021 23:59:00", 20: "31.12.2021 23:59:00", 21: "31.12.2021 23:59:00", 22: "31.12.2021 23:59:00", 23: "31.12.2021 23:59:00", 24: "31.12.2021 23:59:00", 25: "31.12.2021 23:59:00", 26: "31.12.2021 23:59:00", 27: "31.12.2021 23:59:00", 28: "31.12.2021 23:59:00", 29: "31.12.2021 23:59:00", 30: "31.12.2021 23:59:00", 31: "31.12.2021 23:59:00", 32: "31.12.2021 23:59:00", 33: "31.12.2021 23:59:00", 34: "31.12.2021 23:59:00", 35: "31.12.2021 23:59:00", 36: "31.12.2021 23:59:00", 37: "31.12.2021 23:59:00", 38: "31.12.2021 23:59:00", 39: "31.12.2021 23:59:00", 40: "31.12.2021 23:59:00", 41: "31.12.2021 23:59:00", 42: "31.12.2021 23:59:00", 43: "31.12.2021 23:59:00", 44: "31.12.2021 23:59:00", 45: "31.12.2021 23:59:00", 46: "31.12.2021 23:59:00", 47: "31.12.2021 23:59:00", 48: "31.12.2021 23:59:00", 49: "31.12.2021 23:59:00", 50: "31.12.2021 23:59:00", 51: "31.12.2021 23:59:00", 52: "31.12.2021 23:59:00", 53: "31.12.2021 23:59:00", 54: "31.12.2021 23:59:00", 55: "31.12.2021 23:59:00", 56: "31.12.2021 23:59:00", 57: "31.12.2021 23:59:00", 58: "31.12.2021 23:59:00", 59: "31.12.2021 23:59:00", 60: "31.12.2021 23:59:00", 61: "31.12.2021 23:59:00", 62: "31.12.2021 23:59:00", 63: "31.12.2021 23:59:00", 64: "31.12.2021 23:59:00", 65: "31.12.2021 23:59:00", 66: "31.12.2021 23:59:00", 67: "31.12.2021 23:59:00", 68: "31.12.2021 23:59:00", 69: "31.12.2021 23:59:00", 70: "31.12.2021 23:59:00", 71: "31.12.2021 23:59:00", 72: "31.12.2021 23:59:00", 73: "31.12.2021 23:59:00", 74: "31.12.2021 23:59:00", 75: "31.12.2021 23:59:00", 76: "31.12.2021 23:59:00", 77: "31.12.2021 23:59:00", 78: "31.12.2021 23:59:00", 79: "31.12.2021 23:59:00", 80: "31.12.2021 23:59:00", 81: "31.12.2021 23:59:00", 82: "31.12.2021 23:59:00", 83: "31.12.2021 23:59:00", 84: "31.12.2021 23:59:00", 85: "31.12.2021 23:59:00", 86: "31.12.2021 23:59:00", 87: "31.12.2021 23:59:00", 88: "31.12.2021 23:59:00", 89: "31.12.2021 23:59:00", 90: "31.12.2021 23:59:00", 91: "31.12.2021 23:59:00", 92: "31.12.2021 23:59:00", 93: "31.12.2021 23:59:00", 94: "31.12.2021 23:59:00", 95: "31.12.2021 23:59:00", 96: "31.12.2021 23:59:00", 97: "31.12.2021 23:59:00", 98: "31.12.2021 23:59:00", 99: "31.12.2021 23:59:00",
			},
		}
		excelCpMeta, cpMeta, err := buildMatrixMetaStruct(db, header)
		require.NoError(t, err)
		require.NotNil(t, cpMeta)
		require.Equal(t, 33, len(cpMeta))
		require.NotNil(t, excelCpMeta)
		require.Equal(t, 33, len(excelCpMeta))

		cConsuption := 0
		cGeneration := 0
		var currentIdx []int
		currentSourceIdx := []int{}
		consumptionIdx := []int{}
		productionIdx := []int{}
		for j := 0; j < len(excelCpMeta); j++ {
			v := excelCpMeta[j]
			if v.Dir == "CONSUMPTION" {
				cConsuption += 1
				consumptionIdx = append(consumptionIdx, v.Idx)
			} else if v.Dir == "GENERATION" {
				cGeneration += 1
				productionIdx = append(productionIdx, v.Idx)
			}
			currentIdx = append(currentIdx, v.Idx)
			currentSourceIdx = append(currentSourceIdx, v.SourceIdx)
			fmt.Printf("Meta: %+v\n", v)
		}

		require.Equal(t, 1, cGeneration)
		require.Equal(t, 32, cConsuption)

		fmt.Printf("currentIdx: (%+v)\n", currentIdx)
		fmt.Printf("currentSourceIdx: (%+v)\n", currentSourceIdx)
		fmt.Printf("productionSourceIdx: (%+v)\n", productionIdx)

		expectedIdx := []int{0, 3, 6, 9, 12, 15, 18, 21, 24, 27, 30, 33, 36, 39, 42, 45, 48, 51, 54, 57, 60, 63, 66, 69, 72, 75, 78, 81, 84, 87, 90, 93, 96}
		require.ElementsMatch(t, expectedIdx, currentIdx)
		expectedSourceIdx := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 0}
		require.ElementsMatch(t, expectedSourceIdx, currentSourceIdx)
	})
}
