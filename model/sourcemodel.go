package model

type RawSourceLine struct {
	Id           string    `bow:"key"`
	Consumers    []float64 `bow:"consumers"`
	Producers    []float64 `bow:"producers"`
	QoVConsumers []int     `bow:"qovconsumers"`
	QoVProducers []int     `bow:"qovproducers"`
}

func (c RawSourceLine) Copy(cLength int) RawSourceLine {
	r := RawSourceLine{
		Id:           c.Id,
		Consumers:    make([]float64, len(c.Consumers)),
		Producers:    make([]float64, len(c.Producers)),
		QoVConsumers: make([]int, len(c.Consumers)),
		QoVProducers: make([]int, len(c.Producers)),
	}
	copy(r.Consumers[:], c.Consumers[:])
	copy(r.Producers[:], c.Producers[:])
	copy(r.QoVConsumers[:], c.QoVConsumers[:])
	copy(r.QoVProducers[:], c.QoVProducers[:])
	return r
}

func MakeRawSourceLine(id string, consumerSize, producerSize int) *RawSourceLine {
	return &RawSourceLine{Id: id,
		Consumers: make([]float64, consumerSize), Producers: make([]float64, producerSize),
		QoVConsumers: make([]int, consumerSize), QoVProducers: make([]int, consumerSize),
	}
}

func CreateInitializedIntSlice(size int, initVal int) []int {
	intSlice := make([]int, size)
	for i, _ := range intSlice {
		intSlice[i] = initVal
	}
	return intSlice
}

func CreateInitializedBoolSlice(size int, initVal bool) []bool {
	boolSlice := make([]bool, size)
	for i, _ := range boolSlice {
		boolSlice[i] = initVal
	}
	return boolSlice
}

type EegEnergy struct {
	Report  *EnergyReport       `json:"report"`
	Results []*EnergyReport     `json:"intermediateReportResults"`
	Meta    []*CounterPointMeta `json:"meta"`
}

type RawSourceMeta struct {
	Id               string              `bow:"key"`
	CounterPoints    []*CounterPointMeta `bow:"counterpoints"`
	NumberOfMetering int                 `bow:"numberOfMetering"`
}

func (rsm RawSourceMeta) Copy() RawSourceMeta {
	r := RawSourceMeta{Id: rsm.Id, CounterPoints: []*CounterPointMeta{}, NumberOfMetering: rsm.NumberOfMetering}
	for i := 0; i < len(rsm.CounterPoints); i++ {
		r.CounterPoints = append(r.CounterPoints,
			&CounterPointMeta{ID: rsm.CounterPoints[i].ID,
				Name: rsm.CounterPoints[i].Name,
				//Idx:         rsm.CounterPoints[i].Idx,
				Dir:         rsm.CounterPoints[i].Dir,
				Count:       rsm.CounterPoints[i].Count,
				PeriodStart: rsm.CounterPoints[i].PeriodStart,
				PeriodEnd:   rsm.CounterPoints[i].PeriodEnd,
			})
	}
	return r
}

/*
CounterPointMeta describe the raw data source in db. Be aware, the meta data are just applicable for one year.
*/
type CounterPointMeta struct {
	ID          string         `bow:"key" json:"id"`                   // Usually, the Id represents Table-Type and Year (e.g. cpmeta/2021)
	Name        string         `bow:"name" json:"name"`                // Counterpoint name (e.g. ZPxxxxxxxxZaehlpunktxx)
	SourceIdx   int            `bow:"srcIdx" json:"sourceIdx"`         // Index of rawdata array in db
	Dir         MeterDirection `bow:"dir" json:"dir"`                  // Direction of consumption (GENERATOR or. CONSUMER)
	Count       uint16         `bow:"count" json:"count"`              // Number of measurements
	PeriodStart string         `bow:"periodstart" json:"period_start"` // Period Start
	PeriodEnd   string         `bow:"periodend" json:"period_end"`     // Period End
}

type CounterPointMetaInfo struct {
	ConsumerCount  int // Max. number of consumer in the CounterPointMeta data structure
	ProducerCount  int // Max. number of producer in the CounterPointMeta data structure
	MaxConsumerIdx int // Highest index of consumer in the CounterPointMeta data structure
	MaxProducerIdx int // Highest index of producer in the CounterPointMeta data structure
}

type ByReportDate []EnergyReport

func (a ByReportDate) Len() int           { return len(a) }
func (a ByReportDate) Less(i, j int) bool { return a[i].Id < a[j].Id }
func (a ByReportDate) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type EnergyReport struct {
	Id            string    `bow:"key" json:"id"`
	Allocated     []float64 `bow:"values" json:"allocated"`
	Consumed      []float64 `bow:"consumed" json:"consumed"`
	Produced      []float64 `bow:"produced" json:"produced"`
	Distributed   []float64 `bow:"distributed" json:"distributed"`
	Shared        []float64 `bow:"shared" json:"shared"`
	TotalProduced float64   `bow:"totalProduced" json:"total_produced"`
}
