package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/openconfig/aft-simulator/pkg/api"
	"github.com/openconfig/aft-simulator/pkg/config"
	"github.com/openconfig/aft-simulator/pkg/fib"
	"github.com/openconfig/aft-simulator/pkg/installers/mock"
	"github.com/openconfig/aft-simulator/pkg/rib"
	"github.com/openconfig/aft-simulator/pkg/telemetry"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	configFile = flag.String("config", "config.json", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Load Configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Printf("Failed to load config from %s: %v. Using defaults.", *configFile, err)
		cfg = config.DefaultConfig()
	}

	// Create context that cancels on SIGINT or SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize Channels
	// Increased buffer size to handle high churn rates
	ribChan := make(chan api.RIBUpdate, 10000)
	fibChan := make(chan api.FIBUpdate, 10000)
	telemetryChan := make(chan api.AFTUpdate, 10000)

	// Initialize Components
	r := rib.New(fibChan)
	f := fib.New(telemetryChan)
	ts := telemetry.New(f, telemetryChan)
	m := mock.New(cfg.Mock)

	g, ctx := errgroup.WithContext(ctx)

	// 1. RIB
	g.Go(func() error {
		return r.Start(ctx, ribChan)
	})

	// 2. FIB
	g.Go(func() error {
		return f.Start(ctx, fibChan)
	})

	// 3. Telemetry Server Logic
	g.Go(func() error {
		return ts.Run(ctx)
	})

	// 4. gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GNMIPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterGNMIServer(s, ts)
	reflection.Register(s)

	g.Go(func() error {
		log.Printf("server listening at %v", lis.Addr())
		errChan := make(chan error, 1)
		go func() {
			errChan <- s.Serve(lis)
		}()

		select {
		case <-ctx.Done():
			s.GracefulStop()
			return <-errChan
		case err := <-errChan:
			return err
		}
	})

	// 5. Mock Installer
	g.Go(func() error {
		defer close(ribChan)
		return m.Run(ctx, ribChan)
	})

	fmt.Println("Daemon running. Press Ctrl+C to stop.")
	if err := g.Wait(); err != nil {
		if err != context.Canceled {
			log.Printf("Daemon error: %v", err)
		}
	}
	fmt.Println("Daemon stopped.")
}
