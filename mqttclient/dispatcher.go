package mqttclient

import (
	"context"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
)

type TagValue struct {
	Topic string `json:"topic"`
	Value []byte `json:"value"`
}

type Executor interface {
	Execute(msg mqtt.Message)
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

type TenantWorker struct {
	tenant     string
	JobChannel chan mqtt.Message
	executor   Executor
	ctx        context.Context
	wg         *sync.WaitGroup
}

type TenantSubscriber struct {
	receiver mqtt.MessageHandler
}

type TopicDispatcher struct {
	Inbound  chan mqtt.Message
	Finished chan bool
	//workers  map[string]*TenantWorker
	workers    sync.Map
	ctx        context.Context
	quit       chan struct{}
	stopWorker chan string
	wg         sync.WaitGroup
	stop       context.CancelFunc
	mu         *sync.RWMutex
}

func NewTopicDispatcher(ctx context.Context, topic string, streamer *MQTTStreamer) *TopicDispatcher {
	ctx, cancel := context.WithCancel(ctx)

	dispatcher := &TopicDispatcher{
		Inbound:  make(chan mqtt.Message),
		Finished: make(chan bool),
		//workers:  make(map[string]*TenantWorker),
		workers:    sync.Map{},
		ctx:        ctx,
		quit:       make(chan struct{}),
		stopWorker: make(chan string),
		//wg:       sync.WaitGroup,
		stop: cancel,
		mu:   &sync.RWMutex{},
	}

	sub := &TenantSubscriber{}
	sub.receiver = func(client mqtt.Client, msg mqtt.Message) {
		glog.V(5).Infof("Receive msg from topic %s - %v", msg.Topic(), string(msg.Payload()))
		dispatcher.Submit(msg)
	}
	streamer.AddRoutes(MqttRoutes{
		topic:    topic,
		callback: sub.receiver,
	})

	go dispatcher.process()
	return dispatcher
}

func (d *TopicDispatcher) process() {
	for {
		select {
		case job := <-d.Inbound: // listen to a submitted job on WorkChannel
			tenant := TopicType(job.Topic()).Tenant()
			jobChan := d.getWorker(tenant) // pull out an available jobchannel from queue
			jobChan <- job                 // submit the job on the available jobchannel
		case <-d.quit:
			return
		case workerId := <-d.stopWorker:
			d.putWorker(workerId)
		}
	}
}

func (d *TopicDispatcher) Submit(job mqtt.Message) {
	d.Inbound <- job
}

func (d *TopicDispatcher) Close() {
	close(d.quit)
	d.stop()
	d.wg.Wait()
}

func (d *TopicDispatcher) getWorker(tenant string) chan mqtt.Message {
	//d.mu.RLock()
	//defer d.mu.RUnlock()

	//worker, ok := d.workers[tenant]
	worker, ok := d.workers.Load(tenant)
	if !ok {
		worker = &TenantWorker{
			tenant:     tenant,
			JobChannel: make(chan mqtt.Message, 10),
			executor:   NewTenantEnergyImporter(tenant),
			ctx:        d.ctx,
			wg:         &d.wg,
		}
		go worker.(*TenantWorker).Run()
		//d.workers[tenant] = worker
		d.workers.Store(tenant, worker)
	}
	return worker.(*TenantWorker).JobChannel
}

func (d *TopicDispatcher) putWorker(workerId string) {
	//d.mu.Lock()
	//defer d.mu.Unlock()

	//if w, ok := d.workers[workerId]; ok {
	if w, ok := d.workers.Load(workerId); ok {
		w.(*TenantWorker).executor.Close()
		//delete(d.workers, workerId)
		d.workers.Delete(workerId)
		glog.Infof("Release worker %s.", workerId)
	}
}

func (worker *TenantWorker) Run() {
	worker.wg.Add(1)
	defer worker.wg.Done()

	timer := time.NewTimer(1 * time.Minute) // 15 seconds timeout
	defer timer.Stop()

	for {
		select {
		case job := <-worker.JobChannel:
			worker.executor.Execute(job)
			timer.Reset(1 * time.Minute)
		case <-timer.C:
			glog.Infof("No message received for 1 minute. Close DB tenant=%s.", worker.tenant)
			worker.executor.Close()
			glog.V(4).Infof("DB closed after 1 minute. tenant=%s.", worker.tenant)
		case <-worker.ctx.Done():
			glog.Infof("Stop Worker tenant=%s", worker.tenant)
			worker.executor.Close()
			glog.Infof("Stopped Worker tenant=%s", worker.tenant)
			return
		}
	}
}
