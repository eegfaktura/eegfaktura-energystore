package mqttclient

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"at.ourproject/energystore/model"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

type TopicType string

func (t TopicType) Tenant() string {
	elems := strings.Split(string(t), "/")
	if len(elems) > 3 {
		return strings.ToUpper(elems[2])
	}
	return ""
}

type MQTTStreamer struct {
	client   mqtt.Client
	outbound chan energyHistory
	mutex    sync.Mutex
}

func NewMqttStreamer() (*MQTTStreamer, error) {
	opts := mqtt.NewClientOptions()

	brokerHost := viper.GetString("mqtt.host")
	brokerId := viper.GetString("mqtt.id")

	glog.Infof("Use MQTT broker with address %s and Id %s", brokerHost, brokerId)

	opts.AddBroker(brokerHost)
	opts.SetClientID(brokerId)
	opts.SetProtocolVersion(4)
	opts.SetAutoAckDisabled(false)
	opts.SetCleanSession(false)

	opts.SetOrderMatters(false)            // Allow out of order messages (use this option unless in order delivery is essential)
	opts.ConnectTimeout = 30 * time.Second // Minimal delays on connect
	opts.WriteTimeout = 30 * time.Second   // Minimal delays on writes
	opts.KeepAlive = 60                    // Keepalive every 10 seconds so we quickly detect network outages
	opts.PingTimeout = 30 * time.Second    // local broker so response should be quick

	// Automate connection management (will keep trying to connect and will reconnect if network drops)
	opts.ConnectRetry = true
	opts.AutoReconnect = true

	// Log events
	opts.OnConnectionLost = func(cl mqtt.Client, err error) {
		glog.Infof("connection lost Err: %+v (%+v)", err.Error(), cl)
	}
	opts.OnConnect = func(mqtt.Client) {
		glog.Info("connection established")
	}
	opts.OnReconnecting = func(mqtt.Client, *mqtt.ClientOptions) {
		glog.Info("attempting to reconnect")
	}

	client := mqtt.NewClient(opts)

	return &MQTTStreamer{client: client}, nil
}

func (m *MQTTStreamer) Connect() error {
	client := m.client
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

type MqttRoutes struct {
	topic    string
	callback mqtt.MessageHandler
}

func (m *MQTTStreamer) AddRoutes(routes ...MqttRoutes) {
	client := m.client
	for _, r := range routes {
		client.AddRoute(r.topic, r.callback)
	}
}

func (m *MQTTStreamer) SubscribeTopic(ctx context.Context, topic string, callback mqtt.MessageHandler) {
	brokerQos := viper.GetInt("mqtt.qos")
	s := m.client.Subscribe(topic, byte(brokerQos), callback)
	s.Wait()
	if err := s.Error(); err != nil {
		glog.Error(err)
	}
}

func (m *MQTTStreamer) SendMessage(tenant string, msg *model.MqttEnergyMessage) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	historyData := energyHistory{Meter: msg.Meter, EcId: msg.EcId, ConversationId: msg.ConversationId, MessageCode: msg.MessageCode, Energy: make([]energyDate, 0)}
	for _, e := range msg.Energy {
		historyData.Energy = append(historyData.Energy, energyDate{Start: e.Start, End: e.End})
	}

	payload, err := json.Marshal(historyData)
	if err != nil {
		glog.Error("Marshaling Message")
		return
	}
	token := m.client.Publish(fmt.Sprintf("eda/response/%s/protocol/cr_msg_history", tenant), 1, false, payload)
	go func() {
		<-token.Done()
		if token.Error() != nil {
			fmt.Printf("MQTT ERROR PUBLISHING: %s\n", token.Error())
		}
	}()
	token.Wait()
}

type energyHistory struct {
	Meter          model.EnergyMeter `json:"meter"`
	Energy         []energyDate      `json:"energy"`
	EcId           string            `json:"ecId"`
	ConversationId string            `json:"conversationId"`
	MessageCode    string            `json:"messageCode"`
}

type energyDate struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
