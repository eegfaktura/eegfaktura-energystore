package mqttclient

import (
	"context"
	"sort"
	"time"

	"at.ourproject/energystore/internal/logging/glog"
)

// Lifecycle of the TopicDispatcher's executors: the cap on open stores (import.maxOpen) and the janitor.

// reserveOpen makes room for one more executor under the cap and counts it. At the cap it closes a victim
// in this lane (without holding mu) and waits for that close, or waits until an executor becomes idle. It
// returns false when the dispatcher stops meanwhile. Caller holds mu.
func (d *TopicDispatcher) reserveOpen() bool {
	for d.open >= d.cfg.maxOpen {
		if d.closed || d.ctx.Err() != nil {
			return false
		}
		victim := d.capVictim()
		if victim == nil {
			d.capFree.Wait()
			continue
		}
		victim.busy = true
		d.mu.Unlock()
		glog.Infof("%d stores open (%s), close DB tenant=%s.", d.cfg.maxOpen, maxOpenKey, victim.tenant)
		victim.executor.Close()
		d.mu.Lock()
		d.markClosed(victim)
	}
	d.open++
	return true
}

// capVictim is the idle executor without queued work that was used longest ago, else the one of the tenant
// that stands last in ready. Caller holds mu.
func (d *TopicDispatcher) capVictim() *tenantQueue {
	var best *tenantQueue
	for _, tq := range d.tenants {
		if tq.executor != nil && !tq.busy && !tq.isReady && (best == nil || tq.lastUsed.Before(best.lastUsed)) {
			best = tq
		}
	}
	if best != nil {
		return best
	}
	for i := len(d.ready) - 1; i >= 0; i-- {
		if tq := d.ready[i]; tq.executor != nil && !tq.busy {
			return tq
		}
	}
	return nil
}

// janitor closes executors that became idle.
func (d *TopicDispatcher) janitor(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(min(time.Second, d.cfg.idleTimeout/2))
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			d.closeIdle(now)
		case <-ctx.Done():
			return
		}
	}
}

// closeIdle closes the executors idle for idleTimeout, and the idle ones beyond the newest parallel after
// minIdleClose: at most parallel per call, oldest first, so a tick does not flush the memtables of dozens of
// stores at once. Each closes in its own goroutine (a few hundred ms); the tenant is busy meanwhile, so no
// lane runs it.
func (d *TopicDispatcher) closeIdle(now time.Time) {
	d.mu.Lock()
	var idle []*tenantQueue
	for _, tq := range d.tenants {
		if tq.executor != nil && !tq.busy && !tq.isReady {
			idle = append(idle, tq)
		}
	}
	sort.Slice(idle, func(i, j int) bool { return idle[i].lastUsed.After(idle[j].lastUsed) })
	var closing []*tenantQueue
	for i := len(idle) - 1; i >= 0 && len(closing) < d.cfg.parallel; i-- { // oldest first
		tq := idle[i]
		idleFor := now.Sub(tq.lastUsed)
		if (i >= d.cfg.parallel && idleFor >= minIdleClose) || idleFor >= d.cfg.idleTimeout {
			tq.busy = true
			closing = append(closing, tq)
		}
	}
	d.mu.Unlock()

	// The caller (the janitor) still holds its own count, so Close's Wait cannot have returned.
	d.wg.Add(len(closing))
	for _, tq := range closing {
		go d.closeExecutor(tq, now.Sub(tq.lastUsed))
	}
}

func (d *TopicDispatcher) closeExecutor(tq *tenantQueue, idle time.Duration) {
	defer d.wg.Done()
	glog.Infof("No message received for %s. Close DB tenant=%s.", idle.Round(time.Second), tq.tenant)
	tq.executor.Close()

	d.mu.Lock()
	d.markClosed(tq)
	d.mu.Unlock()
}

// markClosed records that the executor of tq is closed: it no longer counts against the cap, and the tenant
// is free again (at its place in ready, or ready if it got messages meanwhile). Caller holds mu.
func (d *TopicDispatcher) markClosed(tq *tenantQueue) {
	tq.executor = nil
	tq.busy = false
	d.open--
	switch {
	case tq.isReady:
		d.work.Signal()
	case len(tq.queue) == 0:
		delete(d.tenants, tq.tenant)
	default:
		d.markReady(tq)
	}
	d.capFree.Broadcast()
}
