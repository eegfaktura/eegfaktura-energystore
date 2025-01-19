package mqttclient

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
)

type MockMQTTStreamer struct {
	mock.Mock
}

func (m *MockMQTTStreamer) Connect() error {
	_ = m.Called()
	return nil
}

func (m *MockMQTTStreamer) AddRoutes(routes ...MqttRoutes) {
	_ = m.Called(routes)
}

func (m *MockMQTTStreamer) SubscribeTopic(ctx context.Context, topic string, callback mqtt.MessageHandler) {
	_ = m.Called(ctx, topic, callback)
}
