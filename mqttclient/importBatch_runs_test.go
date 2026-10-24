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

// runIdx lists the message indices per run; a single run (stored with Import) is marked with -1 first.
func runIdx(runs []run) [][]int {
	var out [][]int
	for _, r := range runs {
		if r.single {
			out = append(out, append([]int{-1}, r.idx...))
		} else {
			out = append(out, r.idx)
		}
	}
	return out
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
	assert.Equal(t, [][]int{indices(10)}, runIdx(splitRuns(msgs, indices(10))))
}

// The same day twice, or a day inside a longer period, starts a new run.
func TestSplitRunsOverlapStartsNewRun(t *testing.T) {
	msgs := dailyMsgs(ackMeter, 1, 2, 1)
	assert.Equal(t, [][]int{{0, 1}, {2}}, runIdx(splitRuns(msgs, indices(3))))

	msgs = append(dailyMsgs(ackMeter, 5), dailyMsgs("AT0030000000000000000000000000099", 5)...)
	msgs = append(msgs, periodMsg(ackMeter, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 10, 8),
		func(int) float64 { return 1 }).Message())
	assert.Equal(t, [][]int{{0, 1}, {2}}, runIdx(splitRuns(msgs, indices(3))), "other metering points do not split")
}

// A message whose own entries overlap is a run of its own, stored with Import.
func TestSplitRunsSelfOverlappingMessageAlone(t *testing.T) {
	msgs := dailyMsgs(ackMeter, 1, 2)
	msgs = append(msgs[:1], selfOverlappingMsg(ackMeter), msgs[1])
	assert.Equal(t, [][]int{{0}, {-1, 1}, {2}}, runIdx(splitRuns(msgs, indices(3))))
}

// selfOverlappingMsg has two entries of the same meter code: the 27.10 with 0.5, then October with small values.
func selfOverlappingMsg(meter string) *model.MqttEnergyMessage {
	day := periodMsg(meter, model.CONSUMER_DIRECTION, vienna(2026, 10, 27), vienna(2026, 10, 28), func(int) float64 { return 0.5 }).Message()
	month := periodMsg(meter, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 11, 1),
		func(i int) float64 { return float64(i%96+1) / 1000 }).Message()
	day.Energy = append(day.Energy, month.Energy...)
	return day
}
