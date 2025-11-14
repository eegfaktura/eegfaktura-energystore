package store

import (
	"fmt"
	"math"
	"testing"
	"time"

	"at.ourproject/energystore/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"at.ourproject/energystore/model"
)

func CreateTestMetaInfo() (map[string]*model.CounterPointMeta, *model.CounterPointMetaInfo) {
	metaMap := map[string]*model.CounterPointMeta{"AT002000000000000000000011111": {
		ID:          "1",
		Name:        "AT002000000000000000000011111",
		SourceIdx:   0,
		Dir:         "CONSUMPTION",
		Count:       0,
		PeriodStart: "01-01-2022",
		PeriodEnd:   "31-12-2022",
	}}

	info := &model.CounterPointMetaInfo{
		ConsumerCount: 1, ProducerCount: 0,
		MaxConsumerIdx: 0, MaxProducerIdx: -1,
	}

	return metaMap, info
}

func createTestEngineContext(start, end time.Time) (*EngineContext, error) {
	metaMap, info := CreateTestMetaInfo()

	metaRangeConsumer := map[int]periodRange{}
	metaRangeProducer := map[int]periodRange{}
	for _, v := range metaMap {
		ts, _ := utils.ParseTime(v.PeriodStart, 0)
		te, _ := utils.ParseTime(v.PeriodEnd, 0)
		if v.Dir == model.CONSUMER_DIRECTION {
			metaRangeConsumer[v.SourceIdx] = periodRange{start: ts, end: te}
		} else {
			metaRangeProducer[v.SourceIdx] = periodRange{start: ts, end: te}
		}
	}

	metaCon := []*model.CounterPointMeta{}
	metaPro := []*model.CounterPointMeta{}
	for _, v := range metaMap {
		if v.Dir == model.CONSUMER_DIRECTION {
			metaCon = append(metaCon, v)
		} else {
			metaPro = append(metaPro, v)
		}
	}
	meta := append(metaCon, metaPro...)
	countCons, countProd := utils.CountConsumerProducer(meta)

	return &EngineContext{
		start: time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local), /*start*/
		end:   time.Date(end.Year(), end.Month(), end.Day(), 23, 45, 0, 0, time.Local),     /*end*/
		//cps:             cps,
		metaMap:         metaMap,
		meta:            meta,
		info:            info,
		countProd:       countProd,
		countCons:       countCons,
		periodsConsumer: metaRangeConsumer,
		periodsProducer: metaRangeProducer,
		checkBegin: func(lineDate, mDate time.Time) bool {
			if lineDate.Before(mDate) {
				return true
			}
			return false
		},
	}, nil
}

func getLines() []*model.RawSourceLine {
	return []*model.RawSourceLine{
		{Id: "CP/2022/11/08/00/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/00/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/00/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/00/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/00/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/15/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/30/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2022/11/08/01/45/00", Consumers: []float64{0.02}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/02/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/03/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/04/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/05/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/06/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/07/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/08/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/09/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/10/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/11/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/12/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/13/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/14/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/15/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/16/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/17/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/18/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/19/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/20/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/00/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/15/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/30/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/21/45/00", Consumers: []float64{0.01}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/00/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/15/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/30/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2022/11/08/22/45/00", Consumers: []float64{0.05}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/00/00", Consumers: []float64{0.1}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/15/00", Consumers: []float64{0.1}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/30/00", Consumers: []float64{0.1}, Producers: []float64{}},
		{Id: "CP/2022/11/08/23/45/00", Consumers: []float64{0.1}, Producers: []float64{}},

		{Id: "CP/2022/11/09/00/00/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2022/11/09/00/15/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2022/11/09/00/30/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2022/11/09/00/45/00", Consumers: []float64{0.2}, Producers: []float64{}},
		{Id: "CP/2022/11/09/01/00/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2022/11/09/01/15/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2022/11/09/01/30/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2022/11/09/01/45/00", Consumers: []float64{0.3}, Producers: []float64{}},
		{Id: "CP/2022/11/09/02/00/00", Consumers: []float64{0.4}, Producers: []float64{}},
		{Id: "CP/2022/11/09/02/15/00", Consumers: []float64{0.4}, Producers: []float64{}},
		{Id: "CP/2022/11/09/02/30/00", Consumers: []float64{0.4}, Producers: []float64{}},
		{Id: "CP/2022/11/09/02/45/00", Consumers: []float64{0.4}, Producers: []float64{}},
	}
}

func printResult(r map[string]*RawDataResult) {
	for k, v := range r {
		fmt.Printf("Key: %s\n", k)
		for _, d := range v.Data {
			fmt.Printf("\t%v %v\n", time.UnixMilli(d.Ts).String(), d.Value)
		}
	}
}

func TestAggregate_HandleFinish(t *testing.T) {
	ctx, err := createTestEngineContext(time.Date(2022, time.Month(1), 1, 0, 0, 0, 0, time.Local),
		time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local))
	require.NoError(t, err)

	type fields struct {
		ParentFunction ParentFunction
		Cache          Cache
	}
	type args struct {
		ctx *EngineContext
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Handle Finish",
			fields: fields{
				ParentFunction: ParentFunction{cps: []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}}, Result: make(map[string]*RawDataResult)},
				Cache:          Cache{cacheTsFn: AddDuration(1)},
			},
			args:    args{ctx: ctx},
			wantErr: assert.NoError,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := &Aggregate{
				ParentFunction: tt.fields.ParentFunction,
				Cache:          tt.fields.Cache,
			}
			tt.wantErr(t, agg.HandleFinish(tt.args.ctx), fmt.Sprintf("HandleFinish(%v)", tt.args.ctx))
		})
	}
}

func TestAggregate_HandleInit(t *testing.T) {
	ctx, err := createTestEngineContext(time.Date(2022, time.Month(1), 1, 0, 0, 0, 0, time.Local),
		time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local))
	require.NoError(t, err)

	type fields struct {
		ParentFunction ParentFunction
		Cache          Cache
	}
	type args struct {
		ctx *EngineContext
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{{
		name: "Handle Init",
		fields: fields{
			ParentFunction: ParentFunction{cps: []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}}, Result: make(map[string]*RawDataResult)},
			Cache:          Cache{cacheTsFn: AddDuration(1)},
		},
		args:    args{ctx: ctx},
		wantErr: assert.NoError,
	},
	// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := &Aggregate{
				ParentFunction: tt.fields.ParentFunction,
				Cache:          tt.fields.Cache,
			}
			tt.wantErr(t, agg.HandleInit(tt.args.ctx), fmt.Sprintf("HandleInit(%v)", tt.args.ctx))
		})
	}
}

func TestAggregate_HandleLine(t *testing.T) {
	ctx, err := createTestEngineContext(time.Date(2022, time.Month(1), 1, 0, 0, 0, 0, time.Local),
		time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local))
	require.NoError(t, err)

	type fields struct {
		ParentFunction ParentFunction
		Cache          Cache
	}
	type args struct {
		ctx  *EngineContext
		line *model.RawSourceLine
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Aggregate Hour",
			fields: fields{
				ParentFunction: ParentFunction{cps: []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}}, Result: make(map[string]*RawDataResult)},
				Cache:          Cache{cacheTsFn: AddDuration(1)},
			},
			args: args{
				ctx:  ctx,
				line: &model.RawSourceLine{Id: "CP/2022/11/09/00/00/00", Consumers: []float64{0.118}, Producers: []float64{}},
			},
			wantErr: assert.NoError,
		}, // TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := &Aggregate{
				ParentFunction: tt.fields.ParentFunction,
				Cache:          tt.fields.Cache,
			}
			tt.wantErr(t, agg.HandleLine(tt.args.ctx, tt.args.line), fmt.Sprintf("HandleLine(%v, %v)", tt.args.ctx, tt.args.line))
		})
	}
}

func TestAggregateFunction(t *testing.T) {

	ctx, err := createTestEngineContext(time.Date(2022, time.November, 8, 0, 0, 0, 0, time.Local),
		time.Date(2022, time.November, 10, 0, 0, 0, 0, time.Local))
	require.NoError(t, err)

	type args struct {
		ctx   *EngineContext
		args  []string
		cps   []TargetMP
		lines []*model.RawSourceLine
	}
	tests := []struct {
		name    string
		args    args
		want    func(t *testing.T, result map[string]*RawDataResult)
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Aggregate Hours",
			args: args{
				ctx:   ctx,
				args:  []string{"12h"},
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: getLines(),
			},
			want: func(t *testing.T, result map[string]*RawDataResult) {
				assert.Equal(t, 1, len(result))
				assert.Equal(t, 3, len(result["AT002000000000000000000011111"].Data))
				assert.Equal(t, 0.52, math.Round(result["AT002000000000000000000011111"].Data[0].Value[0]*100)/100)
				assert.Equal(t, time.Date(2022, 11, 8, 0, 0, 0, 0, time.Local).UnixMilli(), result["AT002000000000000000000011111"].Data[0].Ts)
				assert.Equal(t, 1.00, math.Round(result["AT002000000000000000000011111"].Data[1].Value[0]*100)/100)
				assert.Equal(t, time.Date(2022, 11, 8, 12, 0, 0, 0, time.Local).UnixMilli(), result["AT002000000000000000000011111"].Data[1].Ts)
				assert.Equal(t, 3.60, math.Round(result["AT002000000000000000000011111"].Data[2].Value[0]*100)/100)
				assert.Equal(t, time.Date(2022, 11, 9, 0, 0, 0, 0, time.Local).UnixMilli(), result["AT002000000000000000000011111"].Data[2].Ts)

			},
			wantErr: assert.NoError,
		},
		{
			name: "Aggregate Day",
			args: args{
				ctx:   ctx,
				args:  []string{"1d"},
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: getLines(),
			},
			want: func(t *testing.T, result map[string]*RawDataResult) {
				assert.Equal(t, 1, len(result))
				assert.Equal(t, 2, len(result["AT002000000000000000000011111"].Data))
				assert.Equal(t, 1.52, math.Round(result["AT002000000000000000000011111"].Data[0].Value[0]*100)/100)
				assert.Equal(t, time.Date(2022, 11, 8, 0, 0, 0, 0, time.Local).UnixMilli(), result["AT002000000000000000000011111"].Data[0].Ts)
				assert.Equal(t, 3.60, math.Round(result["AT002000000000000000000011111"].Data[1].Value[0]*100)/100)
				assert.Equal(t, time.Date(2022, 11, 9, 0, 0, 0, 0, time.Local).UnixMilli(), result["AT002000000000000000000011111"].Data[1].Ts)
			},
			wantErr: assert.NoError,
		},
		{
			name: "Aggregate Week",
			args: args{
				ctx:   ctx,
				args:  []string{"1w"},
				cps:   []TargetMP{{MeteringPoint: "AT002000000000000000000011111"}},
				lines: getLines(),
			},
			want: func(t *testing.T, result map[string]*RawDataResult) {
				assert.Equal(t, 1, len(result))
				assert.Equal(t, 1, len(result["AT002000000000000000000011111"].Data))
				assert.Equal(t, 5.12, math.Round(result["AT002000000000000000000011111"].Data[0].Value[0]*100)/100)
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg, err := NewAggregateFunction(tt.args.args, tt.args.cps)

			require.NoError(t, agg.HandleInit(tt.args.ctx))
			for _, l := range tt.args.lines {
				require.NoError(t, agg.HandleLine(tt.args.ctx, l))
			}

			require.NoError(t, agg.HandleFinish(tt.args.ctx))

			if !tt.wantErr(t, err, fmt.Sprintf("NewAggregateFunction(%v, %v)", tt.args.args, tt.args.cps)) {
				return
			}
			if tt.want != nil {
				tt.want(t, agg.GetResult())
			}
			printResult(agg.GetResult())
			//assert.Equalf(t, tt.want, agg.GetResult(), "NewAggregateFunction(%v, %v)", tt.args.args, tt.args.cps)
		})
	}
}

func Test_calcQoV(t *testing.T) {
	type args struct {
		current int
		target  int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, calcQoV(tt.args.current, tt.args.target), "calcQoV(%v, %v)", tt.args.current, tt.args.target)
		})
	}
}

func Test_parseArgument(t *testing.T) {
	type args struct {
		arg string
	}
	tests := []struct {
		name    string
		args    args
		want    AddCacheTimeFunc
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "Aggregate Function - Day",
			args:    args{arg: "1d"},
			want:    AddDate(0, 0, 1),
			wantErr: assert.NoError,
		},
		{
			name:    "Aggregate Function - Week",
			args:    args{arg: "1w"},
			want:    AddDate(0, 0, 7),
			wantErr: assert.NoError,
		},
		{
			name:    "Aggregate Function - two Weeks",
			args:    args{arg: "2w"},
			want:    AddDate(0, 0, 14),
			wantErr: assert.NoError,
		},
		{
			name:    "Aggregate Function - Month",
			args:    args{arg: "1m"},
			want:    AddDate(0, 1, 0),
			wantErr: assert.NoError,
		},
		{
			name:    "Aggregate Function - Hour",
			args:    args{arg: "1h"},
			want:    AddDuration(time.Hour),
			wantErr: assert.NoError,
		},
		{
			name:    "Aggregate Function - Year (12 Month)",
			args:    args{arg: "12m"},
			want:    AddDate(0, 12, 0),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgument(tt.args.arg)
			if !tt.wantErr(t, err, fmt.Sprintf("parseArgument(%v)", tt.args.arg)) {
				return
			}
			ct := CacheTime{time.Now()}
			assert.Equal(t, tt.want(1, ct), got(1, ct))
		})
	}
}
