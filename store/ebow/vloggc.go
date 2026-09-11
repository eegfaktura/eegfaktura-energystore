package ebow

// Zyklisches Aufraeumen des Badger-Value-Logs (konzept-energystore-vlog-gc.md).
//
// Warum es das braucht: der gemeinschaftsweite Metadaten-Satz cpmeta/0 liegt ab etwa 750
// Zaehlpunkten ueber der ValueThreshold von 128 KB und landet damit im Value Log. Jede
// Periodenerweiterung schreibt ihn komplett neu; die alten Versionen gibt ausschliesslich
// RunValueLogGC frei -- und das wurde bis hierher nie aufgerufen.
//
// Warum im Dienst und nicht als CronJob: Badger sperrt das Verzeichnis exklusiv. Ein externer Job
// koennte eine ruhende Gemeinschaft oeffnen und damit deren Importe scheitern lassen. Nur im Prozess
// koordiniert der Pool, wer eine Datenbank haelt.

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // Fenster in Europe/Vienna auch auf einem Basis-Image ohne Zonendaten

	"github.com/dgraph-io/badger/v4"
	"github.com/golang/glog"
	"github.com/spf13/viper"
)

const vlogGCKey = "persistence.vlogGC."

// VlogGCConfig steuert den Lauf. Alle Werte stehen unter persistence.vlogGC und sind ueber den
// Viper-Praefix auch als Umgebungsvariable setzbar (ENERGYSTORE_PERSISTENCE_VLOGGC_ENABLED usw.).
type VlogGCConfig struct {
	Enabled        bool
	CheckEvery     time.Duration
	Window         string // "HH:MM-HH:MM" in Europe/Vienna, darf ueber Mitternacht gehen
	DiscardRatio   float64
	ProbeRatio     float64
	MinVlogBytes   int64
	MaxBytesPerRun int64
	BasePath       string

	startMin, endMin int // Fenstergrenzen in Minuten nach Mitternacht
}

var vienna = mustLoadVienna()

func mustLoadVienna() *time.Location {
	loc, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		panic(fmt.Sprintf("vlogGC: Zeitzone Europe/Vienna fehlt: %v", err))
	}
	return loc
}

// ReadVlogGCConfig liest die Konfiguration. Ohne Eintrag ist der Lauf ausgeschaltet.
func ReadVlogGCConfig() (VlogGCConfig, error) {
	viper.SetDefault(vlogGCKey+"enabled", false)
	viper.SetDefault(vlogGCKey+"checkEvery", "10m")
	viper.SetDefault(vlogGCKey+"window", "02:00-04:30")
	viper.SetDefault(vlogGCKey+"discardRatio", 0.5)
	viper.SetDefault(vlogGCKey+"probeRatio", 0.1)
	viper.SetDefault(vlogGCKey+"minVlogMB", 100)
	viper.SetDefault(vlogGCKey+"maxGBPerRun", 50)

	cfg := VlogGCConfig{
		Enabled:        viper.GetBool(vlogGCKey + "enabled"),
		CheckEvery:     viper.GetDuration(vlogGCKey + "checkEvery"),
		Window:         viper.GetString(vlogGCKey + "window"),
		DiscardRatio:   viper.GetFloat64(vlogGCKey + "discardRatio"),
		ProbeRatio:     viper.GetFloat64(vlogGCKey + "probeRatio"),
		MinVlogBytes:   viper.GetInt64(vlogGCKey+"minVlogMB") << 20,
		MaxBytesPerRun: viper.GetInt64(vlogGCKey+"maxGBPerRun") << 30,
		BasePath:       viper.GetString("persistence.path"),
	}
	return cfg, cfg.parse()
}

func (c *VlogGCConfig) parse() error {
	parts := strings.Split(c.Window, "-")
	if len(parts) != 2 {
		return fmt.Errorf("window %q: erwartet HH:MM-HH:MM", c.Window)
	}
	var err error
	if c.startMin, err = parseClock(parts[0]); err != nil {
		return fmt.Errorf("window %q: %w", c.Window, err)
	}
	if c.endMin, err = parseClock(parts[1]); err != nil {
		return fmt.Errorf("window %q: %w", c.Window, err)
	}
	if c.startMin == c.endMin {
		return fmt.Errorf("window %q: Beginn und Ende sind gleich", c.Window)
	}
	if c.CheckEvery <= 0 {
		return fmt.Errorf("checkEvery %v: muss positiv sein", c.CheckEvery)
	}
	if c.DiscardRatio <= 0 || c.DiscardRatio >= 1 || c.ProbeRatio <= 0 || c.ProbeRatio >= 1 {
		return fmt.Errorf("discardRatio %v / probeRatio %v: muessen zwischen 0 und 1 liegen", c.DiscardRatio, c.ProbeRatio)
	}
	return nil
}

func parseClock(s string) (int, error) {
	hm := strings.Split(strings.TrimSpace(s), ":")
	if len(hm) != 2 {
		return 0, fmt.Errorf("%q ist keine Uhrzeit HH:MM", s)
	}
	h, err1 := strconv.Atoi(hm[0])
	m, err2 := strconv.Atoi(hm[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("%q ist keine Uhrzeit HH:MM", s)
	}
	return h*60 + m, nil
}

// window sagt, ob now im Fenster liegt, wann es endet und an welchem Tag es begonnen hat (der
// Schluessel, der je Fenster hoechstens einen Lauf zulaesst). Ausgewertet wird in Europe/Vienna,
// unabhaengig von der Zeitzone des Containers; time.Date normalisiert die Umstellungstage.
func (c VlogGCConfig) window(now time.Time) (in bool, end time.Time, day string) {
	local := now.In(vienna)
	at := func(d time.Time, min int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), min/60, min%60, 0, 0, vienna)
	}
	for _, offset := range []int{0, -1} { // Fenster, das heute oder (ueber Mitternacht) gestern begann
		d := local.AddDate(0, 0, offset)
		start := at(d, c.startMin)
		stop := at(d, c.endMin)
		if c.endMin < c.startMin {
			stop = at(d.AddDate(0, 0, 1), c.endMin)
		}
		if !local.Before(start) && local.Before(stop) {
			return true, stop, start.Format("2006-01-02")
		}
	}
	return false, time.Time{}, ""
}

// StartVlogGC startet den Zeitplan und gibt eine Funktion zurueck, die auf das Ende des
// Hintergrundlaufs wartet. Sie muss VOR ClosePool aufgerufen werden: Pool.Close schliesst jede
// Datenbank, auch wenn der Lauf gerade einen Platz haelt.
func StartVlogGC(ctx context.Context, cfg VlogGCConfig) (wait func()) {
	if !cfg.Enabled {
		glog.Info("vlogGC: ausgeschaltet (persistence.vlogGC.enabled=false)")
		return func() {}
	}
	glog.Infof("vlogGC: eingeschaltet, Fenster %s Europe/Vienna, Pruefung alle %v, discardRatio %.2f, minVlog %d MB, max %d GB je Lauf",
		cfg.Window, cfg.CheckEvery, cfg.DiscardRatio, cfg.MinVlogBytes>>20, cfg.MaxBytesPerRun>>30)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		lastDay := ""
		check := func(now time.Time) {
			in, end, day := cfg.window(now)
			if !in || day == lastDay {
				return
			}
			lastDay = day
			runVlogGC(ctx, cfg, end)
		}
		check(time.Now())
		ticker := time.NewTicker(cfg.CheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				check(now)
			}
		}
	}()
	return wg.Wait
}

// vlogRef ist eine aufgezaehlte Badger-Datenbank <BasePath>/<tenant>/<ecId>.
type vlogRef struct {
	tenant, ecId, dir string
	vlogBytes         int64
	sharedEcId        bool // dieselbe ecId liegt auch unter einem anderen Tenant
}

type vlogGCStats struct {
	databases, skipped, failed, rewrites int
	freed, read                          int64
}

// runVlogGC gibt den Grund zurueck, mit dem der Lauf endete (auch in der Summenzeile).
func runVlogGC(ctx context.Context, cfg VlogGCConfig, deadline time.Time) (stop string) {
	defer func() {
		if r := recover(); r != nil {
			glog.Errorf("vlogGC: Lauf abgebrochen nach panic: %v", r)
			stop = "panic"
		}
	}()
	started := time.Now()
	refs, err := enumerateVlogDBs(cfg.BasePath)
	if err != nil {
		glog.Errorf("vlogGC: Aufzaehlung von %s fehlgeschlagen: %v", cfg.BasePath, err)
		return "Aufzaehlung fehlgeschlagen"
	}
	glog.Infof("vlogGC: Lauf beginnt, %d Datenbanken, Frist %s", len(refs), deadline.In(vienna).Format("15:04"))

	budget := cfg.MaxBytesPerRun
	var st vlogGCStats
	// Der Grund wird auch NACH der letzten Datenbank bestimmt: endet der Lauf mitten in ihr, weil
	// der Dienst beendet wird, soll die Summenzeile das sagen und nicht "alle bearbeitet".
	stopReason := func() string {
		switch {
		case ctx.Err() != nil:
			return "Dienst wird beendet"
		case !time.Now().Before(deadline):
			return "Fensterende erreicht"
		case budget <= 0:
			return "Budget je Lauf erreicht"
		}
		return ""
	}
	stop = ""
	for _, ref := range refs {
		if ref.vlogBytes < cfg.MinVlogBytes {
			break // absteigend sortiert: der Rest ist kleiner
		}
		if stop = stopReason(); stop != "" {
			break
		}
		res := collectDatabase(ctx, cfg, ref, deadline, &budget)
		switch {
		case res.skipped:
			st.skipped++
		case res.err != nil:
			st.failed++
		default:
			st.databases++
		}
		st.rewrites += res.rewrites
		st.freed += res.freed
		st.read += res.read
	}

	if stop == "" {
		if stop = stopReason(); stop == "" {
			stop = "alle Datenbanken bearbeitet"
		}
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	glog.Infof("vlogGC: Lauf beendet (%s) in %v: %d bearbeitet, %d uebersprungen, %d fehlerhaft, %d Umschreibungen, %d MB gelesen, %d MB freigegeben, HeapInuse %d MB",
		stop, time.Since(started).Round(time.Second), st.databases, st.skipped, st.failed, st.rewrites,
		st.read>>20, st.freed>>20, ms.HeapInuse>>20)
	return stop
}

// enumerateVlogDBs liefert alle Badger-Verzeichnisse in genau zwei Ebenen unter base, groesste zuerst.
// Der Pool taugt dafuer nicht: seine Map kennt nur Gemeinschaften, die seit dem Start angefragt wurden.
func enumerateVlogDBs(base string) ([]vlogRef, error) {
	dirs, err := filepath.Glob(filepath.Join(base, "*", "*"))
	if err != nil {
		return nil, err
	}
	var refs []vlogRef
	tenantsPerEcId := map[string]int{}
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(d, "MANIFEST")); err != nil {
			continue // lost+found, Reste, keine Badger-Datenbank
		}
		ref := vlogRef{
			tenant: filepath.Base(filepath.Dir(d)),
			ecId:   filepath.Base(d),
			dir:    d,
		}
		ref.vlogBytes, _, _ = vlogBytes(d)
		tenantsPerEcId[ref.ecId]++
		refs = append(refs, ref)
	}
	for i := range refs {
		refs[i].sharedEcId = tenantsPerEcId[refs[i].ecId] > 1
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].vlogBytes > refs[j].vlogBytes })
	return refs, nil
}

// vlogPreallocated trennt die vorbelegte aktuelle Datei von fertigen: Badger legt die aktuelle
// beim Oeffnen mit 2*ValueLogFileSize = 2 GiB an (value.go:508, auf Linux sparse) und kuerzt sie
// erst beim Schliessen; eine fertige Datei ist hoechstens ValueLogFileSize (1 GiB) plus ein Batch.
const vlogPreallocated = 3 << 29 // 1,5 GiB

// vlogBytes summiert die Value-Log-Dateien eines Verzeichnisses. Die hoechste Datei zaehlt nicht,
// wenn sie vorbelegt ist (Datenbank offen) -- ihre Groesse waere 2 GiB zu hoch. Bei geschlossener
// Datenbank ist sie eine gewoehnliche Datei mit Daten und zaehlt mit.
// Bewusst nicht db.Size(): energystore schaltet Badgers Metriken ab, Size() liefert dann 0, 0.
func vlogBytes(dir string) (total int64, sizes map[uint64]int64, curFid uint64) {
	sizes = map[uint64]int64{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.vlog"))
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		fid, err := strconv.ParseUint(strings.TrimSuffix(filepath.Base(f), ".vlog"), 10, 64)
		if err != nil {
			continue
		}
		sizes[fid] = fi.Size()
		if fid > curFid {
			curFid = fid
		}
	}
	for fid, sz := range sizes {
		if fid != curFid || sz < vlogPreallocated {
			total += sz
		}
	}
	return total, sizes, curFid
}

// discardCandidate liest die Verwurfsstatistik (Datei DISCARD, badger discard.go: Slots zu 16 Byte,
// Big-Endian fid + verworfene Bytes, Ende bei fid == 0) und liefert den Kandidaten, den pickLog
// waehlen wird: die Datei mit dem groessten Verwurf.
type discardInfo struct {
	fid, curFid   uint64
	discard, size int64
	ratio         float64
}

// Je Aufruf wird nur DISCARD gelesen und die Kandidat-Datei geprueft, nicht das ganze Verzeichnis
// aufgelistet -- die groesste Gemeinschaft hat ueber 3.000 Dateien und tausende Aufrufe.
// Eintraege bereits geloeschter Dateien ueberspringt pickLog ebenso (value.go:1003-1009).
func discardCandidate(dir string, curFid uint64) discardInfo {
	info := discardInfo{curFid: curFid}
	buf, err := os.ReadFile(filepath.Join(dir, "DISCARD"))
	if err != nil {
		return info
	}
	type slot struct{ fid, dis uint64 }
	var slots []slot
	for i := 0; i+16 <= len(buf); i += 16 {
		fid := binary.BigEndian.Uint64(buf[i:])
		dis := binary.BigEndian.Uint64(buf[i+8:])
		if fid == 0 {
			break
		}
		if dis > 0 {
			slots = append(slots, slot{fid, dis})
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].dis > slots[j].dis })
	for _, s := range slots {
		fi, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%06d.vlog", s.fid)))
		if err != nil {
			continue
		}
		info.fid, info.discard, info.size = s.fid, int64(s.dis), fi.Size()
		if info.size > 0 {
			info.ratio = float64(s.dis) / float64(info.size)
		}
		break
	}
	return info
}

// cause benennt, warum pickLog nichts umschreibt (Prod laeuft mit -v=3, Badger meldet es nur auf V(5)).
func (d discardInfo) cause(ratio float64) string {
	switch {
	case d.fid == 0:
		return "keine Verwurfsstatistik"
	case d.fid == d.curFid:
		return fmt.Sprintf("Kandidat fid %d ist die aktuelle Datei", d.fid)
	case d.ratio < ratio:
		return fmt.Sprintf("Kandidat fid %d unter der Schwelle (Verhaeltnis %.2f < %.2f)", d.fid, d.ratio, ratio)
	default:
		return fmt.Sprintf("Kandidat fid %d, Verhaeltnis %.2f", d.fid, d.ratio)
	}
}

type collectResult struct {
	skipped     bool
	err         error
	rewrites    int
	freed, read int64
}

// holdsEntry sagt, ob der Pool fuer ecId bereits einen Eintrag hat und ob dessen Tenant passt.
// Unter derselben Sperre wie Pool.Get, das die Map beschreibt; Eintraege werden nie ersetzt oder
// entfernt, zwischen Nachschauen und Oeffnen kann sich also nichts aendern.
func (p *Pool) holdsEntry(ecId, tenant string) (exists, sameTenant bool) {
	p.mutexPut.Lock()
	defer p.mutexPut.Unlock()
	obj, ok := p.pool[ecId]
	if !ok {
		return false, false
	}
	return true, obj.tenant == strings.ToLower(tenant)
}

func collectDatabase(ctx context.Context, cfg VlogGCConfig, ref vlogRef, deadline time.Time, budget *int64) (res collectResult) {
	name := ref.tenant + "/" + ref.ecId

	// Der Pool legt den Tenant beim ERSTEN Zugriff auf eine ecId fest. Liegt die ecId unter mehreren
	// Tenants, darf der Lauf nie dieser erste Zugriff sein -- sonst landeten danach die Importe der
	// echten Gemeinschaft in der falschen Kopie. Also nur mit vorhandenem, passendem Pool-Eintrag.
	if ref.sharedEcId {
		if exists, same := connectionPool.holdsEntry(ref.ecId, ref.tenant); !exists || !same {
			glog.Infof("vlogGC: %s uebersprungen: ecId liegt unter mehreren Tenants und der Pool haelt sie nicht unter diesem Tenant (Eintrag vorhanden: %v)", name, exists)
			res.skipped = true
			return res
		}
	}

	// Das Oeffnen ist nicht unterbrechbar und liest die juengste Value-Log-Datei ganz (Badger prueft,
	// ob sie gekuerzt werden muss, value.go:593) -- bei einer grossen Datei dauert es entsprechend.
	openStarted := time.Now()
	st, err := OpenStorage(ref.tenant, ref.ecId)
	opened := time.Since(openStarted)
	if err != nil {
		glog.Errorf("vlogGC: %s nicht geoeffnet: %v", name, err)
		res.err = err
		return res
	}
	defer st.Close()
	bdb := st.db.Badger()

	// Der Pool ist nur nach ecId geschluesselt; ohne diese Pruefung koennte der Lauf Verzeichnis A
	// messen und Datenbank B aufraeumen. BowStorage.GetTenant() taugt dafuer nicht -- es gibt nur
	// den uebergebenen Tenant zurueck.
	if dir := filepath.Clean(bdb.Opts().Dir); dir != filepath.Clean(ref.dir) {
		glog.Infof("vlogGC: %s uebersprungen: der Pool lieferte %s", name, dir)
		res.skipped = true
		return res
	}

	started := time.Now()
	before, _, curFid := vlogBytes(ref.dir)
	probed := false
	end := "nichts mehr umzuschreiben"
	for {
		if ctx.Err() != nil {
			end = "Dienst wird beendet"
			break
		}
		if !time.Now().Before(deadline) {
			end = "Fensterende"
			break
		}
		if *budget <= 0 {
			end = "Budget je Lauf erreicht"
			break
		}
		cand := discardCandidate(ref.dir, curFid)
		err := bdb.RunValueLogGC(cfg.DiscardRatio)
		if err == nil {
			res.rewrites++
			res.read += cand.size
			*budget -= cand.size
			continue
		}
		if errors.Is(err, badger.ErrNoRewrite) {
			if res.rewrites == 0 && !probed && before >= cfg.MinVlogBytes {
				// Einmalige Sondierung: sie beantwortet die Ursachenfrage, senkt aber nicht die Grenze.
				probed = true
				probeErr := bdb.RunValueLogGC(cfg.ProbeRatio)
				glog.Infof("vlogGC: %s: nichts ueber %.2f (%s); Sondierung mit %.2f: %s",
					name, cfg.DiscardRatio, cand.cause(cfg.DiscardRatio), cfg.ProbeRatio, probeResult(probeErr))
				if probeErr == nil {
					res.rewrites++
					res.read += cand.size
					*budget -= cand.size
				}
			}
			break
		}
		if errors.Is(err, badger.ErrRejected) {
			end = "auf dieser Datenbank laeuft bereits ein GC"
			break
		}
		glog.Errorf("vlogGC: %s: RunValueLogGC: %v", name, err)
		res.err = err
		end = "Fehler"
		break
	}

	after, _, _ := vlogBytes(ref.dir)
	res.freed = before - after
	l0 := 0
	for _, l := range bdb.Levels() {
		if l.Level == 0 {
			l0 = l.NumTables
		}
	}
	glog.Infof("vlogGC: %s: vorher %d MB, nachher %d MB, %d Umschreibungen (%d MB gelesen) in %v, Oeffnen %v, L0-Tabellen %d, Ende: %s",
		name, before>>20, after>>20, res.rewrites, res.read>>20, time.Since(started).Round(time.Millisecond),
		opened.Round(time.Millisecond), l0, end)
	if res.rewrites > 0 && after >= before {
		glog.Infof("vlogGC: %s: umgeschriebene Dateien noch nicht geloescht -- ein offener Iterator (z. B. Export) haelt sie bis zu seinem Ende", name)
	}
	return res
}

func probeResult(err error) string {
	switch {
	case err == nil:
		return "hat umgeschrieben (Statistik vorhanden, aber unter der Schwelle)"
	case errors.Is(err, badger.ErrNoRewrite):
		return "nichts umgeschrieben (Statistik fehlt oder hinkt hinterher)"
	default:
		return err.Error()
	}
}
