package excel

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
	"testing"
	"time"
)

func TestEnergySheet(t *testing.T) {
	var tests = []struct {
		name     string
		metaData *model.RawSourceMeta
		cps      *ExportCPs
		entries  []*model.RawSourceLine
		check    func(t *testing.T, f *excelize.File)
	}{
		{
			name:     "fillupMissingValues",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries: append(exportEntries, &model.RawSourceLine{Id: "CP/2023/01/02/00/00/00/",
				Consumers:    []float64{0, 0, 0, 0, 0, 0},
				Producers:    []float64{0, 0, 0, 0},
				QoVConsumers: []int{1, 1, 1, 1, 1, 1},
				QoVProducers: []int{1, 1, 1, 1},
			}),
			check: func(t *testing.T, f *excelize.File) {
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				assert.Equal(t, len(rows[1]), 11)
				assert.Equal(t, len(rows), 107)

				assert.Equal(t, "01.01.2023 23:30:00", rows[104][0])
				assert.Equal(t, "01.01.2023 23:45:00", rows[105][0])
				assert.Equal(t, "02.01.2023 00:00:00", rows[106][0])
			},
		},
		{
			name:     "noMissingData",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries:  exportEntries,
			check: func(t *testing.T, f *excelize.File) {
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				assert.Equal(t, len(rows[1]), 11)
				assert.Equal(t, len(rows), 19)

				assert.Equal(t, "01.01.2023 00:00:00", rows[10][0])
				assert.Equal(t, "01.01.2023 01:30:00", rows[16][0])
				assert.Equal(t, "01.01.2023 01:45:00", rows[17][0])
				assert.Equal(t, "01.01.2023 02:00:00", rows[18][0])
			},
		},
		{
			name:     "Bad Data - QoV differs to 1",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries: append(exportEntries,
				&model.RawSourceLine{Id: "CP/2023/01/02/00/00/00/",
					Consumers:    []float64{0, 0, 0, 0, 0, 0},
					Producers:    []float64{0, 0, 0, 0},
					QoVConsumers: []int{1, 1, 1, 1, 1, 1},
					QoVProducers: []int{1, 1, 1, 1},
				},
				&model.RawSourceLine{Id: "CP/2023/01/02/00/15/00/",
					Consumers:    []float64{0, 0, 0, 0, 0, 0},
					Producers:    []float64{0, 0, 0, 0},
					QoVConsumers: []int{1, 2, 2, 1, 1, 1},
					QoVProducers: []int{1, 1, 1, 1},
				}),
			check: func(t *testing.T, f *excelize.File) {
				rows, err := f.GetRows("QoV Log")
				assert.NoError(t, err)
				assert.Equal(t, len(rows[1]), 21)
				assert.Equal(t, len(rows), 96)
				//
				//for i, r := range rows {
				//	if len(r) > 0 {
				//		fmt.Printf("L%.3d: %s\n", i, r[0])
				//	}
				//}

				assert.Equal(t, "01.01.2023 02:15:00", rows[8][0])
				assert.Equal(t, "01.01.2023 02:45:00", rows[10][0])
				assert.Equal(t, "01.01.2023 23:45:00", rows[94][0])
				assert.Equal(t, "02.01.2023 00:15:00", rows[95][0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRange := &mocks.MockBowRange{Entries: tt.entries}
			mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()

			mockBow := &mocks.MockBowStorage{}
			mockBow.On("GetMeta", "cpmeta/0").Return(tt.metaData)
			mockBow.On("GetLineRange", "CP", "2023/01/01/", "2023/01/02/").Return(mockRange)

			f := excelize.NewFile()
			defer func() {
				if err := f.Close(); err != nil {
					fmt.Println(err)
				}
			}()

			runner := NewEnergyRunner([]Sheet{
				&SummarySheet{name: "Summary", excel: f},
				&EnergySheet{name: "Energiedaten", excel: f},
			})

			_, err := runner.run(mockBow, f,
				time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
				time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
				tt.cps)
			assert.NoError(t, err)
			tt.check(t, f)
		})
	}
}
