package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The CR_MSG payload as eda-xp publishes it (decoded from base64/gzip): the message object is
// the root, "energy" is an array of blocks (known-errors #4: the test still expected the old
// form with a "message" wrapper and a single "energy" object). M6 checks it against eda-xp.

func TestMessage(t *testing.T) {
	m := MqttEnergyMessage{
		ConversationId: "RC100130202206290921026980008077710",
		MessageCode:    "DATEN_CRMSG",
		Meter:          EnergyMeter{MeteringPoint: "AT0030000000000000000000000123456"},
		Energy: []MqttEnergy{{
			Start: 1667948400000,
			End:   1667949300000,
			Data: []MqttEnergyData{{
				MeterCode: CODE_CON,
				Value:     []MqttEnergyValue{{From: 1667948400000, To: 1667949300000, Method: "L1", Value: 0.118}},
			}},
		}},
		EcId: "AT00300000000RC100130000000952217",
	}

	b, err := json.Marshal(m)
	require.NoError(t, err)

	require.JSONEq(t, `{"conversationId":"RC100130202206290921026980008077710","messageCode":"DATEN_CRMSG",`+
		`"meter":{"meteringPoint":"AT0030000000000000000000000123456"},`+
		`"energy":[{"start":1667948400000,"end":1667949300000,"data":[{"meterCode":"1-1:1.9.0 G.01",`+
		`"value":[{"from":1667948400000,"to":1667949300000,"method":"L1","value":0.118}]}]}],`+
		`"ecId":"AT00300000000RC100130000000952217"}`, string(b))
}

func TestJsonStruct(t *testing.T) {
	// Fields the importer does not read (messageId, sender, receiver, messageCodeVersion) are
	// ignored; "direction" is optional.
	jsonString := `{"messageId":"AT003000202211111446152980115933630","conversationId":"AT003000202206290921026980008077710",` +
		`"sender":"AT003000","receiver":"RC100130","messageCode":"DATEN_CRMSG","messageCodeVersion":"03.00",` +
		`"meter":{"meteringPoint":"AT0030000000000000000000000123456"},` +
		`"energy":[{"start":1667948400000,"end":1668034800000,"data":[{"meterCode":"1-1:1.9.0 G.01",` +
		`"value":[{"from":1667948400000,"to":1667949300000,"method":"L1","value":0.118}]}]}],"ecId":"ecId"}`

	m := MqttEnergyMessage{}
	require.NoError(t, json.Unmarshal([]byte(jsonString), &m))

	require.Equal(t, MqttEnergyMessage{
		ConversationId: "AT003000202206290921026980008077710",
		MessageCode:    "DATEN_CRMSG",
		EcId:           "ecId",
		Meter:          EnergyMeter{MeteringPoint: "AT0030000000000000000000000123456"},
		Energy: []MqttEnergy{{Start: 1667948400000, End: 1668034800000, Data: []MqttEnergyData{{
			MeterCode: CODE_CON,
			Value:     []MqttEnergyValue{{From: 1667948400000, To: 1667949300000, Method: "L1", Value: 0.118}},
		}}}},
	}, m)
}
