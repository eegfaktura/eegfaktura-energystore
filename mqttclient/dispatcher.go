package mqttclient

import (
	"at.ourproject/energystore/calculation"
	"context"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
	log "github.com/sirupsen/logrus"
	"sync"
)

type TagValue struct {
	Topic string `json:"topic"`
	Value []byte `json:"value"`
}

type Executor interface {
	Execute(msg mqtt.Message)
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
	workers  map[string]*TenantWorker
	ctx      context.Context
	quit     chan struct{}
	wg       sync.WaitGroup
	stop     context.CancelFunc
}

func NewTopicDispatcher(ctx context.Context, topic string, streamer *MQTTStreamer) *TopicDispatcher {
	ctx, cancel := context.WithCancel(ctx)

	dispatcher := &TopicDispatcher{
		Inbound:  make(chan mqtt.Message),
		Finished: make(chan bool),
		workers:  make(map[string]*TenantWorker),
		ctx:      ctx,
		quit:     make(chan struct{}),
		//wg:       sync.WaitGroup,
		stop: cancel,
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
	worker, ok := d.workers[tenant]
	if !ok {
		worker = &TenantWorker{
			tenant:     tenant,
			JobChannel: make(chan mqtt.Message),
			executor:   &calculation.TenantEnergyImporter{Tenant: tenant},
			ctx:        d.ctx,
			wg:         &d.wg,
		}
		go worker.Run()
		d.workers[tenant] = worker
	}
	return worker.JobChannel
}

func (worker *TenantWorker) Run() {
	worker.wg.Add(1)
	defer worker.wg.Done()

	for {
		select {
		case job := <-worker.JobChannel:
			worker.executor.Execute(job)
		case <-worker.ctx.Done():
			log.Infof("Stop Worker %s", worker.tenant)
			return
		}
	}
}
