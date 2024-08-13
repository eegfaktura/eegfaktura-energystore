package cmd

import (
	"at.ourproject/energystore/model"
	"sort"
)

func reorganizeMeta(meta *model.RawSourceMeta) error {
	consumerSlice := []*model.CounterPointMeta{}
	producerSlice := []*model.CounterPointMeta{}
	for _, v := range meta.CounterPoints {
		if v.Dir == model.PRODUCER_DIRECTION {
			producerSlice = append(producerSlice, v)
		} else {
			consumerSlice = append(consumerSlice, v)
		}
	}

	sort.SliceStable(consumerSlice, func(i, j int) bool { return consumerSlice[i].SourceIdx < consumerSlice[j].SourceIdx })
	sort.SliceStable(producerSlice, func(i, j int) bool { return producerSlice[i].SourceIdx < producerSlice[j].SourceIdx })

	//idx := -1
	//for _, m := range meta.CounterPoints {
	//	if m.Name == meterpoint {
	//		idx = m.SourceIdx
	//		break
	//	}
	//}

	for i := 1; i < len(consumerSlice); i++ {
		if consumerSlice[i].SourceIdx == consumerSlice[i-1].SourceIdx+1 {
			continue
		}
		consumerSlice[i].SourceIdx = consumerSlice[i-1].SourceIdx + 1
	}

	for i := 1; i < len(producerSlice); i++ {
		if producerSlice[i].SourceIdx == producerSlice[i-1].SourceIdx+1 {
			continue
		}
		producerSlice[i].SourceIdx = producerSlice[i-1].SourceIdx + 1
	}

	return nil
}

func MinOf(vars ...int) int {
	min := vars[0]

	for _, i := range vars {
		if min > i {
			min = i
		}
	}

	return min
}
