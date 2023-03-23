package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
	"sort"
	"time"
)

type MqttEnergyImporter struct{}

func NewMqttEnergyImporter() *MqttEnergyImporter {
	return &MqttEnergyImporter{}
}

func (mw *MqttEnergyImporter) Execute(msg mqtt.Message) {

	tenant := mqttclient.TopicType(msg.Topic()).Tenant()
	if len(tenant) == 0 {
		return
	}
	data := decodeMessage(msg.Payload())
	if data == nil {
		return
	}
	fmt.Printf("Execute Energy Data Message for Topic (%v)\n", tenant)
	err := importEnergy(tenant, data)
	if err != nil {
		glog.Error(err)
	}
}

func decodeMessage(msg []byte) *model.MqttEnergyResponse {
	m := model.MqttEnergyResponse{}
	err := json.Unmarshal(msg, &m)
	if err != nil {
		println(err.Error())
		return nil
	}
	return &m
}

func importEnergy(tenant string, data *model.MqttEnergyResponse) error {
	// GetMetaData from tenant

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	defaultDirection := utils.DetermineDirection(data.Message.Meter.MeteringPoint)

	var consumerCount int
	var producerCount int
	var metaCP *model.CounterPointMeta

	determineMeta := func() error {
		meta, info, err := store.GetMetaInfoMap(db, data.Message.Meter.MeteringPoint, defaultDirection)
		if err != nil {
			return err
		}

		consumerCount = info.ConsumerCount
		producerCount = info.ProducerCount

		metaCP = meta[data.Message.Meter.MeteringPoint]
		return nil
	}

	//// GetRawDataStructur from Period xxxx -> yyyy
	var _line model.RawSourceLine
	var resources map[string]*model.RawSourceLine = map[string]*model.RawSourceLine{}

	begin := time.UnixMilli(data.Message.Energy.Start)
	end := time.UnixMilli(data.Message.Energy.End)

	year, duration := utils.GetMonthDuration(begin, end)
	month := int(begin.Month())
	duration += 1

	if err := determineMeta(); err != nil {
		return err
	}

	y := year
	years := []int{year}
	for i := 0; i < duration; i++ {
		key := fmt.Sprintf("CP/%.4d/%.2d", y, ((month+i-1)%12)+1)
		iter := db.GetLinePrefix(key)
		for iter.Next(&_line) {
			l := _line.Copy(len(_line.Consumers))
			resources[_line.Id] = &l
		}
		if (month+i)%12 == 0 {
			y += 1
			years = append(years, y)
		}
	}
	// Update RawDataStructure
	glog.Infof("Len Loaded Resources %d", len(resources))

	meterCodeMeta := map[string]*model.MeterCodeMeta{}
	for i, d := range data.Message.Energy.Data {
		if meterMeta := utils.DecodeMeterCode(d.MeterCode, i); meterMeta != nil {
			meterCodeMeta[meterMeta.Type] = meterMeta
		}

	}

	///
	var updated []*model.RawSourceLine
	for _, v := range meterCodeMeta {
		updated, err = importEnergyValues(v, data.Message.Energy, metaCP, &year, consumerCount, producerCount, determineMeta)
		// Store updated RawDataStructure
		glog.Infof("Update CP %s energy values (%d) from %s to %s",
			data.Message.Meter.MeteringPoint,
			len(updated),
			time.UnixMilli(data.Message.Energy.Start).Format(time.RFC822),
			time.UnixMilli(data.Message.Energy.End).Format(time.RFC822))
		err = db.SetLines(updated)
		if err != nil {
			return err
		}
	}
	///

	for _, y := range years {
		if err = CalculateMonthlyDash(db, fmt.Sprintf("%d", y), CalculateEEG); err != nil {
			return err
		}
	}
	return nil
}

func importEnergyValues(
	meterCode *model.MeterCodeMeta,
	data model.MqttEnergy,
	metaCP *model.CounterPointMeta,
	year *int,
	consumerCount, producerCount int,
	determineMeta func() error) ([]*model.RawSourceLine, error) {

	var err error

	sort.Slice(data.Data[meterCode.SourceInData].Value, func(i, j int) bool {
		a := time.UnixMilli(data.Data[0].Value[i].From)
		b := time.UnixMilli(data.Data[0].Value[j].From)
		return a.Unix() < b.Unix()
	})

	var tablePrefix = fmt.Sprintf("CP-%s/", meterCode.Code)
	var resources = map[string]*model.RawSourceLine{}
	updated := []*model.RawSourceLine{}
	for _, v := range data.Data[meterCode.SourceInData].Value {
		t := time.UnixMilli(v.From)
		if t.Year() != *year {
			*year = t.Year()
			if err = determineMeta(); err != nil {
				return updated, err
			}
		}

		id, err := utils.ConvertUnixTimeToRowId(tablePrefix, time.UnixMilli(v.From))
		if err != nil {
			return updated, err
		}
		_, ok := resources[id]
		if !ok {
			resources[id] = &model.RawSourceLine{Id: id, Consumers: make([]float64, consumerCount), Producers: make([]float64, producerCount)}
		}

		switch metaCP.Dir {
		case model.CONSUMER_DIRECTION:
			resources[id].Consumers = utils.Insert(resources[id].Consumers, metaCP.SourceIdx, v.Value)
		case model.PRODUCER_DIRECTION:
			resources[id].Producers = utils.Insert(resources[id].Producers, metaCP.SourceIdx, v.Value)
		}
		updated = append(updated, resources[id])
	}
	return updated, nil
}
