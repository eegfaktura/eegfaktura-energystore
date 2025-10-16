package store

import (
	"strings"
	"time"

	"at.ourproject/energystore/model"
)

func determineReportFunctions(reports []string, start, end time.Time) map[string]EnergyConsumer {
	rf := make(map[string]EnergyConsumer)
	for _, report := range reports {
		switch strings.ToLower(report) {
		case "loadcurve":
			rf[report] = &LoadCurve{
				Cache:    Cache{cacheTs: determineTimeShiftFunction(start, end)},
				Result:   make(map[string]*ReportData),
				NameFunc: determineSeriesNameFunction(start, end, nil)}
			break
		case "intraday":
			rf[report] = &IntraDay{Cache: Cache{cacheTs: AddDuration(time.Hour)}, Result: make(map[int]*ReportData)}
			break
		case "summary":
			rf[report] = &EnergySummary{Result: &ReportData{}}
			break
		}
	}
	return rf
}

type CombinedConsumers struct {
	Consumers map[string]EnergyConsumer
}

func NewCombinedConsumers(reports []string, start, end time.Time) (EnergyConsumer, error) {
	return &CombinedConsumers{
		Consumers: determineReportFunctions(reports, start, end),
	}, nil
}

func (cc *CombinedConsumers) HandleStart(ctx *EngineContext) error {
	for _, consumer := range cc.Consumers {
		if consumer != nil {
			if err := consumer.HandleStart(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cc *CombinedConsumers) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	for _, consumer := range cc.Consumers {
		if consumer != nil {
			if err := consumer.HandleLine(ctx, line); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cc *CombinedConsumers) HandleEnd(ctx *EngineContext) error {
	for _, consumer := range cc.Consumers {
		if consumer != nil {
			if err := consumer.HandleEnd(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cc *CombinedConsumers) GetResult() []interface{} {
	type CombinedReport struct {
		ReportName string        `json:"reportName"`
		ReportData []interface{} `json:"reportData"`
	}

	var data []interface{}
	for n, consumer := range cc.Consumers {
		if consumer != nil {
			data = append(data, &CombinedReport{ReportName: n, ReportData: consumer.(EnergyReportConsumer).GetResult()})
		}
	}
	return data
}
