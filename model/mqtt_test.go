package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The CR_MSG wire format: eda-xp publishes its EbMsMessage as JSON without an envelope (MqttSystem,
// null values dropped, dates as epoch ms); energy is an array of blocks with start, end, interval,
// nInterval and data. energystore decodes it into MqttEnergyMessage (mqttclient.decodeMessage).

func TestMessage(t *testing.T) {
	m := MqttEnergyMessage{
		ConversationId: "RC100130202211111446152980115933630",
		MessageCode:    "DATEN_CRMSG",
		Energy: []MqttEnergy{{
			Start: 0,
			End:   0,
			Data: []MqttEnergyData{
				{
					MeterCode: "",
					Value:     nil,
				},
			},
		}},
	}

	b, err := json.Marshal(m)
	require.NoError(t, err)

	require.JSONEq(t,
		`{"conversationId":"RC100130202211111446152980115933630","messageCode":"DATEN_CRMSG","meter":{"meteringPoint":""},"energy":[{"start":0,"end":0,"data":[{"meterCode":"","value":null}]}],"ecId":""}`,
		string(b))
}

func TestJsonStruct(t *testing.T) {
	jsonString := `{"messageId":"AT003000202211111446152980115933630","conversationId":"AT003000202206290921026980008077710","sender":"AT003000","receiver":"RC100130","messageCode":"DATEN_CRMSG","meter":{"meteringPoint":"AT0030000000000000000000000123456","direction":"CONSUMPTION"},"ecId":"ecId","energy":[{"start":1667948400000,"end":1668034800000,"interval":"QH","nInterval":96,"data":[{"meterCode":"1-1:1.9.0 G.01","value":[{"from":1667948400000,"to":1667949300000,"method":"L1","value":0.118}]}]}]}`

	m := MqttEnergyMessage{}
	err := json.Unmarshal([]byte(jsonString), &m)
	require.NoError(t, err)

	require.Equal(t, MqttEnergyMessage{
		ConversationId: "AT003000202206290921026980008077710",
		MessageCode:    "DATEN_CRMSG",
		EcId:           "ecId",
		Meter:          EnergyMeter{MeteringPoint: "AT0030000000000000000000000123456", Direction: "CONSUMPTION"},
		Energy: []MqttEnergy{{Start: 1667948400000, End: 1668034800000, Data: []MqttEnergyData{{
			MeterCode: "1-1:1.9.0 G.01",
			Value: []MqttEnergyValue{{
				From:   1667948400000,
				To:     1667949300000,
				Method: "L1",
				Value:  0.118,
			}},
		}}}},
	}, m)
}
