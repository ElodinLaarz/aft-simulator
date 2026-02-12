package telemetry

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/openconfig/aft-simulator/pkg/api"
	"github.com/openconfig/aft-simulator/pkg/fib"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// GNMIServer implements the gNMI service.
type GNMIServer struct {
	gnmipb.UnimplementedGNMIServer

	fib           *fib.FIB
	telemetryChan <-chan api.AFTUpdate

	subMu        sync.RWMutex
	subscribers  map[int64]chan api.AFTUpdate
	subIDCounter int64
}

// New creates a new GNMIServer.
func New(f *fib.FIB, telemetryChan <-chan api.AFTUpdate) *GNMIServer {
	s := &GNMIServer{
		fib:           f,
		telemetryChan: telemetryChan,
		subscribers:   make(map[int64]chan api.AFTUpdate),
	}
	go s.broadcastLoop()
	return s
}

func (s *GNMIServer) sendToSubscribers(update api.AFTUpdate) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for id, subChan := range s.subscribers {
		select {
		case subChan <- update:
		default:
			go log.Printf("GNMIServer: subscriber channel full unable to send to subscriber %d", id)
		}
	}
}

func (s *GNMIServer) broadcastLoop() {
	for update := range s.telemetryChan {
		s.sendToSubscribers(update)
	}
}

// Subscribe implements the gNMI Subscribe RPC.
func (s *GNMIServer) Subscribe(stream gnmipb.GNMI_SubscribeServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	if req.GetSubscribe().GetMode() != gnmipb.SubscriptionList_STREAM {
		return status.Errorf(codes.Unimplemented, "Only STREAM mode is supported")
	}

	// Register subscriber
	subChan := make(chan api.AFTUpdate, 100)
	s.subMu.Lock()
	s.subIDCounter++
	id := s.subIDCounter
	s.subscribers[id] = subChan
	s.subMu.Unlock()

	defer func() {
		s.subMu.Lock()
		delete(s.subscribers, id)
		close(subChan)
		s.subMu.Unlock()
	}()

	// Send initial snapshot
	snapshot := s.fib.GetSnapshot()
	for _, update := range snapshot {
		notif, err := aftToNotification(update)
		if err != nil {
			continue
		}
		if err := stream.Send(&gnmipb.SubscribeResponse{
			Response: &gnmipb.SubscribeResponse_Update{Update: notif},
		}); err != nil {
			return err
		}
	}

	// Send SyncResponse
	if err := stream.Send(&gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_SyncResponse{SyncResponse: true},
	}); err != nil {
		return err
	}

	// Stream updates
	for {
		select {
		case update := <-subChan:
			notif, err := aftToNotification(update)
			if err != nil {
				continue
			}
			if err := stream.Send(&gnmipb.SubscribeResponse{
				Response: &gnmipb.SubscribeResponse_Update{Update: notif},
			}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func aftToNotification(update api.AFTUpdate) (*gnmipb.Notification, error) {
	ts := time.Now().UnixNano()

	var path *gnmipb.Path
	var val *gnmipb.TypedValue

	switch update.EntryType {
	case api.AFTEntryPrefix:
		prefixStr := update.Prefix.String()
		path = &gnmipb.Path{
			Elem: []*gnmipb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": api.NetworkInstanceDefault}},
				{Name: "afts"},
				{Name: "ipv4-unicast"},
				{Name: "ipv4-entry", Key: map[string]string{"prefix": prefixStr}},
				{Name: "state"},
				{Name: "next-hop-group"},
			},
		}
		val = &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_UintVal{UintVal: update.NextHopGroup},
		}

	case api.AFTEntryNextHopGroup:
		path = &gnmipb.Path{
			Elem: []*gnmipb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": api.NetworkInstanceDefault}},
				{Name: "afts"},
				{Name: "next-hop-groups"},
				{Name: "next-hop-group", Key: map[string]string{"id": fmt.Sprintf("%d", update.NextHopGroup)}},
				{Name: "next-hops"},
				{Name: "next-hop", Key: map[string]string{"index": fmt.Sprintf("%d", update.NextHopGroup)}}, // Assuming index matches NHG ID for simplicity, or use IP string. Let's use IP string as index.
				// Wait, the NextHop index should be the IP address string to match the NextHop entry.
				// Let's use the NextHop IP string as the index in the NHG.
			},
		}
		// Correcting the path for NHG -> NH reference
		path.Elem[len(path.Elem)-1].Key["index"] = update.NextHop.String()
		
		// The value for a next-hop within a next-hop-group is typically its weight.
		// For simplicity, we can just set weight to 1.
		// Actually, the path should be to the `weight` leaf if we are setting a value,
		// or we can just send an empty update to the list element to indicate it exists.
		// Let's set the weight leaf.
		path.Elem = append(path.Elem, &gnmipb.PathElem{Name: "state"}, &gnmipb.PathElem{Name: "weight"})
		val = &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_UintVal{UintVal: 1},
		}

	case api.AFTEntryNextHop:
		path = &gnmipb.Path{
			Elem: []*gnmipb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": api.NetworkInstanceDefault}},
				{Name: "afts"},
				{Name: "next-hops"},
				{Name: "next-hop", Key: map[string]string{"index": update.NextHop.String()}},
				{Name: "state"},
				{Name: "ip-address"},
			},
		}
		val = &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_StringVal{StringVal: update.NextHop.String()},
		}

	default:
		return nil, fmt.Errorf("unknown AFT entry type: %v", update.EntryType)
	}

	if update.Action == api.Delete {
		// For deletes, we typically delete the list element itself, not just the leaf.
		// So we need to trim the path back to the list element.
		switch update.EntryType {
		case api.AFTEntryPrefix:
			path.Elem = path.Elem[:len(path.Elem)-2] // Remove state/next-hop-group
		case api.AFTEntryNextHopGroup:
			path.Elem = path.Elem[:len(path.Elem)-5] // Remove next-hops/next-hop/state/weight
		case api.AFTEntryNextHop:
			path.Elem = path.Elem[:len(path.Elem)-2] // Remove state/ip-address
		}

		return &gnmipb.Notification{
			Timestamp: ts,
			Delete:    []*gnmipb.Path{path},
		}, nil
	}

	return &gnmipb.Notification{
		Timestamp: ts,
		Update: []*gnmipb.Update{
			{
				Path: path,
				Val:  val,
			},
		},
	}, nil
}
