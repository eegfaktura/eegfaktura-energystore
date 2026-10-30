package ebow

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Ready meldet, ob der Speicher Anfragen bedienen kann (GET /readyz): der Pool ist nicht
// heruntergefahren und persistence.path ist ein Verzeichnis. Es oeffnet bewusst keine
// Mandanten-DB -- die werden pro Anfrage geoeffnet, ein Probe soll keine anlegen oder sperren.
func Ready() error {
	connectionPool.mu.Lock()
	closed := connectionPool.closed
	connectionPool.mu.Unlock()
	if closed {
		return errors.New("connection pool closed")
	}
	info, err := os.Stat(viper.GetString("persistence.path"))
	if err != nil {
		return fmt.Errorf("persistence path: %w", err)
	}
	if !info.IsDir() {
		return errors.New("persistence path is not a directory")
	}
	return nil
}
