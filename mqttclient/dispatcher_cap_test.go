package mqttclient

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openCounter counts the executors that exist (created, Close not yet returned), as the cap does.
type openCounter struct {
	mu      sync.Mutex
	cur     int
	max     int
	created int
}

func (c *openCounter) add(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cur += n
	if n > 0 {
		c.created++
	}
	c.max = max(c.max, c.cur)
}

func (c *openCounter) get() (cur, maxOpen, created int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur, c.max, c.created
}

type countedExecutor struct {
	Executor
	c *openCounter
}

func (e countedExecutor) Close() {
	e.Executor.Close()
	e.c.add(-1)
}

func newCapDispatcher(t *testing.T, parallel, maxOpen int, c *openCounter, newExecutor func(string) Executor) *TopicDispatcher {
	t.Helper()
	d := newTopicDispatcher(context.Background(), dispatcherConfig{parallel: parallel, batchMax: 50, maxOpen: maxOpen,
		idleTimeout: time.Hour, newExecutor: func(tenant string) Executor {
			c.add(1)
			return countedExecutor{newExecutor(tenant), c}
		}})
	t.Cleanup(d.Close)
	return d
}

// Ten tenants deliver; at most maxOpen=3 executors exist at any time, and every message is stored,
// replied and acked once.
func TestDispatcherCapsOpenStores(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	c := &openCounter{}
	d := newCapDispatcher(t, 2, 3, c, func(tenant string) Executor {
		return NewTenantEnergyImporter(tenant, &MQTTStreamer{client: client})
	})
	meter := func(i int) string { return fmt.Sprintf("AT00300000000000000000000000000%02d", i) }
	var msgs []*testsupport.Message
	for i := 0; i < 5; i++ {
		for n := 0; n < 10; n++ {
			m := testsupport.CrMsgMessage(fmt.Sprintf("TE10000%d", n), ackCrMsg(ackEcId, meter(i)).Payload(t))
			msgs = append(msgs, m)
			d.Submit(m)
		}
	}
	awaitAcks(t, msgs)
	d.Close()

	_, maxOpen, created := c.get()
	assert.LessOrEqual(t, maxOpen, 3)
	assert.Greater(t, created, 3, "stores were closed to open others")
	assert.Len(t, client.PublishedMessages(), 50)
	for _, m := range msgs {
		assert.Equal(t, 1, m.Acks())
	}
	for n := 0; n < 10; n++ {
		for i := 0; i < 5; i++ {
			assert.Len(t, ackStored(t, fmt.Sprintf("TE10000%d", n), ackEcId, meter(i)), 96, "tenant %d meter %d", n, i)
		}
	}
}

// heldExecutor runs until release is closed (all finished) or ctx ends (none finished).
type heldExecutor struct {
	release <-chan struct{}
	started chan<- string
	tenant  string
}

func (h heldExecutor) Execute(mqtt.Message) {}
func (h heldExecutor) Close()               {}
func (h heldExecutor) ExecuteBatch(ctx context.Context, msgs []mqtt.Message) []bool {
	h.started <- h.tenant
	select {
	case <-h.release:
		return allTrue(len(msgs))
	case <-ctx.Done():
		return make([]bool, len(msgs)) // stopped: nothing finished, as the importer
	}
}

// With every open store busy, a free lane waits instead of opening one more (three lanes, maxOpen 2: below
// parallel only here, to have a free lane); once a store is idle, it is closed and the waiting tenant runs.
func TestDispatcherWaitsAtCapWhileAllBusy(t *testing.T) {
	release := map[string]chan struct{}{"TE100001": make(chan struct{}), "TE100002": make(chan struct{}), "TE100003": make(chan struct{})}
	started := make(chan string, 10)
	c := &openCounter{}
	d := newCapDispatcher(t, 3, 2, c, func(tenant string) Executor {
		return heldExecutor{release: release[tenant], started: started, tenant: tenant}
	})
	for _, tenant := range []string{"TE100001", "TE100002"} {
		d.Submit(laneMsg(tenant, 0))
		<-started
	}
	third := laneMsg("TE100003", 0)
	d.Submit(third)
	select {
	case tenant := <-started:
		t.Fatalf("%s started beyond the cap", tenant)
	case <-time.After(200 * time.Millisecond):
	}
	_, _, created := c.get()
	assert.Equal(t, 2, created)

	close(release["TE100001"])
	require.Equal(t, "TE100003", <-started)
	close(release["TE100003"])
	close(release["TE100002"])
	awaitAcks(t, []*testsupport.Message{third})
	_, maxOpen, _ := c.get()
	assert.LessOrEqual(t, maxOpen, 2)
}

// Close while a lane waits at the cap returns promptly; the waiting message stays unacked.
func TestDispatcherCloseWhileWaitingAtCap(t *testing.T) {
	started := make(chan string, 10)
	never := make(chan struct{})
	c := &openCounter{}
	d := newCapDispatcher(t, 3, 2, c, func(tenant string) Executor {
		return heldExecutor{release: never, started: started, tenant: tenant}
	})
	for _, tenant := range []string{"TE100001", "TE100002"} {
		d.Submit(laneMsg(tenant, 0))
		<-started
	}
	third := laneMsg("TE100003", 0)
	d.Submit(third)
	time.Sleep(50 * time.Millisecond) // the third lane is waiting at the cap

	closed := make(chan struct{})
	go func() {
		d.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close hangs while a lane waits at the cap")
	}
	assert.Zero(t, third.Acks())
	cur, maxOpen, _ := c.get()
	assert.LessOrEqual(t, maxOpen, 2)
	assert.Zero(t, cur, "every executor closed")
}

func TestReadMaxOpen(t *testing.T) {
	for _, tc := range []struct {
		set      any
		parallel int
		want     int
	}{{nil, 4, 32}, {"10", 4, 10}, {"4", 4, 4}, {"3", 4, 32}, {"x", 4, 32}, {"0", 4, 32}, {"8", 40, 40}} {
		v := viper.New()
		if tc.set != nil {
			v.Set(maxOpenKey, tc.set)
		}
		assert.Equal(t, tc.want, readMaxOpen(v, tc.parallel), "%v parallel %d", tc.set, tc.parallel)
	}
}
