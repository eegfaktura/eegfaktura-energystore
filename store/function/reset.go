package function

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
)

func Reset(db ebow.IBowStorage, timeRange *DataTimeRange, meter string) error {
	if len(meter) == 0 {
		return errors.New("empty meter")
	}

	meterMeta, err := GetMetaByName(db, meter)
	if err != nil {
		return err
	}

	var lines []*model.RawSourceLine
	iter := db.GetLineRange("CP", timeRange.key, timeRange.until)
	var _line model.RawSourceLine
	var affected bool
	for iter.Next(&_line) {
		line := _line.Copy(0)
		affected = false
		consumerMatrix, producerMatrix := utils.ConvertLineToMatrix(&line)
		if meterMeta.Dir == model.CONSUMER_DIRECTION {
			if meterMeta.SourceIdx <= consumerMatrix.Rows {
				affected = true
				consumerMatrix.SetRow(meterMeta.SourceIdx, make([]float64, 3))
				copy(line.QoVConsumers[meterMeta.SourceIdx*3:], make([]int, 3))
			}
		}
		if meterMeta.Dir == model.PRODUCER_DIRECTION {
			if meterMeta.SourceIdx <= producerMatrix.Rows {
				affected = true
				producerMatrix.SetRow(meterMeta.SourceIdx, make([]float64, 2))
				copy(line.QoVProducers[meterMeta.SourceIdx*2:], make([]int, 2))
			}
		}
		if affected {
			lines = append(lines, &line)
		}
	}
	fmt.Printf("Count %d, lines affected \n", len(lines))
	for i, line := range lines {
		if meterMeta.Dir == model.CONSUMER_DIRECTION {
			fmt.Printf("L:%.3d Meta: %+v I:%s D:%v\n", i, meterMeta, line.Id, line.Consumers[meterMeta.SourceIdx*3:(meterMeta.SourceIdx*3)+3])
		} else {
			fmt.Printf("L:%.3d ID: %s I:%.3d D:%v\n", i, meterMeta.Name, meterMeta.SourceIdx, line.Producers[meterMeta.SourceIdx*2:(meterMeta.SourceIdx*2)+2])
		}
	}
	return db.SetLines(lines)
}
