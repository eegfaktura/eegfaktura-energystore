package mqttclient

import (
	"context"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
	"github.com/spf13/viper"
	"strings"
	"time"
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
	client mqtt.Client
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

	opts.SetOrderMatters(false)       // Allow out of order messages (use this option unless in order delivery is essential)
	opts.ConnectTimeout = time.Second // Minimal delays on connect
	opts.WriteTimeout = time.Second   // Minimal delays on writes
	opts.KeepAlive = 10               // Keepalive every 10 seconds so we quickly detect network outages
	opts.PingTimeout = time.Second    // local broker so response should be quick

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

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &MQTTStreamer{client: client}, nil
}

func (m *MQTTStreamer) SubscribeTopic(ctx context.Context, topic string, callback mqtt.MessageHandler) {
	brokerQos := viper.GetInt("mqtt.qos")
	s := m.client.Subscribe(topic, byte(brokerQos), callback)
	s.Wait()
	if err := s.Error(); err != nil {
		glog.Error(err)
	}
}
