package testsupport

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"at.ourproject/energystore/internal/testsupport/tz"
	"at.ourproject/energystore/model"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { tz.Main(m) }

// TestBuilders uses every helper once: a pool store and a test-driver store under t.TempDir(),
// the meta and raw-line builders, the CR_MSG wire encoding and the kWh tolerance.
func TestBuilders(t *testing.T) {
	var dir string
	t.Run("TempStore", func(t *testing.T) {
		db := TempStore(t, "TE100001", "RC100001")
		dir = viper.GetString("persistence.path")
		require.NoError(t, db.SetMeta(CpMeta(Consumer("AT001", 0, "01.06.2026 00:00:00", "01.06.2026 23:45:00"))))
		id := RowId(time.Date(2026, 6, 1, 0, 15, 0, 0, Vienna))
		require.Equal(t, "CP/2026/06/01/00/15/00", id)
		require.NoError(t, db.SetLine(RawLine(id, []float64{1.5, 0.5, 0.25}, nil)))

		meta, err := db.GetMeta("cpmeta/0")
		require.NoError(t, err)
		require.Len(t, meta.CounterPoints, 1)
		assert.Equal(t, model.CONSUMER_DIRECTION, meta.CounterPoints[0].Dir)
		line := model.RawSourceLine{Id: id}
		require.NoError(t, db.GetLine(&line))
		InDelta(t, 0.5, line.Consumers[1])
		assert.Equal(t, []int{1, 1, 1}, line.QoVConsumers)
		_, err = os.Stat(filepath.Join(dir, "te100001", "RC100001"))
		require.NoError(t, err, "the store lives under the temporary path")
	})
	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "t.TempDir() removed after the test")
	assert.NotEqual(t, dir, viper.GetString("persistence.path"), "persistence.path restored")

	t.Run("TestDriverStore", func(t *testing.T) {
		db, base := TestDriverStore(t, "excelsource", "ecid")
		require.NoError(t, db.SetLine(RawLineQoV("CP/2026/01/01/00/00/00", nil, []float64{2}, nil, []int{2})))
		_, err := os.Stat(filepath.Join(base, "excelsource", "ecid"))
		require.NoError(t, err)
	})

	t.Run("QuarterHours", func(t *testing.T) {
		assert.Len(t, QuarterHours(tz.SpringForward2026), 92)
		assert.Len(t, QuarterHours(tz.FallBack2026), 100)
		assert.Len(t, QuarterHours(time.Date(2026, 6, 1, 12, 0, 0, 0, Vienna)), 96)
	})

	t.Run("CrMsg", func(t *testing.T) {
		start := time.Date(2026, 6, 1, 0, 0, 0, 0, Vienna)
		b := CrMsg("RC100001", "AT001").Direction(model.CONSUMER_DIRECTION).
			Energy(start, "L1", map[model.MeterCodeValue][]float64{model.CODE_CON: {1, 2}, model.CODE_SHARE: {0.5, 0.5}})
		decoded, err := base64.StdEncoding.DecodeString(string(b.Payload(t)))
		require.NoError(t, err)
		gz, err := gzip.NewReader(bytes.NewReader(decoded))
		require.NoError(t, err)
		raw, err := io.ReadAll(gz)
		require.NoError(t, err)
		var got model.MqttEnergyMessage
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, *b.Message(), got)
		require.Len(t, got.Energy, 1)
		assert.Equal(t, start.Add(30*time.Minute).UnixMilli(), got.Energy[0].End, "exclusive end")
		assert.Equal(t, model.CODE_CON, got.Energy[0].Data[0].MeterCode, "codes sorted")
	})

	t.Run("InDelta", func(t *testing.T) {
		ft := &testing.T{}
		assert.True(t, InDelta(ft, 1.0, 1.0+KWhDelta/2))
		assert.False(t, InDelta(ft, 1.0, 1.0+KWhDelta*2))
	})
}
