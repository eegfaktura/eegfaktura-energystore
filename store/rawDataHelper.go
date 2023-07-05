package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"fmt"
	"math"
)

func GetMetaMap(db *BowStorage) (map[string]*model.CounterPointMeta, error) {
	var err error
	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%s", "0")); err != nil {
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

func GetConsumerMetaMap(db *BowStorage) (map[string]*model.CounterPointMeta, error) {
	var err error
	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%s", "0")); err != nil {
		if err != ebow.ErrNotFound {
			return nil, err
		}
	}
	metaMap := map[string]*model.CounterPointMeta{}
	for i := 0; i < len(meta.CounterPoints); i++ {
		m := meta.CounterPoints[i]
		if m.Dir == "CONSUMPTION" {
			metaMap[m.Name] = m
		}
	}
	return metaMap, nil
}

func GetMetaInfoMap(db *BowStorage, meterpoint string, direction model.MeterDirection) (map[string]*model.CounterPointMeta, *model.CounterPointMetaInfo, error) {
	modified := false
	meta, err := GetMetaMap(db)
	if err != nil {
		return nil, nil, err
	}

	info := &model.CounterPointMetaInfo{
		ConsumerCount: 0, ProducerCount: 0,
		MaxConsumerIdx: -1, MaxProducerIdx: -1,
	}

	for _, v := range meta {
		switch v.Dir {
		case model.CONSUMER_DIRECTION:
			info.ConsumerCount += 1
			info.MaxConsumerIdx = int(math.Max(float64(v.SourceIdx), float64(info.MaxConsumerIdx)))
		case model.PRODUCER_DIRECTION:
			info.ProducerCount += 1
			info.MaxProducerIdx = int(math.Max(float64(v.SourceIdx), float64(info.MaxProducerIdx)))
		}
	}

	_, ok := meta[meterpoint]
	if !ok {
		modified = true
		switch direction {
		case model.CONSUMER_DIRECTION:
			info.ConsumerCount += 1
			info.MaxConsumerIdx += 1
			meta[meterpoint] = &model.CounterPointMeta{
				ID:        fmt.Sprintf("%.3d", len(meta)),
				SourceIdx: info.MaxConsumerIdx,
				Name:      meterpoint,
				Dir:       model.CONSUMER_DIRECTION,
			}
		case model.PRODUCER_DIRECTION:
			info.ProducerCount += 1
			info.MaxProducerIdx += 1
			meta[meterpoint] = &model.CounterPointMeta{
				ID:        fmt.Sprintf("%.3d", len(meta)),
				SourceIdx: info.MaxProducerIdx,
				Name:      meterpoint,
				Dir:       model.PRODUCER_DIRECTION,
			}
		}
	}
	if modified {
		rawMeta := &model.RawSourceMeta{Id: fmt.Sprintf("cpmeta/%s", "0"), CounterPoints: []*model.CounterPointMeta{}, NumberOfMetering: -1}
		for _, v := range meta {
			rawMeta.CounterPoints = append(rawMeta.CounterPoints, v)
		}
		_ = db.SetMeta(rawMeta)
	}
	return meta, info, nil
}

func GetMetaInfo(db *BowStorage) (map[string]*model.CounterPointMeta, *model.CounterPointMetaInfo, error) {
	meta, err := GetMetaMap(db)
	if err != nil {
		return nil, nil, err
	}

	info := &model.CounterPointMetaInfo{
		ConsumerCount: 0, ProducerCount: 0,
		MaxConsumerIdx: -1, MaxProducerIdx: -1,
	}

	for _, v := range meta {
		switch v.Dir {
		case model.CONSUMER_DIRECTION:
			info.ConsumerCount += 1
			info.MaxConsumerIdx = int(math.Max(float64(v.SourceIdx), float64(info.MaxConsumerIdx)))
		case model.PRODUCER_DIRECTION:
			info.ProducerCount += 1
			info.MaxProducerIdx = int(math.Max(float64(v.SourceIdx), float64(info.MaxProducerIdx)))
		}
	}
	return meta, info, nil
}

func ensureMeterPointInMap(meterpoint string, meta map[string]*model.CounterPointMeta) {

}
