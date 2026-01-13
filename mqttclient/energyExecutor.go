package mqttclient

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

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

	monitor := time.Now()
	glog.V(3).Infof("Execute Energy Data Message for Topic (%v)", tmw.Tenant)
	err := tmw.Import(data)
	if err != nil {
		glog.Errorf("%v tenant=%s", err, tmw.Tenant)
		return
	}
	glog.V(3).Infof("Execution finished in %d ms (%v)", time.Since(monitor).Milliseconds(), tmw.Tenant)
}

//func createDayGroups(data *model.MqttEnergyMessage) []model.MqttEnergy {
//	energyGroup := []model.MqttEnergy{}
//	for i := range data.Energy {
//		energy := data.Energy[i]
//		energyEntry := model.MqttEnergy{Start: energy.Start, End: energy.End, Data: make([]model.MqttEnergyData, 0)}
//		startDate := time.UnixMilli(energy.Start).AddDate(0, 0, 1).Truncate(24 * time.Hour).UnixMilli()
//		nn := 0
//		for n := range energy.Data {
//			groupData := model.MqttEnergyData{MeterCode: energy.Data[n].MeterCode, Value: make([]model.MqttEnergyValue, 0)}
//			for x := range energy.Data[n].Value {
//				if energy.Data[n].Value[x].To > startDate {
//					energyEntry.End = energy.Data[n].Value[x].To
//					energyGroup = append(energyGroup, energyEntry)
//					eData = model.MqttEnergyData{MeterCode: energy.Data[n].MeterCode, Value: make([]model.MqttEnergyValue, 0)}
//					nn = 0
//					startDate = time.UnixMilli(energy.Data[n].Value[x].To).AddDate(0, 0, 1).Truncate(24 * time.Hour).UnixMilli()
//				}
//				groupData.Value = append(groupData.Value, energy.Data[n].Value[x])
//			}
//			energyEntry.Data = append(energyEntry.Data, model.MqttEnergyData{MeterCode: energy.Data[n].MeterCode, Value: make([]model.MqttEnergyValue, 0)})
//		}
//	}
//	return energyGroup
//}

const daySeconds = int64(24 * 60 * 60 * 1000)

func dayStart(ts int64) int64 {
	t := time.UnixMilli(ts).In(time.Local)
	return time.Date(
		t.Year(), t.Month(), t.Day(),
		0, 0, 0, 0,
		t.Location(),
	).UnixMilli()
}

func SplitEnergyByDay(src model.MqttEnergy) []model.MqttEnergy {
	var result []model.MqttEnergy

	startDay := dayStart(src.Start)
	endDay := dayStart(src.End)

	for day := startDay; day <= endDay; day += daySeconds {
		dayStartTs := max(day, src.Start)
		dayEndTs := min(day+daySeconds, src.End)

		var dayData []model.MqttEnergyData

		for _, meter := range src.Data {
			var values []model.MqttEnergyValue

			for _, v := range meter.Value {
				// overlap check
				from := max(v.From, dayStartTs)
				to := min(v.To, dayEndTs)

				if from < to {
					values = append(values, model.MqttEnergyValue{
						From:   from,
						To:     to,
						Method: v.Method,
						Value:  v.Value, // see note below
					})
				}
			}

			if len(values) > 0 {
				dayData = append(dayData, model.MqttEnergyData{
					MeterCode: meter.MeterCode,
					Value:     values,
				})
			}
		}

		if len(dayData) > 0 {
			result = append(result, model.MqttEnergy{
				Start: dayStartTs,
				End:   dayEndTs - int64(15*60*1000),
				Data:  dayData,
			})
		}
	}

	return result
}

func (tmw *TenantEnergyImporter) Import(data *model.MqttEnergyMessage) error {
	tmw.ensureDb(data.EcId)

	if tmw.db == nil {
		return errors.New("db not initialized")
	}

	for i := range data.Energy {

		groupedEnergy := SplitEnergyByDay(data.Energy[i])
		var _wg = sync.WaitGroup{}
		for n := range groupedEnergy {
			_wg.Add(1)
			go func(e *model.MqttEnergy) {
				defer _wg.Done()
				if err := store.StoreEnergyV2(tmw.db, data.Meter.MeteringPoint, e); err != nil {
					glog.Errorf("Error storing Energy: %v (Metering-Point: %s)", err, data.Meter.MeteringPoint)
					return
				}
			}(&groupedEnergy[n])
		}
		_wg.Wait()

		//if err := store.StoreEnergyV2(tmw.db, data.Meter.MeteringPoint, &data.Energy[i]); err != nil {
		//	return err
		//}
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
