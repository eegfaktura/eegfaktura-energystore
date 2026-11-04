package excel

import (
	"errors"
	"testing"
	"time"

	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
)

// #31: a row that cannot be read makes the export fail instead of ending it early, also when it
// is the first row (no "no Rows found").
func TestExportUndecodableRowIsAnError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []*model.RawSourceLine
	}{
		{"after some rows", exportEntries[:2]},
		{"first row", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mockRange := &mocks.MockBowRange{Entries: tc.entries, ErrValue: ebow.ErrUndecodableRow}
			mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()
			mockBow := &mocks.MockBowStorage{}
			mockBow.On("GetMeta", "cpmeta/0").Return(exportTestMetaData)
			mockBow.On("GetLineRange", "CP", "2023/01/01/", "2023/01/02/").Return(mockRange)

			f := excelize.NewFile()
			defer func() { _ = f.Close() }()
			runner := NewEnergyRunner([]Sheet{
				&SummarySheet{name: "Summary", excel: f},
				&EnergySheet{name: "Energiedaten", excel: f},
			})
			buf, err := runner.run(mockBow, f,
				time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
				time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
				exportCps)
			assert.True(t, errors.Is(err, ebow.ErrUndecodableRow), "got %v", err)
			assert.Nil(t, buf)
		})
	}
}

// A row whose id is no time is skipped, like the reports do for rows after the first; the export dereferenced the
// nil time of such a row and panicked (ES-CQ-26).
func TestExportSkipsRowWithUnparsableId(t *testing.T) {
	export := func(entries []*model.RawSourceLine) [][]string {
		mockRange := &mocks.MockBowRange{Entries: entries}
		mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()
		mockBow := &mocks.MockBowStorage{}
		mockBow.On("GetMeta", "cpmeta/0").Return(exportTestMetaData)
		mockBow.On("GetLineRange", "CP", "2023/01/01/", "2023/01/02/").Return(mockRange)

		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		runner := NewEnergyRunner([]Sheet{&EnergySheet{name: "Energiedaten", excel: f}})
		buf, err := runner.run(mockBow, f,
			time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
			time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
			exportCps)
		assert.NoError(t, err)
		assert.NotNil(t, buf)
		rows, err := f.GetRows("Energiedaten")
		assert.NoError(t, err)
		return rows
	}

	want := export(exportEntries[:4])
	bad := &model.RawSourceLine{Id: "CP/not-a-time/", Consumers: []float64{9, 9, 9, 9, 9, 9},
		Producers: []float64{9, 9, 9, 9}, QoVConsumers: []int{1, 1, 1, 1, 1, 1}, QoVProducers: []int{1, 1, 1, 1}}
	withBad := []*model.RawSourceLine{exportEntries[0], exportEntries[1], bad, exportEntries[2], exportEntries[3]}
	var got [][]string
	assert.NotPanics(t, func() { got = export(withBad) })
	assert.Equal(t, want, got)
}
