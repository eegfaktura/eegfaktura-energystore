package store

import (
	"encoding/json"
	"errors"
	"math/rand"
	"path/filepath"
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

const (
	batchEcId = "RC100001"
	knownC0   = "AT0030000000000000000000000000C00"
	knownC1   = "AT0030000000000000000000000000C01"
	knownP0   = "AT0030000000000000000000000000P00"
	idleC2    = "AT0030000000000000000000000000I02" // known, no data in the test
	idleC3    = "AT0030000000000000000000000000I03"
	idleC4    = "AT0030000000000000000000000000I04"
	newC5     = "AT0030000000000000000000000000C05"
	newC6     = "AT0030000000000000000000000000C06"
	newP1     = "AT0030000000000000000000000000P01"
)

// 24.–26.10.2026; the 25th has 100 quarter hours (02:00–03:00 twice).
var batchDays = []time.Time{
	time.Date(2026, 10, 24, 0, 0, 0, 0, testsupport.Vienna),
	time.Date(2026, 10, 25, 0, 0, 0, 0, testsupport.Vienna),
	time.Date(2026, 10, 26, 0, 0, 0, 0, testsupport.Vienna),
}

type batchMsg struct {
	mp       string
	producer bool
	days     []int // indices into batchDays
	fromSlot int   // first quarter hour of each day (a partial day)
	tf       bool  // also the participation factor codes (stored after the others)
}

// batchMessages are the messages of the equality test in delivery order: known and new consumers and
// producers, month-like and single days, a partial day, the DST day. A new metering point first appears on
// a later day than one of a later message. Days 25/26 have no rows yet: the length of a row created by a
// block depends on the counts that block sees (visible where a new metering point has no value, newC6 before
// slot 40) and on the re-read between blocks (more consumers than producer slots: QoVProducers starts with
// the consumer count).
var batchMessages = []batchMsg{
	{mp: knownC0, days: []int{0, 1, 2}, tf: true},
	{mp: newP1, producer: true, days: []int{1, 2}, tf: true},
	{mp: newC6, days: []int{2}, fromSlot: 40},
	{mp: knownP0, producer: true, days: []int{0}},
	{mp: newC5, days: []int{0, 1}},
	{mp: knownC1, days: []int{1}, fromSlot: 40},
}

// dayBlocks builds, per day, the blocks of all messages in message order (as the executor groups them).
// The values are deterministic, so two calls give equal but separate data.
func dayBlocks() [][]DayBlock {
	rnd := rand.New(rand.NewSource(7))
	out := make([][]DayBlock, len(batchDays))
	for _, m := range batchMessages {
		codes := []model.MeterCodeValue{model.CODE_CON, model.CODE_SHARE, model.CODE_COVER}
		if m.tf {
			codes = append(codes, model.CODE_CON_TF, model.CODE_COVER_TF)
		}
		if m.producer {
			codes = []model.MeterCodeValue{model.CODE_GEN, model.CODE_PLUS}
			if m.tf {
				codes = append(codes, model.CODE_GEN_TF, model.CODE_PLUS_TF)
			}
		}
		for _, d := range m.days {
			slots := testsupport.QuarterHours(batchDays[d])[m.fromSlot:]
			e := &model.MqttEnergy{Start: slots[0].UnixMilli(), End: slots[len(slots)-1].UnixMilli()}
			for _, c := range codes {
				data := model.MqttEnergyData{MeterCode: c}
				for _, s := range slots {
					data.Value = append(data.Value, model.MqttEnergyValue{From: s.UnixMilli(),
						To: s.Add(15 * time.Minute).UnixMilli(), Method: "L1", Value: float64(rnd.Intn(1000)) / 1000})
				}
				e.Data = append(e.Data, data)
			}
			out[d] = append(out[d], DayBlock{MeteringPoint: m.mp, Energy: e})
		}
	}
	return out
}

// prefillBatchStore writes the known metering points and full rows for the 24th.
func prefillBatchStore(t *testing.T, db *ebow.BowStorage) {
	t.Helper()
	period := "24.10.2026 00:00:00"
	require.NoError(t, db.SetMeta(testsupport.CpMeta(
		testsupport.Consumer(knownC0, 0, period, period),
		testsupport.Consumer(knownC1, 1, period, period),
		testsupport.Consumer(idleC2, 2, period, period),
		testsupport.Consumer(idleC3, 3, period, period),
		testsupport.Consumer(idleC4, 4, period, period),
		testsupport.Producer(knownP0, 0, period, period))))
	var lines []*model.RawSourceLine
	for i, s := range testsupport.QuarterHours(batchDays[0]) {
		v := float64(i) / 100
		c := make([]float64, 15)
		for j := range c {
			c[j] = v
		}
		lines = append(lines, testsupport.RawLine(testsupport.RowId(s), c, []float64{v, v}))
	}
	require.NoError(t, db.SetLines(lines))
}

// storedRowsJSON returns the rows of the three days as stored, one JSON per row in key order. Not through
// fetchSourceRange: its copy evens out the lengths of the QoV arrays.
func storedRowsJSON(t *testing.T, db *ebow.BowStorage) (string, int) {
	t.Helper()
	iter := db.GetLineRange("CP", "2026/10/24/", "2026/10/26/")
	defer iter.Close()
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
	return strings.Join(rows, ";"), len(rows)
}

func storedMeta(t *testing.T, db *ebow.BowStorage) *model.RawSourceMeta {
	t.Helper()
	meta, err := db.GetMeta("cpmeta/0")
	require.NoError(t, err)
	// The record lists the metering points in map order (PrepareMetaInfoMap), so compare sorted.
	sort.Slice(meta.CounterPoints, func(i, j int) bool { return meta.CounterPoints[i].Name < meta.CounterPoints[j].Name })
	return meta
}

// A batch per day gives the same rows and the same meta as StoreEnergyV2 for each of its blocks in turn.
func TestStoreDayBatchMatchesSingle(t *testing.T) {
	single, _ := testsupport.TestDriverStore(t, "TE100001", batchEcId)
	batched, _ := testsupport.TestDriverStore(t, "TE100002", batchEcId)
	prefillBatchStore(t, single)
	prefillBatchStore(t, batched)

	for d, blocks := range dayBlocks() {
		for _, b := range blocks {
			require.NoError(t, StoreEnergyV2(single, b.MeteringPoint, b.Energy), "day %d %s", d, b.MeteringPoint)
		}
	}
	for d, blocks := range dayBlocks() {
		require.NoError(t, StoreDayBatch(batched, blocks), "day %d", d)
	}

	wantRows, n := storedRowsJSON(t, single)
	gotRows, _ := storedRowsJSON(t, batched)
	assert.Equal(t, 3*96, n, "rows of three days (the repeated hour shares its row ids)")
	assert.Equal(t, wantRows, gotRows, "rows")

	want, got := storedMeta(t, single), storedMeta(t, batched)
	assert.Equal(t, want, got, "meta")
	require.Len(t, got.CounterPoints, 9)
	idx := map[string]int{}
	for _, cp := range got.CounterPoints {
		idx[cp.Name] = cp.SourceIdx
	}
	assert.Equal(t, map[string]int{knownC0: 0, knownC1: 1, idleC2: 2, idleC3: 3, idleC4: 4, newC5: 5, newC6: 6,
		knownP0: 0, newP1: 1}, idx,
		"new metering points in the order of the blocks")
}

// A row that cannot be read aborts the batch before anything is written (#31); it is not a meta error.
func TestStoreDayBatchAbortsOnUnreadableRow(t *testing.T) {
	dir := t.TempDir()
	raw, err := ebow.Open(filepath.Join(dir, "TE100001", batchEcId))
	require.NoError(t, err)
	type badLine struct {
		Id        string `bow:"key"`
		Consumers string
	}
	require.NoError(t, raw.Bucket("rawdata").Put(badLine{Id: "CP/2026/10/24/12/00/00", Consumers: "garbage"}))
	require.NoError(t, raw.Close())
	db, err := ebow.OpenStorageTest("TE100001", batchEcId, dir)
	require.NoError(t, err)
	t.Cleanup(db.CloseTestDriver)

	err = StoreDayBatch(db, dayBlocks()[0])
	require.Error(t, err)
	assert.True(t, errors.Is(err, ebow.ErrUndecodableRow), "got %v", err)
	assert.False(t, errors.Is(err, ErrMetaRecord))
	assert.Contains(t, err.Error(), "read stored rows")

	line := &model.RawSourceLine{Id: "CP/2026/10/24/00/00/00"}
	assert.Error(t, db.GetLine(line), "no row of the day written")
}

// An undecodable meta record is a meta error.
func TestStoreDayBatchMetaError(t *testing.T) {
	dir := t.TempDir()
	raw, err := ebow.Open(filepath.Join(dir, "TE100001", batchEcId))
	require.NoError(t, err)
	type badMeta struct {
		Id            string `bow:"key"`
		CounterPoints string
	}
	require.NoError(t, raw.Bucket("metadata").Put(badMeta{Id: "cpmeta/0", CounterPoints: "garbage"}))
	require.NoError(t, raw.Close())
	db, err := ebow.OpenStorageTest("TE100001", batchEcId, dir)
	require.NoError(t, err)
	t.Cleanup(db.CloseTestDriver)

	err = StoreDayBatch(db, dayBlocks()[0])
	assert.True(t, errors.Is(err, ErrMetaRecord), "got %v", err)
}
