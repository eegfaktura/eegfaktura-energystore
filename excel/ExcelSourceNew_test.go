package excel

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
)

func setupMock() *mocks.MockBowStorage {

	mockBow := &mocks.MockBowStorage{}
	mockBow.On("SetMeta", mock.Anything).Return(nil)
	mockBow.On("SetLines", mock.Anything).Return(nil)
	mockBow.On("GetLine", mock.Anything)
	mockBow.On("GetMeta", "cpmeta/0").Return(&model.RawSourceMeta{CounterPoints: make([]*model.CounterPointMeta, 0)})

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
				sort.Slice(line, func(i, j int) bool {
					return line[i].Id < line[j].Id
				})
				assert.ElementsMatch(t, line, expectedLines)
				for _, l := range line {
					fmt.Printf("Line: %+v\n", l)
				}
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

	t.Run("Test short energy data file Extention", func(t *testing.T) {
		excelFile, err := OpenExceFile("../test/ShortTest-EnergiedatenExtention.xlsx")
		require.NoError(t, err)
		defer excelFile.Close()

		expectedLines := []*model.RawSourceLine{}
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/06/30/00", Consumers: []float64{1.704, 0.020107, 0.020107, 1.101, 0.012992, 0.012992, 0.941, 0.011104, 0.011104, 0.056, 0.000661, 0.000661, 3.5475, 0.04186, 0.04186, 2.82, 0.033276, 0.033276}, Producers: []float64{0.12, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/06/45/00", Consumers: []float64{1.574, 0.150035, 0.150035, 1.14, 0.108666, 0.108666, 0.332, 0.031647, 0.031647, 0.073, 0.006958, 0.006958, 3.9075, 0.372467, 0.372467, 2.73, 0.260227, 0.260227}, Producers: []float64{0.93, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/00/00", Consumers: []float64{1.395, 0.36197, 0.36197, 1.081, 0.280494, 0.280494, 0.857, 0.222371, 0.222371, 0.07, 0.018163, 0.018163, 4.7475, 1.231865, 1.231865, 3.18, 0.825136, 0.825136}, Producers: []float64{2.94, 0.1}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/15/00", Consumers: []float64{1.814, 0.619655, 0.619655, 1.146, 0.391469, 0.391469, 0.617, 0.210765, 0.210765, 0.063, 0.021521, 0.021521, 5.16, 1.762634, 1.762634, 4.11, 1.403958, 1.403958}, Producers: []float64{4.41, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/30/00", Consumers: []float64{1.632, 0.630328, 0.630328, 1.124, 0.434123, 0.434123, 0.412, 0.159127, 0.159127, 0.062, 0.023946, 0.023946, 5.175, 1.998744, 1.998744, 6.12, 2.363731, 2.363731}, Producers: []float64{5.61, 0.000001}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/45/00", Consumers: []float64{1.534, 1.198511, 1.198511, 1.281, 1.000842, 1.000842, 0.784, 0.612537, 0.612537, 0.066, 0.051566, 0.051566, 4.2675, 3.334188, 3.334188, 3.51, 2.742355, 2.742355}, Producers: []float64{8.94, 0.1}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/00/00", Consumers: []float64{1.380, 1.234198, 1.234198, 1.166, 1.042808, 1.042808, 0.632, 0.565227, 0.565227, 0.073, 0.065287, 0.065287, 5.6775, 5.077652, 5.077652, 5.16, 4.614828, 4.614828}, Producers: []float64{12.6, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/15/00", Consumers: []float64{1.565, 1.216186, 1.216186, 1.148, 0.892129, 0.892129, 0.786, 0.610813, 0.610813, 0.041, 0.031862, 0.031862, 10.1475, 7.885781, 7.885781, 5.19, 4.03323, 4.03323}, Producers: []float64{14.67, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/30/00", Consumers: []float64{1.545, 1.384754, 1.384754, 1.214, 1.088085, 1.088085, 0.359, 0.321765, 0.321765, 0.095, 0.085147, 0.085147, 9.7875, 8.772349, 8.772349, 6.48, 5.8079, 5.8079}, Producers: []float64{17.46, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/45/00", Consumers: []float64{1.678, 1.430015, 1.430015, 1.116, 0.951071, 0.951071, 0.703, 0.599106, 0.599106, 0.042, 0.035793, 0.035793, 12.4875, 10.642020, 10.64202, 8.58, 7.311995, 7.311995}, Producers: []float64{20.97, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/09/00/00", Consumers: []float64{1.712, 2.069223, 1.712, 0.977, 1.180859, 0.977, 0.833, 1.006812, 0.833, 0.074, 0.089441, 0.074, 9.255, 11.186131, 9.255, 7.8, 9.427534, 7.8}, Producers: []float64{24.96, 4.309}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})

		mockBow := setupMock()
		err = ImportExcelEnergyFileNew(excelFile, "Energiedaten", mockBow)

		var line []*model.RawSourceLine
		for _, c := range mockBow.Calls {
			fmt.Printf("Call %s: %+v\n", c.Method, c.Arguments.Get(0))
			if c.Method == "SetLines" {
				line = c.Arguments.Get(0).([]*model.RawSourceLine)
				for _, l := range line {
					fmt.Printf("Line: %+v\n", l)
				}
			}
		}
		//require.NoError(t, err)
		assert.ElementsMatch(t, line, expectedLines)
	})

	t.Run("Test short energy data file Extention with TF", func(t *testing.T) {
		excelFile, err := OpenExceFile("../test/ShortTest-EnergiedatenExtention.xlsx")
		require.NoError(t, err)
		defer excelFile.Close()

		expectedLines := []*model.RawSourceLine{}
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/06/30/00", Consumers: []float64{1.704, 0.020107, utils.RoundToFixed(0.020107*0.8, 7), 1.101, 0.012992, 0.012992, 0.941, 0.011104, 0.011104, 0.056, 0.000661, 0.000661, 3.5475, 0.04186, 0.04186, 2.82, 0.033276, 0.033276}, Producers: []float64{0.12, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/06/45/00", Consumers: []float64{1.574, 0.150035, utils.RoundToFixed(0.150035*0.8, 7), 1.14, 0.108666, 0.108666, 0.332, 0.031647, 0.031647, 0.073, 0.006958, 0.006958, 3.9075, 0.372467, 0.372467, 2.73, 0.260227, 0.260227}, Producers: []float64{0.93, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/00/00", Consumers: []float64{1.395, 0.36197, utils.RoundToFixed(0.36197*0.8, 7), 1.081, 0.280494, 0.280494, 0.857, 0.222371, 0.222371, 0.07, 0.018163, 0.018163, 4.7475, 1.231865, 1.231865, 3.18, 0.825136, 0.825136}, Producers: []float64{2.94, 0.1}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/15/00", Consumers: []float64{1.814, 0.619655, utils.RoundToFixed(0.619655*0.8, 7), 1.146, 0.391469, 0.391469, 0.617, 0.210765, 0.210765, 0.063, 0.021521, 0.021521, 5.16, 1.762634, 1.762634, 4.11, 1.403958, 1.403958}, Producers: []float64{4.41, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/30/00", Consumers: []float64{1.632, 0.630328, utils.RoundToFixed(0.630328*0.8, 7), 1.124, 0.434123, 0.434123, 0.412, 0.159127, 0.159127, 0.062, 0.023946, 0.023946, 5.175, 1.998744, 1.998744, 6.12, 2.363731, 2.363731}, Producers: []float64{5.61, 0.000001}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/45/00", Consumers: []float64{1.534, 1.198511, utils.RoundToFixed(1.198511*0.8, 7), 1.281, 1.000842, 1.000842, 0.784, 0.612537, 0.612537, 0.066, 0.051566, 0.051566, 4.2675, 3.334188, 3.334188, 3.51, 2.742355, 2.742355}, Producers: []float64{8.94, 0.1}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/00/00", Consumers: []float64{1.380, 1.234198, utils.RoundToFixed(1.234198*0.8, 7), 1.166, 1.042808, 1.042808, 0.632, 0.565227, 0.565227, 0.073, 0.065287, 0.065287, 5.6775, 5.077652, 5.077652, 5.16, 4.614828, 4.614828}, Producers: []float64{12.6, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/15/00", Consumers: []float64{1.565, 1.216186, utils.RoundToFixed(1.216186*0.8, 7), 1.148, 0.892129, 0.892129, 0.786, 0.610813, 0.610813, 0.041, 0.031862, 0.031862, 10.1475, 7.885781, 7.885781, 5.19, 4.03323, 4.03323}, Producers: []float64{14.67, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/30/00", Consumers: []float64{1.545, 1.384754, utils.RoundToFixed(1.384754*0.8, 7), 1.214, 1.088085, 1.088085, 0.359, 0.321765, 0.321765, 0.095, 0.085147, 0.085147, 9.7875, 8.772349, 8.772349, 6.48, 5.8079, 5.8079}, Producers: []float64{17.46, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/45/00", Consumers: []float64{1.678, 1.430015, utils.RoundToFixed(1.430015*0.8, 7), 1.116, 0.951071, 0.951071, 0.703, 0.599106, 0.599106, 0.042, 0.035793, 0.035793, 12.4875, 10.642020, 10.64202, 8.58, 7.311995, 7.311995}, Producers: []float64{20.97, 0}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/09/00/00", Consumers: []float64{1.712, 2.069223, utils.RoundToFixed(1.712*0.8, 7), 0.977, 1.180859, 0.977, 0.833, 1.006812, 0.833, 0.074, 0.089441, 0.074, 9.255, 11.186131, 9.255, 7.8, 9.427534, 7.8}, Producers: []float64{24.96, 4.309}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})

		mockBow := setupMock()
		err = ImportExcelEnergyFileNew(excelFile, "Energiedaten-TF", mockBow)

		var line []*model.RawSourceLine
		for _, c := range mockBow.Calls {
			fmt.Printf("Call %s: %+v\n", c.Method, c.Arguments.Get(0))
			if c.Method == "SetLines" {
				line = c.Arguments.Get(0).([]*model.RawSourceLine)
				sort.Slice(line, func(i, j int) bool {
					return line[i].Id < line[j].Id
				})
				for i, l := range line {
					fmt.Printf("Line: %+v\n", l)
					fmt.Printf("Expe: %+v\n", expectedLines[i])
				}
			}
		}
		//require.NoError(t, err)
		assert.ElementsMatch(t, line, expectedLines)
	})

	t.Run("Test short energy data file Extention with TF - BEG", func(t *testing.T) {
		excelFile, err := OpenExceFile("../test/ShortTest-EnergiedatenExtention.xlsx")
		require.NoError(t, err)
		defer excelFile.Close()

		expectedLines := []*model.RawSourceLine{}
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/06/30/00", Consumers: []float64{utils.RoundToFixed(1.704*0.6, 7), 0.020107, utils.RoundToFixed(0.020107*0.8, 7), 1.101, 0.012992, 0.012992, 0.941, 0.011104, 0.011104, 0.056, 0.000661, 0.000661, 3.5475, 0.04186, 0.04186, 2.82, 0.033276, 0.033276}, Producers: []float64{utils.RoundToFixed(0.12*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/06/45/00", Consumers: []float64{utils.RoundToFixed(1.574*0.6, 7), 0.150035, utils.RoundToFixed(0.150035*0.8, 7), 1.14, 0.108666, 0.108666, 0.332, 0.031647, 0.031647, 0.073, 0.006958, 0.006958, 3.9075, 0.372467, 0.372467, 2.73, 0.260227, 0.260227}, Producers: []float64{utils.RoundToFixed(0.93*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/00/00", Consumers: []float64{utils.RoundToFixed(1.395*0.6, 7), 0.36197, utils.RoundToFixed(0.36197*0.8, 7), 1.081, 0.280494, 0.280494, 0.857, 0.222371, 0.222371, 0.07, 0.018163, 0.018163, 4.7475, 1.231865, 1.231865, 3.18, 0.825136, 0.825136}, Producers: []float64{utils.RoundToFixed(2.94*0.4, 7), utils.RoundToFixed(0.1*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/15/00", Consumers: []float64{utils.RoundToFixed(1.814*0.6, 7), 0.619655, utils.RoundToFixed(0.619655*0.8, 7), 1.146, 0.391469, 0.391469, 0.617, 0.210765, 0.210765, 0.063, 0.021521, 0.021521, 5.16, 1.762634, 1.762634, 4.11, 1.403958, 1.403958}, Producers: []float64{utils.RoundToFixed(4.41*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/30/00", Consumers: []float64{utils.RoundToFixed(1.632*0.6, 7), 0.630328, utils.RoundToFixed(0.630328*0.8, 7), 1.124, 0.434123, 0.434123, 0.412, 0.159127, 0.159127, 0.062, 0.023946, 0.023946, 5.175, 1.998744, 1.998744, 6.12, 2.363731, 2.363731}, Producers: []float64{utils.RoundToFixed(5.61*0.4, 7), utils.RoundToFixed(0.000001*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/07/45/00", Consumers: []float64{utils.RoundToFixed(1.534*0.6, 7), 1.198511, utils.RoundToFixed(1.198511*0.8, 7), 1.281, 1.000842, 1.000842, 0.784, 0.612537, 0.612537, 0.066, 0.051566, 0.051566, 4.2675, 3.334188, 3.334188, 3.51, 2.742355, 2.742355}, Producers: []float64{utils.RoundToFixed(8.94*0.4, 7), utils.RoundToFixed(0.1*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/00/00", Consumers: []float64{utils.RoundToFixed(1.380*0.6, 7), 1.234198, utils.RoundToFixed(1.234198*0.8, 7), 1.166, 1.042808, 1.042808, 0.632, 0.565227, 0.565227, 0.073, 0.065287, 0.065287, 5.6775, 5.077652, 5.077652, 5.16, 4.614828, 4.614828}, Producers: []float64{utils.RoundToFixed(12.6*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/15/00", Consumers: []float64{utils.RoundToFixed(1.565*0.6, 7), 1.216186, utils.RoundToFixed(1.216186*0.8, 7), 1.148, 0.892129, 0.892129, 0.786, 0.610813, 0.610813, 0.041, 0.031862, 0.031862, 10.1475, 7.885781, 7.885781, 5.19, 4.03323, 4.03323}, Producers: []float64{utils.RoundToFixed(14.67*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/30/00", Consumers: []float64{utils.RoundToFixed(1.545*0.6, 7), 1.384754, utils.RoundToFixed(1.384754*0.8, 7), 1.214, 1.088085, 1.088085, 0.359, 0.321765, 0.321765, 0.095, 0.085147, 0.085147, 9.7875, 8.772349, 8.772349, 6.48, 5.8079, 5.8079}, Producers: []float64{utils.RoundToFixed(17.46*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/08/45/00", Consumers: []float64{utils.RoundToFixed(1.678*0.6, 7), 1.430015, utils.RoundToFixed(1.430015*0.8, 7), 1.116, 0.951071, 0.951071, 0.703, 0.599106, 0.599106, 0.042, 0.035793, 0.035793, 12.4875, 10.642020, 10.64202, 8.58, 7.311995, 7.311995}, Producers: []float64{utils.RoundToFixed(20.97*0.4, 7), utils.RoundToFixed(0*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})
		expectedLines = append(expectedLines, &model.RawSourceLine{Id: "CP/2024/04/01/09/00/00", Consumers: []float64{utils.RoundToFixed(1.712*0.6, 7), 2.069223, utils.RoundToFixed(1.712*0.8, 7), 0.977, 1.180859, 0.977, 0.833, 1.006812, 0.833, 0.074, 0.089441, 0.074, 9.255, 11.186131, 9.255, 7.8, 9.427534, 7.8}, Producers: []float64{utils.RoundToFixed(24.96*0.4, 7), utils.RoundToFixed(4.309*0.9, 7)}, QoVConsumers: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1}})

		mockBow := setupMock()
		err = ImportExcelEnergyFileNew(excelFile, "Energiedaten-TF-BEG", mockBow)

		var line []*model.RawSourceLine
		for _, c := range mockBow.Calls {
			fmt.Printf("Call %s: %+v\n", c.Method, c.Arguments.Get(0))
			if c.Method == "SetLines" {
				line = c.Arguments.Get(0).([]*model.RawSourceLine)
				sort.Slice(line, func(i, j int) bool {
					return line[i].Id < line[j].Id
				})
				for i, l := range line {
					fmt.Printf("Line: %+v\n", l)
					fmt.Printf("Expe: %+v\n", expectedLines[i])
				}
			}
		}
		//require.NoError(t, err)
		assert.ElementsMatch(t, line, expectedLines)
	})
}

func TestImportExcelEnergyFileExtention(t *testing.T) {

}
