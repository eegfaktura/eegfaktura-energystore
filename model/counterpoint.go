package model

type MeterDirection string

const (
	CONSUMER_DIRECTION MeterDirection = "CONSUMPTION"
	PRODUCER_DIRECTION MeterDirection = "GENERATION"
)

type CounterPointRole int

const (
	CONSUMER CounterPointRole = iota
	GENERATOR
)

func (cp CounterPointRole) String() string {
	switch cp {
	case CONSUMER:
		return "CONSUMER"
	case GENERATOR:
		return "GENERATOR"
	}
	return "unknown"
}

type CounterPointStatus int

const (
	NEW CounterPointStatus = iota
	REGISTERED
	ACCEPTED
	CONNECTING
	CONNECTED
	UNREGISTERED
)

type MeterCodeValue string

const (
	CODE_GEN   MeterCodeValue = "1-1:2.9.0 G.01"
	CODE_PLUS  MeterCodeValue = "1-1:2.9.0 P.01"
	CODE_CON   MeterCodeValue = "1-1:1.9.0 G.01"
	CODE_SHARE MeterCodeValue = "1-1:2.9.0 G.02"
	CODE_COVER MeterCodeValue = "1-1:2.9.0 G.03"
)

// MeterCodeMeta
// Type of Metercode:
//   - GEN: Energy Generation - GENERATOR  1-1:2.9.0 G.01
//   - PLUS: Energy Overage - GENERATOR 1-1:2.9.0 P.01
//   - CON: Energy Consumption - CONSUMPTION 1-1:1.9.0 G.01
//   - SHARE: Energy Allocation - CONSUMPTION 1-1:2.9.0 G.02
//   - COVER: Energy coverage - CONSUMPTION 1-1:2.9.0 G.03
//
// /*
type MeterCodeMeta struct {
	Type         string
	Code         string
	SourceInData int
	SourceDelta  int
}
