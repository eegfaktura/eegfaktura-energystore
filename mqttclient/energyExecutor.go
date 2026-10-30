package mqttclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"at.ourproject/energystore/internal/logging/glog"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
)

type TenantEnergyImporter struct {
	Tenant   string
	db       map[string]*ebow.BowStorage
	dbMutx   sync.Mutex
	streamer *MQTTStreamer
	// replied: messages of the running batch whose reply went out; kept only when the batch panics, so the
	// message-by-message rerun does not reply them twice. Keys are the broker's messages (pointers).
	replied map[mqtt.Message]bool
}

func NewTenantEnergyImporter(tenant string, streamer *MQTTStreamer) *TenantEnergyImporter {
	return &TenantEnergyImporter{
		Tenant:   tenant,
		db:       make(map[string]*ebow.BowStorage),
		streamer: streamer,
		replied:  map[mqtt.Message]bool{},
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
	for _, b := range tmw.db {
		b.Close()
	}
	tmw.db = make(map[string]*ebow.BowStorage)
	glog.V(4).Infof("Closed Importer DB %s", tmw.Tenant)
}

// ensureDb liefert den geoeffneten Store der ecId und oeffnet ihn bei Bedarf. Schlaegt das
// Oeffnen fehl, wird nur der Eintrag dieser ecId entfernt: frueher wurde die ganze Map auf nil
// gesetzt, die naechste Nachricht des Mandanten schrieb in die nil-Map und brachte den Dienst
// zum Absturz, und die offenen Stores der anderen ecIds blieben haengen.
func (tmw *TenantEnergyImporter) ensureDb(ecId string) (*ebow.BowStorage, error) {
	tmw.dbMutx.Lock()
	defer tmw.dbMutx.Unlock()

	if st := tmw.db[ecId]; st != nil && st.IsOpen() {
		return st, nil
	}
	st, err := ebow.OpenStorage(tmw.Tenant, ecId)
	if err != nil {
		delete(tmw.db, ecId)
		return nil, err
	}
	tmw.db[ecId] = st
	return st, nil
}

// Execute decodes, stores and confirms one CR_MSG: ExecuteBatch with this one message, the path the
// dispatcher takes. It acks the message after ExecuteBatch returns (executeBatch), whatever the outcome (#19):
//   - stored: the cr_msg_history reply goes to the backend ("Energiedaten empfangen").
//   - store error (open store, meta record, a corrupt row, Badger write; could succeed later): ERROR log,
//     no reply. The data is not in the store, the gap shows in the check before billing (#148) and the
//     day is requested again. The message is still acked: mosquitto redelivers an unacked message only
//     when the session reconnects, and until then it holds one of the client's inflight slots
//     (max_inflight_messages, 20 by default) for every tenant.
//   - undecodable payload (Base64, gzip, larger than mqtt.maxDecompressedMB, JSON): permanent, ERROR log,
//     no reply.
//
// Only a message whose processing did not finish (crash, OOM, shutdown) stays unacked and is redelivered.
func (tmw *TenantEnergyImporter) Execute(msg mqtt.Message) {
	tmw.ExecuteBatch(context.Background(), []mqtt.Message{msg})
}

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

// maxImportErrors limits how many day block errors an import reports (the rest is counted), so the
// log line of a long message stays short.
const maxImportErrors = 5

// importErrors joins the errors of failed day blocks on one line; errors.Is/As see the listed ones,
// which always include a meta record error (store.ErrMetaRecord), also beyond the cap.
type importErrors struct {
	errs    []error
	omitted int
}

func (e *importErrors) Error() string {
	msgs := make([]string, len(e.errs))
	for i, err := range e.errs {
		msgs[i] = err.Error()
	}
	s := strings.Join(msgs, "; ")
	if e.omitted > 0 {
		s += fmt.Sprintf("; … %d more", e.omitted)
	}
	return s
}

func (e *importErrors) Unwrap() []error { return e.errs }

func (e *importErrors) add(err error) {
	// A meta record error ends the import and is always kept, also beyond the cap.
	if len(e.errs) < maxImportErrors || errors.Is(err, store.ErrMetaRecord) {
		e.errs = append(e.errs, err)
	} else {
		e.omitted++
	}
}

// err returns nil when no block failed.
func (e *importErrors) err() error {
	if len(e.errs) == 0 {
		return nil
	}
	return e
}

// blockError names the period of the failed day block.
func blockError(block *model.MqttEnergy, err error) error {
	return fmt.Errorf("store %s - %s: %w",
		time.UnixMilli(block.Start).Format(time.RFC3339),
		time.UnixMilli(block.End).Format(time.RFC3339), err)
}

// Import stores every day block of the message. A block that fails does not stop the others; the
// errors of the failed blocks are returned together (#19), so the message is not confirmed as stored.
// An error of the store as a whole (the meta record) ends the import at once: every further block
// would fail the same way.
func (tmw *TenantEnergyImporter) Import(data *model.MqttEnergyMessage) error {
	st, err := tmw.ensureDb(data.EcId)
	if err != nil {
		return fmt.Errorf("open store ecId=%s: %w", data.EcId, err)
	}

	errs := &importErrors{}
	for i := range data.Energy {

		groupedEnergy := SplitEnergyByDay(data.Energy[i])
		// Store day blocks sequentially: StoreEnergyV2 updates the shared cpmeta/0
		// record and may assign SourceIdx values for previously unknown metering points.
		for n := range groupedEnergy {
			if err := store.StoreEnergyV2(st, data.Meter.MeteringPoint, &groupedEnergy[n]); err != nil {
				glog.Errorf("Error storing Energy: %v (Metering-Point: %s)", err, data.Meter.MeteringPoint)
				errs.add(blockError(&groupedEnergy[n], err))
				if errors.Is(err, store.ErrMetaRecord) {
					return errs
				}
			}
		}
	}
	return errs.err()
}

func decodeMessage(msg []byte) *model.MqttEnergyMessage {
	m, _ := decodeMessageSize(msg)
	return m
}

// decodeMessageSize also returns the size of the unpacked JSON (the decoded message on the heap is several
// times larger).
func decodeMessageSize(msg []byte) (*model.MqttEnergyMessage, int64) {
	decompressed, err := decryptMessage(msg)
	if err != nil {
		glog.Errorf("Error decoding CR_MSG transport payload. %s", err.Error())
		return nil, 0
	}

	m := model.MqttEnergyMessage{}
	err = json.Unmarshal(decompressed, &m)
	if err != nil {
		glog.Errorf("Error decoding MQTT message. %s", err.Error())
		return nil, 0
	}
	return &m, int64(len(decompressed))
}

func decryptMessage(msg []byte) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(msg[:]))
	if err != nil {
		return nil, fmt.Errorf("decode Base64: %w", err)
	}

	decompressed, err := gunzipData(decoded)
	if err != nil {
		return nil, fmt.Errorf("decompress gzip: %w", err)
	}
	return decompressed, nil
}

// maxDecompressedKey limits how far a CR_MSG is unpacked (platform#181), so a small but highly compressed payload
// cannot fill the memory. A message carries one metering point and stays far below the default.
// Env: ENERGYSTORE_MQTT_MAXDECOMPRESSEDMB. Read once: messages of several tenants are decoded concurrently and
// viper is not safe for concurrent writes.
const (
	maxDecompressedKey       = "mqtt.maxDecompressedMB"
	defaultMaxDecompressedMB = 512
)

var (
	maxDecompressedOnce sync.Once
	maxDecompressed     int64
)

func maxDecompressedBytes() int64 {
	maxDecompressedOnce.Do(func() { maxDecompressed = readMaxDecompressedBytes(viper.GetViper()) })
	return maxDecompressed
}

// readMaxDecompressedBytes falls back to the default for a missing, non-numeric, non-positive or absurdly large
// value: a limit of 0 would drop every message.
func readMaxDecompressedBytes(v *viper.Viper) int64 {
	if !v.IsSet(maxDecompressedKey) {
		return defaultMaxDecompressedMB << 20
	}
	mb := v.GetInt64(maxDecompressedKey)
	if mb <= 0 || mb > 1<<20 {
		glog.Warningf("%s=%q is not a positive number of MB, using %d", maxDecompressedKey,
			v.GetString(maxDecompressedKey), defaultMaxDecompressedMB)
		return defaultMaxDecompressedMB << 20
	}
	return mb << 20
}

func gunzipData(data []byte) ([]byte, error) {
	return gunzipLimited(data, maxDecompressedBytes())
}

func gunzipLimited(data []byte, limit int64) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	gz, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	var out bytes.Buffer
	n, err := io.Copy(&out, io.LimitReader(gz, limit+1))
	if err != nil {
		return nil, err
	}
	if n > limit {
		return nil, fmt.Errorf("decompressed message larger than %d bytes (%s)", limit, maxDecompressedKey)
	}
	return out.Bytes(), nil
}
