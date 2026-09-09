package excel

import (
	"fmt"
	"testing"
	"time"

	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
)

// buildBenchData baut einen realistischen Monatsdatensatz: nCons Verbraucher,
// nProd Erzeuger, 15-Minuten-Raster ueber `days` Tage.
func buildBenchData(nCons, nProd, days int, start time.Time) (*model.RawSourceMeta, []*model.RawSourceLine, *ExportParticipantEnergy) {
	cps := make([]*model.CounterPointMeta, 0, nCons+nProd)
	part := make([]ParticipantCp, 0, nCons+nProd)

	for i := 0; i < nCons; i++ {
		name := fmt.Sprintf("AT00300000000000000000000000%05d", i)
		cps = append(cps, &model.CounterPointMeta{
			ID: fmt.Sprintf("c%03d", i), Name: name, SourceIdx: i,
			Dir: model.CONSUMER_DIRECTION, Count: 0,
			PeriodStart: "01.08.2026 00:00:00", PeriodEnd: "31.08.2026 23:45:00",
		})
		part = append(part, ParticipantCp{
			MeteringPoint: name, Direction: "CONSUMPTION", Name: fmt.Sprintf("Verbraucher %d", i),
			ActiveSince: start.UnixMilli(), InactiveSince: start.AddDate(1, 0, 0).UnixMilli(),
		})
	}
	for i := 0; i < nProd; i++ {
		name := fmt.Sprintf("AT00300000000000000000000009%05d", i)
		cps = append(cps, &model.CounterPointMeta{
			ID: fmt.Sprintf("p%03d", i), Name: name, SourceIdx: i,
			Dir: model.PRODUCER_DIRECTION, Count: 0,
			PeriodStart: "01.08.2026 00:00:00", PeriodEnd: "31.08.2026 23:45:00",
		})
		part = append(part, ParticipantCp{
			MeteringPoint: name, Direction: "GENERATION", Name: fmt.Sprintf("Erzeuger %d", i),
			ActiveSince: start.UnixMilli(), InactiveSince: start.AddDate(1, 0, 0).UnixMilli(),
		})
	}

	meta := &model.RawSourceMeta{Id: "meta", CounterPoints: cps, NumberOfMetering: nCons + nProd}

	slots := days * 96
	lines := make([]*model.RawSourceLine, 0, slots)
	for s := 0; s < slots; s++ {
		t := start.Add(time.Duration(s) * 15 * time.Minute)
		id := fmt.Sprintf("CP/%.4d/%.2d/%.2d/%.2d/%.2d/%.2d",
			t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
		l := model.MakeRawSourceLine(id, nCons*3, nProd*2)
		for i := range l.Consumers {
			l.Consumers[i] = 0.25 + float64(i%7)/100.0
		}
		for i := range l.Producers {
			l.Producers[i] = 1.5 + float64(i%5)/100.0
		}
		lines = append(lines, l)
	}

	return meta, lines, &ExportParticipantEnergy{
		Start: start.UnixMilli(), End: start.AddDate(0, 0, days).UnixMilli(),
		CommunityId: "ATBENCH0000000000000000000000001", Cps: part,
	}
}

func runExport(b *testing.B, nCons, nProd, days int) {
	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, days)
	meta, lines, cps := buildBenchData(nCons, nProd, days, start)

	from := fmt.Sprintf("%.4d/%.2d/%.2d/", start.Year(), int(start.Month()), start.Day())
	to := fmt.Sprintf("%.4d/%.2d/%.2d/", end.Year(), int(end.Month()), end.Day())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		mockRange := &mocks.MockBowRange{Entries: lines}
		mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()
		mockBow := &mocks.MockBowStorage{}
		mockBow.On("GetMeta", "cpmeta/0").Return(meta)
		mockBow.On("GetLineRange", "CP", from, to).Return(mockRange)
		f := excelize.NewFile()
		runner := NewEnergyRunner([]Sheet{
			&SummarySheet{name: "Summary", excel: f},
			&EnergySheet{name: "Energiedaten", excel: f},
		})
		b.StartTimer()

		buf, err := runner.run(mockBow, f, start, end, cps)
		if err != nil {
			b.Fatalf("run: %v", err)
		}

		b.StopTimer()
		if buf == nil || buf.Len() == 0 {
			b.Fatal("leeres Ergebnis")
		}
		b.ReportMetric(float64(buf.Len())/1024.0, "KiB/xlsx")
		_ = f.Close()
		b.StartTimer()
	}
}

func BenchmarkExport_S_20cons_5prod_31d(b *testing.B)   { runExport(b, 20, 5, 31) }
func BenchmarkExport_M_100cons_20prod_31d(b *testing.B) { runExport(b, 100, 20, 31) }
func BenchmarkExport_L_250cons_40prod_31d(b *testing.B) { runExport(b, 250, 40, 31) }
