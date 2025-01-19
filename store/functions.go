package store

import (
	"at.ourproject/energystore/model"
)

type IQueryFunction interface {
	HandleInit(ctx *EngineContext) error
	HandleLine(ctx *EngineContext, line *model.RawSourceLine) error
	HandleFinish(ctx *EngineContext) error
	GetResult() map[string]*RawDataResult
}

type QueryFunction func(args []string, cps []TargetMP) (IQueryFunction, error)

var Functions map[string]QueryFunction

func init() {
	Functions = make(map[string]QueryFunction)
	Functions["AGG"] = NewAggregateFunction
}
