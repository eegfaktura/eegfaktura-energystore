package ownmeters

import (
	"strings"
	"time"
)

// Window is the time a metering point belonged to the member: its first and its last day (local
// midnight), both inclusive — as energystore reads activeSince/inactiveSince elsewhere (billing check,
// Excel export). A participation without an end date runs until 31.12.2999, as in the backend.
type Window struct {
	From, Until time.Time
}

// Meters maps a member's metering points (upper case) to the windows of its participations. A meter
// without a window (no start date in the backend) is the member's, but no time of it is readable.
type Meters map[string][]Window

// Owns reports whether meter is one of the member's metering points.
func (m Meters) Owns(meter string) bool {
	_, ok := m[strings.ToUpper(meter)]
	return ok
}

// ClampDays limits a day-based window (epoch ms; until <= 0 means open) of meter to the first of the
// member's windows it overlaps; ok is false when it overlaps none.
func (m Meters) ClampDays(meter string, from, until int64) (int64, int64, bool) {
	for _, w := range m[strings.ToUpper(meter)] {
		f, u := from, until
		if dayOf(f).Before(w.From) {
			f = w.From.UnixMilli()
		}
		if u <= 0 || dayOf(u).After(w.Until) {
			u = w.Until.UnixMilli()
		}
		if !dayOf(f).After(dayOf(u)) {
			return f, u, true
		}
	}
	return 0, 0, false
}

// ClampRange limits the time range [start, end] (epoch ms) of a query over several meters to the
// member's windows: per meter the first window it overlaps, the range is cut to all of them. ok is false
// when a meter has no overlapping window or nothing is left.
func (m Meters) ClampRange(meters []string, start, end int64) (int64, int64, bool) {
	s, e := start, end
	for _, meter := range meters {
		found := false
		for _, w := range m[strings.ToUpper(meter)] {
			ws, we := w.From.UnixMilli(), w.Until.AddDate(0, 0, 1).UnixMilli()-1
			if start <= we && end >= ws {
				s, e = max(s, ws), min(e, we)
				found = true
				break
			}
		}
		if !found {
			return 0, 0, false
		}
	}
	return s, e, s <= e
}

func dayOf(ms int64) time.Time {
	t := time.UnixMilli(ms)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// backendMeter is the part of a metering point in the backend's answer that is used.
type backendMeter struct {
	MeteringPoint   string `json:"meteringPoint"`
	RegisteredSince string `json:"registeredSince"`
	State           *struct {
		ActiveSince   *string `json:"activeSince"`
		InactiveSince *string `json:"inactiveSince"`
	} `json:"participantState"`
}

var openEnd = time.Date(2999, 12, 31, 0, 0, 0, 0, time.Local)

// windows reads the participation's window with the backend's defaults (meteringPointDao): activeSince,
// else registeredSince; inactiveSince, else open. Without a valid start date there is no window.
func (bm backendMeter) windows() []Window {
	from, ok := day(bm.RegisteredSince)
	until := openEnd
	if bm.State != nil {
		if bm.State.ActiveSince != nil {
			from, ok = day(*bm.State.ActiveSince)
		}
		if bm.State.InactiveSince != nil {
			if d, valid := day(*bm.State.InactiveSince); valid {
				until = d
			}
		}
	}
	if !ok || until.Before(from) {
		return nil
	}
	return []Window{{From: from, Until: until}}
}

// day parses a backend date (yyyy-mm-dd) as local midnight; dates before 1900 (zero values) are invalid.
func day(s string) (time.Time, bool) {
	d, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil || d.Year() < 1900 {
		return time.Time{}, false
	}
	return d, true
}
