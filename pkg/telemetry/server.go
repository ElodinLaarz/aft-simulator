package telemetry

import (
	"sync"
	"time"

	"github.com/openconfig/aft-simulator/pkg/api"
	"github.com/openconfig/aft-simulator/pkg/fib"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GNMIServer implements the gNMI service.
type GNMIServer struct {
	pb.UnimplementedGNMIServer

	fib           *fib.FIB
	telemetryChan <-chan api.AFTUpdate

	mu          sync.RWMutex
	subscribers map[int64]chan api.AFTUpdate
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

func (s *GNMIServer) broadcastLoop() {
	for update := range s.telemetryChan {
		s.mu.RLock()
		for _, subChan := range s.subscribers {
			// Non-blocking send to avoid slow consumers blocking everyone
			select {
			case subChan <- update:
			default:
				// Drop update if consumer is slow
			}
		}
		s.mu.RUnlock()
	}
}

// Subscribe implements the gNMI Subscribe RPC.
func (s *GNMIServer) Subscribe(stream pb.GNMI_SubscribeServer) error {
	req, err := stream.Recv()
	if err != nil {
		return err
	}

	if req.GetSubscribe().GetMode() != pb.SubscriptionList_STREAM {
		return status.Errorf(codes.Unimplemented, "Only STREAM mode is supported")
	}

	// Register subscriber
	subChan := make(chan api.AFTUpdate, 100)
	s.mu.Lock()
	s.subIDCounter++
	id := s.subIDCounter
	s.subscribers[id] = subChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.subscribers, id)
		close(subChan)
		s.mu.Unlock()
	}()

	// Send initial snapshot
	snapshot := s.fib.GetSnapshot()
	for _, update := range snapshot {
		notif, err := aftToNotification(update)
		if err != nil {
			continue
		}
		if err := stream.Send(&pb.SubscribeResponse{
			Response: &pb.SubscribeResponse_Update{Update: notif},
		}); err != nil {
			return err
		}
	}

	// Send SyncResponse
	if err := stream.Send(&pb.SubscribeResponse{
		Response: &pb.SubscribeResponse_SyncResponse{SyncResponse: true},
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
			if err := stream.Send(&pb.SubscribeResponse{
				Response: &pb.SubscribeResponse_Update{Update: notif},
			}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func aftToNotification(update api.AFTUpdate) (*pb.Notification, error) {
	ts := time.Now().UnixNano()
	prefixStr := update.Prefix.String()

	// Path: /network-instances/network-instance[name=default]/afts/ipv4-unicast/ipv4-entry[prefix=...]/state/next-hop-group
	// Simplified for now: /afts/ipv4-unicast/ipv4-entry[prefix=...]/state/next-hop-group
	// Actually, let's use a cleaner path structure.
	// /afts/ipv4-unicast/ipv4-entry[prefix=...]/state/next-hop-group
	
	path := &pb.Path{
		Elem: []*pb.PathElem{
			{Name: "afts"},
			{Name: "ipv4-unicast"},
			{Name: "ipv4-entry", Key: map[string]string{"prefix": prefixStr}},
			{Name: "state"},
			{Name: "next-hop-group"}, // Simplified: Just value, no complex group resolution
		},
	}

	if update.Action == api.Delete {
		return &pb.Notification{
			Timestamp: ts,
			Delete:    []*pb.Path{path},
		}, nil
	}

	val := &pb.TypedValue{
		Value: &pb.TypedValue_StringVal{StringVal: update.NextHop.String()},
	}

	return &pb.Notification{
		Timestamp: ts,
		Update: []*pb.Update{
			{
				Path: path,
				Val:  val,
			},
		},
	}, nil
}
