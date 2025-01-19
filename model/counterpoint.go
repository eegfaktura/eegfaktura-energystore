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

// 1-1:1.9.0 G.01
const (
	CODE_GEN      MeterCodeValue = "1-1:2.9.0 G.01"  // Erzeugung laut Messung - Producer
	CODE_GEN_TF   MeterCodeValue = "1-1:2.9.0 G.01T" // Erzeugung laut Messung entsprechend dem Teilnahmefaktor bei der EG und je ZP - Producer
	CODE_PLUS     MeterCodeValue = "1-1:2.9.0 P.01"  // QH Lastgang ¼ h Wirkenergiewerte, Bezug vom Endkunden, Gemeinschaftsüberschuss - Producer
	CODE_PLUS_TF  MeterCodeValue = "1-1:2.9.0 P.01T" // QH Restnetzüberschuss bei Energiegemeinschaft - Producer
	CODE_CON      MeterCodeValue = "1-1:1.9.0 G.01"  // Verbrauch laut Messung - Consumer
	CODE_CON_TF   MeterCodeValue = "1-1:1.9.0 G.01T" // QH Verbrauch laut Messung entsprechend dem Teilnahmefaktor bei der EG und je ZP " - Consumer
	CODE_SHARE    MeterCodeValue = "1-1:2.9.0 G.02"  // Anteil an der Erzeugung - Consumer
	CODE_COVER    MeterCodeValue = "1-1:2.9.0 G.03"  // Eigendeckung - Consumer
	CODE_COVER_TF MeterCodeValue = "1-1:2.9.0 G.03R" // QH Eigendeckung aus erneuerbarer Energie - Consumer

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
