package calculation

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"context"
	"encoding/json"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
	"sort"
	"time"
)

type MqttMessage struct {
	data   *model.MqttEnergyMessage
	tenant string
}

type MqttEnergyImporter struct {
	msgChan chan MqttMessage
	ctx     context.Context
}

func NewMqttEnergyImporter(ctx context.Context) *MqttEnergyImporter {
	importer := &MqttEnergyImporter{msgChan: make(chan MqttMessage, 20), ctx: ctx}
	go importer.process()
	return importer
}

var gloablReceivedMsg int = 0

func (mw *MqttEnergyImporter) Execute(msg mqtt.Message) {
	gloablReceivedMsg = gloablReceivedMsg + 1
	tenant := mqttclient.TopicType(msg.Topic()).Tenant()
	if len(tenant) == 0 {
		return
	}
	data := decodeMessage(msg.Payload())
	if data == nil {
		return
	}

	mw.msgChan <- MqttMessage{data: data, tenant: tenant}
	fmt.Printf("Received Messages %d\n", gloablReceivedMsg)
	//msg.Ack()
}

var testCounter int64 = 0

func (mw *MqttEnergyImporter) process() {
	glog.Info("Start MQTT Queue")
	for {
		select {
		case msg := <-mw.msgChan:
			glog.Infof("Execute Energy Data Message for Topic (%v)\n", msg.tenant)
			err := importEnergyV2(msg.tenant, msg.data)
			if err != nil {
				glog.Error(err)
			}
			glog.Infof("Execution finished (%d)", testCounter)
			testCounter += 1
		case <-mw.ctx.Done():
			break
		}
	}
}

func decodeMessage(msg []byte) *model.MqttEnergyMessage {
	//m := model.MqttEnergyResponse{}
	m := model.MqttEnergyMessage{}
	err := json.Unmarshal(msg, &m)
	if err != nil {
		glog.Errorf("Error decoding MQTT message. %s", err.Error())
		return nil
	}
	return &m
}

func importEnergyV2(tenant string, data *model.MqttEnergyMessage) error {
	// GetMetaData from tenant

	db, err := store.OpenStorage(tenant)
	if err != nil {
		return err
	}
	defer func() { db.Close() }()

	defaultDirection := utils.ExamineDirection(data.Energy.Data)

	var consumerCount int
	var producerCount int
	var metaCP *model.CounterPointMeta

	determineMeta := func() error {
		meta, info, err := store.GetMetaInfoMap(db, data.Meter.MeteringPoint, defaultDirection)
		if err != nil {
			return err
		}

		consumerCount = info.ConsumerCount
		producerCount = info.ProducerCount

		metaCP = meta[data.Meter.MeteringPoint]
		return nil
	}

	//// GetRawDataStructur from Period xxxx -> yyyy
	if err := determineMeta(); err != nil {
		return err
	}

	meterCodeMeta := map[string]*model.MeterCodeMeta{}
	for i, d := range data.Energy.Data {
		if meterMeta := utils.DecodeMeterCode(d.MeterCode, i); meterMeta != nil {
			meterCodeMeta[meterMeta.Type] = meterMeta
		}
	}

	var resources map[string]*model.RawSourceLine = map[string]*model.RawSourceLine{}
	begin := time.UnixMilli(data.Energy.Start)
	end := time.UnixMilli(data.Energy.End)
	fetchSourceRange(db, "CP", begin, end, resources)

	///
	for _, v := range meterCodeMeta {
		resources, err = importEnergyValuesV2(v, data.Energy, metaCP, consumerCount, producerCount, resources)
		// Store updated RawDataStructure
		glog.Infof("Update CP %s energy values (%d) from %s to %s",
			data.Meter.MeteringPoint,
			len(resources),
			time.UnixMilli(data.Energy.Start).Format(time.RFC822),
			time.UnixMilli(data.Energy.End).Format(time.RFC822))
		if err != nil {
			return err
		}
	}
	///

	updated := make([]*model.RawSourceLine, len(resources))
	i := 0
	for _, v := range resources {
		updated[i] = v
		i += 1

		glog.V(4).Infof("Update Source Line %+v", v)
	}

	err = db.SetLines(updated)

	if c := updateMetaCP(metaCP, time.UnixMilli(data.Energy.Start), time.UnixMilli(data.Energy.End)); c {
		err = updateMeta(db, metaCP, data.Meter.MeteringPoint)
	}
	return nil
}

func importEnergyValuesV2(
	meterCode *model.MeterCodeMeta,
	data model.MqttEnergy,
	metaCP *model.CounterPointMeta,
	consumerCount, producerCount int,
	resources map[string]*model.RawSourceLine) (map[string]*model.RawSourceLine, error) {

	sort.Slice(data.Data[meterCode.SourceInData].Value, func(i, j int) bool {
		a := time.UnixMilli(data.Data[0].Value[i].From)
		b := time.UnixMilli(data.Data[0].Value[j].From)
		return a.Unix() < b.Unix()
	})

	var tablePrefix = "CP/"
	for _, v := range data.Data[meterCode.SourceInData].Value {
		id, err := utils.ConvertUnixTimeToRowId(tablePrefix, time.UnixMilli(v.From))
		if err != nil {
			return resources, err
		}
		_, ok := resources[id]
		if !ok {
			resources[id] = model.MakeRawSourceLine(id, consumerCount, producerCount) //&model.RawSourceLine{Id: id, Consumers: make([]float64, consumerCount), Producers: make([]float64, producerCount)}
		}

		switch metaCP.Dir {
		case model.CONSUMER_DIRECTION:
			resources[id].Consumers = utils.Insert(resources[id].Consumers, (metaCP.SourceIdx*3)+meterCode.SourceDelta, v.Value)
			resources[id].QoVConsumers = utils.InsertInt(resources[id].QoVConsumers, (metaCP.SourceIdx*3)+meterCode.SourceDelta, utils.CastQoVStringToInt(v.Method))
		case model.PRODUCER_DIRECTION:
			resources[id].Producers = utils.Insert(resources[id].Producers, (metaCP.SourceIdx*2)+meterCode.SourceDelta, v.Value)
			resources[id].QoVProducers = utils.InsertInt(resources[id].QoVProducers, (metaCP.SourceIdx*2)+meterCode.SourceDelta, utils.CastQoVStringToInt(v.Method))
		}
	}
	return resources, nil
}

func fetchSourceRange(db *store.BowStorage, key string, start, end time.Time, resources map[string]*model.RawSourceLine) {
	sYear, sMonth, sDay := start.Year(), int(start.Month()), start.Day()
	eYear, eMonth, eDay := end.Year(), int(end.Month()), end.Day()

	iter := db.GetLineRange(key, fmt.Sprintf("%.4d/%.2d/%.2d/", sYear, sMonth, sDay), fmt.Sprintf("%.4d/%.2d/%.2d/", eYear, eMonth, eDay))
	defer iter.Close()

	var _line model.RawSourceLine
	for iter.Next(&_line) {
		l := _line.Copy(len(_line.Consumers))
		resources[_line.Id] = &l
	}
}

func updateMetaCP(metaCP *model.CounterPointMeta, begin, end time.Time) bool {

	changed := false
	metaBegin := stringToTime(metaCP.PeriodStart, time.Now())
	metaEnd := stringToTime(metaCP.PeriodEnd, time.Unix(1, 0))

	if begin.Before(metaBegin) {
		metaCP.PeriodStart = dateToString(begin)
		changed = true
	}
	if end.After(metaEnd) {
		metaCP.PeriodEnd = dateToString(end)
		changed = true
	}

	return changed
}

func updateMeta(db *store.BowStorage, metaCP *model.CounterPointMeta, cp string) error {
	var err error
	var meta *model.RawSourceMeta
	if meta, err = db.GetMeta(fmt.Sprintf("cpmeta/%s", "0")); err == nil {
		for _, m := range meta.CounterPoints {
			if m.Name == cp {
				m.PeriodStart = metaCP.PeriodStart
				m.PeriodEnd = metaCP.PeriodEnd
				m.Count = metaCP.Count

				return db.SetMeta(meta)
			}
		}
	}
	return err
}

func dateToString(date time.Time) string {
	return fmt.Sprintf("%.2d.%.2d.%.4d %.2d:%.2d:%.4d", date.Day(), date.Month(), date.Year(), date.Hour(), date.Minute(), date.Second())
}

func stringToTime(date string, defaultValue time.Time) time.Time {
	var d, m, y, hh, mm, ss int
	if _, err := fmt.Sscanf(date, "%d.%d.%d %d:%d:%d", &d, &m, &y, &hh, &mm, &ss); err == nil {
		return time.Date(y, time.Month(m), d, hh, mm, ss, 0, time.UTC)
	}
	return defaultValue
}
