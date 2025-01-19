package store

import (
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_updateMetaCP(t *testing.T) {
	type args struct {
		metaCP *model.CounterPointMeta
		begin  time.Time
		end    time.Time
	}
	tests := []struct {
		name string
		args args
		test func(t *testing.T, args args)
	}{
		{
			name: "Adjust meta period end time",
			args: args{
				metaCP: &model.CounterPointMeta{
					ID:          "000",
					Name:        "IV0000999222222222221",
					SourceIdx:   0,
					Dir:         "CONSUMPTION",
					Count:       0,
					PeriodStart: "30.12.2023 15:00:0000",
					PeriodEnd:   "30.12.2023 15:00:0000",
				},
				begin: time.Date(2023, 12, 30, 15, 1, 0, 0, time.Local),
				end:   time.Date(2023, 12, 30, 15, 15, 0, 0, time.Local),
			},
			test: func(t *testing.T, args args) {
				result := updateMetaCP(args.metaCP, args.begin, args.end)
				assert.Equalf(t, true, result, "updateMetaCP(%v, %v, %v)", args.metaCP, args.begin, args.end)
				assert.Equal(t, "30.12.2023 15:00:0000", args.metaCP.PeriodStart)
				assert.Equal(t, "30.12.2023 15:15:0000", args.metaCP.PeriodEnd)
			},
		},
		{
			name: "Adjust meta period start time",
			args: args{
				metaCP: &model.CounterPointMeta{
					ID:          "000",
					Name:        "IV0000999222222222221",
					SourceIdx:   0,
					Dir:         "CONSUMPTION",
					Count:       0,
					PeriodStart: "30.12.2023 15:00:0000",
					PeriodEnd:   "30.12.2023 15:15:0000",
				},
				begin: time.Date(2023, 12, 30, 14, 0, 0, 0, time.Local),
				end:   time.Date(2023, 12, 30, 15, 15, 0, 0, time.Local),
			},
			test: func(t *testing.T, args args) {
				result := updateMetaCP(args.metaCP, args.begin, args.end)
				assert.Equalf(t, true, result, "updateMetaCP(%v, %v, %v)", args.metaCP, args.begin, args.end)
				assert.Equal(t, "30.12.2023 14:00:0000", args.metaCP.PeriodStart)
				assert.Equal(t, "30.12.2023 15:15:0000", args.metaCP.PeriodEnd)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t, tt.args)
		})
	}
}
