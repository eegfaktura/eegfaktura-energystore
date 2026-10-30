package ebow

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// readyTimeout begrenzt die Pruefung des Speicherverzeichnisses (z.B. ein haengendes Volume).
const readyTimeout = 2 * time.Second

// readyCheck ist eine laufende Pruefung; gleichzeitige Aufrufe von Ready teilen ihr Ergebnis.
type readyCheck struct {
	done chan struct{}
	err  error
}

var (
	readyMu      sync.Mutex
	readyRunning *readyCheck
	statPending  chan error // Stat eines frueheren Laufs, der in den Timeout lief und noch haengt
)

// Ready meldet, ob der Speicher Anfragen bedienen kann (GET /readyz): der Pool ist nicht
// heruntergefahren und persistence.path ist ein Verzeichnis. Es nimmt kein Handle aus dem Pool und
// oeffnet keine Mandanten-DB -- ein Probe soll den Pool weder belegen noch eine DB anlegen oder
// sperren. Es laeuft hoechstens eine Pruefung zugleich; wer waehrenddessen fragt, bekommt deren
// Ergebnis, damit oeffentliche Aufrufe die Last nicht vervielfachen.
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

func checkReady() error {
	connectionPool.mu.Lock()
	closed := connectionPool.closed
	connectionPool.mu.Unlock()
	if closed {
		return errors.New("connection pool closed")
	}

	// Haengt ein frueheres Stat noch, kein weiteres starten (sonst sammeln sich Goroutinen an).
	if statPending == nil {
		statPending = make(chan error, 1)
		go func(ch chan error, path string) { ch <- statDir(path) }(statPending, viper.GetString("persistence.path"))
	}
	select {
	case err := <-statPending:
		statPending = nil
		return err
	case <-time.After(readyTimeout):
		return errors.New("persistence path check timed out")
	}
}

func statDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("persistence path: %w", err)
	}
	if !info.IsDir() {
		return errors.New("persistence path is not a directory")
	}
	return nil
}
