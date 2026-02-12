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

	// Expect NH
	select {
	case update := <-telemetryChan:
		if update.Action != api.Add || update.EntryType != api.AFTEntryNextHop || update.NextHop != nh {
			t.Errorf("Expected ADD NH, got %+v", update)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (ADD NH)")
	}

	// Expect NHG
	select {
	case update := <-telemetryChan:
		if update.Action != api.Add || update.EntryType != api.AFTEntryNextHopGroup || update.NextHop != nh {
			t.Errorf("Expected ADD NHG, got %+v", update)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (ADD NHG)")
	}

	// Expect Prefix
	select {
	case update := <-telemetryChan:
		if update.Action != api.Add || update.EntryType != api.AFTEntryPrefix || update.Prefix != prefix {
			t.Errorf("Expected ADD Prefix, got %+v", update)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (ADD Prefix)")
	}

	// Test DELETE
	f.Update(api.FIBUpdate{
		Action: api.Delete,
		Prefix: prefix,
	})

	// Expect Prefix
	select {
	case update := <-telemetryChan:
		if update.Action != api.Delete || update.EntryType != api.AFTEntryPrefix || update.Prefix != prefix {
			t.Errorf("Expected DELETE Prefix, got %+v", update)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (DELETE Prefix)")
	}

	// Expect NHG
	select {
	case update := <-telemetryChan:
		if update.Action != api.Delete || update.EntryType != api.AFTEntryNextHopGroup {
			t.Errorf("Expected DELETE NHG, got %+v", update)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (DELETE NHG)")
	}

	// Expect NH
	select {
	case update := <-telemetryChan:
		if update.Action != api.Delete || update.EntryType != api.AFTEntryNextHop || update.NextHop != nh {
			t.Errorf("Expected DELETE NH, got %+v", update)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for AFT update (DELETE NH)")
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

	// Drain channel (6 updates total: 2 NH, 2 NHG, 2 Prefix)
	for i := 0; i < 6; i++ {
		<-telemetryChan
	}

	snapshot := f.GetSnapshot()
	if len(snapshot) != 6 {
		t.Fatalf("Expected snapshot length 6, got %d", len(snapshot))
	}

	foundPrefix1 := false
	foundPrefix2 := false
	foundNH1 := false
	foundNH2 := false

	for _, update := range snapshot {
		if update.Action != api.Add {
			t.Errorf("Expected snapshot items to be ADD, got %v", update.Action)
		}
		if update.EntryType == api.AFTEntryPrefix {
			if update.Prefix == prefix1 {
				foundPrefix1 = true
			}
			if update.Prefix == prefix2 {
				foundPrefix2 = true
			}
		}
		if update.EntryType == api.AFTEntryNextHop {
			if update.NextHop == nh1 {
				foundNH1 = true
			}
			if update.NextHop == nh2 {
				foundNH2 = true
			}
		}
	}

	if !foundPrefix1 {
		t.Errorf("Snapshot missing prefix1")
	}
	if !foundPrefix2 {
		t.Errorf("Snapshot missing prefix2")
	}
	if !foundNH1 {
		t.Errorf("Snapshot missing nh1")
	}
	if !foundNH2 {
		t.Errorf("Snapshot missing nh2")
	}
}
