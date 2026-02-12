package mock

import (
	"context"
	"fmt"
	"math/rand"
	"net/netip"
	"time"

	"github.com/openconfig/aft-simulator/pkg/api"
	"github.com/openconfig/aft-simulator/pkg/config"
)

// MockInstaller injects a sequence of route updates.
type MockInstaller struct {
	cfg config.MockConfig
}

// New creates a new MockInstaller.
func New(cfg config.MockConfig) *MockInstaller {
	return &MockInstaller{cfg: cfg}
}

// Run begins the mock installer loop.
func (m *MockInstaller) Run(ctx context.Context, ribChan chan<- api.RIBUpdate) error {
	if !m.cfg.Enabled {
		return nil
	}

	fmt.Printf("MockInstaller: Starting with target %d routes, churn rate %d/s\n", m.cfg.RouteCount, m.cfg.ChurnRate)

	// Generate initial routes
	prefixes := generatePrefixes(m.cfg.RouteCount)
	nextHops := []netip.Addr{
		netip.MustParseAddr("192.168.1.1"),
		netip.MustParseAddr("192.168.1.2"),
		netip.MustParseAddr("192.168.1.3"),
		netip.MustParseAddr("192.168.1.4"),
	}

	// Initial population
	ticker := time.NewTicker(time.Second / time.Duration(m.cfg.ChurnRate))
	if m.cfg.ChurnRate <= 0 {
		ticker = time.NewTicker(time.Second) // Default slow if invalid
	}
	defer ticker.Stop()

	// Initial Load Phase
	fmt.Println("MockInstaller: Initializing routes...")
	for i, p := range prefixes {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			nh := nextHops[i%len(nextHops)]
			ribChan <- api.RIBUpdate{
				Action:    api.Add,
				Protocol:  api.ProtocolMock,
				Prefix:    p,
				NextHop:   nh,
				Metric:    10,
				AdminDist: 1,
			}
		}
	}
	fmt.Println("MockInstaller: Initial load complete.")

	// Churn Phase
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Pick a random prefix to update
			idx := rng.Intn(len(prefixes))
			p := prefixes[idx]
			
			// Toggle between two next-hops or flap
			nh := nextHops[rng.Intn(len(nextHops))]
			
			// 10% chance to delete, 90% to update/add
			action := api.Add
			if rng.Float32() < 0.1 {
				action = api.Delete
			}

			ribChan <- api.RIBUpdate{
				Action:    action,
				Protocol:  api.ProtocolMock,
				Prefix:    p,
				NextHop:   nh,
				Metric:    10,
				AdminDist: 1,
			}
		}
	}
}

func generatePrefixes(count int) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, count)
	// Generate 10.x.y.0/24
	for i := 0; i < count; i++ {
		// Simple generation: map i to octets
		// Max 256*256 = 65536 routes supported with this simple scheme
		// If count > 65536, this will wrap/collide, but fine for now.
		o2 := (i >> 8) & 0xFF
		o3 := i & 0xFF
		addr := netip.AddrFrom4([4]byte{10, byte(o2), byte(o3), 0})
		prefixes = append(prefixes, netip.PrefixFrom(addr, 24))
	}
	return prefixes
}
