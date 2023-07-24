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

//type EnergyAllocationLine struct {
//	Time          string
//	CounterPoints []CounterPointAllocation
//	Producers     []ProducerAllocation
//	SumProducers  float32
//	SumConsumers  float32
//	Quota         *QuotaMatrix
//}

//type ProducerAllocation struct {
//	Producer   Producer
//	Allocation float64
//}
//
//type CounterPointAllocation struct {
//	CounterPoint CounterPoint
//	Allocation   float32
//}
//
//type ProducerConfig struct {
//	Producer         Producer
//	Consumer_quota   float32
//	Production_quota float32
//	Value            float32
//}

//type ConsumerMeasure struct {
//	values []float32
//}
//
//type ProducersMeasure struct {
//	values []float32
//}

//type CounterPointPrice struct {
//	Id    int16   `json:"id"`
//	Name  string  `json:"name"`
//	From  int64   `json:"from"`  // price defined from particilar date. The price will change if a new date exists
//	Price float64 `json:"price"` // price in Euro
//}
//
//type CounterPoint struct {
//	Name    string             `json:"name"` // Meteringpoint Number
//	Price   CounterPointPrice  `json:"price"`
//	GroupId int16              `json:"groupId"`
//	Role    string             `json:"role"`   // CONSUMER or. GENERATOR
//	Status  CounterPointStatus `json:"status"` // 0...New, 1...registerd, 2...accepted, 3...connected, 4...unregistered
//}
//
//type Producer struct {
//	Name     string `json:"name"`
//	Location string `json:"location"`
//	EEG      string `json:"eeg"`
//}

//type CounterPointInvoice struct {
//	Sum       float32
//	Consumer  []ConsumerMeasure
//	Producers []ProducersMeasure
//}

//type ParticipantInvoice struct {
//	Id            int16   `json:"id"`
//	Name          string  `json:"name"`
//	ClearingBegin int64   `json:"clearing_begin"`
//	ClearingEnd   int64   `json:"clearing_end,omitempty"`
//	Price         float64 `json:"price"`
//	FromEEG       float64 `json:"from_eeg"`
//	FromGrid      float64 `json:"from_grid"`
//	Total         float64 `json:"total"`
//}
//
//type ParticipantInvoiceHistory struct {
//	Id            int16   `json:"id"`
//	Name          string  `json:"name"`
//	ClearingBegin int64   `json:"clearing_begin"`
//	ClearingEnd   int64   `json:"clearing_end,omitempty"`
//	Price         float64 `json:"price"`
//	User          string  `json:"user,omitempty"`
//	Updated       int64   `json:"updated,omitempty"`
//	Comment       string  `json:"comment,omitempty"`
//}

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
