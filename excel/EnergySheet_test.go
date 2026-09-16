package excel

import (
	"bytes"

	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
	"testing"
	"time"
)

// fillColor liefert die Hintergrundfarbe einer Zelle, "" wenn sie keine hat.
func fillColor(t *testing.T, f *excelize.File, sheet, axis string) string {
	t.Helper()
	id, err := f.GetCellStyle(sheet, axis)
	assert.NoError(t, err)
	st, err := f.GetStyle(id)
	assert.NoError(t, err)
	if st == nil || len(st.Fill.Color) == 0 {
		return ""
	}
	return st.Fill.Color[0]
}

func TestEnergySheet(t *testing.T) {
	var tests = []struct {
		name     string
		metaData *model.RawSourceMeta
		cps      *ExportParticipantEnergy
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

				// Summary uebernimmt das Standardblatt, es bleibt kein "Sheet1" zurueck.
				assert.Equal(t, []string{"Summary", "Energiedaten"}, f.GetSheetList())
				assert.Equal(t, 0, f.GetActiveSheetIndex())
			},
		},
		{
			name:     "L3 markiert die Zelle, ohne eigenes Blatt",
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
					QoVConsumers: []int{1, 3, 3, 1, 1, 1},
					QoVProducers: []int{1, 1, 1, 1},
				}),
			check: func(t *testing.T, f *excelize.File) {
				assert.Equal(t, []string{"Summary", "Energiedaten"}, f.GetSheetList(),
					"kein eigenes QoV-Blatt mehr")

				// Die betroffenen Zellen tragen die L3-Farbe: Verbraucher 1 (Spalten B-D),
				// zweiter und dritter Wert, in der Zeile zu 02.01.2023 00:15.
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				row := len(rows)
				assert.Equal(t, "02.01.2023 00:15:00", rows[row-1][0])
				assert.Equal(t, qovColorL3, fillColor(t, f, "Energiedaten", fmt.Sprintf("C%d", row)))
				assert.Equal(t, qovColorL3, fillColor(t, f, "Energiedaten", fmt.Sprintf("D%d", row)))
				assert.Equal(t, "", fillColor(t, f, "Energiedaten", fmt.Sprintf("B%d", row)),
					"L1 bleibt ungefaerbt")
			},
		},
		{
			name:     "L0 wird grau markiert statt leer zu bleiben",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries: append(exportEntries,
				&model.RawSourceLine{Id: "CP/2023/01/02/00/00/00/",
					Consumers:    []float64{0, 0, 0, 0, 0, 0},
					Producers:    []float64{0, 0, 0, 0},
					QoVConsumers: []int{0, 0, 0, 1, 1, 1},
					QoVProducers: []int{1, 1, 1, 1},
				}),
			check: func(t *testing.T, f *excelize.File) {
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				row := len(rows)
				assert.Equal(t, qovColorL0, fillColor(t, f, "Energiedaten", fmt.Sprintf("B%d", row)))
			},
		},
		{
			name:     "L2 gilt weiter als in Ordnung",
			metaData: exportTestMetaData,
			cps:      exportCps,
			entries: append(exportEntries,
				&model.RawSourceLine{Id: "CP/2023/01/02/00/00/00/",
					Consumers:    []float64{0, 0, 0, 0, 0, 0},
					Producers:    []float64{0, 0, 0, 0},
					QoVConsumers: []int{1, 2, 2, 1, 1, 1},
					QoVProducers: []int{1, 2, 1, 1},
				}),
			check: func(t *testing.T, f *excelize.File) {
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				row := len(rows)
				assert.Equal(t, qovColorL2, fillColor(t, f, "Energiedaten", fmt.Sprintf("C%d", row)))
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

// Prueft die GESCHRIEBENE Datei, nicht nur die im Speicher: erst beim Schreiben entscheidet sich,
// welche Blaetter drin sind und welches aktiv ist.
func TestExportedWorkbookSheets(t *testing.T) {
	mockRange := &mocks.MockBowRange{Entries: exportEntries}
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
	assert.NoError(t, err)

	out, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)
	defer func() { _ = out.Close() }()
	assert.Equal(t, []string{"Summary", "Energiedaten"}, out.GetSheetList(), "kein Sheet1 in der Datei")
	assert.Equal(t, 0, out.GetActiveSheetIndex(), "Summary ist beim Oeffnen aktiv")
}

// Grosse Gemeinschaft, ein Tag: die Breiten muessen bis zur letzten Datenspalte reichen, und
// der Export darf nicht mit der Zaehlpunktzahl explodieren. Frueher setzte das QoV-Blatt jede
// Breite einzeln -- bei 500 Zaehlpunkten 6 s, bei 2.000 333 s. Die Zeitgrenze ist grosszuegig
// (lokal ~0,5 s), faengt aber einen Rueckfall in das kubische Verhalten sicher.
func TestExportColumnWidthsLargeCommunity(t *testing.T) {
	const nCons, nProd = 800, 200
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.Local)
	meta, lines, cps := buildBenchData(nCons, nProd, 1, start)
	mockRange := &mocks.MockBowRange{Entries: lines}
	mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()
	mockBow := &mocks.MockBowStorage{}
	mockBow.On("GetMeta", "cpmeta/0").Return(meta)
	mockBow.On("GetLineRange", "CP", "2026/08/01/", "2026/08/02/").Return(mockRange)

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	runner := NewEnergyRunner([]Sheet{
		&SummarySheet{name: "Summary", excel: f},
		&EnergySheet{name: "Energiedaten", excel: f},
	})
	began := time.Now()
	buf, err := runner.run(mockBow, f, start, start.AddDate(0, 0, 1), cps)
	elapsed := time.Since(began)
	assert.NoError(t, err)
	assert.Less(t, elapsed, 20*time.Second, "Export einer grossen Gemeinschaft dauert zu lange")

	out, err := excelize.OpenReader(bytes.NewReader(buf.Bytes()))
	assert.NoError(t, err)
	defer func() { _ = out.Close() }()
	assert.Equal(t, []string{"Summary", "Energiedaten"}, out.GetSheetList())

	colName := func(n int) string { c, _ := excelize.ColumnNumberToName(n); return c }
	for sheet, cols := range map[string]int{"Energiedaten": nCons*3 + nProd*2} {
		w, err := out.GetColWidth(sheet, colName(cols+1))
		assert.NoError(t, err)
		assert.Equal(t, 25.0, w, "%s: letzte Datenspalte hat die Datenbreite", sheet)
		w, err = out.GetColWidth(sheet, "A")
		assert.NoError(t, err)
		assert.Equal(t, 30.0, w, "%s: Spalte A", sheet)
	}
}
