package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"time"
)

type ParentFunction struct {
	cps    []TargetMP
	Result map[string]*RawDataResult `json:"result"`
}

func (paf *ParentFunction) addToResult(ctx *EngineContext, ts time.Time, line *model.RawSourceLine) error {
	for _, cp := range paf.cps {
		m := ctx.metaMap[cp.MeteringPoint]
		if _, ok := paf.Result[cp.MeteringPoint]; !ok {
			paf.Result[cp.MeteringPoint] = &RawDataResult{Direction: m.Dir}
		}

		if m.Dir == model.CONSUMER_DIRECTION {
			result := RawData{Ts: ts.UnixMilli(), Value: make([]float64, 3), Qov: make([]int, 3)}
			if len(line.Consumers) > m.SourceIdx*3 {
				copy(result.Value, line.Consumers[m.SourceIdx*3:])
				copy(result.Qov, line.QoVConsumers[m.SourceIdx*3:])
			}
			paf.Result[cp.MeteringPoint].Data = append(paf.Result[cp.MeteringPoint].Data, result)
		} else {
			result := RawData{Ts: ts.UnixMilli(), Value: make([]float64, 2), Qov: make([]int, 2)}
			if len(line.Producers) > m.SourceIdx*2 {
				copy(result.Value, line.Producers[m.SourceIdx*2:])
				copy(result.Qov, line.QoVProducers[m.SourceIdx*2:])
			}
			paf.Result[cp.MeteringPoint].Data = append(paf.Result[cp.MeteringPoint].Data, result)
		}
	}
	return nil
}

func (paf *ParentFunction) GetResult() map[string]*RawDataResult {
	return paf.Result
}

type DefaultFunction struct {
	ParentFunction
}

func (def *DefaultFunction) HandleInit(ctx *EngineContext) error {
	def.Result = make(map[string]*RawDataResult)
	return nil
}

func (def *DefaultFunction) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {

	ts, err := utils.ConvertRowIdToTime("CP", line.Id)
	if err != nil {
		return err
	}

	err = def.addToResult(ctx, ts, line)
	if err != nil {
		return err
	}

	return nil
}

func (def *DefaultFunction) HandleFinish(ctx *EngineContext) error {
	return nil
}
