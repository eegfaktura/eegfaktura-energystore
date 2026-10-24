package mqttclient

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// periodMsg is one block of meter from start to end (exclusive) with value(slot) per quarter hour.
func periodMsg(meter string, dir model.MeterDirection, start, end time.Time, value func(int) float64) *testsupport.CrMsgBuilder {
	v := make([]float64, int(end.Sub(start)/(15*time.Minute)))
	for i := range v {
		v[i] = value(i)
	}
	codes := map[model.MeterCodeValue][]float64{model.CODE_CON: v}
	if dir == model.PRODUCER_DIRECTION {
		codes = map[model.MeterCodeValue][]float64{model.CODE_GEN: v, model.CODE_PLUS: v}
	}
	return testsupport.CrMsg(ackEcId, meter).Direction(dir).Energy(start, "L1", codes)
}

func batchOf(t *testing.T, tenant string, msgs ...*testsupport.CrMsgBuilder) []mqtt.Message {
	t.Helper()
	out := make([]mqtt.Message, len(msgs))
	for i, m := range msgs {
		out[i] = testsupport.CrMsgMessage(tenant, m.Payload(t))
	}
	return out
}

func vienna(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, testsupport.Vienna)
}

// ImportBatch stores what Import stores message by message, also for a day followed by a month over the DST
// change of the same metering point: SplitEnergyByDay cuts the month's days after the change from 23:00, so
// the month's block for the 27th belongs to the 26th, and the later message must still win.
func TestImportBatchMatchesImport(t *testing.T) {
	testsupport.UseTempPersistence(t)
	const a, b, c = "AT0030000000000000000000000000A01", "AT0030000000000000000000000000B01", "AT0030000000000000000000000000C01"
	small := func(i int) float64 { return float64(i%96+1) / 1000 }
	msgs := func() []*model.MqttEnergyMessage {
		return []*model.MqttEnergyMessage{
			periodMsg(a, model.CONSUMER_DIRECTION, vienna(2026, 10, 27), vienna(2026, 10, 28), func(int) float64 { return 0.5 }).Message(),
			periodMsg(b, model.PRODUCER_DIRECTION, vienna(2026, 10, 25), vienna(2026, 10, 26), small).Message(),
			periodMsg(a, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 11, 1), small).Message(),
			periodMsg(c, model.CONSUMER_DIRECTION, vienna(2026, 10, 26), vienna(2026, 10, 29), small).Message(),
		}
	}
	batched := NewTenantEnergyImporter("TE100001", &MQTTStreamer{client: testsupport.NewFakeClient()})
	defer batched.Close()
	single := NewTenantEnergyImporter("TE100002", &MQTTStreamer{client: testsupport.NewFakeClient()})
	defer single.Close()

	for i, err := range batched.ImportBatch(msgs()) {
		require.NoError(t, err, "message %d", i)
	}
	for i, m := range msgs() {
		require.NoError(t, single.Import(m), "message %d", i)
	}

	targets := []store.TargetMP{{MeteringPoint: a}, {MeteringPoint: b}, {MeteringPoint: c}}
	want, err := store.QueryRawData("TE100002", ackEcId, vienna(2026, 10, 1), vienna(2026, 10, 31), targets, nil)
	require.NoError(t, err)
	got, err := store.QueryRawData("TE100001", ackEcId, vienna(2026, 10, 1), vienna(2026, 10, 31), targets, nil)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	wantMeta, err := store.QueryMetaData("TE100002", ackEcId)
	require.NoError(t, err)
	gotMeta, err := store.QueryMetaData("TE100001", ackEcId)
	require.NoError(t, err)
	assert.Equal(t, wantMeta, gotMeta)

	day27 := vienna(2026, 10, 27).UnixMilli()
	for _, r := range got[a].Data {
		if r.Ts >= day27 && r.Ts < vienna(2026, 10, 28).UnixMilli() {
			require.Less(t, r.Value[0], 0.5, "the later month message wins at %d", r.Ts)
		}
	}
}

// Every stored message gets its reply; an undecodable one in the batch is skipped.
func TestExecuteBatchRepliesPerStoredMessage(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	meters := []string{"AT0030000000000000000000000000001", "AT0030000000000000000000000000002", "AT0030000000000000000000000000003"}
	batch := batchOf(t, ackTenant, ackCrMsg(ackEcId, meters[0]), ackCrMsg(ackEcId, meters[1]))
	batch = append(batch, testsupport.CrMsgMessage(ackTenant, []byte("not base64 %%%")))
	batch = append(batch, batchOf(t, ackTenant, ackCrMsg(ackEcId, meters[2]))...)
	imp.ExecuteBatch(batch)

	pub := client.PublishedMessages()
	require.Len(t, pub, 3)
	for i, m := range meters {
		assert.Contains(t, string(pub[i].Payload), m, "reply %d in message order", i)
		assert.Len(t, ackStored(t, ackTenant, ackEcId, m), 96)
	}
}

// A day that cannot be stored (a corrupt row on 2.6.) affects only the messages with that day; the others
// are stored and get their reply. A message with that day keeps its other days, as with Import.
func TestExecuteBatchDayErrorOnlyAffectsThatDay(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	db, err := ebow.Open(filepath.Join(dir, strings.ToLower(ackTenant), ackEcId))
	require.NoError(t, err)
	type badLine struct {
		Id        string `bow:"key"`
		Consumers string
	}
	require.NoError(t, db.Bucket("rawdata").Put(badLine{Id: "CP/2026/06/02/12/00/00", Consumers: "garbage"}))
	require.NoError(t, db.Close())

	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()
	const day1, day2, both = "AT0030000000000000000000000000011", "AT0030000000000000000000000000012", "AT0030000000000000000000000000013"
	one := func(int) float64 { return 1 }
	msgs := []*model.MqttEnergyMessage{
		periodMsg(day1, model.CONSUMER_DIRECTION, ackDay, ackDay.AddDate(0, 0, 1), one).Message(),
		periodMsg(day2, model.CONSUMER_DIRECTION, ackDay.AddDate(0, 0, 1), ackDay.AddDate(0, 0, 2), one).Message(),
		periodMsg(both, model.CONSUMER_DIRECTION, ackDay, ackDay.AddDate(0, 0, 3), one).Message(),
	}
	errs := imp.ImportBatch(msgs)
	assert.NoError(t, errs[0])
	for _, i := range []int{1, 2} {
		require.Error(t, errs[i], "message %d", i)
		assert.True(t, errors.Is(errs[i], ebow.ErrUndecodableRow), "message %d: %v", i, errs[i])
		assert.False(t, errors.Is(errs[i], store.ErrMetaRecord))
		assert.Contains(t, errs[i].Error(), "store 2026-06-02T00:00:00+02:00")
	}
	assert.Len(t, ackStored(t, ackTenant, ackEcId, day1), 96)
	assert.Len(t, ackStored(t, ackTenant, ackEcId, both), 96, "day 1 of the three-day message is stored")

	imp.ExecuteBatch(batchOf(t, ackTenant,
		periodMsg(day1, model.CONSUMER_DIRECTION, ackDay, ackDay.AddDate(0, 0, 1), one),
		periodMsg(day2, model.CONSUMER_DIRECTION, ackDay.AddDate(0, 0, 1), ackDay.AddDate(0, 0, 2), one)))
	pub := client.PublishedMessages()
	require.Len(t, pub, 1)
	assert.Contains(t, string(pub[0].Payload), day1)
}

// A meta record error hands the messages to Import, which reports it per message as before.
func TestImportBatchMetaErrorPerMessage(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	corruptMeta(t, dir, ackTenant, ackEcId)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	var msgs []*model.MqttEnergyMessage
	for i := 0; i < 3; i++ {
		msgs = append(msgs, ackCrMsg(ackEcId, fmt.Sprintf("AT00300000000000000000000000000%02d", i)).Message())
	}
	for i, err := range imp.ImportBatch(msgs) {
		require.Error(t, err, "message %d", i)
		assert.True(t, errors.Is(err, store.ErrMetaRecord), "message %d: %v", i, err)
		var ie *importErrors
		require.True(t, errors.As(err, &ie))
		assert.Len(t, ie.errs, 1, "as Import: one error, no further day tried")
	}
	imp.ExecuteBatch(batchOf(t, ackTenant, ackCrMsg(ackEcId, "AT0030000000000000000000000000001")))
	assert.Empty(t, client.PublishedMessages())
}
