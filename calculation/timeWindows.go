package calculation

import (
	"fmt"

	"at.ourproject/energystore/model"
)

// Time-of-use window folding (ZVT). Windows are generic wall-clock daytime
// ranges on a 15-min raster; energystore has no tariff/price knowledge.
//
// Timezone note (B4): v1 Badger row ids encode the LOCAL wall-clock time of
// the import process (time.Local; the image bakes TZ=Europe/Berlin, which is
// offset-identical to Europe/Vienna). Window membership therefore compares
// directly against the HH:MM encoded in the row id - no timezone conversion.
// DST: on the 23h day a window quarter-hour occurs once less; on the 25h day
// the duplicated hour collides in the v1 row-id key space at import time
// (pre-existing), so it is counted once.

// timeWindowRange is a parsed window; from inclusive, to exclusive, minutes
// since midnight. from > to means the window crosses midnight.
type timeWindowRange struct {
	rangeKey string // "HH:MM-HH:MM", used to de-duplicate identical ranges
	fromMin  int
	toMin    int
}

func parseWindowTime(s string) (int, error) {
	var hh, mm int
	if _, err := fmt.Sscanf(s, "%d:%d", &hh, &mm); err != nil {
		return 0, fmt.Errorf("invalid time %q (expected HH:MM)", s)
	}
	if hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, fmt.Errorf("invalid time %q (expected HH:MM)", s)
	}
	if mm%15 != 0 {
		return 0, fmt.Errorf("time %q not on 15-min raster (00/15/30/45)", s)
	}
	return hh*60 + mm, nil
}

func parseTimeWindow(tw model.TimeWindow) (timeWindowRange, error) {
	if tw.Key != "T1" && tw.Key != "T2" {
		return timeWindowRange{}, fmt.Errorf("invalid time window key %q (expected T1 or T2)", tw.Key)
	}
	fromMin, err := parseWindowTime(tw.From)
	if err != nil {
		return timeWindowRange{}, err
	}
	toMin, err := parseWindowTime(tw.To)
	if err != nil {
		return timeWindowRange{}, err
	}
	if fromMin == toMin {
		return timeWindowRange{}, fmt.Errorf("time window %s: from == to (%s) is not allowed", tw.Key, tw.From)
	}
	return timeWindowRange{rangeKey: tw.From + "-" + tw.To, fromMin: fromMin, toMin: toMin}, nil
}

// contains reports whether the quarter-hour starting at minute-of-day m lies
// inside the window ([from, to), cyclic over midnight when from > to).
func (w timeWindowRange) contains(m int) bool {
	if w.fromMin < w.toMin {
		return m >= w.fromMin && m < w.toMin
	}
	return m >= w.fromMin || m < w.toMin
}

// ValidateTimeWindows checks all time windows of a report request:
// max 2 per meter, unique keys T1/T2, HH:MM on the 15-min raster, from != to.
func ValidateTimeWindows(participants []model.ParticipantReport) error {
	for _, p := range participants {
		for _, m := range p.Meters {
			if len(m.TimeWindows) > 2 {
				return fmt.Errorf("meter %s: more than 2 time windows", m.MeterId)
			}
			seen := map[string]bool{}
			for _, tw := range m.TimeWindows {
				if seen[tw.Key] {
					return fmt.Errorf("meter %s: duplicate time window key %s", m.MeterId, tw.Key)
				}
				seen[tw.Key] = true
				if _, err := parseTimeWindow(tw); err != nil {
					return fmt.Errorf("meter %s: %w", m.MeterId, err)
				}
			}
		}
	}
	return nil
}

// distinctWindowRanges collects the de-duplicated window ranges of all
// requested meters (different tariffs may define different windows).
func distinctWindowRanges(meters map[string][]*model.MeterReport) []timeWindowRange {
	var ranges []timeWindowRange
	seen := map[string]bool{}
	for _, reports := range meters {
		for _, r := range reports {
			for _, tw := range r.TimeWindows {
				w, err := parseTimeWindow(tw)
				if err != nil {
					continue // rejected upstream by ValidateTimeWindows
				}
				if !seen[w.rangeKey] {
					seen[w.rangeKey] = true
					ranges = append(ranges, w)
				}
			}
		}
	}
	return ranges
}
