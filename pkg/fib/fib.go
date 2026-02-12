package fib

import (
	"fmt"
	"net/netip"
	"sync"

	"github.com/openconfig/aft-simulator/pkg/api"
)

// FIB maintains the active forwarding state.
type FIB struct {
	mu            sync.RWMutex
	activeRoutes  map[netip.Prefix]netip.Addr
	telemetryChan chan<- api.AFTUpdate
}

// New creates a new FIB.
func New(telemetryChan chan<- api.AFTUpdate) *FIB {
	return &FIB{
		activeRoutes:  make(map[netip.Prefix]netip.Addr),
		telemetryChan: telemetryChan,
	}
}

// Start listens for updates on the input channel and processes them.
func (f *FIB) Start(inputChan <-chan api.FIBUpdate) {
	for update := range inputChan {
		f.Update(update)
	}
}

// Update updates the FIB state and notifies the telemetry server.
func (f *FIB) Update(update api.FIBUpdate) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch update.Action {
	case api.Add:
		f.activeRoutes[update.Prefix] = update.NextHop
		f.telemetryChan <- api.AFTUpdate{
			Action:  api.Add,
			Prefix:  update.Prefix,
			NextHop: update.NextHop,
		}
		fmt.Printf("FIB: Added/Updated route %s via %s\n", update.Prefix, update.NextHop)

	case api.Delete:
		if _, exists := f.activeRoutes[update.Prefix]; exists {
			delete(f.activeRoutes, update.Prefix)
			f.telemetryChan <- api.AFTUpdate{
				Action: api.Delete,
				Prefix: update.Prefix,
			}
			fmt.Printf("FIB: Deleted route %s\n", update.Prefix)
		}
	}
}

// GetSnapshot returns the current state of the FIB as a list of AFTUpdates.
// This is used to synchronize new telemetry clients.
func (f *FIB) GetSnapshot() []api.AFTUpdate {
	f.mu.RLock()
	defer f.mu.RUnlock()

	snapshot := make([]api.AFTUpdate, 0, len(f.activeRoutes))
	for prefix, nh := range f.activeRoutes {
		snapshot = append(snapshot, api.AFTUpdate{
			Action:  api.Add,
			Prefix:  prefix,
			NextHop: nh,
		})
	}
	return snapshot
}
