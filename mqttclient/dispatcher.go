package mqttclient

import (
	"context"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
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
		glog.V(3).Infof("Receive msg from topic %s - %v", msg.Topic(), string(msg.Payload()))
		worker.Execute(msg)
	}
	streamer.SubscribeTopic(ctx, topic, sub.receiver)
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
