package mqttclient

import (
	"testing"

	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
)

func dailyMsgs(meter string, days ...int) []*model.MqttEnergyMessage {
	one := func(int) float64 { return 1 }
	var msgs []*model.MqttEnergyMessage
	for _, d := range days {
		msgs = append(msgs, periodMsg(meter, model.CONSUMER_DIRECTION, vienna(2026, 10, d), vienna(2026, 10, d+1), one).Message())
	}
	return msgs
}

func indices(n int) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	return idx
}

// A CR_MSG's End is exclusive (the last value's "to"): a daily backlog of one metering point is one run.
func TestSplitRunsBackToBackDaysShareARun(t *testing.T) {
	msgs := dailyMsgs(ackMeter, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	assert.Equal(t, [][]int{indices(10)}, splitRuns(msgs, indices(10)))
}

// The same day twice, or a day inside a longer period, starts a new run.
func TestSplitRunsOverlapStartsNewRun(t *testing.T) {
	msgs := dailyMsgs(ackMeter, 1, 2, 1)
	assert.Equal(t, [][]int{{0, 1}, {2}}, splitRuns(msgs, indices(3)))

	msgs = append(dailyMsgs(ackMeter, 5), dailyMsgs("AT0030000000000000000000000000099", 5)...)
	msgs = append(msgs, periodMsg(ackMeter, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 10, 8),
		func(int) float64 { return 1 }).Message())
	assert.Equal(t, [][]int{{0, 1}, {2}}, splitRuns(msgs, indices(3)), "other metering points do not split")
}
