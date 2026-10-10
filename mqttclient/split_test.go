package mqttclient

import (
	"encoding/base64"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// block builds one energy block of n quarter hours starting at start, value i+1 for slot i.
func block(start time.Time, n int) model.MqttEnergy {
	values := make([]float64, n)
	for i := range values {
		values[i] = float64(i + 1)
	}
	return testsupport.CrMsg("RC100001", "AT001").
		Energy(start, "L1", map[model.MeterCodeValue][]float64{model.CODE_CON: values}).Message().Energy[0]
}

// splitSummary returns the number of values, their sum and the local start of every day block.
func splitSummary(blocks []model.MqttEnergy) (count int, sum float64, starts []time.Time) {
	for _, b := range blocks {
		starts = append(starts, time.UnixMilli(b.Start).In(testsupport.Vienna))
		for _, d := range b.Data {
			for _, v := range d.Value {
				count++
				sum += v.Value
			}
		}
	}
	return
}

func gauss(n int) float64 { return float64(n*(n+1)) / 2 }

func TestSplitEnergyByDayNormalDays(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, testsupport.Vienna)
	blocks := SplitEnergyByDay(block(start, 2*96))

	count, sum, starts := splitSummary(blocks)
	require.Len(t, blocks, 2)
	assert.Equal(t, 192, count)
	testsupport.InDelta(t, gauss(192), sum)
	assert.Equal(t, start, starts[0])
	assert.Equal(t, start.AddDate(0, 0, 1), starts[1])
	assert.Equal(t, start.Add(95*15*time.Minute).UnixMilli(), blocks[0].End, "End is the start of the day's last slot")
}

func TestSplitEnergyByDayStartsInsideADay(t *testing.T) {
	start := time.Date(2026, 6, 1, 22, 0, 0, 0, testsupport.Vienna)
	blocks := SplitEnergyByDay(block(start, 16)) // 22:00 – 02:00

	count, sum, starts := splitSummary(blocks)
	require.Len(t, blocks, 2)
	assert.Equal(t, 16, count)
	testsupport.InDelta(t, gauss(16), sum)
	assert.Equal(t, start, starts[0])
	assert.Equal(t, time.Date(2026, 6, 2, 0, 0, 0, 0, testsupport.Vienna), starts[1])
}

// With an exclusive End at local midnight nothing is lost over the DST days (known-errors #42 traced).
func TestSplitEnergyByDayKeepsEveryValueOverDST(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start time.Time
	}{
		{"spring 28.–31.03.2026", time.Date(2026, 3, 28, 0, 0, 0, 0, testsupport.Vienna)},
		{"autumn 24.–27.10.2026", time.Date(2026, 10, 24, 0, 0, 0, 0, testsupport.Vienna)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			end := tc.start.AddDate(0, 0, 4)
			n := int(end.Sub(tc.start) / (15 * time.Minute))
			count, sum, _ := splitSummary(SplitEnergyByDay(block(tc.start, n)))
			assert.Equal(t, n, count)
			testsupport.InDelta(t, gauss(n), sum)
		})
	}
}

// After a DST switch the day blocks must still start at local midnight (F27).
func TestSplitEnergyByDayBlocksOnLocalDays(t *testing.T) {
	t.Skip("known-errors #42")
	for _, start := range []time.Time{
		time.Date(2026, 3, 28, 0, 0, 0, 0, testsupport.Vienna),
		time.Date(2026, 10, 24, 0, 0, 0, 0, testsupport.Vienna),
	} {
		end := start.AddDate(0, 0, 4)
		_, _, starts := splitSummary(SplitEnergyByDay(block(start, int(end.Sub(start)/(15*time.Minute)))))
		require.Len(t, starts, 4)
		for i, s := range starts {
			assert.Equal(t, start.AddDate(0, 0, i), s, "block %d", i)
		}
	}
}

// A message that does not end at local midnight after the autumn switch loses no value (F27).
func TestSplitEnergyByDayEndNotAtMidnight(t *testing.T) {
	t.Skip("known-errors #42")
	start := time.Date(2026, 10, 24, 0, 0, 0, 0, testsupport.Vienna)
	end := time.Date(2026, 10, 26, 23, 30, 0, 0, testsupport.Vienna)
	n := int(end.Sub(start) / (15 * time.Minute))
	count, sum, _ := splitSummary(SplitEnergyByDay(block(start, n)))
	assert.Equal(t, n, count)
	testsupport.InDelta(t, gauss(n), sum)
}

func TestDecodeMessage(t *testing.T) {
	b := testsupport.CrMsg("RC100001", "AT0030000000000000000000000000001").
		Energy(time.Date(2026, 6, 1, 0, 0, 0, 0, testsupport.Vienna), "L1",
			map[model.MeterCodeValue][]float64{model.CODE_CON: {0.5, 0.25}})
	valid := b.Payload(t)

	got := decodeMessage(valid)
	require.NotNil(t, got)
	assert.Equal(t, b.Message(), got)

	raw, err := base64.StdEncoding.DecodeString(string(valid))
	require.NoError(t, err)
	truncated := []byte(base64.StdEncoding.EncodeToString(raw[:len(raw)/2]))

	for name, payload := range map[string][]byte{
		"empty":           {},
		"bad base64":      []byte("%%%"),
		"truncated gzip":  truncated,
		"invalid JSON":    testsupport.EncodeCrMsg(t, []byte(`{"ecId":`)),
		"energy not list": testsupport.EncodeCrMsg(t, []byte(`{"energy":{"start":0}}`)),
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() { assert.Nil(t, decodeMessage(payload)) })
		})
	}
}
