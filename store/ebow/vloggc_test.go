package ebow

// Tests mit echtem Badger (konzept-energystore-vlog-gc.md, Akzeptanzkriterien). Laufen unter Linux;
// in CI ueber eine eigene Zeile mit -run TestVlogGC, weil der vorhandene Test "Test" in diesem Paket
// auf main rot ist. Die grosse Datei (1 GB) nur mit VLOGGC_BIG=1.

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wie cpmeta/0 einer grossen Gemeinschaft: ueber der ValueThreshold von 128 KB, landet im Value Log.
const vlogGCTestValue = 200 << 10

func vlogGCTestConfig(base string) VlogGCConfig {
	cfg := VlogGCConfig{
		Enabled: true, CheckEvery: time.Minute, Window: "00:00-23:59",
		DiscardRatio: 0.5, ProbeRatio: 0.1,
		MinVlogBytes: 1 << 20, MaxBytesPerRun: 100 << 30, BasePath: base,
	}
	if err := cfg.parse(); err != nil {
		panic(err)
	}
	return cfg
}

func vlogGCTestBase(t *testing.T) string {
	base := t.TempDir()
	viper.Set("persistence.path", base)
	return base
}

// fillVlog ueberschreibt einen grossen Schluessel wiederholt, mit Oeffnen und Schliessen dazwischen
// wie im Betrieb: jede Rueckgabe des letzten Pool-Platzes schliesst die Datenbank, das naechste
// Oeffnen beginnt eine neue Value-Log-Datei.
func fillVlog(t *testing.T, tenant, ecId string, cycles, writesPerCycle int) {
	t.Helper()
	val := make([]byte, vlogGCTestValue)
	for c := 0; c < cycles; c++ {
		st, err := OpenStorage(tenant, ecId)
		require.NoError(t, err)
		bdb := st.db.Badger()
		for i := 0; i < writesPerCycle; i++ {
			_, _ = rand.Read(val)
			require.NoError(t, bdb.Update(func(txn *badger.Txn) error { return txn.Set([]byte("cpmeta/0"), val) }))
		}
		st.Close()
	}
}

// flatten erzwingt die Kompaktierung und damit die Verwurfsstatistik. In Prod ist sie laut Diagnose
// vom 11.09.2026 bereits zu 100 % verbucht; der Test stellt denselben Ausgangszustand her.
func flatten(t *testing.T, tenant, ecId string) {
	t.Helper()
	st, err := OpenStorage(tenant, ecId)
	require.NoError(t, err)
	require.NoError(t, st.db.Badger().Flatten(1))
	st.Close()
}

func refFor(base, tenant, ecId string) vlogRef {
	dir := filepath.Join(base, tenant, ecId)
	b, _, _ := vlogBytes(dir)
	return vlogRef{tenant: tenant, ecId: ecId, dir: dir, vlogBytes: b}
}

func TestVlogGCReclaimsValueLog(t *testing.T) {
	base := vlogGCTestBase(t)
	tenant, ecId := "tegc0001", "ECIDVLOGGC0001"
	fillVlog(t, tenant, ecId, 12, 40) // 12 Dateien zu ~8 MB, wie im Prod-Mittel
	flatten(t, tenant, ecId)

	ref := refFor(base, tenant, ecId)
	info := discardCandidate(ref.dir, 0)
	t.Logf("vorher: %d MB Value Log, Kandidat fid %d Verhaeltnis %.2f", ref.vlogBytes>>20, info.fid, info.ratio)
	require.Greater(t, ref.vlogBytes, int64(50<<20))

	budget := int64(100 << 30)
	started := time.Now()
	res := collectDatabase(context.Background(), vlogGCTestConfig(base), ref, time.Now().Add(time.Hour), &budget)
	elapsed := time.Since(started)

	after, _, _ := vlogBytes(ref.dir)
	require.NoError(t, res.err)
	assert.False(t, res.skipped)
	assert.Greater(t, res.rewrites, 5)
	assert.Less(t, after, ref.vlogBytes/4, "Value Log muss deutlich schrumpfen")
	assert.Greater(t, res.freed, ref.vlogBytes/2)
	t.Logf("nachher: %d MB, %d Umschreibungen, %d MB gelesen, %v gesamt, %v je Aufruf",
		after>>20, res.rewrites, res.read>>20, elapsed.Round(time.Millisecond),
		(elapsed / time.Duration(res.rewrites)).Round(time.Millisecond))
}

// Zeit je Aufruf fuer eine 1-GB-Datei (Dateien aus der Zeit vor der Rotation, 862 davon in Prod).
func TestVlogGCBigFile(t *testing.T) {
	if os.Getenv("VLOGGC_BIG") == "" {
		t.Skip("VLOGGC_BIG=1 setzen: schreibt rund 1 GB")
	}
	base := vlogGCTestBase(t)
	tenant, ecId := "tegc0002", "ECIDVLOGGC0002"
	fillVlog(t, tenant, ecId, 1, 5000) // eine Datei um 1 GB
	// Danach weitere Sitzungen: die grosse Datei ist nicht mehr die aktuelle, und mit mehr als
	// NumLevelZeroTables (4) L0-Tabellen kompaktiert Badger -- sonst findet Flatten nichts.
	fillVlog(t, tenant, ecId, 8, 5)
	flatten(t, tenant, ecId)

	ref := refFor(base, tenant, ecId)
	cand := discardCandidate(ref.dir, 0)
	t.Logf("Kandidat fid %d, %d MB, Verhaeltnis %.2f", cand.fid, cand.size>>20, cand.ratio)

	st, err := OpenStorage(tenant, ecId)
	require.NoError(t, err)
	defer st.Close()
	started := time.Now()
	require.NoError(t, st.db.Badger().RunValueLogGC(0.5))
	t.Logf("ein Aufruf auf %d MB: %v", cand.size>>20, time.Since(started).Round(time.Millisecond))
}

// Ein offener Iterator (z. B. ein laufender Export) schiebt das Loeschen umgeschriebener Dateien auf.
func TestVlogGCDeferredWithOpenIterator(t *testing.T) {
	base := vlogGCTestBase(t)
	tenant, ecId := "tegc0003", "ECIDVLOGGC0003"
	fillVlog(t, tenant, ecId, 8, 40)
	flatten(t, tenant, ecId)
	ref := refFor(base, tenant, ecId)

	st, err := OpenStorage(tenant, ecId) // haelt die Datenbank offen wie ein Export
	require.NoError(t, err)
	txn := st.db.Badger().NewTransaction(false)
	it := txn.NewIterator(badger.DefaultIteratorOptions)

	budget := int64(100 << 30)
	res := collectDatabase(context.Background(), vlogGCTestConfig(base), ref, time.Now().Add(time.Hour), &budget)
	require.NoError(t, res.err)
	assert.Greater(t, res.rewrites, 0)
	// Nicht exakt gleich: das Oeffnen beginnt eine neue aktuelle Datei, die vorige zaehlt wieder mit.
	during, _, _ := vlogBytes(ref.dir)
	assert.Greater(t, during, ref.vlogBytes*3/4, "bei offenem Iterator bleiben die Dateien liegen")

	it.Close()
	txn.Discard()
	afterIter, _, _ := vlogBytes(ref.dir)
	assert.Less(t, afterIter, ref.vlogBytes/4, "nach dem Schliessen des Iterators werden sie geloescht")
	st.Close()
}

// Der Pool ist nur nach ecId geschluesselt: liefert er eine andere Datenbank, wird uebersprungen.
func TestVlogGCDirectoryCheck(t *testing.T) {
	base := vlogGCTestBase(t)
	ecId := "ECIDVLOGGC0004"
	fillVlog(t, "tegcaaaa", ecId, 3, 40) // Pool-Eintrag fuer ecId mit Tenant tegcaaaa

	other, err := OpenStorageTest("tegcbbbb", ecId, base) // gleiche ecId, anderes Verzeichnis, am Pool vorbei
	require.NoError(t, err)
	require.NoError(t, other.db.Badger().Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("cpmeta/0"), make([]byte, vlogGCTestValue))
	}))
	other.CloseTestDriver()

	ref := refFor(base, "tegcbbbb", ecId) // bewusst ohne sharedEcId: hier greift die Verzeichnispruefung
	budget := int64(100 << 30)
	res := collectDatabase(context.Background(), vlogGCTestConfig(base), ref, time.Now().Add(time.Hour), &budget)
	assert.True(t, res.skipped)
	assert.Equal(t, 0, res.rewrites)
}

// Mehrfach vergebene ecId: der Lauf legt nie selbst einen Pool-Eintrag an.
func TestVlogGCSharedEcIdNeverPinsPool(t *testing.T) {
	base := vlogGCTestBase(t)
	ecId := "ECIDVLOGGC0005"
	for _, tenant := range []string{"tegccccc", "tegcdddd"} {
		st, err := OpenStorageTest(tenant, ecId, base)
		require.NoError(t, err)
		require.NoError(t, st.db.Badger().Update(func(txn *badger.Txn) error {
			return txn.Set([]byte("cpmeta/0"), make([]byte, vlogGCTestValue))
		}))
		st.CloseTestDriver()
	}
	refs, err := enumerateVlogDBs(base)
	require.NoError(t, err)
	require.Len(t, refs, 2)
	budget := int64(100 << 30)
	for _, ref := range refs {
		assert.True(t, ref.sharedEcId)
		res := collectDatabase(context.Background(), vlogGCTestConfig(base), ref, time.Now().Add(time.Hour), &budget)
		assert.True(t, res.skipped, ref.dir)
	}
	exists, _ := connectionPool.holdsEntry(ecId, "tegccccc")
	assert.False(t, exists, "der Lauf darf keinen Pool-Eintrag anlegen")

	// Haelt der Pool die ecId bereits unter einem der Tenants, wird genau dieser bearbeitet.
	st, err := OpenStorage("tegccccc", ecId)
	require.NoError(t, err)
	st.Close()
	for _, ref := range refs {
		res := collectDatabase(context.Background(), vlogGCTestConfig(base), ref, time.Now().Add(time.Hour), &budget)
		assert.Equal(t, ref.tenant != "tegccccc", res.skipped, ref.dir)
	}
}

func TestVlogGCStopsOnCancel(t *testing.T) {
	base := vlogGCTestBase(t)
	tenant, ecId := "tegc0006", "ECIDVLOGGC0006"
	fillVlog(t, tenant, ecId, 4, 40)
	flatten(t, tenant, ecId)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	budget := int64(100 << 30)
	res := collectDatabase(ctx, vlogGCTestConfig(base), refFor(base, tenant, ecId), time.Now().Add(time.Hour), &budget)
	assert.Equal(t, 0, res.rewrites)
}

func TestVlogGCBudget(t *testing.T) {
	base := vlogGCTestBase(t)
	tenant, ecId := "tegc0007", "ECIDVLOGGC0007"
	fillVlog(t, tenant, ecId, 10, 40)
	flatten(t, tenant, ecId)
	budget := int64(1) // erschoepft nach dem ersten Aufruf
	res := collectDatabase(context.Background(), vlogGCTestConfig(base), refFor(base, tenant, ecId), time.Now().Add(time.Hour), &budget)
	assert.Equal(t, 1, res.rewrites)
	assert.LessOrEqual(t, budget, int64(0))
}

// Ganzer Lauf: nur Datenbanken ueber minVlog, eine Summenzeile, keine Fehler.
func TestVlogGCRun(t *testing.T) {
	base := vlogGCTestBase(t)
	fillVlog(t, "tegc0008", "ECIDVLOGGC0008", 6, 40)
	flatten(t, "tegc0008", "ECIDVLOGGC0008")
	fillVlog(t, "tegc0009", "ECIDVLOGGC0009", 1, 1) // klein, unter minVlog
	before := refFor(base, "tegc0008", "ECIDVLOGGC0008").vlogBytes

	cfg := vlogGCTestConfig(base)
	cfg.MinVlogBytes = 8 << 20
	runVlogGC(context.Background(), cfg, time.Now().Add(time.Hour))
	after := refFor(base, "tegc0008", "ECIDVLOGGC0008").vlogBytes
	assert.Less(t, after, before/4)
}

// Endet der Lauf in der letzten Datenbank am Budget, muss die Summenzeile das sagen (in Dev
// meldete sie nach einem Neustart mitten im Lauf "alle Datenbanken bearbeitet").
func TestVlogGCRunReportsStopInLastDatabase(t *testing.T) {
	base := vlogGCTestBase(t)
	fillVlog(t, "tegc0010", "ECIDVLOGGC0010", 6, 40)
	flatten(t, "tegc0010", "ECIDVLOGGC0010")
	cfg := vlogGCTestConfig(base)
	cfg.MaxBytesPerRun = 1
	assert.Equal(t, "Budget je Lauf erreicht", runVlogGC(context.Background(), cfg, time.Now().Add(time.Hour)))
}

// Die hoechste Datei zaehlt nur bei offener Datenbank nicht (vorbelegt auf 2 GiB); bei
// geschlossener enthaelt sie Daten. In Dev blieb sonst eine Datenbank mit 280 MB in ihrer
// letzten Datei unter minVlog.
func TestVlogGCSizeCountsClosedNewestFile(t *testing.T) {
	base := vlogGCTestBase(t)
	tenant, ecId := "tegc0011", "ECIDVLOGGC0011"
	fillVlog(t, tenant, ecId, 1, 40) // eine Sitzung: alle Daten in der hoechsten Datei
	dir := filepath.Join(base, tenant, ecId)
	closed, _, _ := vlogBytes(dir)
	assert.Greater(t, closed, int64(5<<20), "geschlossen: die hoechste Datei zaehlt mit")

	st, err := OpenStorage(tenant, ecId) // Oeffnen legt eine neue, vorbelegte Datei an
	require.NoError(t, err)
	open, _, _ := vlogBytes(dir)
	st.Close()
	assert.InDelta(t, closed, open, float64(1<<20), "offen: die vorbelegte Datei zaehlt nicht")
}

func TestVlogGCDisabledStartsNothing(t *testing.T) {
	wait := StartVlogGC(context.Background(), VlogGCConfig{Enabled: false})
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("wait blockiert, obwohl ausgeschaltet")
	}
}

func TestVlogGCStartStopsOnCancel(t *testing.T) {
	cfg := vlogGCTestConfig(vlogGCTestBase(t))
	cfg.Window = "03:00-03:01" // praktisch nie offen: der Test prueft nur das Beenden
	require.NoError(t, cfg.parse())
	ctx, cancel := context.WithCancel(context.Background())
	wait := StartVlogGC(ctx, cfg)
	cancel()
	done := make(chan struct{})
	go func() { wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Hintergrundlauf endet nicht nach cancel")
	}
}

func TestVlogGCConfigDefaultsOff(t *testing.T) {
	cfg, err := ReadVlogGCConfig()
	require.NoError(t, err)
	assert.False(t, cfg.Enabled)
	assert.Equal(t, "02:00-04:30", cfg.Window)
	assert.Equal(t, 0.5, cfg.DiscardRatio)
	assert.Equal(t, int64(100<<20), cfg.MinVlogBytes)
	assert.Equal(t, int64(50<<30), cfg.MaxBytesPerRun)

	for _, w := range []string{"02:00", "25:00-03:00", "02:00-02:00", "a-b"} {
		c := VlogGCConfig{Window: w, CheckEvery: time.Minute, DiscardRatio: 0.5, ProbeRatio: 0.1}
		assert.Error(t, c.parse(), w)
	}
}

// Fenster in Europe/Vienna, unabhaengig von der Zeitzone des Containers (Eingaben in UTC).
func TestVlogGCWindow(t *testing.T) {
	night := VlogGCConfig{Window: "02:00-04:30", CheckEvery: time.Minute, DiscardRatio: 0.5, ProbeRatio: 0.1}
	require.NoError(t, night.parse())
	cross := VlogGCConfig{Window: "23:00-01:00", CheckEvery: time.Minute, DiscardRatio: 0.5, ProbeRatio: 0.1}
	require.NoError(t, cross.parse())

	utc := func(s string) time.Time {
		tm, err := time.Parse("2006-01-02 15:04", s)
		require.NoError(t, err)
		return tm
	}
	tests := []struct {
		cfg     VlogGCConfig
		now     string // UTC
		in      bool
		day     string
		comment string
	}{
		{night, "2026-07-01 00:30", true, "2026-07-01", "02:30 Sommerzeit"},
		{night, "2026-07-01 02:29", true, "2026-07-01", "04:29 Sommerzeit"},
		{night, "2026-07-01 02:31", false, "", "04:31 Sommerzeit"},
		{night, "2026-07-01 02:00", true, "2026-07-01", "04:00 Sommerzeit -- in UTC laege das Fenster woanders"},
		{night, "2026-01-15 01:00", true, "2026-01-15", "02:00 Winterzeit"},
		{night, "2026-01-15 03:31", false, "", "04:31 Winterzeit"},
		{night, "2026-03-29 00:30", false, "", "Umstellung Sommerzeit: 01:30, 02:00 gibt es nicht"},
		{night, "2026-03-29 01:30", true, "2026-03-29", "Umstellung Sommerzeit: 03:30"},
		// Die Stunde 02:00-03:00 gibt es zweimal; time.Date nimmt die zweite. Der Lauf beginnt an
		// diesem einen Tag also eine Stunde spaeter -- unkritisch, aber hier festgehalten.
		{night, "2026-10-25 00:30", false, "", "Umstellung Winterzeit: erste 02:30, Fenster beginnt mit der zweiten"},
		{night, "2026-10-25 01:30", true, "2026-10-25", "Umstellung Winterzeit: zweite 02:30"},
		{night, "2026-10-25 03:31", false, "", "Umstellung Winterzeit: 04:31"},
		{cross, "2026-07-01 22:30", true, "2026-07-01", "00:30 am Folgetag, Fenster begann am 01.07."},
		{cross, "2026-07-01 21:30", true, "2026-07-01", "23:30"},
		{cross, "2026-07-01 23:30", false, "", "01:30"},
	}
	for _, tt := range tests {
		in, _, day := tt.cfg.window(utc(tt.now))
		assert.Equal(t, tt.in, in, fmt.Sprintf("%s (%s)", tt.now, tt.comment))
		assert.Equal(t, tt.day, day, fmt.Sprintf("%s (%s)", tt.now, tt.comment))
	}
}
