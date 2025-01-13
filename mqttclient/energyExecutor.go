package mqttclient

import (
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"encoding/json"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
)

type TenantEnergyImporter struct {
	Tenant string
	db     *ebow.BowStorage
}

func NewTenantEnergyImporter(tenant string) *TenantEnergyImporter {
	return &TenantEnergyImporter{
		Tenant: tenant,
		db:     nil,
	}
}

func (tmw *TenantEnergyImporter) Close() {
	if tmw.db != nil {
		tmw.db.Close()
	}
}

func (tmw *TenantEnergyImporter) Execute(msg mqtt.Message) {
	data := decodeMessage(msg.Payload())
	if data == nil {
		glog.Error("Data without content")
		return
	}

	if tmw.db == nil {
		var err error
		tmw.db, err = ebow.OpenStorage(tmw.Tenant, data.EcId)
		if err != nil {
			glog.Error(err)
			return
		}
	}

	glog.Infof("Execute Energy Data Message for Topic (%v)", tmw.Tenant)
	err := tmw.Import(data)
	if err != nil {
		glog.Error(err)
	}
	glog.Infof("Execution finished (%v)", tmw.Tenant)
}

func (tmw *TenantEnergyImporter) Import(data *model.MqttEnergyMessage) error {
	for i := range data.Energy {
		if err := store.StoreEnergyV2(tmw.db, data.Meter.MeteringPoint, &data.Energy[i]); err != nil {
			return err
		}
	}
	return nil
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
