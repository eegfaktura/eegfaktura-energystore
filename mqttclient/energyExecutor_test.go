package mqttclient

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encryptMessage(msg []byte) ([]byte, error) {
	compressed, err := gzipData(msg)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(compressed)
	return []byte(encoded), nil
}

func gzipData(data []byte) ([]byte, error) {
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	defer gz.Close()
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func TestDecodeMessageRejectsIncompatibleTransportPayload(t *testing.T) {
	require.Nil(t, decodeMessage([]byte("not-a-base64-cr-msg")))

	notGzip := base64.StdEncoding.EncodeToString([]byte("not-gzip"))
	require.Nil(t, decodeMessage([]byte(notGzip)))
}

func TestNewMqttEnergyImporter(t *testing.T) {
	timeV1, err := utils.ParseTime("24.10.2022 00:00:00", time.Now().UnixMilli())
	timeV2, err := utils.ParseTime("24.10.2022 00:15:00", time.Now().UnixMilli())
	require.NoError(t, err)
	tests := []struct {
		name     string
		energy   *model.MqttEnergyMessage
		expected func(t *testing.T, l *model.RawSourceLine)
	}{
		{
			name: "Insert New Energy Allocated",
			energy: &model.MqttEnergyMessage{
				EcId: "ecIdTest1",
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000000000001",
					Direction:     string(model.CONSUMER_DIRECTION),
				},
				Energy: []model.MqttEnergy{{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: "1-1:1.9.0 G.01",
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "L1",
									Value:  1.11,
								},
							},
						},
					},
				}},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 1, len(l.Consumers))
				assert.Equal(t, 1.11, l.Consumers[0])
			},
		},
		{
			name: "Second Energy Consumer",
			energy: &model.MqttEnergyMessage{
				EcId: "ecIdTest1",
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000000000002",
					Direction:     "",
				},
				Energy: []model.MqttEnergy{{
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
				}},
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
				EcId: "ecIdTest1",
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000030000011",
					Direction:     "",
				},
				Energy: []model.MqttEnergy{{
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
				}},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 2, len(l.Producers))
				assert.Equal(t, 10.1, l.Producers[0])
			},
		},
		{
			name: "Insert second Generator Allocated",
			energy: &model.MqttEnergyMessage{
				EcId: "ecIdTest1",
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000030000010",
					Direction:     "",
				},
				Energy: []model.MqttEnergy{{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: model.CODE_PLUS,
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
							MeterCode: model.CODE_GEN,
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "",
									Value:  21.1,
								},
							},
						},
					},
				}},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				require.Equal(t, 4, len(l.Producers))
				assert.Equal(t, 10.1, l.Producers[0])
				assert.Equal(t, 21.1, l.Producers[2])
				assert.Equal(t, 20.1, l.Producers[3])
			},
		},
		{
			name: "Insert Generator - summarize energy values",
			energy: &model.MqttEnergyMessage{
				EcId: "ecIdTest2",
				Meter: model.EnergyMeter{
					MeteringPoint: "AT0030000000000000000000030000010",
					Direction:     "",
				},
				Energy: []model.MqttEnergy{{
					Start: timeV1.UnixMilli(),
					End:   timeV2.UnixMilli(),
					Data: []model.MqttEnergyData{
						model.MqttEnergyData{
							MeterCode: model.CODE_PLUS,
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "L1",
									Value:  20.1,
								},
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "L1",
									Value:  10.1,
								},
							},
						},
						model.MqttEnergyData{
							MeterCode: model.CODE_GEN,
							Value: []model.MqttEnergyValue{
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "L1",
									Value:  5.1,
								},
								model.MqttEnergyValue{
									From:   timeV1.UnixMilli(),
									To:     timeV2.UnixMilli(),
									Method: "L1",
									Value:  2.2,
								},
							},
						},
					},
				}},
			},
			expected: func(t *testing.T, l *model.RawSourceLine) {
				fmt.Printf("Producer Line: %+v\n", l)
				require.Equal(t, 2, len(l.Producers))
				assert.Equal(t, 7.3, utils.RoundToFixed(l.Producers[0], 1))
				assert.Equal(t, 1, l.QoVProducers[0])
				assert.Equal(t, 30.2, utils.RoundToFixed(l.Producers[1], 1))
				assert.Equal(t, 1, l.QoVProducers[1])
			},
		},
	}

	testsupport.UseTempPersistence(t)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			importer := NewTenantEnergyImporter("importer", &MQTTStreamer{})
			defer importer.Close() // an unclosed importer keeps its pool objects (known-errors #54)
			err = importer.Import(tt.energy)
			require.NoError(t, err)

			db, err := ebow.OpenStorage("importer", tt.energy.EcId)
			require.NoError(t, err)
			defer db.Close()

			it := db.GetLinePrefix(fmt.Sprintf("CP/%s", "2022/10/24"))
			defer it.Close()

			var _line model.RawSourceLine

			r := it.Next(&_line)
			assert.Equal(t, true, r)
			assert.Equal(t, "CP/2022/10/24/00/00/00", _line.Id)
			tt.expected(t, &_line)
		})
	}

}

func TestImportRawdataStore(t *testing.T) {

	dir := testsupport.UseTempPersistence(t)

	jsonRaw, err := os.ReadFile("../test/energy-response-new-text.json")
	require.NoError(t, err)

	compressed, err := encryptMessage(jsonRaw)
	require.NoError(t, err)

	rawData := decodeMessage(compressed)
	require.NotNil(t, rawData)

	importer := NewTenantEnergyImporter("te100190", &MQTTStreamer{})

	err = importer.Import(rawData)
	require.NoError(t, err)

	rawData.Meter.MeteringPoint = "AT0030000000000000000000000381702"
	err = importer.Import(rawData)
	require.NoError(t, err)

	importer.Close()

	db, err := ebow.OpenStorageTest("te100190", "AT00400000000RC101590000000400111", dir)
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
	db.CloseTestDriver()

	require.Equal(t, 24*4, len(lines)) // one hour is missing from the test source file

	participantReports := []model.ParticipantReport{model.ParticipantReport{
		ParticipantId: "Participant01",
		Meters: []*model.MeterReport{
			&model.MeterReport{
				MeterId: "AT0030000000000000000000000381702",
				From:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.Local).UnixMilli(),
				Until:   time.Date(2023, 12, 31, 0, 0, 0, 0, time.Local).UnixMilli(),
				Report:  nil,
			},
		},
	}}

	energy, err := calculation.EnergyReportV2("te100190", "AT00400000000RC101590000000400111", participantReports, 2023, 3, "YM")
	require.NoError(t, err)

	response, err := json.Marshal(energy)
	require.NoError(t, err)

	require.Equal(t, 1, len(energy.ParticipantReports[0].Meters[0].Report.Intermediate.Allocation))
	//require.Equal(t, 1.088021, energy.Report.Allocated[0])
	//require.Equal(t, 3, len(energy.Report.Consumed))
	//require.Equal(t, 5.388, energy.Report.Consumed[0])

	fmt.Printf("META_DATA: %+v\n", string(response))
}

// TestMassImport imports one CR_MSG with three days of quarter-hour values (generated here,
// formerly a 2976-slot file that was in no repository, known-errors #5) and reads them back
// through QueryRawData as /query/rawdata does.
func TestMassImport(t *testing.T) {
	testsupport.UseTempPersistence(t)

	tenant := "TE100888"
	ecId := "AT00300000000RC100181000000956509"
	meter := "AT0030000000000000000000000383545"
	start := time.Date(2025, time.October, 1, 0, 0, 0, 0, testsupport.Vienna)
	const slots = 3 * 96

	con, share, cover := make([]float64, slots), make([]float64, slots), make([]float64, slots)
	for i := range con {
		con[i] = float64(i%96+1) / 1000
		share[i] = float64(i%7) / 10000
		cover[i] = share[i] / 2
	}
	msg := testsupport.CrMsg(ecId, meter).Direction(model.CONSUMER_DIRECTION).
		Energy(start, "L1", map[model.MeterCodeValue][]float64{
			model.CODE_CON: con, model.CODE_SHARE: share, model.CODE_COVER: cover}).Message()

	importer := NewTenantEnergyImporter(tenant, &MQTTStreamer{})
	require.NoError(t, importer.Import(msg))
	importer.Close()

	last := start.Add((slots - 1) * 15 * time.Minute)
	resp, err := store.QueryRawData(tenant, ecId, start, last, []store.TargetMP{{MeteringPoint: meter}}, map[string][]string{})
	require.NoError(t, err)

	result := resp[meter]
	require.NotNil(t, result)
	require.Equal(t, slots, len(result.Data))
	for _, i := range []int{0, 95, 96, 150, slots - 1} {
		ts := start.Add(time.Duration(i) * 15 * time.Minute)
		assert.Equal(t, ts.UnixMilli(), result.Data[i].Ts, "slot %d", i)
		testsupport.InDelta(t, con[i], result.Data[i].Value[0], "G.01 slot %d", i)
		testsupport.InDelta(t, share[i], result.Data[i].Value[1], "G.02 slot %d", i)
		testsupport.InDelta(t, cover[i], result.Data[i].Value[2], "G.03 slot %d", i)
	}
}
