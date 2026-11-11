package scenario_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/internal/testsupport/tz"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestMain(m *testing.M) { tz.Main(m) }

const (
	tenant   = "TE100001"
	ecId     = "RC100001"
	consumer = "AT0030000000000000000000000000001"
	producer = "AT0030000000000000000000030000001"
)

var vienna = testsupport.Vienna

// community points the persistence path at a fresh t.TempDir() for one scenario.
func community(t *testing.T) {
	t.Helper()
	testsupport.UseTempPersistence(t)
}

// series is the oracle of one meter code: value per quarter hour, f(i) for slot i of the block.
type series func(i int) float64

func cons(i int) float64  { return float64(i%96+1) / 100 }      // G.01: 0.01 … 0.96
func share(i int) float64 { return float64(i%96+1) / 400 }      // G.02: a quarter of it
func cover(i int) float64 { return float64(i%96+1) / 800 }      // G.03: an eighth
func prod(i int) float64  { return float64((i+48)%96+1) / 50 }  // G.01 producer
func surp(i int) float64  { return float64((i+48)%96+1) / 200 } // P.01 surplus

// values evaluates s over n slots.
func values(s series, n int) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = s(i)
	}
	return v
}

func sum(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s
}

// importMQTT imports one CR_MSG per metering point through the production importer.
func importMQTT(t *testing.T, msgs ...*model.MqttEnergyMessage) {
	t.Helper()
	imp := mqttclient.NewTenantEnergyImporter(tenant, &mqttclient.MQTTStreamer{})
	defer imp.Close()
	for _, m := range msgs {
		require.NoError(t, imp.Import(m))
	}
}

// consumerMsg / producerMsg build the CR_MSG of the test consumer/producer for n slots from start.
func consumerMsg(start time.Time, n int, method string) *model.MqttEnergyMessage {
	return testsupport.CrMsg(ecId, consumer).Direction(model.CONSUMER_DIRECTION).Energy(start, method,
		map[model.MeterCodeValue][]float64{model.CODE_CON: values(cons, n), model.CODE_SHARE: values(share, n), model.CODE_COVER: values(cover, n)}).Message()
}

func producerMsg(start time.Time, n int, method string) *model.MqttEnergyMessage {
	return testsupport.CrMsg(ecId, producer).Direction(model.PRODUCER_DIRECTION).Energy(start, method,
		map[model.MeterCodeValue][]float64{model.CODE_GEN: values(prod, n), model.CODE_PLUS: values(surp, n)}).Message()
}

// raw reads the stored slots of one metering point between two local days, as /eeg/v2/{ecid}/raw.
func raw(t *testing.T, meter string, from, until time.Time) []store.RawData {
	t.Helper()
	resp, err := store.QueryRawData(tenant, ecId, from, until, []store.TargetMP{{MeteringPoint: meter}}, nil)
	require.NoError(t, err)
	if resp[meter] == nil {
		return nil
	}
	return resp[meter].Data
}

// storedIds lists the row ids of the store in [from, until] (local days), as written on disk.
func storedIds(t *testing.T, day time.Time) []string {
	t.Helper()
	db, err := ebow.OpenStorage(tenant, ecId)
	require.NoError(t, err)
	defer db.Close()
	it := db.GetLinePrefix(fmt.Sprintf("CP/%04d/%02d/%02d/", day.Year(), int(day.Month()), day.Day()))
	defer it.Close()
	var ids []string
	var l model.RawSourceLine
	for it.Next(&l) {
		ids = append(ids, l.Id)
	}
	return ids
}

// excelSheet builds an EDA consumption report (the format of ImportExcelEnergyFileNew) for one
// consumer: header rows, then one row per timestamp text with G.01, G.02, G.03 and optional TF.
type excelRow struct {
	ts               string
	g1, g2, g3, g1TF string
}

func excelWorkbook(t *testing.T, direction string, withMeterCode bool, rows []excelRow) *excelize.File {
	t.Helper()
	f := excelize.NewFile()
	sh := "ConsumptionDataReport"
	_, err := f.NewSheet(sh)
	require.NoError(t, err)
	set := func(r int, vals ...string) {
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, r)
			require.NoError(t, f.SetCellStr(sh, cell, v))
		}
	}
	set(1, "MeteringpointID", consumer, consumer, consumer, consumer)
	set(2, "Energy direction", direction, direction, direction, direction)
	set(3, "Period start", "01.06.2026 00:00:00", "01.06.2026 00:00:00", "01.06.2026 00:00:00", "01.06.2026 00:00:00")
	set(4, "Period end", "31.12.2026 23:45:00", "31.12.2026 23:45:00", "31.12.2026 23:45:00", "31.12.2026 23:45:00")
	r := 5
	if withMeterCode {
		set(5, "Metercode",
			"Gesamtverbrauch lt. Messung (bei Teilnahme gem. Erzeugung) [KWH]",
			"Anteil gemeinschaftliche Erzeugung [KWH]",
			"Eigendeckung gemeinschaftliche Erzeugung [KWH]",
			"Verbrauch lt. Messung entsprechend dem Teilnahmefaktor je ZP und EC-ID [KWH]")
		r = 6
	}
	for i, row := range rows {
		set(r+i, row.ts, row.g1, row.g2, row.g3, row.g1TF)
	}
	return f
}

// importExcel runs the production Excel import (excel.ImportFile, as GraphQL singleUpload does).
func workbookBytes(t *testing.T, f *excelize.File) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, f.Write(&b))
	return &b
}
