package fib

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/netip"
	"sync"

	"github.com/openconfig/aft-simulator/pkg/api"
)

// FIB maintains the active forwarding state.
type FIB struct {
	mu            sync.RWMutex
	activeRoutes  map[netip.Prefix]netip.Addr
	nhRefCount    map[netip.Addr]int
	nhgRefCount   map[uint64]int
	telemetryChan chan<- api.AFTUpdate
}

// New creates a new FIB.
func New(telemetryChan chan<- api.AFTUpdate) *FIB {
	return &FIB{
		activeRoutes:  make(map[netip.Prefix]netip.Addr),
		nhRefCount:    make(map[netip.Addr]int),
		nhgRefCount:   make(map[uint64]int),
		telemetryChan: telemetryChan,
	}
}

// Start listens for updates on the input channel and processes them.
func (f *FIB) Start(ctx context.Context, inputChan <-chan api.FIBUpdate) error {
	defer close(f.telemetryChan)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update, ok := <-inputChan:
			if !ok {
				return nil
			}
			f.Update(update)
		}
	}
}

// nhgID generates a deterministic ID for a NextHopGroup based on the NextHop IP.
func nhgID(nh netip.Addr) uint64 {
	h := fnv.New64a()
	h.Write(nh.AsSlice())
	return h.Sum64()
}

// Update updates the FIB state and notifies the telemetry server.
func (f *FIB) Update(update api.FIBUpdate) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch update.Action {
	case api.Add:
		// If the route already exists, we might need to handle the old NH/NHG.
		// For simplicity, let's assume it's a new route or the NH is the same.
		// A full implementation would decrement ref counts for the old NH/NHG.
		if oldNH, exists := f.activeRoutes[update.Prefix]; exists && oldNH != update.NextHop {
			f.deleteRoute(update.Prefix, oldNH)
		}

		f.activeRoutes[update.Prefix] = update.NextHop
		nhg := nhgID(update.NextHop)

		// 1. Add NextHop if new
		f.nhRefCount[update.NextHop]++
		if f.nhRefCount[update.NextHop] == 1 {
			f.telemetryChan <- api.AFTUpdate{
				Action:    api.Add,
				EntryType: api.AFTEntryNextHop,
				NextHop:   update.NextHop,
			}
		}

		// 2. Add NextHopGroup if new
		f.nhgRefCount[nhg]++
		if f.nhgRefCount[nhg] == 1 {
			f.telemetryChan <- api.AFTUpdate{
				Action:       api.Add,
				EntryType:    api.AFTEntryNextHopGroup,
				NextHopGroup: nhg,
				NextHop:      update.NextHop,
			}
		}

		// 3. Add Prefix
		f.telemetryChan <- api.AFTUpdate{
			Action:       api.Add,
			EntryType:    api.AFTEntryPrefix,
			Prefix:       update.Prefix,
			NextHopGroup: nhg,
		}
		fmt.Printf("FIB: Added/Updated route %s via %s (NHG: %d)\n", update.Prefix, update.NextHop, nhg)

	case api.Delete:
		if oldNH, exists := f.activeRoutes[update.Prefix]; exists {
			f.deleteRoute(update.Prefix, oldNH)
		}
	}
}

func (f *FIB) deleteRoute(prefix netip.Prefix, nh netip.Addr) {
	delete(f.activeRoutes, prefix)
	nhg := nhgID(nh)

	// 1. Delete Prefix
	f.telemetryChan <- api.AFTUpdate{
		Action:    api.Delete,
		EntryType: api.AFTEntryPrefix,
		Prefix:    prefix,
	}

	// 2. Delete NextHopGroup if no longer used
	f.nhgRefCount[nhg]--
	if f.nhgRefCount[nhg] == 0 {
		delete(f.nhgRefCount, nhg)
		f.telemetryChan <- api.AFTUpdate{
			Action:       api.Delete,
			EntryType:    api.AFTEntryNextHopGroup,
			NextHopGroup: nhg,
		}
	}

	// 3. Delete NextHop if no longer used
	f.nhRefCount[nh]--
	if f.nhRefCount[nh] == 0 {
		delete(f.nhRefCount, nh)
		f.telemetryChan <- api.AFTUpdate{
			Action:    api.Delete,
			EntryType: api.AFTEntryNextHop,
			NextHop:   nh,
		}
	}
	fmt.Printf("FIB: Deleted route %s\n", prefix)
}

// GetSnapshot returns the current state of the FIB as a list of AFTUpdates.
// This is used to synchronize new telemetry clients.
func (f *FIB) GetSnapshot() []api.AFTUpdate {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var snapshot []api.AFTUpdate

	// 1. Add all NextHops
	for nh := range f.nhRefCount {
		snapshot = append(snapshot, api.AFTUpdate{
			Action:    api.Add,
			EntryType: api.AFTEntryNextHop,
			NextHop:   nh,
		})
	}

	// 2. Add all NextHopGroups
	for nhg := range f.nhgRefCount {
		// We need to find the NH for this NHG to populate the snapshot correctly.
		// Since NHG ID is derived from NH, we can find it by iterating over activeRoutes
		// or we could store the NH in the nhgRefCount map.
		// For simplicity, let's just find one route that uses this NHG.
		var nh netip.Addr
		for _, routeNH := range f.activeRoutes {
			if nhgID(routeNH) == nhg {
				nh = routeNH
				break
			}
		}
		snapshot = append(snapshot, api.AFTUpdate{
			Action:       api.Add,
			EntryType:    api.AFTEntryNextHopGroup,
			NextHopGroup: nhg,
			NextHop:      nh,
		})
	}

	// 3. Add all Prefixes
	for prefix, nh := range f.activeRoutes {
		snapshot = append(snapshot, api.AFTUpdate{
			Action:       api.Add,
			EntryType:    api.AFTEntryPrefix,
			Prefix:       prefix,
			NextHopGroup: nhgID(nh),
		})
	}

	return snapshot
}
