package mqttclient

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keysOf lists the JSON keys of an object and, with a dot, of its nested objects and the first
// element of its arrays.
func keysOf(prefix string, v interface{}) []string {
	var out []string
	switch x := v.(type) {
	case map[string]interface{}:
		for k, e := range x {
			out = append(out, prefix+k)
			out = append(out, keysOf(prefix+k+".", e)...)
		}
	case []interface{}:
		if len(x) > 0 {
			out = append(out, keysOf(prefix, x[0])...)
		}
	}
	sort.Strings(out)
	return out
}

// TestCrMsgContract (M6): the payloads of eda-xp and of v3's energy-mock import into a store with
// their slots and values, and the cr_msg_history reply has exactly the shape v3 reads
// (contract/testdata/README.md names the sources).
func TestCrMsgContract(t *testing.T) {
	var historyFixture interface{}
	raw, err := os.ReadFile("../contract/testdata/v3/cr-msg-history.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &historyFixture))

	for _, tc := range []struct {
		name, file, tenant string
		slots              int
	}{
		{"eda-xp", "../test/energy-response-new-text.json", "RC101590", 96},
		{"v3 energy-mock", "../contract/testdata/v3/mock-cr-msg.json", "TE100001", 96},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testsupport.UseTempPersistence(t)
			payload, err := os.ReadFile(tc.file)
			require.NoError(t, err)
			var msg struct {
				EcId  string `json:"ecId"`
				Meter struct {
					MeteringPoint string `json:"meteringPoint"`
				} `json:"meter"`
				Energy []struct {
					Start int64 `json:"start"`
					Data  []struct {
						Value []struct{ Value float64 } `json:"value"`
					} `json:"data"`
				} `json:"energy"`
			}
			require.NoError(t, json.Unmarshal(payload, &msg))

			client := testsupport.NewFakeClient()
			imp := NewTenantEnergyImporter(tc.tenant, &MQTTStreamer{client: client})
			imp.Execute(testsupport.CrMsgMessage(tc.tenant, testsupport.EncodeCrMsg(t, payload)))
			imp.Close()

			day := time.UnixMilli(msg.Energy[0].Start)
			resp, err := store.QueryRawData(tc.tenant, msg.EcId, day, day, []store.TargetMP{{MeteringPoint: msg.Meter.MeteringPoint}}, nil)
			require.NoError(t, err)
			require.Contains(t, resp, msg.Meter.MeteringPoint)
			data := resp[msg.Meter.MeteringPoint].Data
			require.Len(t, data, tc.slots, "a day of quarter hours (read-side gap filling included, #22)")
			var sent, read float64
			for _, v := range msg.Energy[0].Data[0].Value {
				sent += v.Value
			}
			for _, d := range data {
				read += d.Value[0]
			}
			testsupport.InDelta(t, sent, read, "sum of the first meter code (G.01) as sent")

			pub := client.PublishedMessages()
			require.Len(t, pub, 1)
			var reply interface{}
			require.NoError(t, json.Unmarshal(pub[0].Payload, &reply))
			got := keysOf("", reply)
			for _, k := range []string{"meter.meteringPoint", "ecId", "conversationId", "energy.start", "energy.end"} {
				assert.Contains(t, got, k, "v3 CrMsgHistoryWriter reads %s", k)
			}
			assert.Subset(t, keysOf("", historyFixture), got, "every reply key is one v3's fixture knows (meter.direction is optional)")
		})
	}
}
