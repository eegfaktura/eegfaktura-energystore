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

// Der gemischte Fall: ein Zaehlpunkt mit L0 UND L3 bekommt zwei getrennte
// Kommentare -- je einer an der Zahl seiner Stufe, mit den Tagen dahinter.
func TestSummaryQoVDayComments(t *testing.T) {
	resetTestData()

	// Zwei Tage lueckenlos, sonst fuellt der Runner die Luecken auf und die
	// Fuellzeilen zaehlen zu Recht als L0.
	start := time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local)
	entries := make([]*model.RawSourceLine, 0, 192)
	for s := 0; s < 192; s++ {
		ts := start.Add(time.Duration(s) * 15 * time.Minute)
		l := &model.RawSourceLine{
			Id: fmt.Sprintf("CP/%.4d/%.2d/%.2d/%.2d/%.2d/00/",
				ts.Year(), int(ts.Month()), ts.Day(), ts.Hour(), ts.Minute()),
			Consumers:    []float64{1, 1, 1, 1, 1, 1},
			Producers:    []float64{1, 1, 1, 1},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		}
		switch {
		case s == 0: // 01.01. 00:00 -- ein einzelner fehlerhafter Wert
			l.QoVConsumers[0] = 3
		case s == 128 || s == 129: // 02.01. 08:00 und 08:15 -- keine Werte
			l.QoVConsumers[0] = 0
		}
		entries = append(entries, l)
	}

	mockRange := &mocks.MockBowRange{Entries: entries}
	mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()
	mockBow := &mocks.MockBowStorage{}
	mockBow.On("GetMeta", "cpmeta/0").Return(exportTestMetaData)
	mockBow.On("GetLineRange", "CP", "2023/01/01/", "2023/01/03/").Return(mockRange)

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	runner := NewEnergyRunner([]Sheet{
		&SummarySheet{name: "Summary", excel: f},
		&EnergySheet{name: "Energiedaten", excel: f},
	})
	_, err := runner.run(mockBow, f, start, start.AddDate(0, 0, 2), exportCps)
	assert.NoError(t, err)

	comments, err := f.GetComments("Summary")
	assert.NoError(t, err)
	byCell := map[string]string{}
	for _, c := range comments {
		byCell[c.Cell] = c.Text
	}

	// Der erste Verbraucher steht in Zeile 13; L0 in Spalte G, L3 in Spalte I.
	assert.Equal(t, "L0 (kein Messwert): 2 Viertelstunden an 1 Tag\n02.01.2023", byCell["G13"])
	assert.Equal(t, "L3 (fehlerhaft): 1 Viertelstunde an 1 Tag\n01.01.2023", byCell["I13"])
	assert.NotContains(t, byCell, "H13", "ohne L2 kein Kommentar")

	// Die Zahlen stehen sichtbar in der Zelle, auf der Farbe ihrer Stufe.
	rows, err := f.GetRows("Summary")
	assert.NoError(t, err)
	assert.Equal(t, "2", rows[12][6])
	assert.Equal(t, "", rows[12][7])
	assert.Equal(t, "1", rows[12][8])
	assert.Equal(t, qovColorL0, fillColor(t, f, "Summary", "G13"))
	assert.Equal(t, qovColorL3, fillColor(t, f, "Summary", "I13"))
}
