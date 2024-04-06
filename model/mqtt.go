package model

type MqttEnergyValue struct {
	From   int64   `json:"from"`
	To     int64   `json:"to"`
	Method string  `json:"method"`
	Value  float64 `json:"value"`
}

type MqttEnergyData struct {
	MeterCode MeterCodeValue    `json:"meterCode"`
	Value     []MqttEnergyValue `json:"value"`
}

type MqttEnergy struct {
	Start int64            `json:"start"`
	End   int64            `json:"end"`
	Data  []MqttEnergyData `json:"data"`
}

type EnergyMeter struct {
	MeteringPoint string `json:"meteringPoint"`
	Direction     string `json:"direction,omitempty"`
}

type MqttEnergyMessage struct {
	Meter  EnergyMeter `json:"meter"`
	Energy MqttEnergy  `json:"energy"`
	EcId   string      `json:"ecId"`
}

type MqttEnergyResponse struct {
	Message MqttEnergyMessage `json:"message"`
}
