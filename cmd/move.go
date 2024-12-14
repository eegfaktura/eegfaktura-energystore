package cmd

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"at.ourproject/energystore/utils"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	RootCmd.AddCommand(moveCmd)
	moveCmd.Flags().StringVar(&meter, "meter", "",
		"Consider only the keys with specified prefix")
}

var moveCmd = &cobra.Command{
	Use:   "move",
	Short: "Change energy direction",
	Long: `
This command switch the energy direction of persisted meter.
`,
	RunE: handleMove,
}

func handleMove(cmd *cobra.Command, args []string) error {
	viper.Set("persistence.path", dir)

	fmt.Printf("%s, %s, %s, %s\n", viper.GetString("persistence.path"), tenant, meter, ecId)

	db, err := ebow.OpenStorage(tenant, ecId)
	if err != nil {
		return err
	}
	defer db.Close()

	sourceMeta, err := getMetaOf(db, meter)
	if err != nil {
		return err
	}
	targetMeta, err := getTargetMeta(db, meter, sourceMeta)
	if err != nil {
		return err
	}

	beginDate, endDate, err := determineTimeRangeOfMeter(sourceMeta)
	if err != nil {
		return err
	}

	sourceIdx := sourceMeta.SourceIdx
	targetIdx := targetMeta.SourceIdx

	fmt.Printf("Move B: %s E:%s (%d, %d)\n", beginDate, endDate, sourceIdx, targetIdx)
	fmt.Printf("Source Meta %v\n", sourceMeta)
	fmt.Printf("Target Meta %v\n", targetMeta)

	iter := db.GetLineRange("CP", beginDate, endDate)
	defer iter.Close()

	var _line model.RawSourceLine
	targetCache := []*model.RawSourceLine{}
	for iter.Next(&_line) {
		if sourceMeta.Dir == model.CONSUMER_DIRECTION {
			targetCache = append(targetCache, moveToProducer(_line, sourceIdx, targetIdx))
		}
	}

	if err := db.SetLines(targetCache); err != nil {
		return err
	}
	return updateMeta(db, meter, targetMeta)
}

func moveToProducer(line model.RawSourceLine, sourceIdx, targetIdx int) *model.RawSourceLine {
	targetLine := line.Copy(0)
	_sourceIdx := sourceIdx * 3
	_targetIdx := targetIdx * 2

	targetLine.Producers = utils.Insert(targetLine.Producers, _targetIdx, line.Consumers[_sourceIdx])
	targetLine.Producers = utils.Insert(targetLine.Producers, _targetIdx+1, line.Consumers[_sourceIdx+1])

	targetLine.Consumers = append(targetLine.Consumers[:_sourceIdx], targetLine.Consumers[MinOf(_sourceIdx+3, len(targetLine.Consumers)):]...)

	//resetValue(targetLine.Consumers, _sourceIdx)
	//resetValue(targetLine.Consumers, _sourceIdx+1)
	//resetValue(targetLine.Consumers, _sourceIdx+2)

	targetLine.QoVProducers = utils.InsertInt(targetLine.QoVProducers, _targetIdx, line.QoVConsumers[_sourceIdx])
	targetLine.QoVProducers = utils.InsertInt(targetLine.QoVProducers, _targetIdx+1, line.QoVConsumers[_sourceIdx+1])

	targetLine.QoVConsumers = append(targetLine.QoVConsumers[:_sourceIdx], targetLine.QoVConsumers[MinOf(_sourceIdx+3, len(targetLine.Consumers)):]...)

	//resetQoV(targetLine.QoVConsumers, _sourceIdx)
	//resetQoV(targetLine.QoVConsumers, _sourceIdx+1)
	//resetQoV(targetLine.QoVConsumers, _sourceIdx+2)

	return &targetLine
}

func getMetaOf(db *ebow.BowStorage, meter string) (*model.CounterPointMeta, error) {
	m, err := db.GetMeta("cpmeta/0")
	if err != nil {
		return nil, err
	}

	for _, cm := range m.CounterPoints {
		if cm.Name == meter {
			return cm, nil
		}
	}
	return nil, errors.New("meter not found")
}

func getTargetMeta(db *ebow.BowStorage, meterpoint string, source *model.CounterPointMeta) (*model.CounterPointMeta, error) {
	var direction model.MeterDirection
	if source.Dir == model.PRODUCER_DIRECTION {
		direction = model.CONSUMER_DIRECTION
	} else {
		direction = model.PRODUCER_DIRECTION
	}

	info, metaMap, err := store.CalcMetaInfo(db)
	if err != nil {
		return nil, err
	}

	var targetMeta *model.CounterPointMeta
	switch direction {
	case model.CONSUMER_DIRECTION:
		info.ConsumerCount += 1
		info.MaxConsumerIdx += 1
		targetMeta = &model.CounterPointMeta{
			ID:          fmt.Sprintf("%.3d", len(metaMap)+1),
			SourceIdx:   info.MaxConsumerIdx,
			Name:        meterpoint,
			Dir:         model.CONSUMER_DIRECTION,
			Count:       source.Count,
			PeriodStart: source.PeriodStart,
			PeriodEnd:   source.PeriodEnd,
		}
	case model.PRODUCER_DIRECTION:
		info.ProducerCount += 1
		info.MaxProducerIdx += 1
		targetMeta = &model.CounterPointMeta{
			ID:          fmt.Sprintf("%.3d", len(metaMap)+1),
			SourceIdx:   info.MaxProducerIdx,
			Name:        meterpoint,
			Dir:         model.PRODUCER_DIRECTION,
			Count:       source.Count,
			PeriodStart: source.PeriodStart,
			PeriodEnd:   source.PeriodEnd,
		}
	}
	return targetMeta, nil
}

func determineTimeRangeOfMeter(meta *model.CounterPointMeta) (string, string, error) {
	periodBeginDate := utils.StringToTime(meta.PeriodStart)
	lastEntryDate := utils.StringToTime(meta.PeriodEnd)

	beginPeriod, err := utils.ConvertUnixTimeToRowId("", periodBeginDate)
	if err != nil {
		return "", "", err
	}
	endPeriod, err := utils.ConvertUnixTimeToRowId("", lastEntryDate)
	if err != nil {
		return "", "", err
	}

	return beginPeriod, endPeriod, nil
}

func resetQoV(arr []int, idx int) {
	if len(arr) <= idx {
		return
	}
	arr[idx] = 0
}

func resetValue(arr []float64, idx int) {
	if len(arr) <= idx {
		return
	}
	arr[idx] = 0
}

func updateMeta(db *ebow.BowStorage, meter string, targetMeta *model.CounterPointMeta) error {
	m, err := db.GetMeta("cpmeta/0")
	if err != nil {
		return err
	}

	for _, cm := range m.CounterPoints {
		if cm.Name == meter {
			//cm.Name = fmt.Sprintf("Dummy_%s", meter)
			cm.Dir = targetMeta.Dir
			cm.Count = targetMeta.Count
			cm.PeriodStart = targetMeta.PeriodStart
			cm.PeriodEnd = targetMeta.PeriodEnd
			cm.SourceIdx = targetMeta.SourceIdx
			cm.ID = targetMeta.ID
		}
	}
	_ = reorganizeMeta(m)
	//m.CounterPoints = append(m.CounterPoints, targetMeta)
	return db.SetMeta(m)
}
