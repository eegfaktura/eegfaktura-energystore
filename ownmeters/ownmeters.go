// Package ownmeters asks the backend which metering points belong to a member, and from when until when
// (energystore 261025-ca1).
//
// The backend answers GET /participant with the caller's bearer token: for a member token it returns the
// member's own participants (matched by tenant and token email), each with its "meters". The answer is
// cached briefly per token subject and tenant, so a dashboard does not call the backend for every request.
package ownmeters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	cacheTTL   = time.Minute
	maxEntries = 1000
	timeout    = 5 * time.Second
	maxBody    = 4 << 20
)

// ErrNotConfigured: no backend URL is set; member requests must then be refused.
var ErrNotConfigured = errors.New("backend url not configured")

type entry struct {
	meters Meters
	until  time.Time
}

// Resolver resolves and caches the own metering points of member tokens.
type Resolver struct {
	baseURL string
	client  *http.Client
	group   singleflight.Group

	mu    sync.Mutex
	cache map[string]entry
}

// New returns a resolver for the backend at baseURL (e.g. http://eegfaktura-backend-service:8080).
// An empty baseURL yields ErrNotConfigured on every lookup.
func New(baseURL string) *Resolver {
	return &Resolver{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
		cache:   map[string]entry{},
	}
}

// Meters returns the caller's own metering points. authorization is the caller's Authorization header,
// forwarded unchanged; subject and tenant key the cache, and parallel misses of one key share one
// backend call. The result must not be changed. A nil Resolver counts as not configured.
func (r *Resolver) Meters(ctx context.Context, authorization, subject, tenant string) (Meters, error) {
	if r == nil || r.baseURL == "" {
		return nil, ErrNotConfigured
	}
	if subject == "" {
		return r.fetch(ctx, authorization, tenant)
	}
	key := subject + "\x00" + strings.ToUpper(tenant)
	r.mu.Lock()
	e, ok := r.cache[key]
	r.mu.Unlock()
	if ok && time.Now().Before(e.until) {
		return e.meters, nil
	}

	v, err, _ := r.group.Do(key, func() (any, error) {
		// shared by every caller of the key: not cancelled with the first caller's request
		meters, err := r.fetch(context.WithoutCancel(ctx), authorization, tenant)
		if err == nil {
			r.store(key, meters)
		}
		return meters, err
	})
	if err != nil {
		return nil, err
	}
	return v.(Meters), nil
}

func (r *Resolver) fetch(ctx context.Context, authorization, tenant string) (Meters, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/participant", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("tenant", tenant)
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("backend participant: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend participant: status %d", resp.StatusCode)
	}

	var participants []struct {
		Meters []backendMeter `json:"meters"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&participants); err != nil {
		return nil, fmt.Errorf("backend participant: %w", err)
	}
	meters := Meters{}
	for _, p := range participants {
		for _, m := range p.Meters {
			if m.MeteringPoint == "" {
				continue
			}
			id := strings.ToUpper(m.MeteringPoint)
			meters[id] = append(meters[id], m.windows()...)
		}
	}
	return meters, nil
}

func (r *Resolver) store(key string, meters Meters) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if len(r.cache) >= maxEntries {
		for k, e := range r.cache {
			if !now.Before(e.until) {
				delete(r.cache, k)
			}
		}
		if len(r.cache) >= maxEntries {
			r.cache = map[string]entry{}
		}
	}
	r.cache[key] = entry{meters: meters, until: now.Add(cacheTTL)}
}
