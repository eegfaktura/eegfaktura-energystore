package services

import (
	"at.ourproject/energystore/mocks"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/mock"
	"testing"
)

func buildMock(entries []*model.RawSourceLine) *mocks.MockBowStorage {
	testMetaData := &model.RawSourceMeta{Id: "meta", CounterPoints: []*model.CounterPointMeta{
		&model.CounterPointMeta{
			ID:          "001",
			Name:        "AT0030000000000000000000000000001",
			SourceIdx:   0,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "01.02.2023 00:00:00",
		},
		&model.CounterPointMeta{
			ID:          "002",
			Name:        "AT0030000000000000000000000000002",
			SourceIdx:   1,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "01.02.2023 00:00:00",
		},
		&model.CounterPointMeta{
			ID:          "003",
			Name:        "AT0030000000000000000000000000003",
			SourceIdx:   0,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "01.02.2023 00:00:00",
		},
		&model.CounterPointMeta{
			ID:          "004",
			Name:        "AT0030000000000000000000000000004",
			SourceIdx:   1,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       0,
			PeriodStart: "01.01.2023 00:00:00",
			PeriodEnd:   "01.02.2023 00:00:00",
		},
	}, NumberOfMetering: 10}

	mockRange := &mocks.MockBowRange{Entries: entries}
	mockRange.On("Next", mock.AnythingOfType("*model.RawSourceLine")).Return()

	mockBow := &mocks.MockBowStorage{}
	mockBow.On("GetMeta", "cpmeta/0").Return(testMetaData)

	return mockBow
}

func TestGetLastEnergyEntry(t *testing.T) {
	type args struct {
		tenant string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetLastEnergyEntry(tt.args.tenant)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLastEnergyEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetLastEnergyEntry() got = %v, want %v", got, tt.want)
			}
		})
	}
}
