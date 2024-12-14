package model

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMessage(t *testing.T) {
	m := MqttEnergyResponse{
		Message: MqttEnergyMessage{
			Energy: MqttEnergy{
				Start: 0,
				End:   0,
				Data: []MqttEnergyData{
					{
						MeterCode: "",
						Value:     nil,
					},
				},
			},
		}}

	b, err := json.Marshal(m)
	require.NoError(t, err)

	fmt.Println(string(b))

	require.Equal(t,
		[]byte(string(`{"message":{"meter":{"meteringPoint":""},"energy":{"start":0,"end":0,"data":[{"meterCode":"","value":null}]},"ecId":""}}`)),
		b)
}

func TestJsonStruct(t *testing.T) {

	//jsonString := `{"message" : {"messageId" : "AT003000202209171439474210109421497","conversationId" : "AT003000202209171439474220008412731","sender" : "AT003000","receiver" : "RC100130","messageCode" : "DATEN_CRMSG","requestId" : null,"meter" : null,"ecId" : null,"responseData" : null,"energy" : {"data" : [{"start" : 1663192800000,"end" : 1663279200000,"interval" : "QH","nInterval" : 96,"meterCode" : "1-1:2.9.0 G.01","value" : [{"from" : 1663192800000,"to" : 1663193700000,"method" : "L1","value" : 0.00025},{"from" : 1663193700000,"to" : 1663194600000,"method" : "L1","value" : 0},{"from" : 1663194600000,"to" : 1663195500000,"method" : "L1","value" : 0},{"from" : 1663195500000,"to" : 1663196400000,"method" : "L1","value" : 0},{"from" : 1663196400000,"to" : 1663197300000,"method" : "L1","value" : 0},{"from" : 1663197300000,"to" : 1663198200000,"method" : "L1","value" : 0},{"from" : 1663198200000,"to" : 1663199100000,"method" : "L1","value" : 0},{"from" : 1663199100000,"to" : 1663200000000,"method" : "L1","value" : 0},{"from" : 1663200000000,"to" : 1663200900000,"method" : "L1","value" : 0.00025},]}]}}}`
	//jsonString := `{"message" : {"messageId" : "AT003000202209171439474210109421497","conversationId" : "AT003000202209171439474220008412731","sender" : "AT003000","receiver" : "RC100130","messageCode" : "DATEN_CRMSG","requestId" : null,"meter" : null,"ecId" : null,"responseData" : null,"energy" : {"data" : [{"start" : 1663192800000,"end" : 1663279200000,"interval" : "QH","nInterval" : 96,"meterCode" : "1-1:2.9.0 G.01","value" : [{"from" : 1663192800000,"to" : 1663193700000,"method" : "L1","value" : 0.00025},{"from" : 1663193700000,"to" : 1663194600000,"method" : "L1","value" : 0},{"from" : 1663194600000,"to" : 1663195500000,"method" : "L1","value" : 0},{"from" : 1663195500000,"to" : 1663196400000,"method" : "L1","value" : 0},{"from" : 1663196400000,"to" : 1663197300000,"method" : "L1","value" : 0},{"from" : 1663197300000,"to" : 1663198200000,"method" : "L1","value" : 0},{"from" : 1663198200000,"to" : 1663199100000,"method" : "L1","value" : 0},{"from" : 1663199100000,"to" : 1663200000000,"method" : "L1","value" : 0},{"from" : 1663200000000,"to" : 1663200900000,"method" : "L1","value" : 0.00025}]}]}}}`
	jsonString := `{"message":{"messageId":"AT003000202211111446152980115933630","conversationId":"AT003000202206290921026980008077710","sender":"AT003000","receiver":"RC100130","messageCode":"DATEN_CRMSG","meter":{"meteringPoint":"AT0030000000000000000000000123456"},"energy":{"data":[{"start":1667948400000,"end":1668034800000,"interval":"QH","nInterval":288,"meterCode":"1-1:1.9.0 G.01","value":[{"from":1667948400000,"to":1667949300000,"method":"L1","value":0.118}]}]},"ecId":"ecId"}}`

	m := MqttEnergyResponse{}
	err := json.Unmarshal([]byte(jsonString), &m)
	require.NoError(t, err)

	require.Equal(t, MqttEnergyResponse{
		Message: MqttEnergyMessage{
			EcId:  "ecId",
			Meter: EnergyMeter{MeteringPoint: "AT0030000000000000000000000123456", Direction: ""},
			Energy: MqttEnergy{Start: 0, End: 0, Data: []MqttEnergyData{{
				MeterCode: "1-1:1.9.0 G.01",
				Value: []MqttEnergyValue{{
					From:   1667948400000,
					To:     1667949300000,
					Method: "L1",
					Value:  0.118,
				}},
			}}},
		},
	}, m)
}
