package main

import (
	"context"
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
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Create context that cancels on SIGINT or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize Channels
	ribChan := make(chan api.RIBUpdate, 100)
	fibChan := make(chan api.FIBUpdate, 100)
	telemetryChan := make(chan api.AFTUpdate, 100)

	// Initialize Components
	r := rib.New(fibChan)
	f := fib.New(telemetryChan)
	ts := telemetry.New(f, telemetryChan)
	m := mock.New()

	g, ctx := errgroup.WithContext(ctx)

	// 1. RIB
	g.Go(func() error {
		defer close(fibChan) // Ideally rib.Start handles this, but let's be explicit or let rib.Start do it?
		// rib.Start closes fibChan on return.
		return r.Start(ctx, ribChan)
	})

	// 2. FIB
	g.Go(func() error {
		// fib.Start closes telemetryChan on return.
		return f.Start(ctx, fibChan)
	})

	// 3. Telemetry Server Logic
	g.Go(func() error {
		return ts.Run(ctx)
	})

	// 4. gRPC Server
	lis, err := net.Listen("tcp", ":50099")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGNMIServer(s, ts)
	reflection.Register(s)

	g.Go(func() error {
		log.Printf("server listening at %v", lis.Addr())
		// Serve blocks until error or Stop.
		// To handle graceful shutdown with context:
		errChan := make(chan error, 1)
		go func() {
			errChan <- s.Serve(lis)
		}()

		select {
		case <-ctx.Done():
			s.GracefulStop()
			// Wait for Serve to return
			return <-errChan
		case err := <-errChan:
			return err
		}
	})

	// 5. Mock Installer
	g.Go(func() error {
		defer close(ribChan) // Close ribChan when installer is done to signal RIB to finish
		return m.Run(ctx, ribChan)
	})

	fmt.Println("Daemon running. Press Ctrl+C to stop.")
	if err := g.Wait(); err != nil {
		// Context cancellation is not an error for us, usually.
		if err != context.Canceled {
			log.Printf("Daemon error: %v", err)
		}
	}
	fmt.Println("Daemon stopped.")
}
