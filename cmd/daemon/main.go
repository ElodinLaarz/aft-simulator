package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/openconfig/aft-simulator/pkg/api"
	"github.com/openconfig/aft-simulator/pkg/fib"
	"github.com/openconfig/aft-simulator/pkg/installers/mock"
	"github.com/openconfig/aft-simulator/pkg/rib"
	"github.com/openconfig/aft-simulator/pkg/telemetry"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Initialize Channels
	ribChan := make(chan api.RIBUpdate, 100)
	fibChan := make(chan api.FIBUpdate, 100)
	telemetryChan := make(chan api.AFTUpdate, 100)

	// Initialize Components
	// 1. RIB
	r := rib.New(fibChan)
	go r.Start(ribChan)

	// 2. FIB
	f := fib.New(telemetryChan)
	go f.Start(fibChan)

	// 3. Telemetry Server
	ts := telemetry.New(f, telemetryChan)

	// Start gRPC Server
	lis, err := net.Listen("tcp", ":50099")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGNMIServer(s, ts)
	reflection.Register(s)

	go func() {
		log.Printf("server listening at %v", lis.Addr())
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// 4. Mock Installer
	m := mock.New()
	if err := m.Start(ribChan); err != nil {
		log.Fatalf("failed to start mock installer: %v", err)
	}

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down...")
	s.GracefulStop()
	m.Stop()
}
