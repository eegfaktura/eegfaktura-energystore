package excel

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/mock"
)

func buildExportMock(entries []*model.RawSourceLine) *mocks.MockBowStorage {
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

var (
	exportTestMetaData = &model.RawSourceMeta{Id: "meta", CounterPoints: []*model.CounterPointMeta{
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

	exportCps = &ExportCPs{
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

	exportEntries = []*model.RawSourceLine{
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
)
