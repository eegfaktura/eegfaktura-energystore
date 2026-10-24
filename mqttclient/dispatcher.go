package mqttclient

import (
	"context"
	"sort"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

type TagValue struct {
	Topic string `json:"topic"`
	Value []byte `json:"value"`
}

type Executor interface {
	Execute(msg mqtt.Message)
	// ExecuteBatch processes messages of one tenant in their order (TopicDispatcher) and returns which of
	// them it finished; after ctx ends it may leave the rest unfinished (not acked, redelivered).
	ExecuteBatch(ctx context.Context, msgs []mqtt.Message) []bool
	Close()
}

type Worker struct {
}

func (w *Worker) Execute(msg mqtt.Message) {

}

type Subscriber struct {
	workerChan chan TagValue
	worker     *Worker
	receiver   mqtt.MessageHandler
}

func NewSubscriber(ctx context.Context, streamer *MQTTStreamer, topic string, worker Executor) *Subscriber {
	sub := &Subscriber{}
	sub.receiver = func(client mqtt.Client, msg mqtt.Message) {
		glog.V(5).Infof("Receive msg from topic %s - %v", msg.Topic(), string(msg.Payload()))
		defer msg.Ack() // auto-ack is off (#19)
		worker.Execute(msg)
	}
	//streamer.SubscribeTopic(ctx, topic, sub.receiver)
	streamer.AddRoutes(MqttRoutes{
		topic:    topic,
		callback: sub.receiver,
	})
	return sub
}

type Dispatcher struct {
	subscriber map[string]*Subscriber
	quitChan   chan struct{}
}

func NewDispatcher(ctx context.Context, streamer *MQTTStreamer, worker map[string]Executor) *Dispatcher {
	quitChan := make(chan struct{})
	disp := &Dispatcher{quitChan: quitChan}
	disp.subscriber = make(map[string]*Subscriber, len(worker))
	glog.Infof("Start Dispatcher with %d worker(s): %+v\n", len(worker), worker)

	for topic, worker := range worker {
		glog.Infof("Start Worker %s\n", topic)
		disp.subscriber[topic] = NewSubscriber(ctx, streamer, topic, worker)
	}
	return disp
}

func (d *Dispatcher) Stop() {
	close(d.quitChan)
}

type TopicValue struct {
	Topic string `json:"topic"`
	Value []byte `json:"value"`
}

type TenantSubscriber struct {
	receiver mqtt.MessageHandler
}

// Settings of the TopicDispatcher: import.parallel lanes import at the same time (env
// ENERGYSTORE_IMPORT_PARALLEL), each one batch of at most import.batchMax messages of one tenant (env
// ENERGYSTORE_IMPORT_BATCHMAX). Read once when the dispatcher starts.
const (
	parallelKey        = "import.parallel"
	batchMaxKey        = "import.batchMax"
	defaultParallel    = 4
	defaultBatchMax    = 50
	idleDBCloseTimeout = 15 * time.Second
	// minIdleClose: an idle executor beyond the newest parallel ones is closed only after this time, so a
	// tenant that delivers every few seconds does not reopen its store (~0.3 s to close, then open) each time.
	minIdleClose = 2 * time.Second
)

type dispatcherConfig struct {
	parallel    int
	batchMax    int
	idleTimeout time.Duration
	newExecutor func(tenant string) Executor
}

// TopicDispatcher distributes the messages of all tenants fairly over a fixed number of lanes:
//   - Submit (the MQTT callback) appends a message to its tenant's queue and returns; it never waits for
//     another tenant.
//   - A tenant with queued messages that is not running stands once in the FIFO list ready.
//   - A lane takes the first ready tenant and runs one batch of its queue (at most batchMax messages, no
//     waiting for more). If the tenant has more, it goes to the back of ready (round robin): a large
//     request occupies one lane for one batch at a time, and every other waiting tenant comes in between.
//   - A tenant never runs in two lanes at once: its order is kept and cpmeta/0 has one writer.
//
// Executors are created on demand and closed (with their stores) by the janitor after idleTimeout without
// work, or after minIdleClose when more than parallel of them are idle. A tenant waiting in ready keeps its
// store open.
type TopicDispatcher struct {
	cfg dispatcherConfig

	mu      sync.Mutex
	work    *sync.Cond // a tenant became ready, or the dispatcher stops
	tenants map[string]*tenantQueue
	ready   []*tenantQueue
	closed  bool

	ctx  context.Context // ends with Close; a running batch stops at its next step
	stop context.CancelFunc
	wg   sync.WaitGroup
}

// tenantQueue is guarded by TopicDispatcher.mu.
type tenantQueue struct {
	tenant   string
	queue    []mqtt.Message
	executor Executor // nil while closed
	busy     bool     // a lane runs a batch, or the executor is being closed
	isReady  bool     // stands in TopicDispatcher.ready
	lastUsed time.Time
}

func NewTopicDispatcher(ctx context.Context, topic string, streamer *MQTTStreamer) *TopicDispatcher {
	v := viper.GetViper()
	dispatcher := newTopicDispatcher(ctx, dispatcherConfig{
		parallel:    readPositiveInt(v, parallelKey, defaultParallel),
		batchMax:    readPositiveInt(v, batchMaxKey, defaultBatchMax),
		idleTimeout: idleDBCloseTimeout,
		newExecutor: func(tenant string) Executor { return NewTenantEnergyImporter(tenant, streamer) },
	})

	sub := &TenantSubscriber{}
	sub.receiver = func(client mqtt.Client, msg mqtt.Message) {
		glog.V(5).Infof("Receive msg from topic %s - %v", msg.Topic(), string(msg.Payload()))
		dispatcher.Submit(msg)
	}
	streamer.AddRoutes(MqttRoutes{
		topic:    topic,
		callback: sub.receiver,
	})
	return dispatcher
}

func newTopicDispatcher(ctx context.Context, cfg dispatcherConfig) *TopicDispatcher {
	ctx, cancel := context.WithCancel(ctx)
	d := &TopicDispatcher{cfg: cfg, tenants: map[string]*tenantQueue{}, ctx: ctx, stop: cancel}
	d.work = sync.NewCond(&d.mu)
	glog.Infof("Start TopicDispatcher with %d lanes, batches of at most %d messages", cfg.parallel, cfg.batchMax)

	// wg.Add before the goroutines start, so Close cannot return before they ran (known-errors #41).
	d.wg.Add(cfg.parallel + 2)
	for i := 0; i < cfg.parallel; i++ {
		go d.lane()
	}
	go d.janitor(ctx)
	go func() {
		defer d.wg.Done()
		<-ctx.Done()
		d.mu.Lock()
		d.closed = true
		d.mu.Unlock()
		d.work.Broadcast()
	}()
	return d
}

// readPositiveInt falls back to def for a missing, non-numeric or non-positive value.
func readPositiveInt(v *viper.Viper, key string, def int) int {
	if !v.IsSet(key) {
		return def
	}
	n := v.GetInt(key)
	if n <= 0 {
		glog.Warningf("%s=%q is not a positive number, using %d", key, v.GetString(key), def)
		return def
	}
	return n
}

// Submit queues the message for its tenant. After Close it is dropped unacked: mosquitto delivers it again
// when energystore reconnects.
func (d *TopicDispatcher) Submit(job mqtt.Message) {
	tenant := TopicType(job.Topic()).Tenant()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		glog.Warningf("dispatcher closed, message not processed tenant=%s", tenant)
		return
	}
	tq := d.tenants[tenant]
	if tq == nil {
		tq = &tenantQueue{tenant: tenant}
		d.tenants[tenant] = tq
	}
	tq.queue = append(tq.queue, job)
	d.markReady(tq)
}

// markReady puts a tenant with work at the back of ready, unless it is busy or stands there. Caller holds mu.
func (d *TopicDispatcher) markReady(tq *tenantQueue) {
	if tq.busy || tq.isReady || len(tq.queue) == 0 || d.closed {
		return
	}
	tq.isReady = true
	d.ready = append(d.ready, tq)
	d.work.Signal()
}

// Close stops taking messages, stops the running batches at their next step (message, run or day) and closes
// every executor. It returns when all of that is done. Unfinished and queued messages stay unacked.
func (d *TopicDispatcher) Close() {
	d.stop()
	d.wg.Wait()

	d.mu.Lock()
	var executors []Executor
	for _, tq := range d.tenants {
		if tq.executor != nil {
			executors = append(executors, tq.executor)
			tq.executor = nil
		}
	}
	d.mu.Unlock()
	// In parallel: closing a store takes a few hundred ms, and up to all tenants' stores may be open.
	var wg sync.WaitGroup
	wg.Add(len(executors))
	for _, e := range executors {
		go func() {
			defer wg.Done()
			e.Close()
		}()
	}
	wg.Wait()
}

func (d *TopicDispatcher) lane() {
	defer d.wg.Done()
	for {
		d.mu.Lock()
		for len(d.ready) == 0 && !d.closed {
			d.work.Wait()
		}
		if d.closed {
			d.mu.Unlock()
			return
		}
		tq := d.ready[0]
		d.ready[0] = nil
		d.ready = d.ready[1:]
		tq.isReady = false
		n := min(len(tq.queue), d.cfg.batchMax)
		batch := append([]mqtt.Message(nil), tq.queue[:n]...)
		clear(tq.queue[:n])
		tq.queue = tq.queue[n:]
		tq.busy = true
		if tq.executor == nil {
			tq.executor = d.cfg.newExecutor(tq.tenant)
		}
		executor := tq.executor
		d.mu.Unlock()

		executeBatch(d.ctx, tq.tenant, executor, batch)

		d.mu.Lock()
		tq.busy = false
		tq.lastUsed = time.Now()
		d.markReady(tq)
		d.mu.Unlock()
	}
}

// janitor closes executors that became idle.
func (d *TopicDispatcher) janitor(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(min(time.Second, d.cfg.idleTimeout/2))
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			d.closeIdle(now)
		case <-ctx.Done():
			return
		}
	}
}

// closeIdle closes the executors idle for idleTimeout, and the idle ones beyond the newest parallel after
// minIdleClose, each in its
// own goroutine (closing a store takes a few hundred ms; the lanes go on meanwhile). The tenant is busy while
// its executor closes, so no lane runs it meanwhile.
func (d *TopicDispatcher) closeIdle(now time.Time) {
	d.mu.Lock()
	var idle []*tenantQueue
	for _, tq := range d.tenants {
		if tq.executor != nil && !tq.busy && !tq.isReady {
			idle = append(idle, tq)
		}
	}
	sort.Slice(idle, func(i, j int) bool { return idle[i].lastUsed.After(idle[j].lastUsed) })
	var closing []*tenantQueue
	for i, tq := range idle {
		idleFor := now.Sub(tq.lastUsed)
		if (i >= d.cfg.parallel && idleFor >= minIdleClose) || idleFor >= d.cfg.idleTimeout {
			tq.busy = true
			closing = append(closing, tq)
		}
	}
	d.mu.Unlock()

	// The callers (lanes, janitor) still hold their own count, so Close's Wait cannot have returned.
	d.wg.Add(len(closing))
	for _, tq := range closing {
		go d.closeExecutor(tq, now.Sub(tq.lastUsed))
	}
}

func (d *TopicDispatcher) closeExecutor(tq *tenantQueue, idle time.Duration) {
	defer d.wg.Done()
	glog.Infof("No message received for %s. Close DB tenant=%s.", idle.Round(time.Second), tq.tenant)
	tq.executor.Close()

	d.mu.Lock()
	defer d.mu.Unlock()
	tq.executor = nil
	tq.busy = false
	if len(tq.queue) == 0 {
		delete(d.tenants, tq.tenant)
	} else {
		d.markReady(tq)
	}
}

// executeBatch fuehrt ein Buendel von Nachrichten eines Mandanten aus. Eine Panic wird protokolliert
// und beendet nicht den ganzen Dienst: die Spur laeuft ohne recover in einer eigenen Goroutine, und eine
// weiterhin zugestellte Nachricht fuehrte nach dem Neustart wieder zum Absturz. Nach einer Panic im Buendel
// laufen dessen Nachrichten einzeln, jede mit eigenem recover: nur die Nachricht, die wieder abstuerzt,
// geht verloren; die anderen werden gespeichert und bestaetigt (Import ist idempotent).
//
// Ack (#19): jede Nachricht des Buendels genau einmal, erst wenn sie verarbeitet ist, bei Erfolg wie bei
// Fehler und auch nach einer Panic (sonst stellte der Broker die Nachricht nach jedem Neustart wieder zu).
// Nicht geackt bleibt nur, was bei Absturz, OOM oder Shutdown noch nicht fertig war (beim Shutdown bricht
// ExecuteBatch zwischen Nachrichten, Laeufen und Tagen ab); das stellt mosquitto beim naechsten Verbinden
// erneut zu.
func executeBatch(ctx context.Context, tenant string, executor Executor, batch []mqtt.Message) {
	finished, ok := runRecovered(ctx, tenant, executor, batch)
	switch {
	case !ok && len(batch) == 1:
		finished = []bool{true} // the message that panics is acked
	case !ok:
		finished = make([]bool, len(batch))
		for i := range batch {
			if ctx.Err() != nil {
				break
			}
			f, ok := runRecovered(ctx, tenant, executor, batch[i:i+1])
			finished[i] = !ok || f[0]
		}
	}
	for i, msg := range batch {
		if finished[i] {
			msg.Ack()
		}
	}
}

// runRecovered runs ExecuteBatch and reports false after a recovered panic.
func runRecovered(ctx context.Context, tenant string, executor Executor, batch []mqtt.Message) (finished []bool, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("panic while executing message for tenant=%s: %v (%d messages)", tenant, r, len(batch))
			ok = false
		}
	}()
	return executor.ExecuteBatch(ctx, batch), true
}
