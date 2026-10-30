package ownmeters

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tenants = []string{"TE100001"}

const answer = `[
 {"id":"P1","meters":[
  {"meteringPoint":"at001","registeredSince":"2025-01-01","participantState":{"activeSince":"2026-06-01","inactiveSince":null}},
  {"meteringPoint":"AT002","registeredSince":"2025-01-01","participantState":{"activeSince":null,"inactiveSince":null}},
  {"meteringPoint":"AT003","registeredSince":"0000-01-01","participantState":{"activeSince":"0000-01-01","inactiveSince":null}},
  {"meteringPoint":"AT004","registeredSince":"2025-01-01","processState":"REJECTED","participantState":{"activeSince":"2026-01-01","inactiveSince":null}},
  {"meteringPoint":"AT005","registeredSince":"2025-01-01"}]},
 {"id":"P2","meters":[
  {"meteringPoint":"AT001","registeredSince":"2025-01-01","participantState":{"activeSince":"2026-02-01","inactiveSince":"2026-03-31"}}]}]`

func backend(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/participant", r.URL.Path)
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "TE100001", r.Header.Get("tenant"))
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(s.Close)
	return s, &calls
}

func date(s string) time.Time {
	d, _ := time.ParseInLocation("2006-01-02", s, time.Local)
	return d
}

func TestNotConfigured(t *testing.T) {
	var nilResolver *Resolver
	_, err := nilResolver.Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
	assert.ErrorIs(t, err, ErrNotConfigured)
	_, err = New("").Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestMetersAndWindows(t *testing.T) {
	s, _ := backend(t, 200, answer)
	m, err := New(s.URL+"/").Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
	require.NoError(t, err)
	assert.Equal(t, Meters{
		"AT001": {{From: date("2026-02-01"), Until: date("2026-03-31")}, {From: date("2026-06-01"), Until: openEnd}}, // sorted by start
		"AT002": {{From: date("2025-01-01"), Until: openEnd}},                                                        // activeSince null: registeredSince, open end (open point)
		"AT003": nil,                                                                                                 // no valid start date
		"AT004": {{From: date("2026-01-01"), Until: openEnd}},                                                        // rejected: not excluded yet (open point)
		"AT005": {{From: date("2025-01-01"), Until: openEnd}},                                                        // no participantState: registeredSince
	}, m)
	assert.True(t, m.Owns("at003"))
	assert.False(t, m.Owns("AT009"))
}

func TestBackendErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
	}{{401, ""}, {500, "[]"}, {200, "{not json"}} {
		s, _ := backend(t, c.status, c.body)
		_, err := New(s.URL).Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
		assert.Error(t, err, "%d %s", c.status, c.body)
	}
	s, _ := backend(t, 200, "null")
	m, err := New(s.URL).Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
	require.NoError(t, err)
	assert.Empty(t, m, "no participant for the token's email")
}

func TestCachePerSubjectAndTenant(t *testing.T) {
	s, calls := backend(t, 200, answer)
	r := New(s.URL)
	for i := 0; i < 3; i++ {
		_, err := r.Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), calls.Load())
	_, _ = r.Meters(context.Background(), "Bearer tok", "sub", "te100001", tenants)
	assert.Equal(t, int32(1), calls.Load(), "tenant case does not matter")
	_, _ = r.Meters(context.Background(), "Bearer tok", "other", "TE100001", tenants)
	assert.Equal(t, int32(2), calls.Load(), "another subject")
	_, _ = r.Meters(context.Background(), "Bearer tok", "", "TE100001", tenants)
	_, _ = r.Meters(context.Background(), "Bearer tok", "", "TE100001", tenants)
	assert.Equal(t, int32(4), calls.Load(), "no subject: not cached")
}

func TestParallelMissesShareOneCall(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-release
		_, _ = fmt.Fprint(w, answer)
	}))
	t.Cleanup(s.Close)
	r := New(s.URL)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := r.Meters(context.Background(), "Bearer tok", "sub", "TE100001", tenants)
			assert.NoError(t, err)
		}()
	}
	time.Sleep(100 * time.Millisecond) // let the goroutines join the pending call
	close(release)
	wg.Wait()
	assert.Equal(t, int32(1), calls.Load())
}

func TestClampDays(t *testing.T) {
	m := Meters{"AT001": {{From: date("2026-02-01"), Until: date("2026-03-31")}, {From: date("2026-06-01"), Until: openEnd}}}
	ms := func(s string) int64 { return date(s).UnixMilli() }
	utcMidnight := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli() // as the web sends activeSince

	cases := []struct {
		name              string
		from, until       int64
		wantFrom, wantTil int64
		ok                bool
	}{
		{"inside unchanged", utcMidnight, ms("2026-03-31"), utcMidnight, ms("2026-03-31"), true},
		{"partly before", ms("2026-01-01"), ms("2026-02-15"), ms("2026-02-01"), ms("2026-02-15"), true},
		{"partly after", ms("2026-03-01"), ms("2026-04-30"), ms("2026-03-01"), ms("2026-03-31"), true},
		{"open until overlaps the open window most", ms("2026-02-10"), 0, ms("2026-06-01"), 0, true},
		{"second window", ms("2026-05-01"), ms("2026-07-01"), ms("2026-06-01"), ms("2026-07-01"), true},
		{"gap between", ms("2026-04-01"), ms("2026-05-31"), 0, 0, false},
		{"both, first overlaps more", ms("2026-03-01"), ms("2026-06-10"), ms("2026-03-01"), ms("2026-03-31"), true},
		{"both, second overlaps more", ms("2026-03-25"), ms("2026-12-31"), ms("2026-06-01"), ms("2026-12-31"), true},
		{"before all", ms("2025-01-01"), ms("2026-01-31"), 0, 0, false},
	}
	for _, c := range cases {
		f, u, ok := m.ClampDays("at001", c.from, c.until)
		assert.Equal(t, c.ok, ok, c.name)
		assert.Equal(t, c.wantFrom, f, c.name)
		assert.Equal(t, c.wantTil, u, c.name)
	}
	tie := Meters{"AT001": {{From: date("2026-01-01"), Until: date("2026-01-10")}, {From: date("2026-01-21"), Until: date("2026-01-30")}}}
	f, u, ok := tie.ClampDays("AT001", ms("2026-01-06"), ms("2026-01-25"))
	assert.True(t, ok)
	assert.Equal(t, [2]int64{ms("2026-01-21"), ms("2026-01-25")}, [2]int64{f, u}, "tie: the latest window")
	f, u, ok = tie.ClampDays("AT001", ms("2026-01-25"), 0)
	assert.True(t, ok)
	assert.Equal(t, [2]int64{ms("2026-01-25"), ms("2026-01-30")}, [2]int64{f, u}, "open until, ended window: its last day")

	_, _, ok = m.ClampDays("AT009", ms("2026-02-10"), ms("2026-02-11"))
	assert.False(t, ok, "foreign meter")
	_, _, ok = Meters{"AT003": nil}.ClampDays("AT003", ms("2026-02-10"), ms("2026-02-11"))
	assert.False(t, ok, "own meter without a window")
}

func TestClampRange(t *testing.T) {
	m := Meters{
		"AT001": {{From: date("2026-02-01"), Until: date("2026-03-31")}},
		"AT002": {{From: date("2026-03-01"), Until: openEnd}},
	}
	ms := func(s string) int64 { return date(s).UnixMilli() }
	endOf := func(s string) int64 { return date(s).AddDate(0, 0, 1).UnixMilli() - 1 }

	s, e, ok := m.ClampRange([]string{"AT001"}, ms("2026-03-31"), endOf("2026-04-01"))
	assert.True(t, ok)
	assert.Equal(t, ms("2026-03-31"), s)
	assert.Equal(t, endOf("2026-03-31"), e, "the last day counts whole")

	s, e, ok = m.ClampRange([]string{"AT001", "AT002"}, ms("2026-01-01"), endOf("2026-12-31"))
	assert.True(t, ok)
	assert.Equal(t, ms("2026-03-01"), s)
	assert.Equal(t, endOf("2026-03-31"), e)

	_, _, ok = m.ClampRange([]string{"AT001"}, ms("2026-04-01"), endOf("2026-04-01"))
	assert.False(t, ok, "outside")
	_, _, ok = m.ClampRange([]string{"AT001", "AT002"}, ms("2026-02-01"), endOf("2026-02-10"))
	assert.False(t, ok, "AT002 not yet the member's")
}

// GEA: the backend keys participants by tenant; the lookup unites every tenant of the token (distinct,
// case-insensitive, at most maxTenants) and fails when one call fails.
func TestEveryTenantOfTheToken(t *testing.T) {
	var asked []string
	var mu sync.Mutex
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.Header.Get("tenant"))
		mu.Unlock()
		switch r.Header.Get("tenant") {
		case "GC123456-001":
			_, _ = fmt.Fprint(w, `[{"meters":[{"meteringPoint":"AT001","participantState":{"activeSince":"2026-01-01","inactiveSince":null}}]}]`)
		case "BROKEN":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			_, _ = fmt.Fprint(w, `[]`)
		}
	}))
	t.Cleanup(s.Close)

	m, err := New(s.URL).Meters(context.Background(), "Bearer tok", "sub", "RC123456",
		[]string{"RC123456", "GC123456-001", "gc123456-001", ""})
	require.NoError(t, err)
	assert.True(t, m.Owns("AT001"))
	assert.Equal(t, []string{"RC123456", "GC123456-001"}, asked)

	_, err = New(s.URL).Meters(context.Background(), "Bearer tok", "sub", "RC123456", []string{"RC123456", "BROKEN"})
	assert.Error(t, err)

	asked = nil
	many := make([]string, 15)
	for i := range many {
		many[i] = fmt.Sprintf("T%02d", i)
	}
	_, err = New(s.URL).Meters(context.Background(), "Bearer tok", "sub", "T00", many)
	require.NoError(t, err)
	assert.Len(t, asked, maxTenants)
}
