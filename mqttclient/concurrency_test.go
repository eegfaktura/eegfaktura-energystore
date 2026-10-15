package mqttclient

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storeIsClosed opens the Badger directory directly: it succeeds only when no pool object holds it.
func storeIsClosed(t *testing.T, dir, tenant, ecId string) bool {
	t.Helper()
	db, err := ebow.Open(filepath.Join(dir, strings.ToLower(tenant), ecId))
	if err != nil {
		return false
	}
	_ = db.Close()
	return true
}

// 100 messages for 5 tenants, then Close: Close returns after every worker closed its store.
func TestDispatcherCloseAfterTraffic(t *testing.T) {
	dir := testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	d := NewTopicDispatcher(context.Background(), route, &MQTTStreamer{client: client})

	for i := 0; i < 100; i++ {
		tenant := fmt.Sprintf("TE10000%d", i%5)
		meter := fmt.Sprintf("AT00300000000000000000000000000%02d", i/5)
		client.Deliver(route, testsupport.CrMsgMessage(tenant, crMsg("RC100001", meter).Payload(t)))
	}
	await(t, client, 100)
	d.Close()

	for i := 0; i < 5; i++ {
		assert.True(t, storeIsClosed(t, dir, fmt.Sprintf("TE10000%d", i), "RC100001"), "tenant %d", i)
	}
}

// Close right after the first messages: TenantWorker.Run calls wg.Add inside its goroutine, so
// Close's wg.Wait can return before a worker started and closed its store (known-errors #41, F26).
func TestDispatcherCloseRightAfterStart(t *testing.T) {
	t.Skip("known-errors #41")
	dir := testsupport.UseTempPersistence(t)
	client := testsupport.NewFakeClient()
	d := NewTopicDispatcher(context.Background(), route, &MQTTStreamer{client: client})
	for i := 0; i < 5; i++ {
		client.Deliver(route, testsupport.CrMsgMessage(fmt.Sprintf("TE10000%d", i), crMsg("RC100001", m5Meter).Payload(t)))
	}
	d.Close()
	for i := 0; i < 5; i++ {
		assert.True(t, storeIsClosed(t, dir, fmt.Sprintf("TE10000%d", i), "RC100001"), "tenant %d: store still open after Close", i)
	}
}

// F18 (known-errors #33, reproduced in M5): two imports of two new metering points into one ecId at
// the same time must give them different SourceIdx and keep both. Today one meter is lost from
// cpmeta/0 in most runs (read-modify-write without a lock).
func TestConcurrentNewMeteringPoints(t *testing.T) {
	t.Skip("known-errors #33")
	testsupport.UseTempPersistence(t)
	meters := []string{"AT0030000000000000000000000000011", "AT0030000000000000000000000000012"}
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, len(meters))
	for i, m := range meters {
		wg.Add(1)
		go func(i int, m string) {
			defer wg.Done()
			imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: testsupport.NewFakeClient()})
			defer imp.Close()
			<-start
			errs[i] = imp.Import(crMsg(m5EcId, m).Message())
		}(i, m)
	}
	close(start)
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])

	meta, err := store.QueryMetaData(m5Tenant, m5EcId)
	require.NoError(t, err)
	require.Len(t, meta, 2)
	for _, m := range meters {
		data := stored(t, m5Tenant, m5EcId, m)
		require.Len(t, data, 96, m)
		testsupport.InDelta(t, 0.96, data[95].Value[0], m)
	}
}

// F19 (known-errors #34, reproduced in M5): a raw-data delete of meter A and an import of meter B on
// the same day at the same time: A outside the range and every value of B survive. Today the delete
// writes back the lines it read before the import: B's new values in the range are lost.
func TestDeleteDuringImport(t *testing.T) {
	t.Skip("known-errors #34")
	testsupport.UseTempPersistence(t)
	a, b := "AT0030000000000000000000000000021", "AT0030000000000000000000000000022"
	imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: testsupport.NewFakeClient()})
	require.NoError(t, imp.Import(crMsg(m5EcId, a).Message()))
	require.NoError(t, imp.Import(crMsg(m5EcId, b).Message())) // b known, so its index is stable
	imp.Close()

	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, _, err := store.DeleteRawDataForMeteringPoint(m5Tenant, m5EcId, a, m5Day.Add(10*time.Hour), m5Day.Add(11*time.Hour), false)
		assert.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		imp := NewTenantEnergyImporter(m5Tenant, &MQTTStreamer{client: testsupport.NewFakeClient()})
		defer imp.Close()
		<-start
		msg := crMsg(m5EcId, b).Message()
		for i := range msg.Energy[0].Data[0].Value {
			msg.Energy[0].Data[0].Value[i].Value *= 2
		}
		assert.NoError(t, imp.Import(msg))
	}()
	close(start)
	wg.Wait()

	da, db := stored(t, m5Tenant, m5EcId, a), stored(t, m5Tenant, m5EcId, b)
	require.Len(t, da, 96)
	require.Len(t, db, 96)
	for i := 0; i < 96; i++ {
		if i < 40 || i > 44 {
			testsupport.InDelta(t, float64(i+1)/100, da[i].Value[0], "A slot %d outside the range", i)
		}
		testsupport.InDelta(t, 2*float64(i+1)/100, db[i].Value[0], "B slot %d", i)
	}
}
