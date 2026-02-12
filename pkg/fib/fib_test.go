package fib

import (
	"net/netip"
	"testing"
	"time"

	"github.com/openconfig/aft-simulator/pkg/api"
)

func TestFIB_Update_AddDelete(t *testing.T) {
	telemetryChan := make(chan api.AFTUpdate, 10)
	f := New(telemetryChan)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nh := netip.MustParseAddr("192.168.1.1")

	// Test ADD
	f.Update(api.FIBUpdate{
		Action:  api.Add,
		Prefix:  prefix,
		NextHop: nh,
	})

	select {
	case update := <-telemetryChan:
		if update.Action != api.Add {
			t.Errorf("Expected ADD, got %v", update.Action)
		}
		if update.Prefix != prefix {
			t.Errorf("Expected Prefix %s, got %s", prefix, update.Prefix)
		}
		if update.NextHop != nh {
			t.Errorf("Expected NextHop %s, got %s", nh, update.NextHop)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (ADD)")
	}

	// Test DELETE
	f.Update(api.FIBUpdate{
		Action: api.Delete,
		Prefix: prefix,
	})

	select {
	case update := <-telemetryChan:
		if update.Action != api.Delete {
			t.Errorf("Expected DELETE, got %v", update.Action)
		}
		if update.Prefix != prefix {
			t.Errorf("Expected Prefix %s, got %s", prefix, update.Prefix)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (DELETE)")
	}
}

func TestFIB_GetSnapshot(t *testing.T) {
	telemetryChan := make(chan api.AFTUpdate, 10)
	f := New(telemetryChan)

	prefix1 := netip.MustParsePrefix("10.0.0.0/24")
	nh1 := netip.MustParseAddr("192.168.1.1")
	prefix2 := netip.MustParsePrefix("20.0.0.0/24")
	nh2 := netip.MustParseAddr("192.168.1.2")

	f.Update(api.FIBUpdate{Action: api.Add, Prefix: prefix1, NextHop: nh1})
	f.Update(api.FIBUpdate{Action: api.Add, Prefix: prefix2, NextHop: nh2})

	// Drain channel
	<-telemetryChan
	<-telemetryChan

	snapshot := f.GetSnapshot()
	if len(snapshot) != 2 {
		t.Fatalf("Expected snapshot length 2, got %d", len(snapshot))
	}

	found1 := false
	found2 := false

	for _, update := range snapshot {
		if update.Action != api.Add {
			t.Errorf("Expected snapshot items to be ADD, got %v", update.Action)
		}
		if update.Prefix == prefix1 && update.NextHop == nh1 {
			found1 = true
		}
		if update.Prefix == prefix2 && update.NextHop == nh2 {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("Snapshot missing prefix1")
	}
	if !found2 {
		t.Errorf("Snapshot missing prefix2")
	}
}
