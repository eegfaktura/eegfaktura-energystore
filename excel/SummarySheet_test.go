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

func TestSummaryResult(t *testing.T) {
	var tests = []struct {
		name     string
		metaData *model.RawSourceMeta
		cps      *ExportCPs
		entries  []*model.RawSourceLine
		check    func(t *testing.T, sheet *SummarySheet, result *SummaryResult)
	}{
		{
			name:     "fillupMissingValues",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries:  exportEntries,
			check: func(t *testing.T, sheet *SummarySheet, result *SummaryResult) {
				assert.Equal(t, []float64{1.01, 2.1}, sheet.report.Allocated)
				assert.Equal(t, 1.1, sheet.report.Consumed[0])
				assert.Equal(t, 1.5, sheet.report.Shared[0])
				assert.Equal(t, []float64{1.8, 1.7}, sheet.report.Distributed)
				assert.ElementsMatch(t, []float64{1.5, 1}, sheet.report.Produced)
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
