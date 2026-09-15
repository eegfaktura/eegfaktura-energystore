package excel

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
	"testing"
	"time"
)

func TestSummaryResult(t *testing.T) {
	resetTestData()
	var tests = []struct {
		name     string
		metaData *model.RawSourceMeta
		cps      *ExportParticipantEnergy
		entries  []*model.RawSourceLine
		check    func(t *testing.T, sheet *SummarySheet, result *SummaryResult)
	}{
		{
			name:     "fillupMissingValues",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries:  exportEntries,
			check: func(t *testing.T, sheet *SummarySheet, result *SummaryResult) {
				//assert.Equal(t, []float64{1.01, 2.1}, sheet.report.Allocated)
				//assert.Equal(t, 1.1, sheet.report.Consumed[0])
				//assert.Equal(t, 1.5, sheet.report.Shared[0])
				//assert.Equal(t, []float64{1.8, 1.7}, sheet.report.Distributed)
				//assert.ElementsMatch(t, []float64{1.5, 1}, sheet.report.Produced)
				assert.Equal(t, []float64{1.01, 2.1}, []float64{result.Consumer[0].Share, result.Consumer[1].Share})
				assert.Equal(t, 1.1, result.Consumer[0].Total)
				//assert.Equal(t, 1.5, sheet.report.Shared[0])
				//assert.Equal(t, []float64{1.8, 1.7}, sheet.report.Distributed)
				//assert.ElementsMatch(t, []float64{1.5, 1}, sheet.report.Produced)
				assert.ElementsMatch(t, []bool{true, true}, sheet.qovConsumerSlice)
				assert.ElementsMatch(t, []bool{true, true}, sheet.qovProducerSlice)
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

			summarySheet := &SummarySheet{name: "Summary", excel: f}
			runner := NewEnergyRunner([]Sheet{
				summarySheet,
			})

			_, err := runner.run(mockBow, f,
				time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
				time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
				tt.cps)
			assert.NoError(t, err)

			ctx, err := createRunnerContext(mockBow, time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
				time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
				tt.cps)
			assert.NoError(t, err)

			result, err := summarySheet.summaryMeteringPoints(ctx)
			assert.NoError(t, err)

			tt.check(t, summarySheet, result)
		})
	}
}

// "Daten ok" in der Uebersicht: L1 und L2 gelten als in Ordnung, L0 und L3 nicht.
func TestSummaryDataOkQoV(t *testing.T) {
	tests := []struct {
		name string
		qov  []int
		want bool
	}{
		{name: "L1", qov: []int{1, 1, 1, 1, 1, 1}, want: true},
		{name: "L2 is ok", qov: []int{1, 2, 2, 1, 1, 1}, want: true},
		{name: "L3", qov: []int{1, 3, 1, 1, 1, 1}, want: false},
		{name: "L0", qov: []int{0, 1, 1, 1, 1, 1}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTestData()
			mockBow := &mocks.MockBowStorage{}
			mockBow.On("GetMeta", "cpmeta/0").Return(exportTestMetaData)
			start := time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local)
			ctx, err := createRunnerContext(mockBow, start, start.AddDate(0, 0, 1), exportCps)
			assert.NoError(t, err)

			line := &model.RawSourceLine{Id: "CP/2023/01/01/12/00/00/",
				Consumers:    []float64{1, 1, 1, 1, 1, 1},
				Producers:    []float64{1, 1, 1, 1},
				QoVConsumers: tt.qov,
				QoVProducers: []int{1, 1, 1, 1},
			}
			consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(line)
			p := ctx.cps[0] // erster Verbraucher, SourceIdx 0
			ss := &SummarySheet{name: "Summary"}
			assert.NoError(t, ss.handleParticipantReport(ctx, p, consumerMatrix, producerMatrix,
				time.Date(2023, time.Month(1), 1, 12, 0, 0, 0, time.Local), line.QoVConsumers, line.QoVProducers))
			assert.Equal(t, tt.want, p.QoV)
		})
	}
}
