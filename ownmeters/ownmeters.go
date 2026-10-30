// Package ownmeters asks the backend which metering points belong to a member (energystore 261025-ca1).
//
// The backend answers GET /participant with the caller's bearer token: for a member token (group /EEG_USER)
// it returns the member's own participants (matched by tenant and token email), each with its "meters".
// The answer is cached briefly per token subject and tenant, so a dashboard does not call the backend for
// every request.
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
	meters map[string]bool
	until  time.Time
}

// Resolver resolves and caches the own metering points of member tokens.
type Resolver struct {
	baseURL string
	client  *http.Client

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

// Meters returns the caller's own metering points (upper case). authorization is the caller's
// Authorization header, forwarded unchanged; subject and tenant key the cache. The returned map must
// not be changed.
func (r *Resolver) Meters(ctx context.Context, authorization, subject, tenant string) (map[string]bool, error) {
	if r.baseURL == "" {
		return nil, ErrNotConfigured
	}
	key := subject + "\x00" + strings.ToUpper(tenant)
	if subject != "" {
		r.mu.Lock()
		e, ok := r.cache[key]
		r.mu.Unlock()
		if ok && time.Now().Before(e.until) {
			return e.meters, nil
		}
	}

	meters, err := r.fetch(ctx, authorization, tenant)
	if err != nil {
		return nil, err
	}
	if subject != "" {
		r.store(key, meters)
	}
	return meters, nil
}

func (r *Resolver) fetch(ctx context.Context, authorization, tenant string) (map[string]bool, error) {
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
		Meters []struct {
			MeteringPoint string `json:"meteringPoint"`
		} `json:"meters"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&participants); err != nil {
		return nil, fmt.Errorf("backend participant: %w", err)
	}
	meters := map[string]bool{}
	for _, p := range participants {
		for _, m := range p.Meters {
			if m.MeteringPoint != "" {
				meters[strings.ToUpper(m.MeteringPoint)] = true
			}
		}
	}
	return meters, nil
}

func (r *Resolver) store(key string, meters map[string]bool) {
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
