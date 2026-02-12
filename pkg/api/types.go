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
	Protocol  string // e.g., ProtocolStatic, ProtocolBGP
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

// AFTEntryType defines the type of AFT entry being updated.
type AFTEntryType string

const (
	// AFTEntryPrefix indicates an IPv4 or IPv6 prefix entry.
	AFTEntryPrefix AFTEntryType = "PREFIX"
	// AFTEntryNextHopGroup indicates a next-hop-group entry.
	AFTEntryNextHopGroup AFTEntryType = "NEXT_HOP_GROUP"
	// AFTEntryNextHop indicates a next-hop entry.
	AFTEntryNextHop AFTEntryType = "NEXT_HOP"
)

// AFTUpdate represents an update from the FIB to the Telemetry server.
// It is used to generate gNMI notifications.
type AFTUpdate struct {
	Action       ActionType
	EntryType    AFTEntryType
	Prefix       netip.Prefix // Used if EntryType == AFTEntryPrefix
	NextHopGroup uint64       // Used if EntryType == AFTEntryPrefix or AFTEntryNextHopGroup
	NextHop      netip.Addr   // Used if EntryType == AFTEntryNextHopGroup or AFTEntryNextHop
}

// RouteInstaller is the interface for modules that inject routes into the RIB.
type RouteInstaller interface {
	// Start begins the installer loop, passing updates into the provided channel.
	Start(ribChan chan<- RIBUpdate) error
	// Stop terminates the installer loop.
	Stop() error
}

// Common Protocol Constants
const (
	ProtocolStatic = "STATIC"
	ProtocolOSPF   = "OSPF"
	ProtocolMock   = "MOCK"
	ProtocolBGP    = "BGP"
)

// Common Network Instance Constants
const (
	NetworkInstanceDefault = "DEFAULT"
)
