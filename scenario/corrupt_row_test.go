package scenario_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #31: a stored row that cannot be decoded. The day is in time.Local, the zone the importer
// writes the row ids in.

const (
	tenant    = "TE100001"
	ecId      = "RC100001"
	consumer  = "AT0030000000000000000000000000001"
	consumer2 = "AT0030000000000000000000000000002"
	badRowId  = "CP/2026/06/01/12/00/00"
)

var june1 = time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)

func community(t *testing.T) { t.Helper(); testsupport.UseTempPersistence(t) }

func values(n int, f func(i int) float64) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = f(i)
	}
	return v
}

// consumerMsg is one day of a consumer: G.01, G.02 and G.03, scaled by factor.
func consumerMsg(meter string, factor float64) *model.MqttEnergyMessage {
	return testsupport.CrMsg(ecId, meter).Direction(model.CONSUMER_DIRECTION).Energy(june1, "L1",
		map[model.MeterCodeValue][]float64{
			model.CODE_CON:   values(96, func(i int) float64 { return factor * float64(i+1) / 100 }),
			model.CODE_SHARE: values(96, func(i int) float64 { return factor * float64(i+1) / 400 }),
			model.CODE_COVER: values(96, func(i int) float64 { return factor * float64(i+1) / 800 }),
		}).Message()
}

// importMQTT imports CR_MSGs through the production importer and returns its first error.
func importMQTT(t *testing.T, msgs ...*model.MqttEnergyMessage) error {
	t.Helper()
	imp := mqttclient.NewTenantEnergyImporter(tenant, &mqttclient.MQTTStreamer{})
	defer imp.Close()
	for _, m := range msgs {
		if err := imp.Import(m); err != nil {
			return err
		}
	}
	return nil
}

// corruptRow overwrites the row id with a record that cannot be decoded as a RawSourceLine.
func corruptRow(t *testing.T, id string) {
	t.Helper()
	db, err := ebow.Open(filepath.Join(viper.GetString("persistence.path"), strings.ToLower(tenant), ecId))
	require.NoError(t, err)
	type badLine struct {
		Id        string `bow:"key"`
		Consumers string
	}
	require.NoError(t, db.Bucket("rawdata").Put(badLine{Id: id, Consumers: "garbage"}))
	require.NoError(t, db.Close())
}

func storedLine(t *testing.T, id string) model.RawSourceLine {
	t.Helper()
	db, err := ebow.OpenStorage(tenant, ecId)
	require.NoError(t, err)
	defer db.Close()
	l := model.RawSourceLine{Id: id}
	require.NoError(t, db.GetLine(&l))
	return l
}

// S11 (ported from improve-test-environment, known-errors #31): with one undecodable row the raw
// read and the report return an error instead of a truncated result.
func TestS11_CorruptRowIsAnError(t *testing.T) {
	community(t)
	require.NoError(t, importMQTT(t, consumerMsg(consumer, 1)))
	corruptRow(t, badRowId)

	_, err := store.QueryRawData(tenant, ecId, june1, june1, []store.TargetMP{{MeteringPoint: consumer}}, nil)
	assert.Error(t, err, "raw data")
	_, err = calculation.EnergyReportV2(tenant, ecId, []model.ParticipantReport{{ParticipantId: "C",
		Meters: []*model.MeterReport{{MeterId: consumer, MeterDir: "CONSUMPTION", From: june1.UnixMilli(), Until: june1.UnixMilli()}}}}, 2026, 6, "YM")
	assert.Error(t, err, "report")
	assert.True(t, errors.Is(err, ebow.ErrUndecodableRow), "got %v", err)
}

// On the write path a corrupt row aborts the store: before #31 the rows after it were missing from
// the read, rebuilt with the imported metering point only and written back, which dropped the
// values of the other metering points in those slots.
func TestImportOverCorruptRowAbortsWithoutOverwriting(t *testing.T) {
	community(t)
	require.NoError(t, importMQTT(t, consumerMsg(consumer, 1), consumerMsg(consumer2, 1)))
	before := storedLine(t, "CP/2026/06/01/12/15/00")
	beforeMorning := storedLine(t, "CP/2026/06/01/06/00/00")
	require.GreaterOrEqual(t, len(before.Consumers), 6, "both metering points in the row")

	corruptRow(t, badRowId)

	err := importMQTT(t, consumerMsg(consumer, 2))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ebow.ErrUndecodableRow), "got %v", err)

	assert.Equal(t, before, storedLine(t, "CP/2026/06/01/12/15/00"), "row after the corrupt one unchanged")
	assert.Equal(t, beforeMorning, storedLine(t, "CP/2026/06/01/06/00/00"), "nothing of the day written")
}
