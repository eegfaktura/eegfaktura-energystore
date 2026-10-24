package mqttclient

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"at.ourproject/energystore/internal/testsupport"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
)

// Panics in a batch (#19): the rerun one by one, replies and acks.

// panicOn wraps an executor and panics whenever a batch holds the payload poison.
type panicOn struct {
	Executor
	poison []byte
}

func (p panicOn) ExecuteBatch(ctx context.Context, msgs []mqtt.Message) []bool {
	for _, m := range msgs {
		if string(m.Payload()) == string(p.poison) {
			panic("boom")
		}
	}
	return p.Executor.ExecuteBatch(ctx, msgs)
}

// A panic in a batch costs only the message that panics: after it, the batch runs message by message; the
// other four are stored and replied, and every message is acked exactly once.
func TestBatchPanicLosesOnlyThatMessage(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	var batch []mqtt.Message
	var msgs []*testsupport.Message
	for i := 0; i < 5; i++ {
		m := testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, fmt.Sprintf("AT00300000000000000000000000000%02d", i)).Payload(t))
		batch, msgs = append(batch, m), append(msgs, m)
	}
	executeBatch(context.Background(), ackTenant, panicOn{Executor: imp, poison: msgs[2].Body}, batch)

	assert.Len(t, client.PublishedMessages(), 4)
	for i, m := range msgs {
		assert.Equal(t, 1, m.Acks(), "message %d", i)
		want := 96
		if i == 2 {
			want = 0
		}
		assert.Len(t, ackStored(t, ackTenant, ackEcId, fmt.Sprintf("AT00300000000000000000000000000%02d", i)), want, "message %d", i)
	}
}

// A panic while replying (here: the third reply) makes executeBatch run the batch again one by one; the
// messages replied before the panic are not replied a second time.
func TestBatchPanicInReplyRepliesOnce(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	var publishes int
	client.Gate = func(string) {
		if publishes++; publishes == 3 {
			panic("boom")
		}
	}
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	var batch []mqtt.Message
	var msgs []*testsupport.Message
	for i := 0; i < 5; i++ {
		m := testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, fmt.Sprintf("AT00300000000000000000000000000%02d", i)).Payload(t))
		batch, msgs = append(batch, m), append(msgs, m)
	}
	executeBatch(context.Background(), ackTenant, imp, batch)

	replies := map[string]int{}
	for _, p := range client.PublishedMessages() {
		for i := 0; i < 5; i++ {
			if mp := fmt.Sprintf("AT00300000000000000000000000000%02d", i); strings.Contains(string(p.Payload), mp) {
				replies[mp]++
			}
		}
	}
	for i, m := range msgs {
		assert.Equal(t, 1, m.Acks(), "message %d acks", i)
		assert.Equal(t, 1, replies[fmt.Sprintf("AT00300000000000000000000000000%02d", i)], "message %d replies", i)
	}
	assert.Empty(t, imp.replied, "nothing left over")
}

// Shutdown right after a panic in the reply loop: the messages replied before the panic are still acked
// (no second reply after a redelivery), the others stay unacked.
func TestBatchPanicThenStopAcksReplied(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var publishes int
	client.Gate = func(string) {
		if publishes++; publishes == 3 {
			cancel()
			panic("boom")
		}
	}
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	var batch []mqtt.Message
	var msgs []*testsupport.Message
	for i := 0; i < 5; i++ {
		m := testsupport.CrMsgMessage(ackTenant, ackCrMsg(ackEcId, fmt.Sprintf("AT00300000000000000000000000000%02d", i)).Payload(t))
		batch, msgs = append(batch, m), append(msgs, m)
	}
	executeBatch(ctx, ackTenant, imp, batch)

	acks := make([]int, len(msgs))
	for i, m := range msgs {
		acks[i] = m.Acks()
	}
	assert.Equal(t, []int{1, 1, 0, 0, 0}, acks)
	assert.Len(t, client.PublishedMessages(), 2)
	assert.Empty(t, imp.replied)
}
