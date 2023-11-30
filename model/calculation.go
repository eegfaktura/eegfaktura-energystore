package model

import (
	"math"
)

type EnergyReportRequest struct {
	Year    int    `json:"year"`
	Period  string `json:"type"`
	Segment int    `json:"segment"`
}

type Recort struct {
	Consumption float64 `json:"consumption"` // Consumption total energy consumption - value for CONSUMER
	Utilization float64 `json:"utilization"` // Utilization energy used from the EEG  - value for CONSUMER
	Allocation  float64 `json:"allocation"`  // Allocation calculated energy value that can be allocated to all participants - value for CONSUMER / GENERATOR
	Production  float64 `json:"production"`  // Production total value of energy production - value for GENERATOR
}

func (R *Recort) RoundToFixed(precision uint) {
	ratio := math.Pow(10, float64(precision))

	R.Utilization = math.Round(R.Utilization*ratio) / ratio
	R.Consumption = math.Round(R.Consumption*ratio) / ratio
	R.Allocation = math.Round(R.Allocation*ratio) / ratio
	R.Production = math.Round(R.Production*ratio) / ratio
}

type IntermediateRecord struct {
	Id          string    `json:"id"`
	Consumption []float64 `json:"consumption"` // Consumption total energy consumption - value for CONSUMER
	Utilization []float64 `json:"utilization"` // Utilization energy used from the EEG  - value for CONSUMER
	Allocation  []float64 `json:"allocation"`  // Allocation calculated energy value that can be allocated to all participants - value for CONSUMER / GENERATOR
	Production  []float64 `json:"production"`  // Production total value of energy production - value for GENERATOR

}

func (ir *IntermediateRecord) RoundToFixed(precision uint) {
	ratio := math.Pow(10, float64(precision))
	for _, v := range ir.Utilization {
		v = math.Round(v*ratio) / ratio
	}
}

type Report struct {
	Id      string `json:"id"`
	Summary Recort `json:"summary"`
	//Intermediate []Recort `json:"intermediate"`
	Intermediate IntermediateRecord `json:"intermediate"`
}

func (report *Report) RoundToFixed(precision uint) {
	report.Summary.RoundToFixed(precision)
	//report.Intermediate.RoundToFixed(precision)
	//for _, i := range report.Intermediate {
	//	i.RoundToFixed(precision)
	//}
}

type MeterReport struct {
	MeterId  string  `json:"meterId"`
	MeterDir string  `json:"meterDir"`
	From     int64   `json:"from"`
	Until    int64   `json:"until"`
	Report   *Report `json:"report"`
}

func (meterReport *MeterReport) SetReport(r *Report) {
	meterReport.Report = r
}

type PeriodReport struct {
	participants []ParticipantReport
}

type ParticipantReport struct {
	ParticipantId string         `json:"participantId"`
	Meters        []*MeterReport `json:"meters"`
}

type ReportRequest struct {
	ReportInterval EnergyReportRequest `json:"reportInterval"`
	Participants   []ParticipantReport `json:"participants"`
}

type ReportResponse struct {
	Id                 string              `json:"id"`
	ParticipantReports []ParticipantReport `json:"participantReports"`
	Meta               []*CounterPointMeta `json:"meta"`
	TotalProduction    float64             `json:"totalProduction"`
	TotalConsumption   float64             `json:"totalConsumption"`
}
