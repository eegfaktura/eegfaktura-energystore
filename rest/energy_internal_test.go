package rest

import (
	"testing"

	protobuf "at.ourproject/energystore/protoc"
	"at.ourproject/energystore/store"
	"github.com/stretchr/testify/assert"
)

// The raw data export without a meter list built its targets with make(len) and append: the list
// started with one empty TargetMP per metering point of the community.
func TestTargetsOfMeters(t *testing.T) {
	meters := []*protobuf.MeteringPoint{{MeteringPointId: "AT001"}, {MeteringPointId: "AT002"}}
	assert.Equal(t, []store.TargetMP{{MeteringPoint: "AT001"}, {MeteringPoint: "AT002"}}, targetsOf(meters))
	assert.Empty(t, targetsOf(nil))
}
