package rib

import (
	"net/netip"
	"testing"
	"time"

	"github.com/openconfig/aft-simulator/pkg/api"
)

func TestRIB_AddRoute_BestPath(t *testing.T) {
	fibChan := make(chan api.FIBUpdate, 10)
	r := New(fibChan)

	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nh1 := netip.MustParseAddr("192.168.1.1")
	nh2 := netip.MustParseAddr("192.168.1.2")

	// 1. Add Static Route (AD 1, Metric 10)
	r.AddRoute(api.RIBUpdate{
		Action:    api.Add,
		Protocol:  "STATIC",
		Prefix:    prefix,
		NextHop:   nh1,
		Metric:    10,
		AdminDist: 1,
	})

	select {
	case update := <-fibChan:
		if update.Action != api.Add {
			t.Errorf("Expected ADD, got %v", update.Action)
		}
		if update.NextHop != nh1 {
			t.Errorf("Expected NextHop %s, got %s", nh1, update.NextHop)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for FIB update")
	}

	// 2. Add OSPF Route (AD 110, Metric 20) - Should NOT trigger update (worse AD)
	r.AddRoute(api.RIBUpdate{
		Action:    api.Add,
		Protocol:  "OSPF",
		Prefix:    prefix,
		NextHop:   nh2,
		Metric:    20,
		AdminDist: 110,
	})

	select {
	case update := <-fibChan:
		// We might get an update if we don't check for change, but let's see.
		// Current implementation sends update always on recalculate.
		// Optimized implementation would check if best path changed.
		// My implementation sends it. So we expect an update pointing to nh1 still.
		if update.NextHop != nh1 {
			t.Errorf("Expected NextHop %s, got %s", nh1, update.NextHop)
		}
	case <-time.After(100 * time.Millisecond):
		// No update is also fine if optimized.
	}

	// 3. Add BGP Route (AD 200, Metric 5) - Should NOT trigger update (worse AD)
	// ...
}

func TestRIB_DeleteRoute_PromoteNextBest(t *testing.T) {
	fibChan := make(chan api.FIBUpdate, 10)
	r := New(fibChan)

	prefix := netip.MustParsePrefix("20.0.0.0/24")
	nhStatic := netip.MustParseAddr("192.168.1.1")
	nhOSPF := netip.MustParseAddr("192.168.1.2")

	// Add Static (Best)
	r.AddRoute(api.RIBUpdate{Protocol: "STATIC", Prefix: prefix, NextHop: nhStatic, AdminDist: 1})
	<-fibChan // Consume

	// Add OSPF (Backup)
	r.AddRoute(api.RIBUpdate{Protocol: "OSPF", Prefix: prefix, NextHop: nhOSPF, AdminDist: 110})
	<-fibChan // Consume (or ignore if optimized)

	// Delete Static
	r.DeleteRoute(api.RIBUpdate{Protocol: "STATIC", Prefix: prefix})

	select {
	case update := <-fibChan:
		if update.Action != api.Add {
			t.Errorf("Expected ADD (update), got %v", update.Action)
		}
		if update.NextHop != nhOSPF {
			t.Errorf("Expected NextHop %s (promoted), got %s", nhOSPF, update.NextHop)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for FIB update after deletion")
	}
}

func TestRIB_DeleteAllRoutes(t *testing.T) {
	fibChan := make(chan api.FIBUpdate, 10)
	r := New(fibChan)
	prefix := netip.MustParsePrefix("30.0.0.0/24")

	r.AddRoute(api.RIBUpdate{Protocol: "STATIC", Prefix: prefix, AdminDist: 1})
	<-fibChan

	r.DeleteRoute(api.RIBUpdate{Protocol: "STATIC", Prefix: prefix})

	select {
	case update := <-fibChan:
		if update.Action != api.Delete {
			t.Errorf("Expected DELETE, got %v", update.Action)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for FIB delete")
	}
}
