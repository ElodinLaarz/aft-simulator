# aft-simulator

A modular routing daemon simulator in Go.

## Overview

The `aft-simulator` demonstrates a decoupled routing system architecture:
1.  **Installers** (e.g., `mock`) inject routes into a **RIB** (Routing Information Base).
2.  The **RIB** selects the best path (Admin Distance < Metric) and updates the **FIB** (Forwarding Information Base).
3.  The **FIB** maintains the active forwarding state and streams updates to a **gNMI Telemetry Server**.

## Directory Structure

*   `cmd/daemon`: Main entry point.
*   `pkg/api`: Core data structures and interfaces.
*   `pkg/rib`: RIB implementation (Best Path Selection).
*   `pkg/fib`: FIB implementation (Active State).
*   `pkg/telemetry`: gNMI Server implementation.
*   `pkg/installers`: Route injectors (currently `mock`).
*   `pkg/config`: Configuration loading logic.

## Configuration

The simulator can be configured via a JSON file. By default, it looks for `config.json` in the current directory, or you can specify one with the `-config` flag.

**Example `config.json`:**
```json
{
  "gnmi_port": 50099,
  "mock_installer": {
    "enabled": true,
    "route_count": 1000,
    "churn_rate": 100
  }
}
```

## Running

```bash
# Run with default config (config.json)
go run ./cmd/daemon

# Run with custom config
go run ./cmd/daemon -config my_config.json
```

## gNMI Telemetry

The daemon listens on the configured port (default `50099`) (TCP).
You can subscribe to AFT updates using a gNMI client (e.g., `gnmic`).

```bash
gnmic -a localhost:50099 --insecure subscribe --path /afts/ipv4-unicast/ipv4-entry/state/next-hop-group
```
