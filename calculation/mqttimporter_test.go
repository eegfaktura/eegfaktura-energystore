package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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

//func setUp() {
//	viper.Set("persistence.path", "../../../rawdata/test")
//
//}
//
//func setDown() {
//	os.RemoveAll("../../../rawdata/test/importer")
//	fmt.Println("Dataset (rawdata/test/importer) removed!")
//}

func TestNewMqttEnergyImporter(t *testing.T) {
	timeV1, err := utils.ParseTime("24.10.2022 00:00:00")
	timeV2, err := utils.ParseTime("24.10.2022 00:15:00")
	require.NoError(t, err)
	tests := []struct {
		name     string
		energy   *model.MqttEnergyMessage
		expected func(t *testing.T, l *model.RawSourceLine)
	}{
		{
			name: "Insert New Energy Allocated",
			energy: &model.MqttEnergyMessage{
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000000000001",
					Direction:     "",
				},
				Energy: model.MqttEnergy{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: "1-1:1.9.0 G.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  1.11,
								},
							},
						},
					},
				},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 1, len(l.Consumers))
				assert.Equal(t, 1.11, l.Consumers[0])
			},
		},
		{
			name: "Second Energy Consumer",
			energy: &model.MqttEnergyMessage{
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000000000002",
					Direction:     "",
				},
				Energy: model.MqttEnergy{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: "1-1:1.9.0 G.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  0.11,
								},
							},
						},
					},
				},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 4, len(l.Consumers))
				assert.Equal(t, 1.11, l.Consumers[0])
				assert.Equal(t, 0.11, l.Consumers[3])
			},
		},
		{
			name: "Insert Generator energy values",
			energy: &model.MqttEnergyMessage{
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000030000011",
					Direction:     "",
				},
				Energy: model.MqttEnergy{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: "1-1:2.9.0 P.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  0.11,
								},
							},
						},
						model.MqttEnergyData{
							MeterCode: "1-1:1.9.0 G.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  10.1,
								},
							},
						},
					},
				},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 2, len(l.Producers))
				assert.Equal(t, 10.1, l.Producers[0])
			},
		},
		{
			name: "Insert second Generator Allocated",
			energy: &model.MqttEnergyMessage{
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000030000010",
					Direction:     "",
				},
				Energy: model.MqttEnergy{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: "1-1:2.9.0 P.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  20.1,
								},
							},
						},
						model.MqttEnergyData{
							MeterCode: "1-1:1.9.0 G.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  20.1,
								},
							},
						},
					},
				},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 4, len(l.Producers))
				assert.Equal(t, 10.1, l.Producers[0])
				assert.Equal(t, 20.1, l.Producers[2])
			},
		},
	}

	viper.Set("persistence.path", "../test/rawdata")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err = importEnergyV2("importer", tt.energy)
			require.NoError(t, err)

			db, err := store.OpenStorageTest("importer", "../test/rawdata")
			require.NoError(t, err)
			it := db.GetLinePrefix(fmt.Sprintf("CP/%s", "2022/10/24"))
			defer it.Close()
			defer db.Close()

			var _line model.RawSourceLine

			r := it.Next(&_line)
			assert.Equal(t, true, r)
			assert.Equal(t, "CP/2022/10/24/00/00/00", _line.Id)
			tt.expected(t, &_line)
		})
	}

	os.RemoveAll("../test/rawdata/importer")
}

func TestImportRawdataStore(t *testing.T) {

	viper.Set("persistence.path", "../test/rawdata")

	jsonRaw, err := os.ReadFile("../test/energy-response-text.json")
	require.NoError(t, err)

	rawData := decodeMessage(jsonRaw)
	require.NotNil(t, rawData)

	err = importEnergyV2("te100190", rawData)
	require.NoError(t, err)

	rawData.Meter.MeteringPoint = "AT0030000000000000000000000381702"
	err = importEnergyV2("te100190", rawData)
	require.NoError(t, err)

	db, err := store.OpenStorageTest("te100190", "../test/rawdata")
	require.NoError(t, err)

	meta, err := db.GetMeta("cpmeta/0")
	for i, v := range meta.CounterPoints {
		fmt.Printf("[%d]: %+v\n", i, v)
	}

	it := db.GetLinePrefix("CP/")

	line := model.RawSourceLine{}
	lines := []*model.RawSourceLine{}
	for it.Next(&line) {
		_line := line.Copy(len(line.Consumers))
		lines = append(lines, &_line)
	}
	it.Close()
	db.Close()

	require.Equal(t, 23*4, len(lines)) // one hour is missing from the test source file

	energy, err := EnergyReport("te100190", 2023, 3, "YM")
	require.NoError(t, err)

	response, err := json.Marshal(energy)
	require.NoError(t, err)

	require.Equal(t, 2, len(energy.Report.Allocated))
	require.Equal(t, 1.088021, energy.Report.Allocated[0])
	require.Equal(t, 2, len(energy.Report.Consumed))
	require.Equal(t, 5.388, energy.Report.Consumed[0])

	fmt.Printf("META_DATA: %+v\n", string(response))

	os.RemoveAll("../test/rawdata/rc100190")
}
