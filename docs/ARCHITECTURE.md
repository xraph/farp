# FARP Architecture Guide

## Architectural Boundaries

### Core Philosophy

FARP is a **protocol specification library**, NOT a complete gateway or service framework. This document clarifies what FARP provides versus what implementers (services and gateways) must build.

### Responsibility Matrix

| Concern | FARP Library | Service Implementation | Gateway Implementation |
|---------|--------------|------------------------|------------------------|
| **Data Structures** | ✅ Defines types | Uses types | Uses types |
| **Schema Generation** | ✅ Providers | Calls providers | - |
| **Schema Merging** | ✅ Merge logic | - | Calls merge logic |
| **HTTP Endpoints** | ❌ Examples only | ✅ **Must implement** | ✅ **Must implement** |
| **Service Discovery** | ❌ Interface only | ✅ **Must integrate** | ✅ **Must integrate** |
| **Registry Backend** | ❌ Interface only | ✅ **Must choose/config** | ✅ **Must choose/config** |
| **Route Configuration** | ❌ Examples only | - | ✅ **Must implement** |
| **Health Monitoring** | ❌ Not provided | ✅ **Must expose** | ✅ **Must poll** |
| **Webhook Transport** | ❌ Types only | ✅ **Must implement** | ✅ **Must implement** |

### What FARP Provides (✅)

1. **Type System** (`types.go`)
   - `SchemaManifest`, `SchemaDescriptor`, `InstanceMetadata`, etc.
   - Routing, authentication, and webhook configuration types
   - JSON serialization/deserialization

2. **Schema Providers** (`providers/*`)
   - OpenAPI, AsyncAPI, gRPC, GraphQL, oRPC, Thrift, Avro generators
   - Extract schemas from application code/IDL files
   - Return standardized schema format

3. **Schema Merging** (`merger/*`)
   - Combine multiple service schemas into unified docs
   - Conflict resolution strategies
   - Support for all protocol types

4. **Validation Logic** (`manifest.go`)
   - Ensure manifests are spec-compliant
   - Checksum calculation and verification
   - Version compatibility checks

5. **Storage Abstractions** (`registry.go`)
   - Interface definitions only
   - No backend implementations (except `memory` for testing)

### What Service Frameworks Must Implement (Services using FARP)

1. **HTTP Server Endpoints**
   - `GET /_farp/manifest` - Return `SchemaManifest` JSON
   - `GET /openapi.json` - Return OpenAPI schema (if using HTTP location)
   - `GET /asyncapi.json` - Return AsyncAPI schema (if using HTTP location)
   - `GET /health` - Health check endpoint
   - `GET /metrics` - Metrics endpoint

2. **Discovery Backend Integration**
   - Register service with Consul/etcd/K8s/mDNS
   - Store FARP manifest in backend metadata
   - Handle TTL/heartbeats

3. **Schema Generation Workflow**
   ```go
   // Service framework (e.g., Forge) must:
   1. Initialize FARP providers
   2. Generate schemas from router
   3. Create SchemaManifest
   4. Expose via HTTP or store in registry
   5. Register with discovery backend
   ```

4. **Optional: Webhook Receivers**
   - Accept HTTP POST from gateway with events
   - Validate signatures
   - Handle event delivery

**Example**: The **Forge framework** would integrate FARP by calling providers during startup and exposing the manifest via HTTP handlers.

### What Gateways Must Implement (Consuming FARP)

1. **Service Discovery Client**
   - Watch Consul/etcd/K8s/mDNS for service registrations
   - Extract FARP manifest from service metadata
   - Handle service additions/removals/updates

2. **HTTP Client**
   - Fetch schemas from `LocationTypeHTTP` URLs
   - Handle timeouts, retries, TLS verification
   - Parse and validate fetched schemas

3. **Schema-to-Route Conversion**
   - Parse OpenAPI paths → HTTP routes
   - Parse AsyncAPI channels → WebSocket routes
   - Parse gRPC services → gRPC routes
   - Apply routing strategies (mount at root, prefix with service name, etc.)

4. **Route Configuration**
   - Apply routes to gateway-specific config (Kong, Traefik, Envoy, etc.)
   - Handle route updates and removals
   - Traffic splitting for multiple versions

5. **Health Monitoring**
   - Poll service health endpoints
   - Update routing based on health status
   - Circuit breaker logic

6. **Optional: Webhook Dispatching**
   - Send events to service webhook endpoints
   - Retry with exponential backoff
   - Track delivery status

**Example**: **octopus-gateway** (Rust) would watch mDNS for services, fetch FARP manifests, and configure its internal routing table.

### FARP's `gateway/client.go` - What Is It?

The `gateway/client.go` package is a **reference implementation/helper**, NOT production-ready gateway code. It demonstrates:

- ✅ How to structure a gateway integration
- ✅ How to watch for manifest changes
- ✅ How to convert schemas to routes
- ✅ How to cache schemas

**It does NOT provide**:
- ❌ Complete HTTP client (HTTP fetch has `TODO`)
- ❌ Production-ready error handling
- ❌ Gateway-specific route application
- ❌ Health monitoring
- ❌ Load balancing logic

**Real gateways should use it as a reference**, not a dependency.

---

## Design Principles

### 1. **Separation of Concerns**

FARP is designed with clear boundaries:

- **Protocol Core**: Types, interfaces, validation (backend-agnostic)
- **Schema Providers**: Protocol-specific generators (OpenAPI, gRPC, etc.)
- **Storage Interfaces**: Abstract registry operations (implementations separate)
- **Gateway Helpers**: Reference integration examples (not production)

### 2. **Pluggability**

Every major component is pluggable via interfaces:

```
┌─────────────────────────────────────────┐
│         Application Layer               │
│  (Services, Gateways, Tooling)         │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│         FARP Protocol Core              │
│  - SchemaManifest types                 │
│  - SchemaProvider interface             │
│  - SchemaRegistry interface             │
│  - Validation & serialization           │
└─────────────────┬───────────────────────┘
                  │
         ┌────────┴────────┐
         │                 │
┌────────▼─────┐  ┌───────▼──────────┐
│  Providers   │  │  Registry Impls  │
│              │  │                  │
│ - OpenAPI    │  │ - Consul         │
│ - AsyncAPI   │  │ - etcd           │
│ - gRPC       │  │ - Kubernetes     │
│ - GraphQL    │  │ - Redis          │
│ - Custom     │  │ - Memory         │
└──────────────┘  └──────────────────┘
```

### 3. **Protocol Independence**

The core protocol has zero dependencies on:
- Discovery backend implementations
- Forge framework internals
- Gateway implementations

This allows:
- Use FARP with non-Forge services
- Integrate with any gateway (Kong, Traefik, Envoy, etc.)
- Swap backends without protocol changes

### 4. **Production-First**

Design decisions prioritize production requirements:

- **Checksums**: Detect schema corruption, enable efficient change detection
- **Versioning**: Support blue-green deployments, gradual rollouts
- **Size Limits**: Prevent backend overload, force HTTP strategy for large schemas
- **Rate Limiting**: Prevent DoS via excessive updates
- **Audit Logging**: Track all schema changes for compliance

---

## Component Architecture

### Layer 1: Protocol Core (`farp/`)

**Responsibilities**:
- Define canonical types (`SchemaManifest`, `SchemaDescriptor`)
- Define interfaces (`SchemaProvider`, `SchemaRegistry`)
- Provide validation and serialization utilities
- Calculate checksums
- Version compatibility checks

**Dependencies**: None (only Go stdlib)

**Package Structure**:

```
farp/
├── types.go          # Core types (SchemaManifest, SchemaDescriptor, etc.)
├── manifest.go       # Manifest operations (checksum, validation)
├── provider.go       # SchemaProvider interface
├── registry.go       # SchemaRegistry interface
├── storage.go        # Storage abstraction
├── validation.go     # Schema validation
├── checksum.go       # Checksum calculation
├── version.go        # Protocol version constants
└── errors.go         # Error types
```

### Layer 2: Schema Providers (`farp/providers/`)

**Responsibilities**:
- Generate schemas from application code
- Implement `SchemaProvider` interface
- Protocol-specific logic (OpenAPI path extraction, gRPC reflection, etc.)

**Dependencies**: Protocol Core + specific schema libraries

**Package Structure**:

```
farp/providers/
├── openapi/
│   ├── provider.go       # OpenAPIProvider implementation
│   ├── generator.go      # OpenAPI spec generation
│   └── validator.go      # OpenAPI validation
├── asyncapi/
│   ├── provider.go       # AsyncAPIProvider implementation
│   ├── generator.go      # AsyncAPI spec generation
│   └── validator.go      # AsyncAPI validation
├── grpc/
│   ├── provider.go       # GRPCProvider implementation
│   ├── reflection.go     # gRPC reflection client
│   └── protobuf.go       # Protobuf parsing
└── graphql/
    ├── provider.go       # GraphQLProvider implementation
    └── introspection.go  # GraphQL introspection query
```

### Layer 3: Registry Implementations (`farp/registry/`)

**Responsibilities**:
- Store/retrieve manifests and schemas
- Implement `SchemaRegistry` interface
- Backend-specific optimizations

**Dependencies**: Protocol Core + backend client libraries

**Package Structure**:

```
farp/registry/
├── consul/
│   ├── registry.go       # Consul implementation
│   ├── watcher.go        # Consul watch support
│   └── config.go         # Consul-specific config
├── etcd/
│   ├── registry.go       # etcd implementation
│   ├── watcher.go        # etcd watch support
│   └── config.go         # etcd-specific config
├── kubernetes/
│   ├── registry.go       # K8s ConfigMap implementation
│   └── watcher.go        # K8s watch support
├── redis/
│   ├── registry.go       # Redis implementation
│   └── pubsub.go         # Redis pub/sub for changes
└── memory/
    └── registry.go       # In-memory (for testing)
```

### Layer 4: Gateway Integration (`farp/gateway/`)

**Responsibilities**:
- Watch for schema changes
- Fetch schemas
- Convert schemas to gateway-specific route configs
- Reference implementation for gateway developers

**Dependencies**: Protocol Core + Registry

**Package Structure**:

```
farp/gateway/
├── client.go         # Gateway client (watches manifests)
├── watcher.go        # Change notification handler
├── fetcher.go        # Schema fetcher (HTTP + registry)
├── converter.go      # Schema → Route conversion
├── cache.go          # Local schema cache
└── examples/
    ├── kong.go       # Kong gateway integration example
    ├── traefik.go    # Traefik integration example
    └── envoy.go      # Envoy xDS integration example
```

---

## Data Flow

### Service Startup

```
┌─────────────────┐
│  Forge App      │
│  Startup        │
└────────┬────────┘
         │
         │ 1. Initialize router
         ▼
┌─────────────────┐
│  Schema         │
│  Providers      │ 2. Generate schemas (OpenAPI, AsyncAPI)
└────────┬────────┘
         │
         │ 3. Create manifest
         ▼
┌─────────────────┐
│  Manifest       │
│  Builder        │ 4. Calculate checksums
└────────┬────────┘
         │
         │ 5. Publish schemas (if registry strategy)
         ▼
┌─────────────────┐
│  Schema         │
│  Registry       │ 6. Store in backend (Consul, etcd, etc.)
└────────┬────────┘
         │
         │ 7. Register service instance + manifest
         ▼
┌─────────────────┐
│  Discovery      │
│  Backend        │
└─────────────────┘
```

### Gateway Discovery

```
┌─────────────────┐
│  API Gateway    │
│  Startup        │
└────────┬────────┘
         │
         │ 1. Connect to discovery backend
         ▼
┌─────────────────┐
│  Discovery      │
│  Watch          │ 2. Subscribe to service registrations
└────────┬────────┘
         │
         │ 3. New service registered → event
         ▼
┌─────────────────┐
│  Manifest       │
│  Fetcher        │ 4. Fetch SchemaManifest from instance metadata
└────────┬────────┘
         │
         │ 5. For each schema in manifest
         ▼
┌─────────────────┐
│  Schema         │
│  Fetcher        │ 6. Fetch schema (registry or HTTP)
└────────┬────────┘
         │
         │ 7. Validate checksum
         ▼
┌─────────────────┐
│  Schema         │
│  Converter      │ 8. Convert OpenAPI → HTTP routes
└────────┬────────┘         AsyncAPI → WebSocket routes
         │
         │ 9. Configure gateway routes
         ▼
┌─────────────────┐
│  Gateway        │
│  Route Table    │
└─────────────────┘
```

### Schema Update

```
┌─────────────────┐
│  Service        │
│  Hot Reload     │
└────────┬────────┘
         │
         │ 1. Routes changed
         ▼
┌─────────────────┐
│  Schema         │
│  Providers      │ 2. Regenerate schemas
└────────┬────────┘
         │
         │ 3. Calculate new checksums
         ▼
┌─────────────────┐
│  Checksum       │
│  Comparator     │ 4. Compare with previous
└────────┬────────┘
         │
         │ 5. If changed
         ▼
┌─────────────────┐
│  Schema         │
│  Registry       │ 6. Update schemas in backend
└────────┬────────┘
         │
         │ 7. Update manifest (new checksum + timestamp)
         ▼
┌─────────────────┐
│  Discovery      │
│  Backend        │ 8. Trigger change notification
└────────┬────────┘
         │
         │ 9. Gateway watch detects change
         ▼
┌─────────────────┐
│  Gateway        │
│  Reconfigure    │ 10. Fetch updated schemas, reconfigure routes
└─────────────────┘
```

---

## Integration Patterns

### Pattern 1: Registry-First (Recommended for Production)

**Flow**:
1. Service publishes schemas to backend KV store
2. Service registers with manifest pointing to registry paths
3. Gateway fetches schemas from backend

**Advantages**:
- Schemas persist even if service dies
- Fast gateway startup (no service polling)
- Centralized schema storage
- Backend handles high availability

**Configuration**:

```go
registry := consul.NewSchemaRegistry(consulClient)
registry.PublishSchema(ctx, "/schemas/user-service/v1/openapi", openAPISpec)

manifest := &farp.SchemaManifest{
    Schemas: []farp.SchemaDescriptor{{
        Type: farp.SchemaTypeOpenAPI,
        Location: farp.SchemaLocation{
            Type: farp.LocationTypeRegistry,
            RegistryPath: "/schemas/user-service/v1/openapi",
        },
    }},
}
```

### Pattern 2: HTTP-First (Simpler, No Backend Storage)

**Flow**:
1. Service serves schemas via HTTP endpoints
2. Service registers with manifest pointing to HTTP URLs
3. Gateway fetches schemas directly from service

**Advantages**:
- No backend storage required
- Service controls schema freshness
- Simpler deployment

**Configuration**:

```go
manifest := &farp.SchemaManifest{
    Schemas: []farp.SchemaDescriptor{{
        Type: farp.SchemaTypeOpenAPI,
        Location: farp.SchemaLocation{
            Type: farp.LocationTypeHTTP,
            URL: "http://user-service:8080/openapi.json",
        },
    }},
}
```

### Pattern 3: Hybrid (Best of Both)

**Flow**:
1. Service publishes schemas to backend
2. Service also serves schemas via HTTP
3. Gateway tries registry first, falls back to HTTP

**Advantages**:
- High availability (two sources)
- Works even if backend is down
- Self-healing

**Configuration**:

```go
manifest := &farp.SchemaManifest{
    Schemas: []farp.SchemaDescriptor{{
        Type: farp.SchemaTypeOpenAPI,
        Location: farp.SchemaLocation{
            Type: farp.LocationTypeRegistry,
            RegistryPath: "/schemas/user-service/v1/openapi",
        },
    }},
    Endpoints: farp.SchemaEndpoints{
        OpenAPI: "/openapi.json",  // Fallback HTTP endpoint
    },
}
```

---

## Deployment Strategies

### Blue-Green Deployment

```
Step 1: Deploy v2 with new schema
┌──────────────┐    ┌──────────────┐
│  Service v1  │    │  Service v2  │
│  (100%)      │    │  (0%)        │
└──────┬───────┘    └──────┬───────┘
       │                   │
       │  v1 manifest      │  v2 manifest
       ▼                   ▼
    ┌───────────────────────────┐
    │  Gateway                  │
    │  - Registers v2 routes    │
    │  - Keeps v1 routes active │
    └───────────────────────────┘

Step 2: Shift traffic gradually
┌──────────────┐    ┌──────────────┐
│  Service v1  │    │  Service v2  │
│  (90%)       │    │  (10%)       │
└──────┬───────┘    └──────┬───────┘
       ▼                   ▼
    ┌───────────────────────────┐
    │  Gateway                  │
    │  - Traffic split 90/10    │
    └───────────────────────────┘

Step 3: Complete migration
┌──────────────┐    ┌──────────────┐
│  Service v1  │    │  Service v2  │
│  (0%)        │    │  (100%)      │
└──────────────┘    └──────┬───────┘
                           ▼
    ┌───────────────────────────┐
    │  Gateway                  │
    │  - Removes v1 routes      │
    │  - v2 routes active       │
    └───────────────────────────┘
```

### Canary Deployment

Similar to blue-green, but with smaller traffic percentages (1%, 5%, 10%, etc.)

### Rolling Deployment

```
Initial state: 3 instances v1
┌──────┐ ┌──────┐ ┌──────┐
│ v1-1 │ │ v1-2 │ │ v1-3 │
└──┬───┘ └──┬───┘ └──┬───┘
   └────────┴────────┘
          │
     ┌────▼────┐
     │ Gateway │  (All use v1 schema)
     └─────────┘

Step 1: Update instance 1
┌──────┐ ┌──────┐ ┌──────┐
│ v2-1 │ │ v1-2 │ │ v1-3 │
└──┬───┘ └──┬───┘ └──┬───┘
   └────────┴────────┘
          │
     ┌────▼────┐
     │ Gateway │  (Supports v1 + v2 schemas)
     └─────────┘

Step 2: Update instance 2
┌──────┐ ┌──────┐ ┌──────┐
│ v2-1 │ │ v2-2 │ │ v1-3 │
└──┬───┘ └──┬───┘ └──┬───┘
   └────────┴────────┘
          │
     ┌────▼────┐
     │ Gateway │  (Majority on v2)
     └─────────┘

Step 3: Update instance 3
┌──────┐ ┌──────┐ ┌──────┐
│ v2-1 │ │ v2-2 │ │ v2-3 │
└──┬───┘ └──┬───┘ └──┬───┘
   └────────┴────────┘
          │
     ┌────▼────┐
     │ Gateway │  (All v2, remove v1 routes)
     └─────────┘
```

---

## Performance Considerations

### 1. Schema Caching

Gateway maintains local cache to avoid repeated fetches:

```go
type SchemaCache struct {
    mu      sync.RWMutex
    schemas map[string]CachedSchema  // key: hash
    ttl     time.Duration
}

type CachedSchema struct {
    Schema     interface{}
    FetchedAt  time.Time
    AccessedAt time.Time
}

// Fetch with cache
func (g *Gateway) GetSchema(descriptor SchemaDescriptor) (interface{}, error) {
    // Check cache by hash
    if cached, ok := g.cache.Get(descriptor.Hash); ok {
        return cached.Schema, nil
    }
    
    // Cache miss, fetch from source
    schema, err := g.fetchSchema(descriptor)
    if err != nil {
        return nil, err
    }
    
    // Store in cache
    g.cache.Set(descriptor.Hash, schema)
    return schema, nil
}
```

### 2. Watch Efficiency

Use backend-native watch mechanisms:

| Backend | Mechanism | Efficiency |
|---------|-----------|------------|
| Consul | Blocking queries (long polling) | High |
| etcd | gRPC streaming | Very high |
| Kubernetes | Watch API (HTTP streaming) | High |
| Redis | Pub/Sub | High |

### 3. Batch Operations

Group schema publishes:

```go
func (r *Registry) PublishManifest(ctx context.Context, manifest *SchemaManifest) error {
    // Batch all schema publishes in a transaction
    txn := r.backend.Transaction()
    
    for _, schema := range manifest.Schemas {
        if schema.Location.Type == LocationTypeRegistry {
            txn.Put(schema.Location.RegistryPath, schema.Data)
        }
    }
    
    // Single commit
    return txn.Commit(ctx)
}
```

### 4. Compression

Compress large schemas:

```go
func compressSchema(data []byte) ([]byte, error) {
    if len(data) < 1024 {
        return data, nil  // Don't compress small schemas
    }
    
    var buf bytes.Buffer
    gzipWriter := gzip.NewWriter(&buf)
    gzipWriter.Write(data)
    gzipWriter.Close()
    
    return buf.Bytes(), nil
}
```

---

## Error Recovery

### 1. Schema Fetch Failure

```go
func (g *Gateway) handleSchemaFetchError(descriptor SchemaDescriptor, err error) {
    // Try fallback locations
    if fallback := g.getFallbackLocation(descriptor); fallback != nil {
        schema, err := g.fetchFromLocation(fallback)
        if err == nil {
            return
        }
    }
    
    // Use cached schema if available
    if cached, ok := g.cache.Get(descriptor.Hash); ok {
        logger.Warn("using stale cached schema", "age", time.Since(cached.FetchedAt))
        return
    }
    
    // Mark service as degraded, continue with existing routes
    g.markServiceDegraded(descriptor.ServiceName)
}
```

### 2. Backend Unavailability

```go
func (r *Registry) RegisterManifest(ctx context.Context, manifest *SchemaManifest) error {
    // Retry with exponential backoff
    backoff := retry.NewExponential(time.Second)
    
    for i := 0; i < 5; i++ {
        err := r.backend.Put(ctx, key, data)
        if err == nil {
            return nil
        }
        
        if !isRetryable(err) {
            return err
        }
        
        time.Sleep(backoff.Next())
    }
    
    // Store locally for eventual consistency
    r.pendingQueue.Add(manifest)
    go r.retryPendingManifests()
    
    return nil
}
```

### 3. Schema Validation Failure

```go
func (g *Gateway) processSchema(descriptor SchemaDescriptor) error {
    schema, err := g.fetchSchema(descriptor)
    if err != nil {
        return err
    }
    
    // Validate schema format
    if err := validateSchema(schema, descriptor.Type); err != nil {
        // Log error but don't fail
        logger.Error("invalid schema received",
            "service", descriptor.ServiceName,
            "type", descriptor.Type,
            "error", err,
        )
        
        // Use last known good schema
        return g.useLastKnownGoodSchema(descriptor)
    }
    
    return nil
}
```

---

## Observability

### Metrics

```go
// Service-side metrics
farp_manifest_publish_total{service="user-service",status="success"}
farp_manifest_publish_duration_seconds{service="user-service"}
farp_schema_size_bytes{service="user-service",type="openapi"}

// Gateway-side metrics
farp_manifest_watch_events_total{service="user-service"}
farp_schema_fetch_total{service="user-service",type="openapi",status="success"}
farp_schema_fetch_duration_seconds{service="user-service",type="openapi"}
farp_schema_cache_hit_ratio{service="user-service"}
farp_route_updates_total{service="user-service",action="add"}
```

### Logs

```go
// Structured logging with context
logger.Info("schema manifest registered",
    "service", manifest.ServiceName,
    "version", manifest.ServiceVersion,
    "instance_id", manifest.InstanceID,
    "schemas", len(manifest.Schemas),
    "checksum", manifest.Checksum,
    "size_bytes", len(manifestJSON),
)

logger.Info("gateway routes configured",
    "service", manifest.ServiceName,
    "routes_added", len(newRoutes),
    "routes_updated", len(updatedRoutes),
    "routes_removed", len(removedRoutes),
    "duration_ms", time.Since(start).Milliseconds(),
)
```

### Traces

Use OpenTelemetry for distributed tracing:

```go
ctx, span := tracer.Start(ctx, "farp.registry.publish_manifest")
defer span.End()

span.SetAttributes(
    attribute.String("service", manifest.ServiceName),
    attribute.String("version", manifest.ServiceVersion),
    attribute.Int("schemas", len(manifest.Schemas)),
)
```

---

## Testing Strategy

### Unit Tests

- Manifest validation
- Checksum calculation
- Schema serialization
- Location resolution

### Integration Tests

- Registry operations with real backends (test containers)
- Schema fetch with HTTP mock servers
- Watch notifications

### End-to-End Tests

- Full flow: service startup → gateway discovery → route configuration
- Schema updates and change propagation
- Failure scenarios (backend down, schema fetch timeout)

### Performance Tests

- Schema registration latency
- Watch notification latency
- Gateway startup time with 100+ services
- Cache hit ratio under load

---

**Design philosophy**: Simple things simple, complex things possible, production things robust.

