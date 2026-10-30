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

// Meters maps a member's metering points (upper case) to the windows of its participations, sorted by
// start. A meter
// without a window (no start date in the backend) is the member's, but no time of it is readable.
type Meters map[string][]Window

// Owns reports whether meter is one of the member's metering points.
func (m Meters) Owns(meter string) bool {
	_, ok := m[strings.ToUpper(meter)]
	return ok
}

// ClampDays limits a day-based window (epoch ms; until <= 0 means open) of meter to the member's window
// it overlaps most (ties: the latest); ok is false when it overlaps none. An open until stays as given
// (0) when that window has no end either, so a member's request keeps the shape of an admin's (billing
// check: 0 = active without end); only an ended window sets it to its last day.
//
// Limitation: one request window is cut to one participation. The time between two participations of the
// same member is never readable, and a request spanning both returns only the larger part.
func (m Meters) ClampDays(meter string, from, until int64) (int64, int64, bool) {
	reqFrom, reqUntil := dayOf(from), openEnd
	if until > 0 {
		reqUntil = dayOf(until)
	}
	best, bestDays := -1, 0
	windows := m[strings.ToUpper(meter)]
	for i, w := range windows {
		days := overlapDays(reqFrom, reqUntil, w)
		if days > 0 && days >= bestDays { // windows are sorted by start: on a tie the later one wins
			best, bestDays = i, days
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	w := windows[best]
	f, u := from, until
	if reqFrom.Before(w.From) {
		f = w.From.UnixMilli()
	}
	if reqUntil.After(w.Until) {
		u = w.Until.UnixMilli()
	}
	return f, u, true
}

// overlapDays counts the days of [from, until] inside w (both inclusive, local days).
func overlapDays(from, until time.Time, w Window) int {
	f, u := from, until
	if w.From.After(f) {
		f = w.From
	}
	if w.Until.Before(u) {
		u = w.Until
	}
	if f.After(u) {
		return 0
	}
	return int(u.Sub(f).Round(24*time.Hour)/(24*time.Hour)) + 1
}

// ClampRange limits the time range [start, end] (epoch ms) of a query over several meters to the
// member's windows: per meter the first window (sorted by start) it overlaps, the range is cut to all of
// them. ok is false when a meter has no overlapping window or nothing is left. Same limitation as
// ClampDays.
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
	MeteringPoint string `json:"meteringPoint"`
	ProcessState  string `json:"processState"`
	State         *struct {
		ActiveSince   *string `json:"activeSince"`
		InactiveSince *string `json:"inactiveSince"`
	} `json:"participantState"`
}

var openEnd = time.Date(2999, 12, 31, 0, 0, 0, 0, time.Local)

// windows reads the participation's window: from activeSince until inactiveSince, open without an end
// date. Only an activated participation has a window: none without a valid activeSince (never activated)
// and none for a rejected registration.
func (bm backendMeter) windows() []Window {
	if bm.State == nil || bm.State.ActiveSince == nil || strings.EqualFold(bm.ProcessState, "REJECTED") {
		return nil
	}
	from, ok := day(*bm.State.ActiveSince)
	until := openEnd
	if bm.State.InactiveSince != nil {
		if d, valid := day(*bm.State.InactiveSince); valid {
			until = d
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
