package mqttclient

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stoppingExecutor finishes its first batch when release is closed; later batches wait for the end of
// ctx and report only their first message as finished.
type stoppingExecutor struct {
	release chan struct{}
	entered chan int // size of each batch at its start
	batches atomic.Int32
}

func (s *stoppingExecutor) Execute(msg mqtt.Message) {}
func (s *stoppingExecutor) Close()                   {}
func (s *stoppingExecutor) ExecuteBatch(ctx context.Context, msgs []mqtt.Message) []bool {
	if ctx.Err() != nil { // like the real ExecuteBatch: a batch started after the stop finishes nothing
		return make([]bool, len(msgs))
	}
	s.entered <- len(msgs)
	if s.batches.Add(1) == 1 {
		<-s.release
		return allTrue(len(msgs))
	}
	<-ctx.Done()
	finished := make([]bool, len(msgs))
	finished[0] = true
	return finished
}

// Close during a batch returns promptly; the batch's unfinished messages and the queued ones stay unacked.
func TestDispatcherCloseStopsRunningBatch(t *testing.T) {
	exec := &stoppingExecutor{release: make(chan struct{}), entered: make(chan int, 10)}
	d := newTopicDispatcher(context.Background(), dispatcherConfig{parallel: 1, batchMax: 3, maxOpen: 32, idleTimeout: time.Hour,
		newExecutor: func(string) Executor { return exec }})
	var msgs []*testsupport.Message
	for i := 0; i < 6; i++ {
		m := laneMsg("TE100001", i)
		msgs = append(msgs, m)
		d.Submit(m)
	}
	first := <-exec.entered
	close(exec.release)
	require.Equal(t, 3, <-exec.entered, "second batch")

	start := time.Now()
	d.Close()
	assert.Less(t, time.Since(start), 2*time.Second)

	want := make([]int, 6)
	for i := 0; i < first; i++ {
		want[i] = 1 // first batch finished
	}
	want[first] = 1 // the second batch finished only its first message
	got := make([]int, 6)
	for i, m := range msgs {
		got[i] = m.Acks()
	}
	assert.Equal(t, want, got, fmt.Sprintf("first batch had %d messages", first))
}

// countdownCtx ends after its Err was asked n times.
type countdownCtx struct {
	context.Context
	n atomic.Int32
}

func (c *countdownCtx) Err() error {
	if c.n.Add(-1) < 0 {
		return context.Canceled
	}
	return nil
}

// Stopped at any step, ExecuteBatch reports as finished exactly the messages it stored and replied; the
// others get no reply. Without a stop all are finished.
func TestExecuteBatchStopsBetweenSteps(t *testing.T) {
	const meters = 4
	partial := 0
	for n := int32(0); n <= 12; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			testsupport.UseTempPersistence(t)
			client := testsupport.NewFakeClient()
			client.Notify = nil
			imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
			defer imp.Close()
			var builders []*testsupport.CrMsgBuilder
			for i := 0; i < meters; i++ {
				day := ackDay.AddDate(0, 0, i%2)
				builders = append(builders, periodMsg(fmt.Sprintf("AT00300000000000000000000000000%02d", i), model.CONSUMER_DIRECTION,
					day, day.AddDate(0, 0, 1+i/2), func(int) float64 { return 1 }))
			}
			ctx := &countdownCtx{Context: context.Background()}
			ctx.n.Store(n)
			finished := imp.ExecuteBatch(ctx, batchOf(t, ackTenant, builders...))

			replied := map[string]bool{}
			for _, p := range client.PublishedMessages() {
				for i := 0; i < meters; i++ {
					if mp := fmt.Sprintf("AT00300000000000000000000000000%02d", i); strings.Contains(string(p.Payload), mp) {
						replied[mp] = true
					}
				}
			}
			for i, f := range finished {
				mp := fmt.Sprintf("AT00300000000000000000000000000%02d", i)
				assert.Equal(t, f, replied[mp], "message %d: finished iff replied", i)
			}
			if n == 0 {
				assert.Equal(t, make([]bool, meters), finished, "stopped before the start")
			}
			if n == 12 {
				assert.Equal(t, allTrue(meters), finished)
			}
			done := 0
			for _, f := range finished {
				if f {
					done++
				}
			}
			if done > 0 && done < meters {
				partial++
			}
		})
	}
	assert.Positive(t, partial, "some stops fall between the messages' days")
}

// One janitor pass closes at most parallel idle executors.
func TestCloseIdleAtMostParallelPerPass(t *testing.T) {
	rec := &laneRecorder{}
	d := newLaneDispatcher(t, 2, 50, time.Hour, rec)
	for n := 0; n < 8; n++ {
		m := laneMsg(fmt.Sprintf("TE10000%d", n), 0)
		d.Submit(m)
		awaitAcks(t, []*testsupport.Message{m})
	}
	d.closeIdle(time.Now().Add(2 * time.Hour)) // all idle for longer than idleTimeout
	require.Eventually(t, func() bool { return rec.snapshot().closed >= 2 }, 5*time.Second, time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 2, rec.snapshot().closed)
}
