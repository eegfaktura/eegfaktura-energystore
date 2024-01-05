package excel

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func setupMock() *mocks.MockBowStorage {

	mockBow := &mocks.MockBowStorage{}
	mockBow.On("SetMeta", mock.Anything).Return(nil)
	mockBow.On("SetLines", mock.Anything).Return(nil)
	mockBow.On("GetLine", mock.Anything)
	mockBow.On("GetMeta", "cpmeta/0")

	return mockBow
}

func TestImportExcelEnergyFileNew(t *testing.T) {

	t.Run("Test short energy data file NEW", func(t *testing.T) {
		excelFile, err := OpenExceFile("../test/ShortTest-Energiedaten New.xlsx")
		require.NoError(t, err)
		defer excelFile.Close()

		expectedLines := []*model.RawSourceLine{}
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/01/01/00/00/00", Consumers: []float64{0.004, 0.000727, 0.000727, 0.002, 0.000364, 0.000364, 0.082, 0.014909, 0.014909, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Producers: []float64{0.016, 0, 0, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/01/01/00/15/00", Consumers: []float64{0.005, 0.000769, 0.000769, 0.003, 0.000462, 0.000462, 0.083, 0.012769, 0.012769, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Producers: []float64{0.014, 0, 0, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/01/01/00/30/00", Consumers: []float64{0.005, 0.001077, 0.001077, 0.002, 0.000431, 0.000431, 0.058, 0.012492, 0.012492, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Producers: []float64{0.014, 0, 0, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/07/09/13/30/00", Consumers: []float64{0, 0, 0, 0.008, 0.087296, 0.008, 0.264, 2.880768, 0.264, 0.459, 5.008608, 0.459, 0, 0, 0, 0.019, 0.207328, 0.019}, Producers: []float64{7.307738, 8.045, 0.126262, 0.139}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})

		mockBow := setupMock()
		err = ImportExcelEnergyFileNew(excelFile, "Energiedaten", mockBow)

		for _, c := range mockBow.Calls {
			fmt.Printf("Call %s: %+v\n", c.Method, c.Arguments.Get(0))
			if c.Method == "SetLines" {
				var line []*model.RawSourceLine = c.Arguments.Get(0).([]*model.RawSourceLine)
				assert.ElementsMatch(t, line, expectedLines)
				//for _, l := range line {
				//	fmt.Printf("Line: %+v\n", l)
				//}
			}
		}
		require.NoError(t, err)
	})

	t.Run("Test double enery ids", func(t *testing.T) {
		excelFile, err := OpenExceFile("../test/ShortTest-Energiedaten-double-ids.xlsx")
		require.NoError(t, err)
		defer excelFile.Close()

		expectedLines := []*model.RawSourceLine{}
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/01/01/00/00/00", Consumers: []float64{0.004 * 2, 0.000727 * 2, 0.000727 * 2, 0.002 * 2, 0.000364 * 2, 0.000364 * 2, 0.082 * 2, 0.014909 * 2, 0.014909 * 2, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Producers: []float64{0.016 * 2, 0, 0, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/01/01/00/15/00", Consumers: []float64{0.005 * 2, 0.000769 * 2, 0.000769 * 2, 0.003 * 2, 0.000462 * 2, 0.000462 * 2, 0.083 * 2, 0.012769 * 2, 0.012769 * 2, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Producers: []float64{0.014 * 2, 0, 0, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2023/01/01/00/30/00", Consumers: []float64{0.005 * 2, 0.001077 * 2, 0.001077 * 2, 0.002 * 2, 0.000431 * 2, 0.000431 * 2, 0.058 * 2, 0.012492 * 2, 0.012492 * 2, 0, 0, 0, 0, 0, 0, 0, 0, 0}, Producers: []float64{0.014 * 2, 0, 0, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}})

		mockBow := setupMock()
		err = ImportExcelEnergyFileNew(excelFile, "Energiedaten", mockBow)

		for _, c := range mockBow.Calls {
			fmt.Printf("Call %s: %+v\n", c.Method, c.Arguments.Get(0))
			if c.Method == "SetLines" {
				var line []*model.RawSourceLine = c.Arguments.Get(0).([]*model.RawSourceLine)
				assert.ElementsMatch(t, line, expectedLines)
				for _, l := range line {
					fmt.Printf("Line: %+v\n", l)
				}
			}
		}
		require.NoError(t, err)
	})
}
