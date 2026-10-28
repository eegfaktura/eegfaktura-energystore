package scenario_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

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

// #31: a stored row that cannot be decoded. tenant, ecId, consumer, june1, community, values and
// storedLine come from helpers_test.go and s07_s12_test.go.

const (
	consumer2 = "AT0030000000000000000000000000002"
	badRowId  = "CP/2026/06/01/12/00/00"
)

// scaledConsumerMsg is one day of meter: G.01, G.02 and G.03, scaled by factor.
func scaledConsumerMsg(meter string, factor float64) *model.MqttEnergyMessage {
	return testsupport.CrMsg(ecId, meter).Direction(model.CONSUMER_DIRECTION).Energy(june1, "L1",
		map[model.MeterCodeValue][]float64{
			model.CODE_CON:   values(func(i int) float64 { return factor * float64(i+1) / 100 }, 96),
			model.CODE_SHARE: values(func(i int) float64 { return factor * float64(i+1) / 400 }, 96),
			model.CODE_COVER: values(func(i int) float64 { return factor * float64(i+1) / 800 }, 96),
		}).Message()
}

// importMQTTErr imports CR_MSGs through the production importer and returns its first error.
func importMQTTErr(t *testing.T, msgs ...*model.MqttEnergyMessage) error {
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

// S11 (ported from improve-test-environment, known-errors #31): with one undecodable row the raw
// read and the report return an error instead of a truncated result.
func TestS11_CorruptRowIsAnError(t *testing.T) {
	community(t)
	require.NoError(t, importMQTTErr(t, scaledConsumerMsg(consumer, 1)))
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
	require.NoError(t, importMQTTErr(t, scaledConsumerMsg(consumer, 1), scaledConsumerMsg(consumer2, 1)))
	before := storedLine(t, "CP/2026/06/01/12/15/00")
	beforeMorning := storedLine(t, "CP/2026/06/01/06/00/00")
	require.GreaterOrEqual(t, len(before.Consumers), 6, "both metering points in the row")

	corruptRow(t, badRowId)

	err := importMQTTErr(t, scaledConsumerMsg(consumer, 2))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ebow.ErrUndecodableRow), "got %v", err)

	assert.Equal(t, before, storedLine(t, "CP/2026/06/01/12/15/00"), "row after the corrupt one unchanged")
	assert.Equal(t, beforeMorning, storedLine(t, "CP/2026/06/01/06/00/00"), "nothing of the day written")
}
