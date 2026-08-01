# FARP - Forge API Gateway Registration Protocol

[![CI](https://github.com/xraph/farp/workflows/CI/badge.svg)](https://github.com/xraph/farp/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/xraph/farp)](https://goreportcard.com/report/github.com/xraph/farp)
[![GoDoc](https://godoc.org/github.com/xraph/farp?status.svg)](https://godoc.org/github.com/xraph/farp)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/xraph/farp)](https://github.com/xraph/farp/releases)

FARP™ is a service manifest and schema-discovery protocol maintained by XRAPH™.

**Authors**: [Rex Raphael](https://github.com/juicycleff)

**Last Updated**: 2025-11-01

## Overview

FARP (Forge API Gateway Registration Protocol) is a **protocol specification library** that enables service instances to communicate their API schemas, health information, and capabilities to API gateways and service meshes. It provides standardized data structures, schema generation tooling, and merging utilities—but **does not implement HTTP servers, gateway logic, or service discovery**.

### What FARP Is

- **Protocol Specification** - Defines the SchemaManifest format and data structures
- **Schema Generation Library** - Providers to generate OpenAPI, AsyncAPI, gRPC, GraphQL, oRPC, Thrift, and Avro schemas from code
- **Schema Merging Utilities** - Tools to compose multiple service schemas into unified API documentation
- **Service Discovery** - Pluggable discovery with Consul, etcd, Kubernetes, Redis, mDNS, and push-based backends
- **Auto-Registration** - `ServiceNode` and `GatewayNode` manage the full FARP lifecycle automatically
- **Validation & Serialization** - Ensure manifests are spec-compliant

### What FARP Is NOT

- ❌ **Not an API Gateway** - No routing, rate limiting, or traffic management (but provides hints for them)
- ❌ **Not a Service Framework** - Services mount FARP HTTP handlers on their own router

## Key Features

- **Pluggable Service Discovery**: Consul, etcd, Kubernetes, Redis, mDNS, and push-based — or bring your own
- **Auto-Lifecycle Management**: `ServiceNode` and `GatewayNode` handle registration, health, schemas, and routes automatically
- **Push-Based Discovery**: Services push manifests directly to gateways — zero infrastructure needed
- **Schema-Aware Service Discovery**: Services register with complete API contracts
- **Multi-Protocol Support**: OpenAPI, AsyncAPI, gRPC, GraphQL, oRPC, Thrift, Avro, and extensible for future protocols
- **Zero-Downtime Route Updates**: Routes checksum + atomic swap prevents intermittent 404s
- **Operational Hints**: Rate limiting, circuit breaker, CORS, caching, load balancing, observability — all declarative
- **FARP HTTP Handler**: Ready-to-mount handler serves `/_farp/manifest`, health, and schema endpoints
- **Dual Language**: Full Go and Rust implementations with consistent APIs
- **Backend Agnostic Storage**: Works with Consul KV, etcd, Redis, or in-memory for schema storage
- **Zero Configuration**: Works with mDNS/Bonjour or push mode for local development without infrastructure

## Architectural Boundaries

FARP provides the protocol, schema tooling, and service discovery — you provide the HTTP router and gateway proxy.

### FARP Library Provides

| Component | Description | Package |
|-----------|-------------|---------|
| **Type Definitions** | `SchemaManifest`, `SchemaDescriptor`, routing/auth/hints | `types.go` |
| **Schema Providers** | Generate schemas from code | `providers/*` |
| **Service Discovery** | Pluggable backends + auto-lifecycle | `discovery/*` |
| **FARP HTTP Handler** | Serves manifest, health, and schema endpoints | `discovery/handler.go` |
| **Gateway Client** | Schema-to-route conversion with atomic swap | `gateway/*` |
| **Registry + Storage** | Manifest storage and caching | `registry.go`, `storage.go` |
| **Merging Logic** | Compose multiple schemas into unified docs | `merger/*` |
| **Validation** | Ensure manifests are spec-compliant | `manifest.go` |

### What You Implement

| Concern | What You Do |
|---------|-------------|
| **HTTP Router** | Mount `node.HTTPHandler()` on your router (e.g., chi, gin, echo) |
| **Gateway Proxy** | Apply routes from `OnRoutesChanged` to your proxy (e.g., reverse proxy, Envoy) |
| **Schema Providers** | (Optional) Custom providers for your framework's routes |
| **Webhooks** | (Optional) Implement webhook receivers for gateway events |

## Repository Structure

```text
farp/
├── README.md                    # This file - project overview
├── docs/
│   ├── SPECIFICATION.md         # Complete protocol specification
│   ├── ARCHITECTURE.md          # Architecture and design decisions
│   ├── DISCOVERY.md             # Service discovery guide
│   └── IMPLEMENTATION_RESPONSIBILITIES.md
├── discovery/                   # Service discovery system
│   ├── discovery.go             # ServiceDiscovery interface, types
│   ├── node.go                  # ServiceNode + GatewayNode (auto-lifecycle)
│   ├── handler.go               # FARP HTTP handler + ManifestFetcher
│   ├── push.go                  # Push-based discovery (service→gateway)
│   ├── push_handler.go          # Push handler (gateway-side receiver)
│   ├── consul/                  # Consul backend
│   ├── etcd/                    # etcd backend
│   ├── kubernetes/              # Kubernetes backend
│   ├── redis/                   # Redis backend
│   └── mdns/                    # mDNS/Bonjour backend
├── providers/                   # Schema provider implementations
│   ├── openapi/                 # OpenAPI 3.x provider
│   ├── asyncapi/                # AsyncAPI 2.x/3.x provider
│   ├── grpc/                    # gRPC FileDescriptorSet provider
│   ├── graphql/                 # GraphQL SDL/introspection provider
│   ├── orpc/                    # oRPC (OpenAPI-based RPC) provider
│   ├── thrift/                  # Apache Thrift IDL provider
│   └── avro/                    # Apache Avro schema provider
├── gateway/                     # Reference gateway client
├── merger/                      # Multi-schema composition
├── types.go                     # Core protocol types
├── manifest.go                  # Schema manifest operations
├── provider.go                  # Schema provider interface
├── registry.go                  # Schema registry interface
├── storage.go                   # Storage abstraction
├── version.go                   # Protocol version constants
└── farp-rust/                   # Rust implementation
```

## Quick Start

### Using Schema Providers

```go
import (
    "github.com/xraph/farp/providers/openapi"
    "github.com/xraph/farp/providers/grpc"
    "github.com/xraph/farp/providers/graphql"
    "github.com/xraph/farp/providers/orpc"
    "github.com/xraph/farp/providers/thrift"
    "github.com/xraph/farp/providers/avro"
)

// OpenAPI provider
openapiProvider := openapi.NewProvider("3.1.0", "/openapi.json")
schema, err := openapiProvider.Generate(ctx, app)

// gRPC provider (with reflection)
grpcProvider := grpc.NewProvider("proto3", nil)
schema, err = grpcProvider.Generate(ctx, app)

// GraphQL provider (SDL format)
graphqlProvider := graphql.NewProvider("2021", "/graphql")
graphqlProvider.UseSDL()
schema, err = graphqlProvider.Generate(ctx, app)

// oRPC provider
orpcProvider := orpc.NewProvider("1.0.0", "/orpc.json")
schema, err = orpcProvider.Generate(ctx, app)

// Thrift provider (from IDL files)
thriftProvider := thrift.NewProvider("0.19.0", []string{"user.thrift"})
schema, err = thriftProvider.Generate(ctx, app)

// Avro provider (from schema files)
avroProvider := avro.NewProvider("1.11.1", []string{"user.avsc"})
schema, err = avroProvider.Generate(ctx, app)
```

### Service Registration

```go
import "github.com/xraph/farp"

// Create schema manifest
manifest := &farp.SchemaManifest{
    Version:        farp.ProtocolVersion,
    ServiceName:    "user-service",
    ServiceVersion: "v1.2.3",
    InstanceID:     "user-service-abc123",
    Schemas: []farp.SchemaDescriptor{
        {
            Type:        farp.SchemaTypeOpenAPI,
            SpecVersion: "3.1.0",
            Location: farp.SchemaLocation{
                Type: farp.LocationTypeHTTP,
                URL:  "http://user-service:8080/openapi.json",
            },
        },
        {
            Type:        farp.SchemaTypeGRPC,
            SpecVersion: "proto3",
            Location: farp.SchemaLocation{
                Type: farp.LocationTypeInline,
            },
        },
        {
            Type:        farp.SchemaTypeGraphQL,
            SpecVersion: "2021",
            Location: farp.SchemaLocation{
                Type: farp.LocationTypeHTTP,
                URL:  "http://user-service:8080/graphql",
            },
        },
    },
    Capabilities: []string{"rest", "grpc", "graphql", "websocket"},
    Endpoints: farp.SchemaEndpoints{
        Health:         "/health",
        Metrics:        "/metrics",
        OpenAPI:        "/openapi.json",
        GraphQL:        "/graphql",
        GRPCReflection: true,
    },
}

// Register with backend
registry.RegisterManifest(ctx, manifest)
```

### Service Discovery (Recommended)

The discovery system handles registration, health, and route management automatically:

```go
import (
    "github.com/xraph/farp/discovery"
    "github.com/xraph/farp/discovery/consul"
)

// === Service side ===
disc, _ := consul.New(consul.Config{Address: "consul:8500"})
node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
    ServiceName: "user-service",
    Address:     "10.0.0.5:8080",
    Discovery:   disc,
})
node.Start(ctx)
defer node.Stop(ctx)
http.Handle("/_farp/", node.HTTPHandler())

// === Gateway side ===
gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    Discovery: disc,
    OnRoutesChanged: func(routes []gateway.ServiceRoute) {
        for _, route := range routes {
            proxy.AddRoute(route.Path, route.TargetURL)
        }
    },
})
gw.Start(ctx)
```

### Push Mode (Zero Infrastructure)

```go
// Service pushes directly to gateway — no Consul/etcd needed
node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
    ServiceName: "user-service",
    Address:     "10.0.0.5:8080",
    GatewayURL:  "http://gateway:9090/_farp/v1",
})
node.Start(ctx)

// Gateway accepts push registrations
gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
    EnablePush: true,
    OnRoutesChanged: updateRoutes,
})
gw.Start(ctx)
http.Handle("/_farp/v1/", gw.PushHandler())
```

### Low-Level Gateway Client

```go
import "github.com/xraph/farp/gateway"

// For advanced use without the discovery system
client := gateway.NewClient(registry)
client.WatchServices(ctx, "", func(routes []gateway.ServiceRoute) {
    for _, route := range routes {
        proxy.AddRoute(route.Path, route.TargetURL)
    }
})
```

## Use Cases

1. **API Gateway Auto-Configuration**: Gateways like Kong, Traefik, or Nginx automatically register routes based on service schemas
2. **Service Mesh Control Plane**: Enable contract-aware routing and validation in service meshes
3. **Developer Portals**: Auto-generate API documentation from registered schemas
4. **Multi-Protocol Systems**: Unified discovery for REST, gRPC, GraphQL, and async protocols
5. **Contract Testing**: Validate service contracts at deployment time
6. **Schema Versioning**: Support multiple API versions with zero-downtime migrations

## Protocol Status

- ✅ Core protocol specification (v1.1.0)
- ✅ Type definitions with route table, routes checksum, and operational hints
- ✅ Schema providers (OpenAPI, AsyncAPI, gRPC, GraphQL, oRPC, Thrift, Avro)
- ✅ Service discovery system (Consul, etcd, Kubernetes, Redis, mDNS, Push)
- ✅ ServiceNode and GatewayNode auto-lifecycle management
- ✅ Gateway client with atomic route swap (zero-downtime)
- ✅ Rust implementation with feature-flagged backends
- ⏳ Community feedback and refinement

## Supported Schema Types

| Protocol | Status | Provider | Spec Version | Content Type |
|----------|--------|----------|--------------|--------------|
| **OpenAPI** | ✅ Complete | `providers/openapi` | 3.1.0 (default) | `application/json` |
| **AsyncAPI** | ✅ Complete | `providers/asyncapi` | 3.0.0 (default) | `application/json` |
| **gRPC** | ✅ Complete | `providers/grpc` | proto3 (default) | `application/json` |
| **GraphQL** | ✅ Complete | `providers/graphql` | 2021 (default) | `application/graphql` or `application/json` |
| **oRPC** | ✅ Complete | `providers/orpc` | 1.0.0 | `application/json` |
| **Thrift** | ✅ Complete | `providers/thrift` | 0.19.0 (default) | `application/json` |
| **Avro** | ✅ Complete | `providers/avro` | 1.11.1 (default) | `application/json` |

See [PROVIDERS_IMPLEMENTATION.md](PROVIDERS_IMPLEMENTATION.md) for detailed provider documentation.

## Documentation

### Essential Reading

- **[Service Discovery Guide](docs/DISCOVERY.md)** - **START HERE** - Full discovery system guide with all backends
- [Complete Specification](docs/SPECIFICATION.md) - Full protocol specification (v1.1.0)
- [Architecture Guide](docs/ARCHITECTURE.md) - Design decisions and architectural boundaries
- [Implementation Responsibilities](docs/IMPLEMENTATION_RESPONSIBILITIES.md) - What FARP provides vs what you implement

### Integration Guides

- [Gateway Discovery Examples](docs/GATEWAY_DISCOVERY_EXAMPLES.md) - Practical gateway integration examples
- [mDNS Service Type Configuration](docs/MDNS_SERVICE_TYPE_GUIDE.md) - Service type filtering for gateways
- [Providers Implementation](PROVIDERS_IMPLEMENTATION.md) - Schema provider guide

### Reference

- [FARP Specification](docs/SPECIFICATION.md) - Complete protocol specification
- [Error Handling](errors.go) - Error types and handling patterns

## Contributing

FARP is part of the Forge framework. Contributions are welcome! Please see the main Forge repository for contribution guidelines.

## License

Apache License 2.0 - See LICENSE file in the Forge repository
