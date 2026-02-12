package mock

import (
	"fmt"
	"net/netip"
	"time"

	"github.com/openconfig/aft-simulator/pkg/api"
)

// MockInstaller injects a sequence of route updates.
type MockInstaller struct {
	stopChan chan struct{}
}

// New creates a new MockInstaller.
func New() *MockInstaller {
	return &MockInstaller{
		stopChan: make(chan struct{}),
	}
}

// Start begins the mock installer loop.
func (m *MockInstaller) Start(ribChan chan<- api.RIBUpdate) error {
	go func() {
		fmt.Println("MockInstaller: Starting...")

		// Route 1: 10.0.0.0/24 via 192.168.1.1
		p1 := netip.MustParsePrefix("10.0.0.0/24")
		nh1 := netip.MustParseAddr("192.168.1.1")

		// Route 2: 20.0.0.0/24 via 192.168.1.2
		p2 := netip.MustParsePrefix("20.0.0.0/24")
		nh2 := netip.MustParseAddr("192.168.1.2")

		select {
		case <-time.After(2 * time.Second):
			fmt.Println("MockInstaller: Injecting Route 1 (ADD)")
			ribChan <- api.RIBUpdate{
				Action:    api.Add,
				Protocol:  "MOCK",
				Prefix:    p1,
				NextHop:   nh1,
				Metric:    10,
				AdminDist: 1,
			}
		case <-m.stopChan:
			return
		}

		select {
		case <-time.After(2 * time.Second):
			fmt.Println("MockInstaller: Injecting Route 2 (ADD)")
			ribChan <- api.RIBUpdate{
				Action:    api.Add,
				Protocol:  "MOCK",
				Prefix:    p2,
				NextHop:   nh2,
				Metric:    10,
				AdminDist: 1,
			}
		case <-m.stopChan:
			return
		}

		select {
		case <-time.After(2 * time.Second):
			fmt.Println("MockInstaller: Deleting Route 1")
			ribChan <- api.RIBUpdate{
				Action:   api.Delete,
				Protocol: "MOCK",
				Prefix:   p1,
			}
		case <-m.stopChan:
			return
		}

		fmt.Println("MockInstaller: Finished sequence.")
	}()
	return nil
}

// Stop stops the mock installer.
func (m *MockInstaller) Stop() error {
	close(m.stopChan)
	return nil
}
