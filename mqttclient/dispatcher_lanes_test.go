package mqttclient

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// laneRecorder records what the fake executors of a dispatcher do.
type laneRecorder struct {
	sleep      time.Duration // per batch
	panicEvery int           // every n-th batch of an executor panics (0: never)

	mu        sync.Mutex
	starts    []string            // tenant per batch start, in order
	batches   map[string][][]byte // payloads per tenant, in batch order
	sizes     []int
	active    int
	maxActive int
	created   int
	closed    int
	misuse    []string // a tenant in two lanes at once, or closed while running
}

func (r *laneRecorder) mark() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.starts)
}

func (r *laneRecorder) snapshot() laneRecorder {
	r.mu.Lock()
	defer r.mu.Unlock()
	return laneRecorder{starts: append([]string{}, r.starts...), sizes: append([]int{}, r.sizes...),
		maxActive: r.maxActive, created: r.created, closed: r.closed, misuse: append([]string{}, r.misuse...)}
}

type fakeExecutor struct {
	tenant  string
	rec     *laneRecorder
	running atomic.Int32
	n       int // batches, only touched by the lane running this executor
}

func (f *fakeExecutor) Execute(msg mqtt.Message) { f.ExecuteBatch([]mqtt.Message{msg}) }

func (f *fakeExecutor) ExecuteBatch(msgs []mqtt.Message) {
	r := f.rec
	r.mu.Lock()
	if f.running.Add(1) > 1 {
		r.misuse = append(r.misuse, "two lanes "+f.tenant)
	}
	r.starts = append(r.starts, f.tenant)
	r.sizes = append(r.sizes, len(msgs))
	for _, m := range msgs {
		r.batches[f.tenant] = append(r.batches[f.tenant], m.Payload())
	}
	r.active++
	r.maxActive = max(r.maxActive, r.active)
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.active--
		r.mu.Unlock()
		f.running.Add(-1)
	}()

	time.Sleep(r.sleep)
	f.n++
	if r.panicEvery > 0 && f.n%r.panicEvery == 0 {
		panic("boom")
	}
}

func (f *fakeExecutor) Close() {
	f.rec.mu.Lock()
	defer f.rec.mu.Unlock()
	if f.running.Load() > 0 {
		f.rec.misuse = append(f.rec.misuse, "closed while running "+f.tenant)
	}
	f.rec.closed++
}

func newLaneDispatcher(t *testing.T, parallel, batchMax int, idle time.Duration, rec *laneRecorder) *TopicDispatcher {
	t.Helper()
	rec.batches = map[string][][]byte{}
	d := newTopicDispatcher(context.Background(), dispatcherConfig{parallel: parallel, batchMax: batchMax, idleTimeout: idle,
		newExecutor: func(tenant string) Executor {
			rec.mu.Lock()
			rec.created++
			rec.mu.Unlock()
			return &fakeExecutor{tenant: tenant, rec: rec}
		}})
	t.Cleanup(d.Close)
	return d
}

func laneMsg(tenant string, i int) *testsupport.Message {
	return testsupport.CrMsgMessage(tenant, []byte(fmt.Sprint(i)))
}

func awaitAcks(t *testing.T, msgs []*testsupport.Message) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, m := range msgs {
			if m.Acks() == 0 {
				return false
			}
		}
		return true
	}, 30*time.Second, 5*time.Millisecond)
}

// Tenant A has 1,000 queued messages; a message of tenant B waits for at most one batch of A, even with one
// lane.
func TestDispatcherFairness(t *testing.T) {
	rec := &laneRecorder{sleep: 5 * time.Millisecond}
	d := newLaneDispatcher(t, 1, 50, time.Hour, rec)
	var msgs []*testsupport.Message
	for i := 0; i < 1000; i++ {
		m := laneMsg("TE100001", i)
		msgs = append(msgs, m)
		d.Submit(m)
	}
	require.Eventually(t, func() bool { return rec.mark() >= 3 }, 10*time.Second, time.Millisecond)
	from := rec.mark()
	b := laneMsg("TE100002", 0)
	d.Submit(b)
	awaitAcks(t, append(msgs, b))

	s := rec.snapshot()
	aBefore, aAfter, seenB := 0, 0, false
	for _, tenant := range s.starts[from:] {
		switch {
		case tenant == "TE100002":
			seenB = true
		case seenB:
			aAfter++
		default:
			aBefore++
		}
	}
	require.True(t, seenB)
	assert.LessOrEqual(t, aBefore, 1, "batches of A that started after B arrived and before B")
	assert.GreaterOrEqual(t, aAfter, 5, "B arrived while A still had batches to go")
}

// At most parallel batches run at the same time, and a tenant never in two lanes.
func TestDispatcherAtMostParallelBatches(t *testing.T) {
	rec := &laneRecorder{sleep: 5 * time.Millisecond}
	d := newLaneDispatcher(t, 3, 2, time.Hour, rec)
	var msgs []*testsupport.Message
	for i := 0; i < 10; i++ {
		for n := 0; n < 20; n++ {
			m := laneMsg(fmt.Sprintf("TE1000%02d", n), i)
			msgs = append(msgs, m)
			d.Submit(m)
		}
	}
	awaitAcks(t, msgs)

	s := rec.snapshot()
	assert.LessOrEqual(t, s.maxActive, 3)
	assert.GreaterOrEqual(t, s.maxActive, 2, "the lanes run in parallel")
	assert.Empty(t, s.misuse)
}

// A batch has at most batchMax messages, and a tenant's messages keep their order across batches.
func TestDispatcherBatchSizeAndOrder(t *testing.T) {
	rec := &laneRecorder{sleep: time.Millisecond}
	d := newLaneDispatcher(t, 4, 7, time.Hour, rec)
	tenants := []string{"TE100001", "TE100002"}
	var msgs []*testsupport.Message
	for i := 0; i < 100; i++ {
		for _, tenant := range tenants {
			m := laneMsg(tenant, i)
			msgs = append(msgs, m)
			d.Submit(m)
		}
	}
	awaitAcks(t, msgs)

	s := rec.snapshot()
	for _, n := range s.sizes {
		assert.LessOrEqual(t, n, 7)
	}
	assert.Contains(t, s.sizes, 7, "queued messages are batched")
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, tenant := range tenants {
		require.Len(t, rec.batches[tenant], 100)
		for i, p := range rec.batches[tenant] {
			assert.Equal(t, fmt.Sprint(i), string(p), "%s position %d", tenant, i)
		}
	}
	assert.Empty(t, rec.misuse)
}

// Every message is acked exactly once, also when its batch panics.
func TestDispatcherAcksOnceAlsoAfterPanic(t *testing.T) {
	rec := &laneRecorder{sleep: time.Millisecond, panicEvery: 2}
	d := newLaneDispatcher(t, 2, 5, time.Hour, rec)
	var msgs []*testsupport.Message
	for i := 0; i < 40; i++ {
		for n := 0; n < 5; n++ {
			m := laneMsg(fmt.Sprintf("TE10000%d", n), i)
			msgs = append(msgs, m)
			d.Submit(m)
		}
	}
	awaitAcks(t, msgs)
	d.Close()

	for i, m := range msgs {
		assert.Equal(t, 1, m.Acks(), "message %d", i)
	}
	s := rec.snapshot()
	assert.Greater(t, len(s.starts), 10, "batches ran, half of them panicked")
}

// An executor is closed after the idle time and created again for the next message; at most parallel idle
// executors stay open. None is closed while its tenant runs.
func TestDispatcherClosesIdleExecutors(t *testing.T) {
	rec := &laneRecorder{}
	d := newLaneDispatcher(t, 2, 50, 50*time.Millisecond, rec)
	first := laneMsg("TE100001", 0)
	d.Submit(first)
	awaitAcks(t, []*testsupport.Message{first})
	require.Eventually(t, func() bool { return rec.snapshot().closed == 1 }, 5*time.Second, 5*time.Millisecond)
	second := laneMsg("TE100001", 1)
	d.Submit(second)
	awaitAcks(t, []*testsupport.Message{second})
	assert.Equal(t, 2, rec.snapshot().created)

	rec2 := &laneRecorder{}
	d2 := newLaneDispatcher(t, 2, 50, time.Hour, rec2)
	for n := 0; n < 6; n++ {
		m := laneMsg(fmt.Sprintf("TE10000%d", n), 0)
		d2.Submit(m)
		awaitAcks(t, []*testsupport.Message{m})
	}
	require.Eventually(t, func() bool { s := rec2.snapshot(); return s.created-s.closed <= 2 }, 5*time.Second, 5*time.Millisecond)
	assert.Empty(t, rec.snapshot().misuse)
	assert.Empty(t, rec2.snapshot().misuse)
}

// After Close, a message is neither processed nor acked (mosquitto redelivers it).
func TestDispatcherDropsAfterClose(t *testing.T) {
	rec := &laneRecorder{}
	d := newLaneDispatcher(t, 1, 50, time.Hour, rec)
	d.Close()
	m := laneMsg("TE100001", 0)
	d.Submit(m)
	assert.Zero(t, m.Acks())
	assert.Zero(t, rec.snapshot().created)
}
