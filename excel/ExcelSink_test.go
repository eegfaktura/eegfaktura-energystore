package excel

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xuri/excelize/v2"
	"testing"
	"time"
)

func buildMock(entries []*model.RawSourceLine) *mocks.MockBowStorage {
	testMetaData := &model.RawSourceMeta{Id: "meta", CounterPoints: []*model.CounterPointMeta{
		&model.CounterPointMeta{
			ID:          "001",
			Name:        "AT0030000000000000000000000351391",
			SourceIdx:   0,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       0,
			PeriodStart: "daf",
			PeriodEnd:   "dfsad",
		},
		&model.CounterPointMeta{
			ID:          "002",
			Name:        "AT0030000000000000000000000379812",
			SourceIdx:   1,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       0,
			PeriodStart: "daf",
			PeriodEnd:   "dfsad",
		},
		&model.CounterPointMeta{
			ID:          "003",
			Name:        "AT0030000000000000000000030043080",
			SourceIdx:   0,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       0,
			PeriodStart: "daf",
			PeriodEnd:   "dfsad",
		},
		&model.CounterPointMeta{
			ID:          "004",
			Name:        "AT0030000000000000000000000381701",
			SourceIdx:   1,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       0,
			PeriodStart: "daf",
			PeriodEnd:   "dfsad",
		},
	}, NumberOfMetering: 10}

	mockRange := &mocks.MockBowRange{Entries: entries}
	mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()

	mockBow := &mocks.MockBowStorage{}
	mockBow.On("GetMeta", "cpmeta/0").Return(testMetaData)
	mockBow.On("GetLineRange", "CP", "2023/01/01/", "2023/01/02/").Return(mockRange)

	return mockBow
}

func TestGenerateEnergyDataSheetV2(t *testing.T) {
	testMetaData := &model.RawSourceMeta{Id: "meta", CounterPoints: []*model.CounterPointMeta{
		&model.CounterPointMeta{
			ID:          "001",
			Name:        "AT0030000000000000000000000351391",
			SourceIdx:   0,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "02.01.2023 00:00:00",
		},
		&model.CounterPointMeta{
			ID:          "002",
			Name:        "AT0030000000000000000000000379812",
			SourceIdx:   1,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "02.01.2023 00:00:00",
		},
		&model.CounterPointMeta{
			ID:          "003",
			Name:        "AT0030000000000000000000030043080",
			SourceIdx:   0,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "02.01.2023 00:00:00",
		},
		&model.CounterPointMeta{
			ID:          "004",
			Name:        "AT0030000000000000000000000381701",
			SourceIdx:   1,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "02.01.2023 00:00:00",
		},
	}, NumberOfMetering: 10}

	cps := &ExportCPs{
		CommunityId: "ATSEPPHUBER",
		Cps: []InvestigatorCP{
			{
				MeteringPoint: "AT0030000000000000000000000351391",
				Direction:     "CONSUMPTION",
				Name:          "Stefan Maier",
			},
			{
				MeteringPoint: "AT0030000000000000000000000379812",
				Direction:     "CONSUMPTION",
				Name:          "Michael Schauer",
			},
			{
				MeteringPoint: "AT0030000000000000000000030043080",
				Direction:     "GENERATION",
				Name:          "Michael Schauer",
			},
			{
				MeteringPoint: "AT0030000000000000000000000381701",
				Direction:     "GENERATION",
				Name:          "Stefan Maier",
			},
		},
	}

	entries := []*model.RawSourceLine{
		&model.RawSourceLine{Id: "CP/2023/01/01/00/00/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/00/15/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/00/30/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/00/45/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/01/00/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/01/15/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/01/30/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{
			Id:           "CP/2023/01/01/01/45/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/02/00/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
	}

	var tests = []struct {
		name     string
		metaData *model.RawSourceMeta
		cps      *ExportCPs
		entries  []*model.RawSourceLine
		check    func(t *testing.T, f *excelize.File, l []model.RawSourceLine)
	}{
		{
			name:     "fillupMissingValues",
			metaData: testMetaData,
			cps:      cps,
			entries: append(entries, &model.RawSourceLine{Id: "CP/2023/01/02/00/00/00/",
				Consumers:    []float64{0, 0, 0, 0, 0, 0},
				Producers:    []float64{0, 0, 0, 0},
				QoVConsumers: []int{1, 1, 1, 1, 1, 1},
				QoVProducers: []int{1, 1, 1, 1},
			}),
			check: func(t *testing.T, f *excelize.File, l []model.RawSourceLine) {
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
			metaData: testMetaData,
			cps:      cps,
			entries:  entries,
			check: func(t *testing.T, f *excelize.File, l []model.RawSourceLine) {
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				assert.Equal(t, len(rows[1]), 11)
				assert.Equal(t, len(rows), 19)

				assert.Equal(t, "01.01.2023 00:00:00", rows[10][0])
				assert.Equal(t, "01.01.2023 01:30:00", rows[16][0])
				assert.Equal(t, "01.01.2023 01:45:00", rows[17][0])
				assert.Equal(t, "01.01.2023 02:00:00", rows[18][0])

				assert.Nil(t, l)
			},
		},
		{
			name:     "Bad Data - QoV differs to 1",
			metaData: testMetaData,
			cps:      cps,
			entries: append(entries, &model.RawSourceLine{Id: "CP/2023/01/02/00/00/00/",
				Consumers:    []float64{0, 0, 0, 0, 0, 0},
				Producers:    []float64{0, 0, 0, 0},
				QoVConsumers: []int{1, 2, 2, 1, 1, 1},
				QoVProducers: []int{1, 1, 1, 1},
			}),
			check: func(t *testing.T, f *excelize.File, l []model.RawSourceLine) {
				rows, err := f.GetRows("Energiedaten")
				assert.NoError(t, err)
				assert.Equal(t, len(rows[1]), 11)
				assert.Equal(t, len(rows), 107)

				assert.Equal(t, "01.01.2023 00:00:00", rows[10][0])
				assert.Equal(t, "01.01.2023 01:30:00", rows[16][0])
				assert.Equal(t, "01.01.2023 01:45:00", rows[17][0])
				assert.Equal(t, "01.01.2023 02:00:00", rows[18][0])

				assert.Equal(t, 1, len(l))

				//rows, err = f.GetRows("QoV Log")
				//assert.NoError(t, err)
				//assert.Equal(t, 10, len(rows))
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

			qovLog, err := generateEnergyDataSheetV2(mockBow,
				f,
				time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
				time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
				tt.cps.Cps)
			assert.NoError(t, err)
			tt.check(t, f, qovLog)
		})
	}
}

func Test_sumEnergyOfPeriod(t *testing.T) {
	entries := []*model.RawSourceLine{
		&model.RawSourceLine{Id: "CP/2023/01/01/00/00/00/",
			Consumers:    []float64{1, 1, 1, 2, 2, 2},
			Producers:    []float64{1, 1, 0.5, 0.5},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/00/15/00/",
			Consumers:    []float64{0.1, 0.5, 0.01, 0.2, 0.5, 0.1},
			Producers:    []float64{0.5, 0.8, 0.5, 1.2},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/00/30/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/00/45/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/01/00/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/01/15/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/01/30/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{
			Id:           "CP/2023/01/01/01/45/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
		&model.RawSourceLine{Id: "CP/2023/01/01/02/00/00/",
			Consumers:    []float64{0, 0, 0, 0, 0, 0},
			Producers:    []float64{0, 0, 0, 0},
			QoVConsumers: []int{1, 1, 1, 1, 1, 1},
			QoVProducers: []int{1, 1, 1, 1},
		},
	}

	type args struct {
		db    store.IBowStorage
		start time.Time
		end   time.Time
		info  *model.CounterPointMetaInfo
	}
	var tests = []struct {
		name    string
		args    args
		entries []*model.RawSourceLine
		check   func(*testing.T, *model.EnergyReport, []bool, []bool, error)
	}{
		{
			name: "fillupMissingValues",
			args: args{db: buildMock(entries),
				start: time.Date(2023, time.Month(1), 1, 0, 0, 0, 0, time.Local),
				end:   time.Date(2023, time.Month(1), 2, 0, 0, 0, 0, time.Local),
				info: &model.CounterPointMetaInfo{
					ConsumerCount:  2,
					ProducerCount:  2,
					MaxConsumerIdx: 2,
					MaxProducerIdx: 2,
				},
			},
			check: func(t *testing.T, report *model.EnergyReport, qovCons []bool, qovProd []bool, err error) {
				assert.NoError(t, err)
				fmt.Printf("Report %v\n", report)
				fmt.Printf("Report %v\n", qovCons)
				assert.Equal(t, report.Allocated, []float64{1.01, 2.1})
				assert.Equal(t, report.Consumed[0], 1.1)
				assert.Equal(t, report.Shared[0], 1.5)
				assert.Equal(t, report.Distributed, []float64{1.8, 1.7})
				assert.ElementsMatch(t, report.Produced, []float64{1.5, 1})
				assert.ElementsMatch(t, qovCons, []bool{true, true})
				assert.ElementsMatch(t, qovProd, []bool{true, true})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, qovcons, qovprod, err := sumEnergyOfPeriod(tt.args.db, tt.args.start, tt.args.end, tt.args.info)
			tt.check(t, report, qovcons, qovprod, err)
		})
	}
}
