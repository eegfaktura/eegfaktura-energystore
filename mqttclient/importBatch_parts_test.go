package mqttclient

import (
	"context"
	"fmt"
	"testing"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// budgetExecutor is the importer with a small decode budget per part.
type budgetExecutor struct {
	*TenantEnergyImporter
	budget int64
}

func (b budgetExecutor) ExecuteBatch(ctx context.Context, msgs []mqtt.Message) []bool {
	return b.executeParts(ctx, msgs, b.budget)
}

// A batch whose unpacked messages exceed the budget is imported in parts, one after the other: every message
// is stored, replied and acked once, and of two messages for the same quarter hours the later one wins.
func TestExecuteBatchInPartsWithinDecodeBudget(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(ackTenant, &MQTTStreamer{client: client})
	defer imp.Close()

	meter := func(i int) string { return fmt.Sprintf("AT00300000000000000000000000000%02d", i) }
	value := func(v float64) func(int) float64 { return func(int) float64 { return v } }
	var builders []*testsupport.CrMsgBuilder
	for i := 0; i < 5; i++ {
		builders = append(builders, periodMsg(meter(i), model.CONSUMER_DIRECTION, ackDay, ackDay.AddDate(0, 0, 1), value(1)))
	}
	builders = append(builders, periodMsg(meter(0), model.CONSUMER_DIRECTION, ackDay, ackDay.AddDate(0, 0, 1), value(2)))
	batch := batchOf(t, ackTenant, builders...)
	raw, err := decryptMessage(batch[0].Payload())
	require.NoError(t, err)

	// At the first reply, the last metering point is not stored yet: the first part is done before the rest.
	var lastStoredAtFirstReply *bool
	client.Gate = func(string) {
		if lastStoredAtFirstReply == nil {
			stored := len(ackStored(t, ackTenant, ackEcId, meter(4))) > 0
			lastStoredAtFirstReply = &stored
		}
	}
	executeBatch(context.Background(), ackTenant, budgetExecutor{imp, 2 * int64(len(raw))}, batch)

	require.NotNil(t, lastStoredAtFirstReply)
	assert.False(t, *lastStoredAtFirstReply, "imported in parts")
	assert.Len(t, client.PublishedMessages(), 6)
	for i, m := range batch {
		assert.Equal(t, 1, m.(*testsupport.Message).Acks(), "message %d", i)
	}
	for i := 0; i < 5; i++ {
		assert.Len(t, ackStored(t, ackTenant, ackEcId, meter(i)), 96)
	}
	got := ackStored(t, ackTenant, ackEcId, meter(0))
	testsupport.InDelta(t, 2, got[0].Value[0], "the later message (last part) wins")
}
