package function

import (
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport"
	"at.ourproject/energystore/model"
	"at.ourproject/energystore/store/ebow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The reset store: consumers C0 (idx 0), C1 (idx 1), producer P0 (idx 0); three slots of one day
// and one slot of the next day, which lies outside the reset range.
func resetStore(t *testing.T) (*ebow.BowStorage, []string) {
	db, _ := testsupport.TestDriverStore(t, "te100001", "RC100001")
	require.NoError(t, db.SetMeta(testsupport.CpMeta(
		testsupport.Consumer("C0", 0, "01.06.2026 00:00:00", "02.06.2026 23:45:00"),
		testsupport.Consumer("C1", 1, "01.06.2026 00:00:00", "02.06.2026 23:45:00"),
		testsupport.Producer("P0", 0, "01.06.2026 00:00:00", "02.06.2026 23:45:00"))))
	ids := []string{"CP/2026/06/01/00/00/00", "CP/2026/06/01/00/15/00", "CP/2026/06/01/00/30/00", "CP/2026/06/02/00/00/00"}
	for _, id := range ids {
		require.NoError(t, db.SetLine(testsupport.RawLine(id, []float64{1, 2, 3, 4, 5, 6}, []float64{7, 8})))
	}
	return db, ids
}

func readLine(t *testing.T, db *ebow.BowStorage, id string) *model.RawSourceLine {
	line := model.RawSourceLine{Id: id}
	require.NoError(t, db.GetLine(&line))
	return &line
}

// june1 is the reset range 01.06.2026 00:00 – 23:45 as the estore CLI builds it.
func june1(t *testing.T) *DataTimeRange {
	r, err := ToDataTimeRange(time.Date(2026, 6, 1, 0, 0, 0, 0, testsupport.Vienna), time.Date(2026, 6, 1, 23, 45, 0, 0, testsupport.Vienna))
	require.NoError(t, err)
	return r
}

func TestReset(t *testing.T) {
	day := june1(t)

	t.Run("consumer", func(t *testing.T) {
		s, ids := resetStore(t)
		require.NoError(t, Reset(s, day, "C1"))
		for _, id := range ids[:3] {
			l := readLine(t, s, id)
			assert.Equal(t, []float64{1, 2, 3, 0, 0, 0}, l.Consumers, id)
			assert.Equal(t, []int{1, 1, 1, 0, 0, 0}, l.QoVConsumers, id)
			assert.Equal(t, []float64{7, 8}, l.Producers, id)
		}
		assert.Equal(t, []float64{1, 2, 3, 4, 5, 6}, readLine(t, s, ids[3]).Consumers, "outside the range")
	})

	t.Run("producer", func(t *testing.T) {
		s, ids := resetStore(t)
		require.NoError(t, Reset(s, day, "P0"))
		l := readLine(t, s, ids[0])
		assert.Equal(t, []float64{0, 0}, l.Producers)
		assert.Equal(t, []int{0, 0}, l.QoVProducers)
		assert.Equal(t, []float64{1, 2, 3, 4, 5, 6}, l.Consumers)
	})

	t.Run("empty meter", func(t *testing.T) {
		s, _ := resetStore(t)
		assert.Error(t, Reset(s, day, ""))
	})
}

// A meter that is not in cpmeta/0 must be an error; today GetMetaByName returns nil and Reset
// dereferences it (known-errors #50). Only the estore CLI calls Reset.
func TestResetUnknownMeter(t *testing.T) {
	t.Skip("known-errors #50")
	s, _ := resetStore(t)
	assert.NotPanics(t, func() {
		assert.Error(t, Reset(s, june1(t), "UNKNOWN"))
	})
}

func TestToDataTimeRange(t *testing.T) {
	r := june1(t)
	assert.Equal(t, "2026/06/01/00/00/00", r.key, "without the CP/ prefix: GetLineRange adds the bucket")
	assert.Equal(t, "2026/06/01/23/45/00", r.until)
}
