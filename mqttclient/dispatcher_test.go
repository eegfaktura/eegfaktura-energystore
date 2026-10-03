package mqttclient

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	m5Tenant = "TE100001"
	m5EcId   = "RC100001"
	m5Meter  = "AT0030000000000000000000000000001"
	route    = "eda/response/+/protocol/cr_msg"
)

var m5Day = time.Date(2026, 6, 1, 0, 0, 0, 0, testsupport.Vienna)

// crMsg is one day of consumption for meter in ecId, value 0.01·(slot+1).
func crMsg(ecId, meter string) *testsupport.CrMsgBuilder {
	v := make([]float64, 96)
	for i := range v {
		v[i] = float64(i+1) / 100
	}
	return testsupport.CrMsg(ecId, meter).Direction(model.CONSUMER_DIRECTION).
		Energy(m5Day, "L1", map[model.MeterCodeValue][]float64{model.CODE_CON: v})
}

func stored(t *testing.T, tenant, ecId, meter string) []store.RawData {
	t.Helper()
	resp, err := store.QueryRawData(tenant, ecId, m5Day, m5Day, []store.TargetMP{{MeteringPoint: meter}}, nil)
	require.NoError(t, err)
	if resp[meter] == nil {
		return nil
	}
	return resp[meter].Data
}

// await waits for n cr_msg_history replies of the fake client (the end of n Execute calls).
func await(t *testing.T, c *testsupport.FakeClient, n int) []testsupport.Published {
	t.Helper()
	var got []testsupport.Published
	timeout := time.After(10 * time.Second)
	for len(got) < n {
		select {
		case p := <-c.Notify:
			got = append(got, p)
		case <-timeout:
			t.Fatalf("%d of %d replies after 10 s", len(got), n)
		}
	}
	return got
}

func TestExecuteStoresAndReplies(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: client})
	defer imp.Close()

	b := crMsg(m5EcId, m5Meter)
	imp.Execute(testsupport.CrMsgMessage(m5Tenant, b.Payload(t)))

	assert.Len(t, stored(t, m5Tenant, m5EcId, m5Meter), 96)
	pub := client.PublishedMessages()
	require.Len(t, pub, 1)
	assert.Equal(t, "eda/response/"+m5Tenant+"/protocol/cr_msg_history", pub[0].Topic)
	var reply struct {
		Meter          model.EnergyMeter `json:"meter"`
		EcId           string            `json:"ecId"`
		ConversationId string            `json:"conversationId"`
		MessageCode    string            `json:"messageCode"`
		Energy         []struct{ Start, End int64 }
	}
	require.NoError(t, json.Unmarshal(pub[0].Payload, &reply))
	assert.Equal(t, m5EcId, reply.EcId)
	assert.Equal(t, m5Meter, reply.Meter.MeteringPoint)
	assert.Equal(t, b.Message().ConversationId, reply.ConversationId)
	require.Len(t, reply.Energy, 1)
	assert.Equal(t, m5Day.UnixMilli(), reply.Energy[0].Start)
}

func TestExecuteDropsUndecodablePayload(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: client})
	defer imp.Close()

	assert.NotPanics(t, func() {
		imp.Execute(testsupport.CrMsgMessage(m5Tenant, []byte("not base64 %%%")))
		imp.Execute(testsupport.CrMsgMessage(m5Tenant, testsupport.EncodeCrMsg(t, []byte(`{"ecId":`))))
	})
	assert.Empty(t, client.PublishedMessages(), "no reply for a message that was not stored")
}

func TestImportIsIdempotent(t *testing.T) {
	testsupport.UseTempPersistence(t)
	imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: testsupport.NewFakeClient()})
	defer imp.Close()
	msg := crMsg(m5EcId, m5Meter).Message()
	require.NoError(t, imp.Import(msg))
	first := stored(t, m5Tenant, m5EcId, m5Meter)
	require.NoError(t, imp.Import(crMsg(m5EcId, m5Meter).Message()))
	assert.Equal(t, first, stored(t, m5Tenant, m5EcId, m5Meter))
}

// When the store fails (here: an undecodable meta record), Import must return the error and no
// "processed" reply may go to eda-xp (known-errors #19, F4: today the error is logged, Import
// returns nil and the reply is published).
func TestImportStoreErrorIsReturned(t *testing.T) {
	t.Skip("known-errors #19")
	dir := testsupport.UseTempPersistence(t)
	db, err := ebow.Open(filepath.Join(dir, strings.ToLower(m5Tenant), m5EcId))
	require.NoError(t, err)
	type badMeta struct {
		Id            string `bow:"key"`
		CounterPoints string
	}
	require.NoError(t, db.Bucket("metadata").Put(badMeta{Id: "cpmeta/0", CounterPoints: "garbage"}))
	require.NoError(t, db.Close())

	client := testsupport.NewFakeClient()
	imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: client})
	defer imp.Close()
	assert.Error(t, imp.Import(crMsg(m5EcId, m5Meter).Message()))
	imp.Execute(testsupport.CrMsgMessage(m5Tenant, crMsg(m5EcId, m5Meter).Payload(t)))
	assert.Empty(t, client.PublishedMessages())
}

// The dispatcher hands every tenant's messages to that tenant's importer.
func TestDispatcherRoutesPerTenant(t *testing.T) {
	testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewTopicDispatcher(ctx, route, &MQTTStreamer{client: client})

	client.Deliver(route, testsupport.CrMsgMessage("TE100001", crMsg("RC100001", m5Meter).Payload(t)))
	client.Deliver(route, testsupport.CrMsgMessage("TE100002", crMsg("RC100002", m5Meter).Payload(t)))
	await(t, client, 2)
	d.Close()

	assert.Len(t, stored(t, "TE100001", "RC100001", m5Meter), 96)
	assert.Len(t, stored(t, "TE100002", "RC100002", m5Meter), 96)
}
