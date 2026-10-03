package testsupport

import (
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Message is a fake mqtt.Message (Paho's interface) with a topic and a payload.
type Message struct {
	TopicName string
	Body      []byte
	mu        sync.Mutex
	acked     bool
}

// CrMsgMessage wraps an encoded CR_MSG as eda-xp publishes it for tenant.
func CrMsgMessage(tenant string, payload []byte) *Message {
	return &Message{TopicName: "eda/response/" + tenant + "/protocol/cr_msg", Body: payload}
}

func (m *Message) Duplicate() bool   { return false }
func (m *Message) Qos() byte         { return 1 }
func (m *Message) Retained() bool    { return false }
func (m *Message) Topic() string     { return m.TopicName }
func (m *Message) MessageID() uint16 { return 1 }
func (m *Message) Payload() []byte   { return m.Body }
func (m *Message) Ack() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acked = true
}

// Published is one Publish call recorded by FakeClient.
type Published struct {
	Topic   string
	Payload []byte
}

// FakeClient is a fake mqtt.Client: it records AddRoute (so a test can call the route's handler
// like the broker would) and Publish (the cr_msg_history replies), and returns completed tokens.
// No network. Paho's auto-ack happens inside Paho after the handler returns; a fake cannot show it.
type FakeClient struct {
	mu        sync.Mutex
	Routes    map[string]mqtt.MessageHandler
	published []Published
	// Notify, if set, receives every Publish (a latch for tests: no sleeps).
	Notify chan Published
	// Gate, if set, is called with the topic before a Publish is recorded; a test can block in it
	// to hold one tenant's worker.
	Gate func(topic string)
}

func NewFakeClient() *FakeClient {
	return &FakeClient{Routes: map[string]mqtt.MessageHandler{}, Notify: make(chan Published, 1000)}
}

// PublishedMessages returns a copy of the recorded Publish calls.
func (c *FakeClient) PublishedMessages() []Published {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Published{}, c.published...)
}

// Deliver calls the handler registered for route with msg, as Paho does for an inbound message.
func (c *FakeClient) Deliver(route string, msg mqtt.Message) {
	c.mu.Lock()
	h := c.Routes[route]
	c.mu.Unlock()
	h(c, msg)
}

func (c *FakeClient) IsConnected() bool       { return true }
func (c *FakeClient) IsConnectionOpen() bool  { return true }
func (c *FakeClient) Connect() mqtt.Token     { return doneToken{} }
func (c *FakeClient) Disconnect(quiesce uint) {}
func (c *FakeClient) Publish(topic string, qos byte, retained bool, payload any) mqtt.Token {
	if c.Gate != nil {
		c.Gate(topic)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b, _ := payload.([]byte)
	p := Published{Topic: topic, Payload: append([]byte{}, b...)}
	c.published = append(c.published, p)
	if c.Notify != nil {
		c.Notify <- p
	}
	return doneToken{}
}
func (c *FakeClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return doneToken{}
}
func (c *FakeClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return doneToken{}
}
func (c *FakeClient) Unsubscribe(topics ...string) mqtt.Token { return doneToken{} }
func (c *FakeClient) AddRoute(topic string, callback mqtt.MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Routes[topic] = callback
}
func (c *FakeClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

type doneToken struct{}

var closed = func() chan struct{} { c := make(chan struct{}); close(c); return c }()

func (doneToken) Wait() bool                     { return true }
func (doneToken) WaitTimeout(time.Duration) bool { return true }
func (doneToken) Done() <-chan struct{}          { return closed }
func (doneToken) Error() error                   { return nil }
