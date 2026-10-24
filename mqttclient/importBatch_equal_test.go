package mqttclient

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storedState returns the rows of the period as stored (one JSON per row, in key order, QoV arrays with their
// stored length) and the whole record cpmeta/0 (metering points sorted by name: the record lists them in map
// order) of tenant/ackEcId.
func storedState(t *testing.T, tenant string, from, until time.Time) (string, string) {
	t.Helper()
	db, err := ebow.OpenStorage(tenant, ackEcId)
	require.NoError(t, err)
	defer db.Close()
	iter := db.GetLineRange("CP", from.Format("2006/01/02/"), until.Format("2006/01/02/"))
	var rows []string
	for {
		var line model.RawSourceLine
		if !iter.Next(&line) {
			break
		}
		js, err := json.Marshal(line)
		require.NoError(t, err)
		rows = append(rows, string(js))
	}
	require.NoError(t, iter.Err())
	iter.Close()
	require.NotEmpty(t, rows)

	meta, err := db.GetMeta("cpmeta/0")
	require.NoError(t, err)
	sort.Slice(meta.CounterPoints, func(i, j int) bool { return meta.CounterPoints[i].Name < meta.CounterPoints[j].Name })
	js, err := json.Marshal(meta)
	require.NoError(t, err)
	return strings.Join(rows, ";"), string(js)
}

// ImportBatch of the messages stores the same rows and the same cpmeta/0 (ID, SourceIdx, Dir, periods) as
// Import of each message in the same order.
func TestImportBatchMatchesImport(t *testing.T) {
	const a, b, c = "AT0030000000000000000000000000A01", "AT0030000000000000000000000000B01", "AT0030000000000000000000000000C01"
	small := func(i int) float64 { return float64(i%96+1) / 1000 }
	half := func(int) float64 { return 0.5 }
	for _, tc := range []struct {
		name        string
		from, until time.Time
		msgs        func() []*model.MqttEnergyMessage
	}{{
		// The second message's new metering point has the earlier day: day order would register it first.
		name: "new metering points in message order", from: vienna(2026, 10, 1), until: vienna(2026, 10, 7),
		msgs: func() []*model.MqttEnergyMessage {
			return []*model.MqttEnergyMessage{
				periodMsg(a, model.CONSUMER_DIRECTION, vienna(2026, 10, 5), vienna(2026, 10, 6), small).Message(),
				periodMsg(b, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 10, 2), small).Message(),
				periodMsg(c, model.PRODUCER_DIRECTION, vienna(2026, 10, 3), vienna(2026, 10, 4), small).Message(),
				periodMsg(a, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 10, 5), half).Message(),
			}
		},
	}, {
		// A day, then a month over the DST change of the same metering point: SplitEnergyByDay cuts the month's
		// days after the change from 23:00, so its block for the 27th belongs to the 26th; the month must win.
		name: "month after a day over the DST change", from: vienna(2026, 10, 1), until: vienna(2026, 10, 31),
		msgs: func() []*model.MqttEnergyMessage {
			return []*model.MqttEnergyMessage{
				periodMsg(a, model.CONSUMER_DIRECTION, vienna(2026, 10, 27), vienna(2026, 10, 28), half).Message(),
				periodMsg(b, model.PRODUCER_DIRECTION, vienna(2026, 10, 25), vienna(2026, 10, 26), small).Message(),
				periodMsg(a, model.CONSUMER_DIRECTION, vienna(2026, 10, 1), vienna(2026, 11, 1), small).Message(),
				periodMsg(c, model.CONSUMER_DIRECTION, vienna(2026, 10, 26), vienna(2026, 10, 29), small).Message(),
			}
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.UseTempPersistence(t)
			batched := NewTenantEnergyImporter("TE100001", &MQTTStreamer{client: testsupport.NewFakeClient()})
			single := NewTenantEnergyImporter("TE100002", &MQTTStreamer{client: testsupport.NewFakeClient()})
			for i, err := range batched.ImportBatch(tc.msgs()) {
				require.NoError(t, err, "message %d", i)
			}
			for i, m := range tc.msgs() {
				require.NoError(t, single.Import(m), "message %d", i)
			}
			batched.Close()
			single.Close()

			wantRows, wantMeta := storedState(t, "TE100002", tc.from, tc.until)
			gotRows, gotMeta := storedState(t, "TE100001", tc.from, tc.until)
			assert.Equal(t, wantMeta, gotMeta, "cpmeta/0")
			assert.Equal(t, wantRows, gotRows, "rows")
		})
	}
}
