package ebow

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Fuer Tests austauschbar: die Pruefung des Verzeichnisses und ihre Frist (z.B. haengendes Volume).
var (
	readyProbe   = probeDir
	readyTimeout = 2 * time.Second
)

// readyCheck ist eine laufende Pruefung; gleichzeitige Aufrufe von Ready teilen ihr Ergebnis.
type readyCheck struct {
	done chan struct{}
	err  error
}

var (
	readyMu      sync.Mutex
	readyRunning *readyCheck
	probeHanging chan error // Pruefung eines frueheren Laufs, die in die Frist lief und noch nicht fertig ist
)

// EnsurePersistencePath legt persistence.path an (beim Start). Auf einem frischen Volume entsteht
// das Verzeichnis sonst erst mit der ersten Mandanten-DB -- und /readyz meldete bis dahin 503.
// 0700 wie Badger fuer die DB-Verzeichnisse.
func EnsurePersistencePath() error {
	return os.MkdirAll(viper.GetString("persistence.path"), 0o700)
}

// Ready meldet, ob der Speicher Anfragen bedienen kann (GET /readyz): in persistence.path laesst
// sich eine Datei schreiben und loeschen (erkennt fehlendes Verzeichnis, schreibgeschuetztes
// Volume, volle Platte). Es nimmt kein Handle aus dem Pool und oeffnet keine Mandanten-DB -- ein
// Probe soll den Pool weder belegen noch eine DB anlegen oder sperren. Es laeuft hoechstens eine
// Pruefung zugleich; wer waehrenddessen fragt, bekommt deren Ergebnis, damit oeffentliche Aufrufe
// die Last nicht vervielfachen.
func Ready() error {
	readyMu.Lock()
	if c := readyRunning; c != nil {
		readyMu.Unlock()
		<-c.done
		return c.err
	}
	c := &readyCheck{done: make(chan struct{})}
	readyRunning = c
	readyMu.Unlock()

	c.err = checkReady()

	readyMu.Lock()
	readyRunning = nil
	readyMu.Unlock()
	close(c.done)
	return c.err
}

// checkReady laeuft nie parallel zu sich selbst (Ready), daher braucht probeHanging keine Sperre.
func checkReady() error {
	// Nur nach dem Herunterfahren gesetzt.
	connectionPool.mu.Lock()
	closed := connectionPool.closed
	connectionPool.mu.Unlock()
	if closed {
		return errors.New("connection pool closed")
	}

	if probeHanging != nil {
		select {
		case <-probeHanging: // spaetes Ergebnis einer abgelaufenen Pruefung: verwerfen, neu pruefen
			probeHanging = nil
		default:
			return errors.New("persistence path check still hanging")
		}
	}
	ch := make(chan error, 1)
	go func(path string) { ch <- readyProbe(path) }(viper.GetString("persistence.path"))
	select {
	case err := <-ch:
		return err
	case <-time.After(readyTimeout):
		probeHanging = ch
		return errors.New("persistence path check timed out")
	}
}

// probeDir schreibt eine kleine Datei in path und loescht sie wieder.
func probeDir(path string) error {
	f, err := os.CreateTemp(path, ".readyz-*")
	if err != nil {
		return fmt.Errorf("persistence path: %w", err)
	}
	name := f.Name()
	_, werr := f.Write([]byte("ok"))
	cerr := f.Close()
	rerr := os.Remove(name)
	if err := errors.Join(werr, cerr, rerr); err != nil {
		return fmt.Errorf("persistence path: %w", err)
	}
	return nil
}
