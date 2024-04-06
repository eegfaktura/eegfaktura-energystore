package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var re = regexp.MustCompile(`^(\w*)[^(]*\(([^)]*)\)$`)

type ReportData struct {
	Consumed    float64 `json:"consumed"`
	Allocated   float64 `json:"allocated"`
	Distributed float64 `json:"distributed"`
	Produced    float64 `json:"produced"`
	QoVConsumer int     `json:"qoVConsumer"`
	QoVProducer int     `json:"qoVProducer"`
}

type RawData struct {
	Ts    int64     `json:"ts"`
	Value []float64 `json:"value"`
	Qov   []int     `json:"qov"`
}

type RawDataResult struct {
	Data      []RawData            `json:"data"`
	Direction model.MeterDirection `json:"direction"`
}

type RawDataEngine struct {
	cps      []TargetMP
	params   map[string][]string
	function IQueryFunction
}

type MetaData struct {
	PeriodBegin int64 `json:"periodBegin"`
	PeriodEnd   int64 `json:"periodEnd"`
}

func (rde *RawDataEngine) HandleStart(ctx *EngineContext) error {

	rde.function = &DefaultFunction{ParentFunction{cps: rde.cps}}

	if len(rde.params) > 0 {
		if v, ok := rde.params["f"]; ok {
			fn, pa, err := parseFunction(v)
			if err != nil {
				return err
			}
			qfn, ok := Functions[strings.ToUpper(fn)]
			if !ok {
				return errors.New(fmt.Sprintf("Unknown function found %s", fn))
			}
			rde.function, err = qfn(strings.Split(pa, ","), rde.cps)
			if err != nil {
				return err
			}
		}
	}

	return rde.function.HandleInit(ctx)
}

func (rde *RawDataEngine) HandleLine(ctx *EngineContext, line *model.RawSourceLine) error {
	return rde.function.HandleLine(ctx, line)
}

func (rde *RawDataEngine) HandleEnd(ctx *EngineContext) error {
	return rde.function.HandleFinish(ctx)
}

func QueryIntraDayReport(tenant, ecid string, start, end time.Time) ([]*ReportData, error) {
	c, _ := NewIntraDayFunction()
	e := &Engine{c}

	if err := e.Query(tenant, ecid, start, end); err != nil {
		return nil, err
	}
	return (c.(*IntraDay)).GetResult(), nil
}

func QueryRawData(tenant, ecid string, start, end time.Time, cps []TargetMP, params map[string][]string) (map[string]*RawDataResult, error) {
	c := &RawDataEngine{cps: cps, params: params}
	e := &Engine{c}

	if err := e.Query(tenant, ecid, start, end); err != nil {
		return nil, err
	}
	return c.function.GetResult(), nil
}

func QueryMetaData(tenant, ecid string) (map[string]*MetaData, error) {
	db, err := OpenStorage(tenant, ecid)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	result := map[string]*MetaData{}
	metaMap, _, err := GetMetaInfo(db)
	for k, v := range metaMap {
		begin, _ := utils.ParseTime(v.PeriodStart, 0)
		end, _ := utils.ParseTime(v.PeriodEnd, 0)
		result[k] = &MetaData{
			PeriodBegin: begin.UnixMilli(),
			PeriodEnd:   end.UnixMilli(),
		}
	}

	return result, err
}

func parseFunction(f []string) (fn string, pa string, err error) {

	if len(f) > 1 {
		err = errors.New(fmt.Sprintf("Unknown function declared %+v", f))
		return
	}
	m := re.FindStringSubmatch(f[0])
	if len(m) < 3 {
		err = errors.New("parser error while interpret function name")
		return
	}

	fn = m[1]
	pa = m[2]
	return
}
