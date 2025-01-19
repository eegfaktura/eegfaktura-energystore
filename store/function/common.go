package function

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"errors"
	"time"
)

type DataTimeRange struct {
	key   string
	until string
}

func mapMeta(cps []*model.CounterPointMeta) (cons map[int]*model.CounterPointMeta, prod map[int]*model.CounterPointMeta) {
	cons = map[int]*model.CounterPointMeta{}
	prod = map[int]*model.CounterPointMeta{}

	for _, c := range cps {
		if c.Dir == model.PRODUCER_DIRECTION {
			prod[c.SourceIdx] = c
		} else {
			cons[c.SourceIdx] = c
		}
	}
	return
}

func GetMetaByName(db ebow.IBowStorage, name string) (cpm *model.CounterPointMeta, err error) {
	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta("cpmeta/0"); err != nil {
		if !errors.Is(err, ebow.ErrNotFound) {
			return nil, err
		}
	}

	for _, m := range meta.CounterPoints {
		if m.Name == name {
			cpm = m
			return
		}
	}
	return
}

func GetMetaMap(db ebow.IBowStorage) (map[string]*model.CounterPointMeta, error) {
	var err error
	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta("cpmeta/0"); err != nil {
		if err != ebow.ErrNotFound {
			return nil, err
		}
	}
	metaMap := map[string]*model.CounterPointMeta{}
	for _, m := range meta.CounterPoints {
		metaMap[m.Name] = m
	}
	return metaMap, nil
}

func ToDataTimeRange(begin, end time.Time) (*DataTimeRange, error) {
	begin.Truncate(15 * time.Minute)
	end.Truncate(15 * time.Minute)

	beginPeriod, err := utils.ConvertUnixTimeToRowId("", begin)
	if err != nil {
		return nil, err
	}
	endPeriod, err := utils.ConvertUnixTimeToRowId("", end)
	if err != nil {
		return nil, err
	}

	return &DataTimeRange{
		key:   beginPeriod,
		until: endPeriod,
	}, nil

}
