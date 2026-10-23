package mqttclient

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
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

// Tests for #19: a CR_MSG is acked only after it has been processed, and a store error is not
// reported to the backend as stored. Ported in part from the branch improve-test-environment
// (M5); the day is in time.Local, the zone the importer splits days in.

const (
	ackTenant = "TE100001"
	ackEcId   = "RC100001"
	ackMeter  = "AT0030000000000000000000000000001"
	ackRoute  = "eda/response/+/protocol/cr_msg"
)

var ackDay = time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)

// ackCrMsg is one day of consumption for meter in ecId, value 0.01·(slot+1).
func ackCrMsg(ecId, meter string) *testsupport.CrMsgBuilder {
	v := make([]float64, 96)
	for i := range v {
		v[i] = float64(i+1) / 100
	}
	return testsupport.CrMsg(ecId, meter).Direction(model.CONSUMER_DIRECTION).
		Energy(ackDay, "L1", map[model.MeterCodeValue][]float64{model.CODE_CON: v})
}

func ackStored(t *testing.T, tenant, ecId, meter string) []store.RawData {
	t.Helper()
	resp, err := store.QueryRawData(tenant, ecId, ackDay, ackDay, []store.TargetMP{{MeteringPoint: meter}}, nil)
	require.NoError(t, err)
	if resp[meter] == nil {
		return nil
	}
	return resp[meter].Data
}

// corruptMeta writes an undecodable meta record cpmeta/0, so every StoreEnergyV2 of the store fails.
func corruptMeta(t *testing.T, dir, tenant, ecId string) {
	t.Helper()
	db, err := ebow.Open(filepath.Join(dir, strings.ToLower(tenant), ecId))
	require.NoError(t, err)
	type badMeta struct {
		Id            string `bow:"key"`
		CounterPoints string
	}
	require.NoError(t, db.Bucket("metadata").Put(badMeta{Id: "cpmeta/0", CounterPoints: "garbage"}))
	require.NoError(t, db.Close())
}

func newAckWorker(executor Executor) *TenantWorker {
	return &TenantWorker{tenant: ackTenant, JobChannel: make(chan mqtt.Message, 10), executor: executor,
		ctx: context.Background(), wg: &sync.WaitGroup{}}
}

func TestStreamerDisablesAutoAck(t *testing.T) {
	opts := clientOptions()
	assert.True(t, opts.AutoAckDisabled, "the worker acks after processing (#19)")
	assert.False(t, opts.CleanSession, "redelivery of unacked messages needs the persistent session")
}

func TestExecuteStoresAndReplies(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	b := ackCrMsg(ackEcId, ackMeter)
	imp.Execute(testsupport.CrMsgMessage(ackTenant, b.Payload(t)))

	assert.Len(t, ackStored(t, ackTenant, ackEcId, ackMeter), 96)
	pub := client.PublishedMessages()
	require.Len(t, pub, 1)
	assert.Equal(t, "eda/response/"+ackTenant+"/protocol/cr_msg_history", pub[0].Topic)
	var reply struct {
		Meter          model.EnergyMeter `json:"meter"`
		EcId           string            `json:"ecId"`
		ConversationId string            `json:"conversationId"`
	}
	require.NoError(t, json.Unmarshal(pub[0].Payload, &reply))
	assert.Equal(t, ackEcId, reply.EcId)
	assert.Equal(t, ackMeter, reply.Meter.MeteringPoint)
	assert.Equal(t, b.Message().ConversationId, reply.ConversationId)
}

func TestExecuteDropsUndecodablePayload(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	assert.NotPanics(t, func() {
		imp.Execute(testsupport.CrMsgMessage(ackTenant, []byte("not base64 %%%")))
		imp.Execute(testsupport.CrMsgMessage(ackTenant, testsupport.EncodeCrMsg(t, []byte(`{"ecId":`))))
	})
	assert.Empty(t, client.PublishedMessages(), "no reply for a message that was not stored")
}

// When the store fails (here: an undecodable meta record), Import must return the error and no
// "processed" reply may go to the backend (known-errors #19, F4).
func TestImportStoreErrorIsReturned(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	corruptMeta(t, dir, ackTenant, ackEcId)

	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()
	assert.Error(t, imp.Import(ackCrMsg(ackEcId, ackMeter).Message()))
	imp.Execute(testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, ackMeter).Payload(t)))
	assert.Empty(t, client.PublishedMessages())
}

// The ack comes after the data is stored and the reply is sent, never before.
func TestWorkerAcksAfterStore(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	msg := testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, ackMeter).Payload(t))
	var repliesAtAck int
	msg.OnAck = func() { repliesAtAck = len(client.PublishedMessages()) }

	newAckWorker(imp).execute(msg)

	assert.Equal(t, 1, msg.Acks())
	assert.Equal(t, 1, repliesAtAck, "reply (sent after the store) was already published at ack time")
	assert.Len(t, ackStored(t, ackTenant, ackEcId, ackMeter), 96)
}

// A payload that can never be decoded is acked (no poison loop) and gets no reply.
func TestWorkerAcksPermanentFailure(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	msg := testsupport.CrMsgMessage(ackTenant, []byte("not base64 %%%"))
	newAckWorker(imp).execute(msg)

	assert.Equal(t, 1, msg.Acks())
	assert.Empty(t, client.PublishedMessages())
}

// A store error is acked as well (an unacked message would hold an inflight slot until the next
// reconnect), but no reply is sent.
func TestWorkerAcksStoreErrorWithoutReply(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	corruptMeta(t, dir, ackTenant, ackEcId)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	msg := testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, ackMeter).Payload(t))
	newAckWorker(imp).execute(msg)

	assert.Equal(t, 1, msg.Acks())
	assert.Empty(t, client.PublishedMessages())
}

type panicExecutor struct{ executed bool }

func (p *panicExecutor) Execute(mqtt.Message) { p.executed = true; panic("boom") }
func (p *panicExecutor) Close()               {}

// A panic is recovered and the message acked afterwards, so it does not crash the service again
// after every restart.
func TestWorkerAcksAfterPanic(t *testing.T) {
	exec := &panicExecutor{}
	msg := testsupport.CrMsgMessage(ackTenant, []byte("x"))
	var executedAtAck bool
	msg.OnAck = func() { executedAtAck = exec.executed }

	assert.NotPanics(t, func() { newAckWorker(exec).execute(msg) })
	assert.Equal(t, 1, msg.Acks())
	assert.True(t, executedAtAck)
}

// Through the dispatcher: while the worker is still processing (held in the reply), the message
// is not acked; it is acked once the processing has finished.
func TestDispatcherAcksOnlyAfterExecute(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	client.Gate = func(string) {
		entered <- struct{}{}
		<-release
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewTopicDispatcher(ctx, ackRoute, &MQTTStreamer{client: client})

	msg := testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, ackMeter).Payload(t))
	client.Deliver(ackRoute, msg)

	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not reach the reply")
	}
	assert.Equal(t, 0, msg.Acks(), "not acked while the message is processed")
	close(release)

	select {
	case <-client.Notify:
	case <-time.After(10 * time.Second):
		t.Fatal("no reply")
	}
	require.Eventually(t, func() bool { return msg.Acks() == 1 }, 10*time.Second, 10*time.Millisecond)
	d.Close()
}
