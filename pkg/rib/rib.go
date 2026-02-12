package rib

import (
	"context"
	"fmt"
	"net/netip"
	"sync"

	"github.com/openconfig/aft-simulator/pkg/api"
)

// RouteEntry represents a single route from a specific protocol.
type RouteEntry struct {
	Protocol  string
	NextHop   netip.Addr
	Metric    uint32
	AdminDist uint8
}

// RIB maintains the routing table and selects the best path for each prefix.
type RIB struct {
	mu      sync.RWMutex
	routes  map[netip.Prefix][]RouteEntry
	fibChan chan<- api.FIBUpdate
}

// New creates a new RIB.
func New(fibChan chan<- api.FIBUpdate) *RIB {
	return &RIB{
		routes:  make(map[netip.Prefix][]RouteEntry),
		fibChan: fibChan,
	}
}

// Start listens for updates on the input channel and processes them.
func (r *RIB) Start(ctx context.Context, inputChan <-chan api.RIBUpdate) error {
	defer close(r.fibChan)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-inputChan:
			if !ok {
				return nil
			}
			switch update.Action {
			case api.Add:
				r.AddRoute(update)
			case api.Delete:
				r.DeleteRoute(update)
			}
		}
	}
}

// AddRoute adds or updates a route in the RIB.
func (r *RIB) AddRoute(update api.RIBUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, exists := r.routes[update.Prefix]
	if !exists {
		entries = []RouteEntry{}
	}

	newEntry := RouteEntry{
		Protocol:  update.Protocol,
		NextHop:   update.NextHop,
		Metric:    update.Metric,
		AdminDist: update.AdminDist,
	}

	// Check if we are updating an existing entry for the same protocol
	updated := false
	for i, entry := range entries {
		if entry.Protocol == update.Protocol {
			entries[i] = newEntry
			updated = true
			break
		}
	}
	if !updated {
		entries = append(entries, newEntry)
	}
	r.routes[update.Prefix] = entries

	r.recalculateBestPath(update.Prefix)
}

// DeleteRoute removes a route from the RIB.
func (r *RIB) DeleteRoute(update api.RIBUpdate) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, exists := r.routes[update.Prefix]
	if !exists {
		return
	}

	newEntries := []RouteEntry{}
	for _, entry := range entries {
		if entry.Protocol != update.Protocol {
			newEntries = append(newEntries, entry)
		}
	}

	if len(newEntries) == 0 {
		delete(r.routes, update.Prefix)
		// Notify FIB of removal
		r.fibChan <- api.FIBUpdate{
			Action: api.Delete,
			Prefix: update.Prefix,
		}
		return
	}

	r.routes[update.Prefix] = newEntries
	r.recalculateBestPath(update.Prefix)
}

// recalculateBestPath determines the best route and updates the FIB if necessary.
// Must be called with lock held.
func (r *RIB) recalculateBestPath(prefix netip.Prefix) {
	entries := r.routes[prefix]
	if len(entries) == 0 {
		return
	}

	best := entries[0]
	for _, entry := range entries[1:] {
		if entry.AdminDist < best.AdminDist {
			best = entry
		} else if entry.AdminDist == best.AdminDist {
			if entry.Metric < best.Metric {
				best = entry
			}
		}
	}

	// For now, always send update. Optimization: Check against current FIB state if we stored it.
	// Since we don't store FIB state in RIB, we rely on FIB to handle no-op updates or
	// we just send it. Sending it is safer to ensure consistency.
	r.fibChan <- api.FIBUpdate{
		Action:  api.Add,
		Prefix:  prefix,
		NextHop: best.NextHop,
	}
	fmt.Printf("RIB: Best path for %s is via %s (Proto: %s, AD: %d, Metric: %d)\n", prefix, best.NextHop, best.Protocol, best.AdminDist, best.Metric)
}
