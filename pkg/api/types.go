package api

import (
	"net/netip"
)

// ActionType defines the type of update (Add or Delete).
type ActionType string

const (
	// Add indicates a route addition or update.
	Add ActionType = "ADD"
	// Delete indicates a route removal.
	Delete ActionType = "DELETE"
)

// RIBUpdate represents an update from an installer to the RIB.
type RIBUpdate struct {
	Action    ActionType
	Protocol  string // e.g., "STATIC", "BGP"
	Prefix    netip.Prefix
	NextHop   netip.Addr
	Metric    uint32
	AdminDist uint8
}

// FIBUpdate represents an update from the RIB to the FIB.
// It indicates a change in the best path for a prefix.
type FIBUpdate struct {
	Action  ActionType
	Prefix  netip.Prefix
	NextHop netip.Addr
}

// AFTUpdate represents an update from the FIB to the Telemetry server.
// It is used to generate gNMI notifications.
type AFTUpdate struct {
	Action  ActionType
	Prefix  netip.Prefix
	NextHop netip.Addr
}

// RouteInstaller is the interface for modules that inject routes into the RIB.
type RouteInstaller interface {
	// Start begins the installer loop, passing updates into the provided channel.
	Start(ribChan chan<- RIBUpdate) error
	// Stop terminates the installer loop.
	Stop() error
}
