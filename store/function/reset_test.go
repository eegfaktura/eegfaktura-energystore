package function

import (
	"at.ourproject/energystore/store/ebow"
	"testing"
)

func TestReset(t *testing.T) {
	type args struct {
		db        ebow.IBowStorage
		timeRange *DataTimeRange
		meter     string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Reset(tt.args.db, tt.args.timeRange, tt.args.meter); (err != nil) != tt.wantErr {
				t.Errorf("Reset() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
