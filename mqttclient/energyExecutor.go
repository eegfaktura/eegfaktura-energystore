package mqttclient

import (
	"encoding/json"
	"errors"
	"sync"

	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang/glog"
)

type TenantEnergyImporter struct {
	Tenant string
	db     *ebow.BowStorage
	dbMutx sync.Mutex
}

func NewTenantEnergyImporter(tenant string) *TenantEnergyImporter {
	return &TenantEnergyImporter{
		Tenant: tenant,
		db:     nil,
	}
}

func (tmw *TenantEnergyImporter) Close() {
	glog.V(4).Infof("Close Importer %s", tmw.Tenant)
	tmw.closeDB()
	glog.V(4).Infof("Closed Importer %s", tmw.Tenant)
}

func (tmw *TenantEnergyImporter) closeDB() {
	tmw.dbMutx.Lock()
	defer tmw.dbMutx.Unlock()

	glog.V(4).Infof("Close Importer DB %s", tmw.Tenant)
	if tmw.db != nil {
		tmw.db.Close()
		tmw.db = nil
	}
	glog.V(4).Infof("Closed Importer DB %s", tmw.Tenant)
}

func (tmw *TenantEnergyImporter) ensureDb(ecId string) {
	tmw.dbMutx.Lock()
	defer tmw.dbMutx.Unlock()

	if tmw.db == nil || !tmw.db.IsOpen() {
		var err error
		tmw.db, err = ebow.OpenStorage(tmw.Tenant, ecId)
		if err != nil {
			glog.Errorf("%v tenant=%s", err, tmw.Tenant)
			tmw.db = nil
		}
	}
}

func (tmw *TenantEnergyImporter) Execute(msg mqtt.Message) {
	data := decodeMessage(msg.Payload())
	if data == nil {
		glog.Errorf("Data without content. tenant=%s", tmw.Tenant)
		return
	}

	glog.Infof("Execute Energy Data Message for Topic (%v)", tmw.Tenant)
	err := tmw.Import(data)
	if err != nil {
		glog.Errorf("%v tenant=%s", err, tmw.Tenant)
		return
	}
	glog.Infof("Execution finished (%v)", tmw.Tenant)
}

func (tmw *TenantEnergyImporter) Import(data *model.MqttEnergyMessage) error {
	tmw.ensureDb(data.EcId)

	if tmw.db == nil {
		return errors.New("db not initialized")
	}

	for i := range data.Energy {
		if err := store.StoreEnergyV2(tmw.db, data.Meter.MeteringPoint, &data.Energy[i]); err != nil {
			return err
		}
	}
	return nil
}

func decodeMessage(msg []byte) *model.MqttEnergyMessage {
	m := model.MqttEnergyMessage{}
	err := json.Unmarshal(msg, &m)
	if err != nil {
		glog.Errorf("Error decoding MQTT message. %s", err.Error())
		return nil
	}
	return &m
}
