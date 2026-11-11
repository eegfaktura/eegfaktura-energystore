package tz

import (
	"testing"
	"time"
)

// TestVienna is the guard of M0: it fails when the zone database is missing, so a runner
// without tzdata cannot compute other row ids silently.
func TestVienna(t *testing.T) {
	if err := Err(); err != nil {
		t.Fatalf("Europe/Vienna not available: %v", err)
	}
	quarterHours := func(day time.Time) int {
		next := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, Vienna)
		return int(next.Sub(day) / (15 * time.Minute))
	}
	if got := quarterHours(SpringForward2026); got != 92 {
		t.Errorf("spring day: %d quarter hours, want 92", got)
	}
	if got := quarterHours(FallBack2026); got != 100 {
		t.Errorf("autumn day: %d quarter hours, want 100", got)
	}
}
