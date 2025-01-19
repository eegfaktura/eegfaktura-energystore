package mqttclient

import (
	"at.ourproject/energystore/model"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"testing"
	"time"
)

type testMsg struct {
	payload []byte
	topic   string
}

func (tm *testMsg) Payload() []byte {
	return tm.payload
}

func (tm *testMsg) Duplicate() bool {
	return false
}

func (tm *testMsg) Qos() byte {
	return byte(1)
}

func (tm *testMsg) Retained() bool {
	return false
}

func (tm *testMsg) Topic() string {
	return tm.topic
}

func (tm *testMsg) MessageID() uint16 {
	return uint16(13)
}

func (tm *testMsg) Ack() {

}

var (
	startTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)
)

func getNext(tenant string) mqtt.Message {
	endTime := startTime.Add(15 * time.Minute)
	msg := model.MqttEnergyMessage{
		Meter: model.EnergyMeter{},
		Energy: []model.MqttEnergy{{
			Start: startTime.UnixMilli(),
			End:   endTime.UnixMilli(),
			Data: []model.MqttEnergyData{{
				MeterCode: "1-1:1.9.0 G.01",
				Value: []model.MqttEnergyValue{{
					From:   startTime.UnixMilli(),
					To:     endTime.UnixMilli(),
					Method: "L1",
					Value:  0,
				}},
			}},
		}},
		EcId: "test",
	}
	startTime = endTime

	payload, _ := json.Marshal(msg)
	return &testMsg{payload: payload, topic: fmt.Sprintf("eda/response/%s/protocol/cr_msg", tenant)}
}

func TestMassSend(t *testing.T) {
	//cancel, ctx := context.WithCancel(context.Background())
	//dispatcher := NewTopicDispatcher(ctx, "eda/response/+/protocol/cr_msg", &MockMQTTStreamer{})
}
