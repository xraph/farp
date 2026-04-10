# FARP Service Discovery Guide

The FARP discovery system provides pluggable service discovery that **automatically manages the full FARP schema communication lifecycle**. Services auto-register, serve endpoints, and maintain health. Gateways auto-discover services, fetch manifests, and keep routes in sync — zero manual wiring.

## Discovery Modes

FARP supports three discovery modes. Choose based on your infrastructure:

### Mode 1: Registry-Based (Pull)

Traditional service discovery via an external registry. Best for production deployments with existing infrastructure.

```
Service → registers in Consul/etcd/K8s/Redis/mDNS
Gateway → watches registry → fetches manifests → mounts routes
```

**Supported backends**: Consul, etcd, Kubernetes, Redis, mDNS/Bonjour

### Mode 2: Push-Based (Reverse Discovery)

No external registry needed. Services discover the gateway address (via config, DNS, or env var) and push their manifest directly. Best for development, small deployments, and edge/IoT.

```
Service → POSTs manifest to gateway HTTP endpoint
Gateway → receives manifest → mounts routes
```

### Mode 3: Hybrid

Uses a registry for discovery but services also push updates directly for faster propagation.

---

## Quick Start

### Service Side

```go
import (
    "github.com/xraph/farp/discovery"
    "github.com/xraph/farp/discovery/consul"
)

// Create a discovery backend
disc, _ := consul.New(consul.Config{Address: "consul:8500"})

// Create a service node
node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
    ServiceName:    "user-service",
    ServiceVersion: "v1.2.0",
    Address:        "10.0.0.5:8080",
    Discovery:      disc,          // register in Consul
    MountStrategy:  farp.MountStrategyService,
})

// Start: registers, generates schemas, starts health loop
node.Start(ctx)
defer node.Stop(ctx)

// Mount FARP HTTP endpoints on your router
http.Handle("/_farp/", node.HTTPHandler())
```

### Gateway Side

```go
import (
    "github.com/xraph/farp/discovery"
    "github.com/xraph/farp/discovery/consul"
)

disc, _ := consul.New(consul.Config{Address: "consul:8500"})

gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    Discovery: disc,
    OnRoutesChanged: func(routes []gateway.ServiceRoute) {
        // Update your proxy/router with new routes
        for _, route := range routes {
            proxy.AddRoute(route.Path, route.Methods, route.TargetURL)
        }
    },
})

// Start: watches discovery, fetches manifests, computes routes
gw.Start(ctx)
defer gw.Stop(ctx)
```

### Push Mode (Zero Infrastructure)

```go
// === Service side ===
node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
    ServiceName: "user-service",
    Address:     "10.0.0.5:8080",
    GatewayURL:  "http://gateway:9090/_farp/v1",  // push directly
})
node.Start(ctx)
http.Handle("/_farp/", node.HTTPHandler())

// === Gateway side ===
gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    EnablePush:      true,
    OnRoutesChanged: updateRoutes,
})
gw.Start(ctx)
http.Handle("/_farp/v1/", gw.PushHandler())
```

---

## Core Components

### ServiceDiscovery Interface

All backends implement this interface:

```go
type ServiceDiscovery interface {
    Discover(ctx context.Context, serviceName string) ([]ServiceInstance, error)
    Watch(ctx context.Context, serviceName string, handler DiscoveryEventHandler) error
    Register(ctx context.Context, instance ServiceInstance) error
    Deregister(ctx context.Context, instanceID string) error
    ReportHealth(ctx context.Context, instanceID string, status InstanceStatus) error
    Close() error
    Health(ctx context.Context) error
}
```

### ServiceNode (Service Side)

`ServiceNode` manages the full FARP lifecycle for a service:

1. Generates schemas from providers (OpenAPI, gRPC, GraphQL, etc.)
2. Builds the `SchemaManifest` with checksums
3. Serves FARP HTTP endpoints (`/_farp/manifest`, `/_farp/health`, `/_farp/schemas/{type}`)
4. Registers in the discovery backend or pushes to gateway
5. Publishes manifest to `SchemaRegistry` (if KV-based backend)
6. Runs background health reporting / TTL renewal
7. Re-registers on failure with exponential backoff
8. Gracefully deregisters on shutdown

**Key methods:**

| Method | Description |
|--------|-------------|
| `Start(ctx)` | Register, generate schemas, start health loop |
| `Stop(ctx)` | Graceful deregister and cleanup |
| `HTTPHandler()` | Returns `http.Handler` for FARP endpoints |
| `Manifest()` | Returns current `SchemaManifest` |
| `UpdateSchema(ctx)` | Re-generate and push updated schemas |

### GatewayNode (Gateway Side)

`GatewayNode` manages the full FARP lifecycle for a gateway:

1. Watches discovery backend for service instances (and/or accepts push registrations)
2. Fetches manifests from discovered services (HTTP or from registry)
3. Registers manifests in its local `SchemaRegistry`
4. Converts schemas to routes via the gateway client
5. Detects route changes via `RoutesChecksum` (prevents unnecessary remounts)
6. Calls route update handler (supports atomic swap for zero-downtime)
7. Removes routes when services disappear
8. Tracks health and evicts unhealthy instances

**Key methods:**

| Method | Description |
|--------|-------------|
| `Start(ctx)` | Begin watching and auto-managing routes |
| `Stop(ctx)` | Stop watching and cleanup |
| `PushHandler()` | Returns `http.Handler` for push registration endpoints |
| `Services()` | Returns all discovered services + manifests |
| `Routes()` | Returns current computed route table |
| `Registry()` | Returns underlying `SchemaRegistry` |
| `GatewayClient()` | Returns underlying `gateway.Client` |

### FARPHandler (HTTP Endpoints)

Serves the FARP protocol HTTP endpoints. Mount on your service's router:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/_farp/manifest` | GET | Returns `SchemaManifest` JSON |
| `/_farp/health` | GET | Returns health status (200 or 503) |
| `/_farp/schemas/{type}` | GET | Returns schema by type (openapi, asyncapi, graphql, etc.) |

```go
handler := discovery.NewFARPHandler(manifest, schemas)
http.Handle("/_farp/", handler)
```

---

## Push Protocol

When using push-based discovery, the gateway exposes these HTTP endpoints:

| Endpoint | Method | Body | Description |
|----------|--------|------|-------------|
| `/_farp/v1/register` | POST | `{instance, manifest}` | Service pushes its registration |
| `/_farp/v1/heartbeat/{id}` | PUT | `{status}` | Service sends heartbeat |
| `/_farp/v1/deregister/{id}` | DELETE | — | Service deregisters |
| `/_farp/v1/services` | GET | — | List all registered services |
| `/_farp/v1/services/{name}` | GET | — | Get instances of a service |

### Heartbeat and Eviction

- Services send heartbeats at `HealthInterval` (default: 10s)
- Gateway evicts instances after `HeartbeatTimeout` (default: 30s) without heartbeat
- Evicted instances trigger a `Removed` event and routes are updated

---

## Discovery Backends

### Consul

```go
import "github.com/xraph/farp/discovery/consul"

disc, _ := consul.New(consul.Config{
    Address:    "consul:8500",
    Datacenter: "dc1",
    Token:      "my-acl-token",
    Namespace:  "farp",
})
```

**How it works:**
- `Register` → `Agent().ServiceRegister()` with TTL health check and `farp` tag
- `Watch` → Consul blocking queries with `WaitIndex` for real-time changes
- `ReportHealth` → `Agent().UpdateTTL()` (pass/warn/fail)
- Also implements `StorageBackend` via Consul KV for schema storage

### etcd

```go
import "github.com/xraph/farp/discovery/etcd"

disc, _ := etcd.New(etcd.Config{
    Endpoints:   []string{"etcd:2379"},
    DialTimeout: 5 * time.Second,
    Namespace:   "farp",
})
```

**How it works:**
- `Register` → `Put` with lease (TTL-based auto-expiry)
- `Watch` → etcd Watch API (gRPC streaming for real-time events)
- `ReportHealth` → Lease `KeepAlive`
- Also implements `StorageBackend` via etcd KV

### Redis

```go
import "github.com/xraph/farp/discovery/redis"

disc, _ := redis.New(redis.Config{
    Address:   "redis:6379",
    Password:  "",
    DB:        0,
    Namespace: "farp",
})
```

**How it works:**
- `Register` → `HSET` + `EXPIRE` for TTL
- `Watch` → Pub/Sub on `farp:events:{service}` + keyspace notifications
- `ReportHealth` → Refresh `EXPIRE` + update status field
- Also implements `StorageBackend` via Redis GET/SET/DEL

### Kubernetes

```go
import "github.com/xraph/farp/discovery/kubernetes"

disc, _ := kubernetes.New(kubernetes.Config{
    // Uses in-cluster config by default
    Namespace: "default",
})
```

**How it works:**
- `Discover` → List EndpointSlices with label `farp.io/enabled=true`
- `Watch` → Kubernetes Watch API (HTTP streaming)
- `Register` → Annotate pod with `farp.io/manifest-url`
- Health managed natively by Kubernetes readiness probes

**Labels and annotations:**
- Label: `farp.io/enabled=true` — marks a service for FARP discovery
- Annotation: `farp.io/manifest-url` — URL to fetch the manifest
- Annotation: `farp.io/instance-id` — FARP instance ID

### mDNS / Bonjour

```go
import "github.com/xraph/farp/discovery/mdns"

disc, _ := mdns.New(mdns.Config{
    ServiceType: "_farp._tcp",
    Domain:      "local.",
})
```

**How it works:**
- `Discover` → mDNS browse for `_farp._tcp` service type
- `Watch` → Continuous browse with periodic polling, diff-based change detection
- `Register` → Register mDNS service record with TXT records
- TXT records: `farp.manifest=http://host:port/_farp/manifest`, `farp.version=1.1.0`

**Best for:** Local development, LAN-based services, IoT/edge deployments

---

## Configuration Reference

### ServiceNodeConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ServiceName` | string | **required** | Logical service name |
| `ServiceVersion` | string | — | Service version (semver) |
| `InstanceID` | string | auto-generated | Unique instance identifier |
| `Address` | string | **required** | host:port this service listens on |
| `Discovery` | ServiceDiscovery | — | Registry backend (mode 1) |
| `GatewayURL` | string | — | Gateway push endpoint (mode 2) |
| `Registry` | SchemaRegistry | — | KV store for manifest publishing |
| `Providers` | []SchemaProvider | — | Schema generators |
| `HealthInterval` | Duration | 10s | Health report frequency |
| `TTL` | Duration | 30s | Service TTL in registry |
| `MaxRetries` | int | 5 | Re-registration retry count |
| `RetryBackoff` | Duration | 2s | Backoff between retries |
| `MountStrategy` | MountStrategy | instance | Route mounting strategy |
| `BasePath` | string | — | Custom base path |
| `Hints` | *ServiceHints | — | Rate limits, CORS, etc. |

### GatewayNodeConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Discovery` | ServiceDiscovery | — | Registry backend (mode 1) |
| `EnablePush` | bool | false | Accept push registrations (mode 2) |
| `Registry` | SchemaRegistry | in-memory | Manifest storage |
| `OnRoutesChanged` | func | — | Route change callback |
| `RouteHandler` | RouteUpdateHandler | — | Atomic swap handler |
| `ServiceNames` | []string | all | Services to watch |
| `HTTPClient` | *http.Client | default | For manifest fetching |
| `Fetcher` | ManifestFetcher | HTTP fetcher | Custom manifest fetcher |
| `HealthPollInterval` | Duration | 15s | Health poll frequency |
| `HeartbeatTimeout` | Duration | 30s | Push mode eviction timeout |

---

## Go Module Structure

Each backend is a separate Go module to avoid transitive dependency bloat:

```
discovery/              → github.com/xraph/farp/discovery     (core, zero external deps)
discovery/consul/       → github.com/xraph/farp/discovery/consul
discovery/etcd/         → github.com/xraph/farp/discovery/etcd
discovery/kubernetes/   → github.com/xraph/farp/discovery/kubernetes
discovery/redis/        → github.com/xraph/farp/discovery/redis
discovery/mdns/         → github.com/xraph/farp/discovery/mdns
```

Import only the backend you need:

```go
import "github.com/xraph/farp/discovery/consul"  // only pulls consul deps
```

## Rust Feature Flags

```toml
[features]
discovery-consul = ["reqwest"]
discovery-etcd = ["etcd-client"]
discovery-kubernetes = ["kube", "k8s-openapi"]
discovery-redis = ["redis"]
discovery-mdns = ["mdns"]
discovery-all = ["discovery-consul", "discovery-etcd", "discovery-kubernetes", "discovery-redis", "discovery-mdns"]
```

---

## Choosing a Backend

| Backend | Best For | Watch Latency | Infra Required | KV Storage |
|---------|----------|---------------|----------------|------------|
| **Push** | Dev, small deploys, edge | Instant | None | No |
| **Consul** | Production, multi-DC | ~100ms | Consul cluster | Yes |
| **etcd** | K8s-adjacent, strong consistency | ~50ms | etcd cluster | Yes |
| **Redis** | Simple prod, caching layer | ~100ms | Redis server | Yes |
| **Kubernetes** | K8s-native workloads | ~1s | K8s cluster | ConfigMap |
| **mDNS** | Local dev, LAN, IoT | ~2s | None | No |

---

## Advanced Patterns

### Atomic Route Swap (Zero-Downtime)

Use `RouteUpdateHandler` for zero-downtime route updates:

```go
gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    Discovery:    disc,
    RouteHandler: &myAtomicHandler{},
})
```

The handler implements prepare/commit/rollback:

```go
type myAtomicHandler struct {
    pending []farp.RouteDescriptor
    current []farp.RouteDescriptor
}

func (h *myAtomicHandler) PrepareRoutes(routes []farp.RouteDescriptor) error {
    // Validate routes (check for conflicts, missing backends, etc.)
    h.pending = routes
    return nil
}

func (h *myAtomicHandler) CommitRoutes() error {
    // Atomic swap: old routes keep serving until new ones are ready
    h.current = h.pending
    h.pending = nil
    return nil
}

func (h *myAtomicHandler) RollbackRoutes() error {
    h.pending = nil
    return nil
}
```

### Schema Updates at Runtime

When a service's API changes, trigger a schema update:

```go
// Service adds a new endpoint
router.POST("/orders", handleCreateOrder)

// Notify FARP to regenerate schemas and push updates
node.UpdateSchema(ctx)
```

### Hybrid Mode

Use both registry and push for maximum reliability:

```go
gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    Discovery:  consulDisc,  // Watch Consul for services
    EnablePush: true,        // Also accept direct pushes
    OnRoutesChanged: updateRoutes,
})
```

### Custom ManifestFetcher

Override how manifests are fetched from services:

```go
type myFetcher struct{}

func (f *myFetcher) FetchManifest(ctx context.Context, inst discovery.ServiceInstance) (*farp.SchemaManifest, error) {
    // Custom logic: fetch from S3, database, etc.
    url := inst.Metadata["manifest_url"]
    // ...
}

gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    Discovery: disc,
    Fetcher:   &myFetcher{},
})
```
