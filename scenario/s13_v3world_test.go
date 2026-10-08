package scenario_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"at.ourproject/energystore/calculation"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/mqttclient"
	"at.ourproject/energystore/store"
	"at.ourproject/energystore/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// S13 — the eegfaktura-v3 world: CR_MSG payloads that v3's energy-mock generated and published, recorded off
// the broker (testdata/v3world/README.md): two communities, 2026-09-01 … 14 and the spring DST days, L1/L2/L3
// methods, gaps and a meter outage. Imported through the production MQTT importer; the oracle is the sum of
// the recorded values, computed here.

type v3Message struct {
	Topic   string                  `json:"topic"`
	Message model.MqttEnergyMessage `json:"message"`
}

// v3Tenant is the tenant the topic names (eda/response/<tenant>/protocol/cr_msg), as the dispatcher reads it.
func (m v3Message) v3Tenant() string { return strings.ToUpper(strings.Split(m.Topic, "/")[2]) }

func v3World(t *testing.T) []v3Message {
	t.Helper()
	f, err := os.Open("testdata/v3world/crmsg.jsonl.gz")
	require.NoError(t, err)
	defer f.Close()
	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	var out []v3Message
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		var m v3Message
		require.NoError(t, json.Unmarshal(sc.Bytes(), &m))
		out = append(out, m)
	}
	require.NoError(t, sc.Err())
	require.NotEmpty(t, out)
	return out
}

// importV3World imports every message with one importer per tenant, as the MQTT dispatcher does.
func importV3World(t *testing.T) []v3Message {
	t.Helper()
	community(t)
	msgs := v3World(t)
	importers := map[string]*mqttclient.TenantEnergyImporter{}
	for i := range msgs {
		tn := msgs[i].v3Tenant()
		if importers[tn] == nil {
			importers[tn] = mqttclient.NewTenantEnergyImporter(tn, &mqttclient.MQTTStreamer{})
		}
		require.NoError(t, importers[tn].Import(&msgs[i].Message))
	}
	for _, imp := range importers {
		imp.Close()
	}
	return msgs
}

// v3Sums: per tenant, meter and meter code the sum of the recorded values with from in [from, until).
func v3Sums(msgs []v3Message, from, until time.Time) map[string]map[string]map[model.MeterCodeValue]float64 {
	out := map[string]map[string]map[model.MeterCodeValue]float64{}
	for _, m := range msgs {
		for _, e := range m.Message.Energy {
			for _, d := range e.Data {
				for _, v := range d.Value {
					if v.From < from.UnixMilli() || v.From >= until.UnixMilli() {
						continue
					}
					tn, mp := m.v3Tenant(), m.Message.Meter.MeteringPoint
					if out[tn] == nil {
						out[tn] = map[string]map[model.MeterCodeValue]float64{}
					}
					if out[tn][mp] == nil {
						out[tn][mp] = map[model.MeterCodeValue]float64{}
					}
					out[tn][mp][d.MeterCode] += v.Value
				}
			}
		}
	}
	return out
}

func v3EcId(msgs []v3Message, tenant string) string {
	for _, m := range msgs {
		if m.v3Tenant() == tenant {
			return m.Message.EcId
		}
	}
	return ""
}

func isProducer(codes map[model.MeterCodeValue]float64) bool {
	_, gen := codes[model.CODE_PLUS]
	return gen
}

// near: sums of a month of quarter hours, so a relative tolerance on top of the kWh delta.
func near(t *testing.T, want, got float64, what string, args ...any) {
	t.Helper()
	assert.InDeltaf(t, want, got, 1e-6+1e-9*math.Abs(want), what, args...)
}

// Every recorded value is stored in its slot with its method as quality (L1 → 1, L2 → 2, L3 → 3).
func TestS13_V3WorldRawKeepsEveryValueAndMethod(t *testing.T) {
	msgs := importV3World(t)
	type key struct{ tenant, ecId, mp string }
	byMeter := map[key][]v3Message{}
	for _, m := range msgs {
		k := key{m.v3Tenant(), m.Message.EcId, m.Message.Meter.MeteringPoint}
		byMeter[k] = append(byMeter[k], m)
	}
	checked := 0
	for k, ms := range byMeter {
		first, last := ms[0].Message.Energy[0].Start, ms[0].Message.Energy[0].Start
		for _, m := range ms {
			first, last = min(first, m.Message.Energy[0].Start), max(last, m.Message.Energy[0].Start)
		}
		resp, err := store.QueryRawData(k.tenant, k.ecId, time.UnixMilli(first).In(vienna), time.UnixMilli(last).In(vienna),
			[]store.TargetMP{{MeteringPoint: k.mp}}, nil)
		require.NoError(t, err)
		require.NotNil(t, resp[k.mp], "%v", k)
		bySlot := map[int64]store.RawData{}
		for _, r := range resp[k.mp].Data {
			if r.Qov[0] != 0 || bySlot[r.Ts].Qov == nil { // a fill row (QoV 0) never hides a real one (known-errors #22)
				bySlot[r.Ts] = r
			}
		}
		for _, m := range ms {
			for _, e := range m.Message.Energy {
				for _, d := range e.Data {
					idx := codeIndex(d.MeterCode)
					for _, v := range d.Value {
						r, ok := bySlot[v.From]
						if !assert.True(t, ok, "%v slot %d missing", k, v.From) {
							continue
						}
						near(t, v.Value, r.Value[idx], "%v %s @%d", k, d.MeterCode, v.From)
						assert.Equal(t, utils.CastQoVStringToInt(v.Method), r.Qov[idx], "%v %s @%d qov", k, d.MeterCode, v.From)
						checked++
					}
				}
			}
		}
	}
	assert.Greater(t, checked, 100_000, "values checked")
}

// codeIndex: the column of a code in the raw row (consumer G.01, G.02, G.03; producer G.01, P.01).
func codeIndex(c model.MeterCodeValue) int {
	switch c {
	case model.CODE_SHARE, model.CODE_PLUS:
		return 1
	case model.CODE_COVER:
		return 2
	}
	return 0
}

// The participant report of September 2026 (the web's /report, the source of billing's allocations) sums the
// recorded values of every meter; the community summary sums them over all meters.
func TestS13_V3WorldReportAndSummarySumTheMessages(t *testing.T) {
	msgs := importV3World(t)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, vienna)
	sums := v3Sums(msgs, from, from.AddDate(0, 1, 0))
	require.Len(t, sums, 2)
	for tn, meters := range sums {
		ecId := v3EcId(msgs, tn)
		var parts []model.ParticipantReport
		mps := make([]string, 0, len(meters))
		for mp := range meters {
			mps = append(mps, mp)
		}
		sort.Strings(mps)
		for _, mp := range mps {
			dir := "CONSUMPTION"
			if isProducer(meters[mp]) {
				dir = "GENERATION"
			}
			parts = append(parts, model.ParticipantReport{ParticipantId: mp, Meters: []*model.MeterReport{
				{MeterId: mp, MeterDir: dir, From: from.UnixMilli(), Until: from.AddDate(0, 1, -1).UnixMilli()}}})
		}
		report, err := calculation.EnergyReportV2(tn, ecId, parts, 2026, 9, "YM")
		require.NoError(t, err, tn)
		var consumed, allocated, distributed, produced float64
		for _, p := range report.ParticipantReports {
			mp := p.ParticipantId
			s := p.Meters[0].Report.Summary
			c := meters[mp]
			if isProducer(c) {
				near(t, c[model.CODE_GEN], s.Production, "%s %s production", tn, mp)
				produced += c[model.CODE_GEN]
				continue
			}
			near(t, c[model.CODE_CON], s.Consumption, "%s %s consumption", tn, mp)
			near(t, c[model.CODE_SHARE], s.Allocation, "%s %s allocation (G.02)", tn, mp)
			near(t, c[model.CODE_COVER], s.Utilization, "%s %s utilization (G.03)", tn, mp)
			consumed += c[model.CODE_CON]
			allocated += c[model.CODE_SHARE]
			distributed += c[model.CODE_COVER]
		}
		require.Len(t, report.ParticipantReports, len(mps), tn)
		near(t, consumed, report.TotalConsumption, "%s total consumption", tn)

		summary, err := calculation.EnergySummary(tn, ecId, 2026, 9, "YM")
		require.NoError(t, err, tn)
		s := summary.([]interface{})[0].(*store.ReportData)
		near(t, consumed, s.Consumed, "%s consumed", tn)
		near(t, allocated, s.Allocated, "%s allocated", tn)
		near(t, distributed, s.Distributed, "%s distributed", tn)
		near(t, produced, s.Produced, "%s produced", tn)
	}
}

// springDay runs check on every meter of the v3 world's spring DST day: the slots the message delivered and
// the rows the store returns for the day.
func springDay(t *testing.T, check func(mp string, delivered int, rows []store.RawData)) {
	msgs := importV3World(t)
	day := time.Date(2026, 3, 29, 0, 0, 0, 0, vienna)
	meters := 0
	for _, m := range msgs {
		e := m.Message.Energy[0]
		if time.UnixMilli(e.Start).In(vienna).Format(time.DateOnly) != day.Format(time.DateOnly) {
			continue
		}
		mp := m.Message.Meter.MeteringPoint
		resp, err := store.QueryRawData(m.v3Tenant(), m.Message.EcId, day, day, []store.TargetMP{{MeteringPoint: mp}}, nil)
		require.NoError(t, err)
		require.NotNil(t, resp[mp], mp)
		delivered := 0
		for _, d := range e.Data {
			delivered = max(delivered, len(d.Value))
		}
		assert.LessOrEqual(t, delivered, 92, "%s delivers at most 92 slots", mp)
		check(mp, delivered, resp[mp].Data)
		meters++
	}
	assert.Greater(t, meters, 10, "meters on the spring day")
}

// Every quarter hour a meter delivered on the spring day comes back with its quality; fill rows (QoV 0) aside.
func TestS13_V3WorldSpringDayKeepsEveryDeliveredSlot(t *testing.T) {
	springDay(t, func(mp string, delivered int, rows []store.RawData) {
		real := 0
		for _, r := range rows {
			if r.Qov[0] != 0 {
				real++
			}
		}
		assert.Equal(t, delivered, real, "%s delivered slots", mp)
	})
}

// The spring day has 92 quarter hours: no phantom rows for 02:00–02:45 (known-errors #22, as S2).
func TestS13_V3WorldSpringDayHasNoPhantomSlots(t *testing.T) {
	t.Skip("known-errors #22")
	springDay(t, func(mp string, delivered int, rows []store.RawData) {
		assert.Len(t, rows, delivered, "%s stored slots", mp)
	})
}
