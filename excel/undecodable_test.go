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
