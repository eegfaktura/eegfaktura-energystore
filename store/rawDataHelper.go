package store

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/store/function"
	"fmt"
	"math"
)

type MetaMapType map[string]*model.CounterPointMeta

func GetConsumerMetaMap(db *ebow.BowStorage) (map[string]*model.CounterPointMeta, error) {
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

func CalcMetaInfo(db *ebow.BowStorage) (*model.CounterPointMetaInfo, map[string]*model.CounterPointMeta, error) {
	meta, err := function.GetMetaMap(db)
	if err != nil {
		return nil, nil, err
	}
	return metaInfoOf(meta), meta, nil
}

// metaInfoOf counts the consumers and producers of meta and finds their highest SourceIdx.
func metaInfoOf(meta map[string]*model.CounterPointMeta) *model.CounterPointMetaInfo {
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
	return info
}

func PrepareMetaInfoMap(db *ebow.BowStorage, meterpoint string, direction model.MeterDirection) (map[string]*model.CounterPointMeta, *model.CounterPointMetaInfo, error) {
	modified := false
	//meta, err := GetMetaMap(db)
	//if err != nil {
	//	return nil, nil, err
	//}
	//
	//info := &model.CounterPointMetaInfo{
	//	ConsumerCount: 0, ProducerCount: 0,
	//	MaxConsumerIdx: -1, MaxProducerIdx: -1,
	//}
	//
	//for _, v := range meta {
	//	switch v.Dir {
	//	case model.CONSUMER_DIRECTION:
	//		info.ConsumerCount += 1
	//		info.MaxConsumerIdx = int(math.Max(float64(v.SourceIdx), float64(info.MaxConsumerIdx)))
	//	case model.PRODUCER_DIRECTION:
	//		info.ProducerCount += 1
	//		info.MaxProducerIdx = int(math.Max(float64(v.SourceIdx), float64(info.MaxProducerIdx)))
	//	}
	//}

	info, meta, err := CalcMetaInfo(db)
	if err != nil {
		return nil, nil, err
	}

	modified = registerMeteringPoint(meta, info, meterpoint, direction)
	if modified {
		if err := db.SetMeta(rawMetaOf(meta)); err != nil {
			return nil, nil, err
		}
	}
	return meta, info, nil
}

// registerMeteringPoint adds an unknown metering point to meta with the next SourceIdx of its direction
// and updates info. It reports whether meta changed.
func registerMeteringPoint(meta map[string]*model.CounterPointMeta, info *model.CounterPointMetaInfo, meterpoint string, direction model.MeterDirection) bool {
	if _, ok := meta[meterpoint]; ok {
		return false
	}
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
	return true
}

// rawMetaOf builds the record cpmeta/0 from the metering points of meta.
func rawMetaOf(meta map[string]*model.CounterPointMeta) *model.RawSourceMeta {
	rawMeta := &model.RawSourceMeta{Id: fmt.Sprintf("cpmeta/%s", "0"), CounterPoints: []*model.CounterPointMeta{}, NumberOfMetering: -1}
	for _, v := range meta {
		rawMeta.CounterPoints = append(rawMeta.CounterPoints, v)
	}
	return rawMeta
}

func GetMetaInfo(db ebow.IBowStorage) (map[string]*model.CounterPointMeta, *model.CounterPointMetaInfo, error) {
	meta, err := function.GetMetaMap(db)
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
