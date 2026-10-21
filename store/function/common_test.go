package function

import (
	"testing"
	"time"
)

// ToDataTimeRange rounds begin and end down to the quarter hour (the row ids of the slots).
func TestToDataTimeRangeTruncatesToQuarterHour(t *testing.T) {
	begin := time.Date(2026, 2, 1, 10, 7, 30, 0, time.Local)
	end := time.Date(2026, 2, 1, 12, 59, 0, 0, time.Local)
	r, err := ToDataTimeRange(begin, end)
	if err != nil {
		t.Fatal(err)
	}
	if r.key != "2026/02/01/10/00/00" || r.until != "2026/02/01/12/45/00" {
		t.Fatalf("got key %q until %q", r.key, r.until)
	}
}
