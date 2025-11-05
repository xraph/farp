# FARP Protocol Specification v1.0.0

## Table of Contents

1. [Introduction](#1-introduction)
2. [Protocol Overview](#2-protocol-overview)
3. [Core Concepts](#3-core-concepts)
4. [Data Structures](#4-data-structures)
5. [Schema Types](#5-schema-types)
6. [Location Strategies](#6-location-strategies)
7. [Route Mounting Strategies](#7-route-mounting-strategies)
8. [Authentication & Authorization](#8-authentication--authorization)
9. [Bidirectional Communication](#9-bidirectional-communication)
10. [Multi-Instance Management](#10-multi-instance-management)
11. [Schema Compatibility](#11-schema-compatibility)
12. [Route Metadata](#12-route-metadata)
13. [Protocol-Specific Metadata](#13-protocol-specific-metadata)
    - [OpenAPI Schema Composition](#127-openapi-schema-composition)
14. [Service Hints](#14-service-hints)
15. [Registration Flow](#15-registration-flow)
16. [Schema Provider Interface](#16-schema-provider-interface)
17. [Registry Interface](#17-registry-interface)
18. [Service Discovery Integration](#18-service-discovery-integration)
19. [Storage Backend](#19-storage-backend)
20. [Change Detection](#20-change-detection)
21. [Security Considerations](#21-security-considerations)
22. [Error Handling](#22-error-handling)
23. [Versioning](#23-versioning)
24. [Extensibility](#24-extensibility)

---

## 1. Introduction

### 1.1 Purpose

The Forge API Gateway Registration Protocol (FARP) provides a standardized mechanism for service instances to register their API schemas, health information, and capabilities with API gateways and service discovery systems. This enables dynamic route configuration, contract-aware service meshes, and automatic API documentation.

### 1.2 Goals

- **Schema-Aware Discovery**: Extend service discovery with API contract information
- **Gateway Automation**: Enable API gateways to auto-configure routes from schemas
- **Multi-Protocol Support**: Support REST (OpenAPI), async (AsyncAPI), gRPC, GraphQL, and future protocols
- **Backend Agnostic**: Work with any service discovery backend (Consul, etcd, Kubernetes, mDNS/Bonjour, Eureka, etc.)
- **Transport Agnostic**: Schema metadata propagates through any service discovery transport (KV stores, DNS TXT records, ConfigMaps, etc.)
- **Production Ready**: Handle versioning, health checks, change detection, and zero-downtime updates

### 1.3 Non-Goals

- Replace existing service discovery protocols (designed to extend them)
- Provide a new API gateway implementation (protocol only)
- Handle service-to-service authentication (delegate to existing systems)
- Implement a schema validation engine (use existing validators)

---

## 2. Protocol Overview

### 2.1 Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                     API Gateway / Service Mesh                 │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  FARP Client (Gateway Integration)                       │ │
│  │  - Watches for schema updates                            │ │
│  │  - Fetches schemas from registry or HTTP                 │ │
│  │  - Converts schemas to gateway routes                    │ │
│  │  - Monitors service health                               │ │
│  └──────────────────────────────────────────────────────────┘ │
└─────────────────────────┬──────────────────────────────────────┘
                          │
                          │ Subscribe to manifest changes
                          │
┌─────────────────────────▼──────────────────────────────────────┐
│              Service Discovery Backend                         │
│  (Consul, etcd, Kubernetes, mDNS/Bonjour, Eureka, etc.)      │
│                                                                 │
│  Storage varies by backend:                                    │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │ KV Store (Consul/etcd):                                 │  │
│  │   /services/user-service/instances/abc123/              │  │
│  │     ├── metadata        (ServiceInstance + FARP)        │  │
│  │     └── manifest        (SchemaManifest)                │  │
│  │   /services/user-service/schemas/v1/openapi.json        │  │
│  │                                                          │  │
│  │ DNS TXT Records (mDNS):                                 │  │
│  │   _user-service._tcp.local.                             │  │
│  │     TXT: farp.manifest=http://.../                      │  │
│  │     TXT: farp.openapi=http://.../openapi.json           │  │
│  │     TXT: farp.enabled=true                              │  │
│  │                                                          │  │
│  │ ConfigMap (Kubernetes):                                 │  │
│  │   metadata:                                             │  │
│  │     annotations:                                        │  │
│  │       farp.manifest: "http://..."                       │  │
│  └─────────────────────────────────────────────────────────┘  │
└─────────────────────────▲──────────────────────────────────────┘
                          │
                          │ Register manifest + schemas
                          │
┌─────────────────────────┴──────────────────────────────────────┐
│                    Service Instance (Forge App)                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │  FARP Publisher                                          │ │
│  │  - Generates schemas from router (OpenAPI, AsyncAPI)    │ │
│  │  - Creates SchemaManifest                               │ │
│  │  - Publishes to registry or serves via HTTP             │ │
│  │  - Updates manifest on schema changes                   │ │
│  └──────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

### 2.2 Workflow

1. **Service Startup**:
   - Service generates schemas (OpenAPI, AsyncAPI, etc.)
   - Creates `SchemaManifest` with schema descriptors
   - Optionally publishes schemas to backend storage
   - Registers `ServiceInstance` with manifest reference

2. **Gateway Discovery**:
   - Gateway watches for service registrations
   - Fetches `SchemaManifest` for new services
   - Retrieves schemas from registry or HTTP endpoints
   - Converts schemas to gateway-specific route configurations

3. **Schema Updates**:
   - Service detects schema changes (code hot reload, config update)
   - Regenerates schemas and updates checksum
   - Publishes updated manifest
   - Gateway detects change and reconfigures routes

4. **Health Monitoring**:
   - Gateway polls health endpoints from manifest
   - Updates routing based on health status
   - Removes unhealthy instances from load balancer

---

## 3. Core Concepts

### 3.1 Schema Manifest

A **Schema Manifest** is a JSON document that describes all API contracts exposed by a service instance. It includes:

- Service identity (name, version, instance ID)
- List of schema descriptors (OpenAPI, AsyncAPI, gRPC, etc.)
- Capabilities (protocols supported: REST, WebSocket, gRPC, GraphQL, oRPC)
- Endpoints for health, metrics, and schema retrieval
- Metadata for versioning and change detection

### 3.2 Schema Descriptor

A **Schema Descriptor** describes a single API schema/contract. It specifies:

- Schema type (OpenAPI, AsyncAPI, gRPC, GraphQL, oRPC)
- Specification version (e.g., OpenAPI 3.1.0)
- Location (HTTP URL, inline JSON, or registry path)
- Content type
- Checksum for integrity validation

### 3.3 Schema Provider

A **Schema Provider** is a pluggable component that generates schemas from application code. Examples:

- `OpenAPIProvider`: Generates OpenAPI 3.1.0 from Forge router
- `AsyncAPIProvider`: Generates AsyncAPI 3.0.0 from streaming routes
- `GRPCProvider`: Extracts Protocol Buffer definitions
- `GraphQLProvider`: Extracts GraphQL SDL

### 3.4 Schema Registry

A **Schema Registry** is an abstraction for storing and retrieving schemas. It provides:

- `RegisterManifest()`: Store a schema manifest
- `GetManifest()`: Retrieve a schema manifest
- `PublishSchema()`: Store a full schema document
- `FetchSchema()`: Retrieve a full schema document
- `WatchManifests()`: Subscribe to manifest changes

### 3.5 Location Strategy

Schemas can be retrieved via three strategies:

- **HTTP**: Gateway fetches schema from service HTTP endpoint
- **Registry**: Gateway fetches schema from backend KV store
- **Inline**: Schema embedded directly in manifest (for small schemas)

---

## 4. Data Structures

### 4.1 SchemaManifest

```go
type SchemaManifest struct {
    // Protocol version (semver)
    Version string `json:"version"`
    
    // Service identity
    ServiceName    string `json:"service_name"`
    ServiceVersion string `json:"service_version"`
    InstanceID     string `json:"instance_id"`
    
    // Instance metadata
    Instance *InstanceMetadata `json:"instance,omitempty"`
    
    // Schemas exposed by this instance
    Schemas []SchemaDescriptor `json:"schemas"`
    
    // Capabilities/protocols supported
    Capabilities []string `json:"capabilities"`
    
    // Endpoints for introspection
    Endpoints SchemaEndpoints `json:"endpoints"`
    
    // Routing configuration for gateway federation
    Routing RoutingConfig `json:"routing"`
    
    // Authentication configuration
    Auth AuthConfig `json:"auth,omitempty"`
    
    // Webhook for bidirectional communication
    Webhook WebhookConfig `json:"webhook,omitempty"`
    
    // Service operational hints (non-binding)
    Hints *ServiceHints `json:"hints,omitempty"`
    
    // Change tracking
    UpdatedAt int64  `json:"updated_at"` // Unix timestamp
    Checksum  string `json:"checksum"`   // SHA256 of all schemas
}
```

**Fields**:

- `version`: Protocol version (currently "1.0.0")
- `service_name`: Logical service name (e.g., "user-service")
- `service_version`: Service version (semver recommended: "v1.2.3")
- `instance_id`: Unique instance identifier
- `instance`: Instance metadata (optional, see [10. Multi-Instance Management](#10-multi-instance-management))
- `schemas`: Array of schema descriptors
- `capabilities`: Supported protocols (e.g., ["rest", "grpc", "websocket"])
- `endpoints`: URLs for health, metrics, schema retrieval
- `routing`: Gateway route mounting configuration (see [7. Route Mounting Strategies](#7-route-mounting-strategies))
- `auth`: Authentication/authorization requirements (optional, see [8. Authentication & Authorization](#8-authentication--authorization))
- `webhook`: Webhook configuration for bidirectional communication (optional, see [9. Bidirectional Communication](#9-bidirectional-communication))
- `hints`: Service operational hints for gateway (optional, non-binding, see [13. Service Hints](#13-service-hints))
- `updated_at`: Timestamp of last manifest update
- `checksum`: SHA256 hash of concatenated schema hashes (for change detection)

### 4.2 SchemaDescriptor

```go
type SchemaDescriptor struct {
    // Type of schema
    Type SchemaType `json:"type"`
    
    // Specification version
    SpecVersion string `json:"spec_version"`
    
    // How to retrieve the schema
    Location SchemaLocation `json:"location"`
    
    // Content type
    ContentType string `json:"content_type"`
    
    // Optional: Inline schema for small schemas
    InlineSchema interface{} `json:"inline_schema,omitempty"`
    
    // Integrity validation
    Hash string `json:"hash"` // SHA256 of schema content
    Size int64  `json:"size"` // Size in bytes
    
    // Schema compatibility metadata
    Compatibility *SchemaCompatibility `json:"compatibility,omitempty"`
    
    // Protocol-specific metadata
    Metadata *ProtocolMetadata `json:"metadata,omitempty"`
}
```

**Fields**:

- `type`: Schema type enum (see [5. Schema Types](#5-schema-types))
- `spec_version`: Version of the schema specification (e.g., "3.1.0" for OpenAPI)
- `location`: How to retrieve the schema (see [6. Location Strategies](#6-location-strategies))
- `content_type`: MIME type (e.g., "application/json", "application/x-protobuf")
- `inline_schema`: Optional inline schema (for schemas < 100KB)
- `hash`: SHA256 checksum of schema content
- `size`: Schema size in bytes
- `compatibility`: Schema compatibility guarantees (optional, see [10. Schema Compatibility](#10-schema-compatibility))
- `metadata`: Protocol-specific metadata (optional, see [12. Protocol-Specific Metadata](#12-protocol-specific-metadata))

### 4.3 SchemaLocation

```go
type SchemaLocation struct {
    // Location type
    Type LocationType `json:"type"`
    
    // HTTP URL (if Type == HTTP)
    URL string `json:"url,omitempty"`
    
    // Registry path (if Type == Registry)
    RegistryPath string `json:"registry_path,omitempty"`
    
    // HTTP headers for authentication
    Headers map[string]string `json:"headers,omitempty"`
}
```

**Fields**:

- `type`: Location type enum (see [6. Location Strategies](#6-location-strategies))
- `url`: HTTP(S) URL for fetching schema
- `registry_path`: Path in backend KV store (e.g., "/schemas/user-service/v1/openapi")
- `headers`: Optional HTTP headers (e.g., `{"Authorization": "Bearer token"}`)

### 4.4 SchemaEndpoints

```go
type SchemaEndpoints struct {
    // Health check endpoint (required)
    Health string `json:"health"`
    
    // Prometheus metrics endpoint
    Metrics string `json:"metrics,omitempty"`
    
    // OpenAPI spec endpoint
    OpenAPI string `json:"openapi,omitempty"`
    
    // AsyncAPI spec endpoint
    AsyncAPI string `json:"asyncapi,omitempty"`
    
    // gRPC reflection enabled
    GRPCReflection bool `json:"grpc_reflection,omitempty"`
    
    // GraphQL introspection endpoint
    GraphQL string `json:"graphql,omitempty"`
}
```

**Fields**:

- `health`: Health check endpoint path (e.g., "/health" or "/healthz")
- `metrics`: Prometheus metrics endpoint (e.g., "/metrics")
- `openapi`: OpenAPI spec endpoint (e.g., "/openapi.json")
- `asyncapi`: AsyncAPI spec endpoint (e.g., "/asyncapi.json")
- `grpc_reflection`: Whether gRPC server reflection is enabled
- `graphql`: GraphQL endpoint that supports introspection queries

### 4.5 RoutingConfig

```go
type RoutingConfig struct {
    // Mounting strategy
    Strategy MountStrategy `json:"strategy"`
    
    // Base path for mounting (used with path-based strategies)
    BasePath string `json:"base_path,omitempty"`
    
    // Subdomain for mounting (used with subdomain strategy)
    Subdomain string `json:"subdomain,omitempty"`
    
    // Path rewriting rules
    Rewrite []PathRewrite `json:"rewrite,omitempty"`
    
    // Strip prefix before forwarding to service
    StripPrefix bool `json:"strip_prefix,omitempty"`
    
    // Priority for conflict resolution (higher = higher priority)
    Priority int `json:"priority,omitempty"`
    
    // Tags for route grouping and filtering
    Tags []string `json:"tags,omitempty"`
}

type MountStrategy string

const (
    // Merge service routes to gateway root (no prefix)
    MountStrategyRoot MountStrategy = "root"
    
    // Mount under /instance-id/* (default)
    MountStrategyInstance MountStrategy = "instance"
    
    // Mount under /service-name/*
    MountStrategyService MountStrategy = "service"
    
    // Mount under /service-name/version/*
    MountStrategyVersioned MountStrategy = "versioned"
    
    // Mount under custom base path
    MountStrategyCustom MountStrategy = "custom"
    
    // Mount on subdomain: service.gateway.com
    MountStrategySubdomain MountStrategy = "subdomain"
)

type PathRewrite struct {
    // Pattern to match (regex)
    Pattern string `json:"pattern"`
    
    // Replacement string
    Replacement string `json:"replacement"`
}
```

**Fields**:

- `strategy`: How to mount service routes in the federated gateway
  - `root`: Merge directly to gateway root (e.g., `/users` → `/users`)
  - `instance`: Mount under instance ID (e.g., `/users` → `/instance-abc123/users`) **[DEFAULT]**
  - `service`: Mount under service name (e.g., `/users` → `/user-service/users`)
  - `versioned`: Mount under service+version (e.g., `/users` → `/user-service/v1/users`)
  - `custom`: Mount under custom base path specified in `base_path`
  - `subdomain`: Mount on subdomain (e.g., `user-service.gateway.com/users`)
- `base_path`: Custom base path for mounting (required when `strategy = custom`)
- `subdomain`: Subdomain for mounting (required when `strategy = subdomain`)
- `rewrite`: Path rewriting rules applied before routing
- `strip_prefix`: If true, remove mount prefix before forwarding to service
- `priority`: Priority for conflict resolution (0-100, default: 50, higher wins)
- `tags`: Tags for route filtering (e.g., ["public", "internal"])

### 4.6 AuthConfig

```go
type AuthConfig struct {
    // Authentication schemes supported by service
    Schemes []AuthScheme `json:"schemes"`
    
    // Required permissions/scopes
    RequiredScopes []string `json:"required_scopes,omitempty"`
    
    // Access control rules
    AccessControl []AccessRule `json:"access_control,omitempty"`
    
    // Token validation endpoint
    TokenValidationURL string `json:"token_validation_url,omitempty"`
    
    // Public (unauthenticated) routes
    PublicRoutes []string `json:"public_routes,omitempty"`
}

type AuthScheme struct {
    // Scheme type
    Type AuthType `json:"type"`
    
    // Scheme configuration (varies by type)
    Config map[string]interface{} `json:"config,omitempty"`
}

type AuthType string

const (
    AuthTypeBearer      AuthType = "bearer"      // Bearer token (JWT, opaque)
    AuthTypeAPIKey      AuthType = "apikey"      // API key
    AuthTypeBasic       AuthType = "basic"       // Basic auth
    AuthTypeMTLS        AuthType = "mtls"        // Mutual TLS
    AuthTypeOAuth2      AuthType = "oauth2"      // OAuth 2.0
    AuthTypeOIDC        AuthType = "oidc"        // OpenID Connect
    AuthTypeCustom      AuthType = "custom"      // Custom scheme
)

type AccessRule struct {
    // Path pattern (glob or regex)
    Path string `json:"path"`
    
    // HTTP methods
    Methods []string `json:"methods"`
    
    // Required roles
    Roles []string `json:"roles,omitempty"`
    
    // Required permissions
    Permissions []string `json:"permissions,omitempty"`
    
    // Allow anonymous access
    AllowAnonymous bool `json:"allow_anonymous,omitempty"`
}
```

**Fields**:

- `schemes`: Authentication schemes accepted by the service
- `required_scopes`: OAuth2/OIDC scopes required for access
- `access_control`: Fine-grained access control rules per route
- `token_validation_url`: Endpoint for gateway to validate tokens
- `public_routes`: Routes that don't require authentication (glob patterns)

### 4.7 WebhookConfig

```go
type WebhookConfig struct {
    // Webhook endpoint on service for gateway notifications
    ServiceWebhook string `json:"service_webhook,omitempty"`
    
    // Webhook endpoint on gateway for service notifications
    GatewayWebhook string `json:"gateway_webhook,omitempty"`
    
    // Webhook secret for HMAC signature verification
    Secret string `json:"secret,omitempty"`
    
    // Event types service wants to receive from gateway
    SubscribeEvents []WebhookEventType `json:"subscribe_events,omitempty"`
    
    // Event types service will send to gateway
    PublishEvents []WebhookEventType `json:"publish_events,omitempty"`
    
    // Retry configuration
    Retry RetryConfig `json:"retry,omitempty"`
    
    // HTTP-based communication routes (alternative to push webhooks)
    HTTPRoutes *HTTPCommunicationRoutes `json:"http_routes,omitempty"`
}

type HTTPCommunicationRoutes struct {
    // Service exposes these routes for gateway to call
    ServiceRoutes []CommunicationRoute `json:"service_routes,omitempty"`
    
    // Gateway exposes these routes for service to call
    GatewayRoutes []CommunicationRoute `json:"gateway_routes,omitempty"`
    
    // Polling configuration if HTTP polling is used
    Polling *PollingConfig `json:"polling,omitempty"`
}

type CommunicationRoute struct {
    // Route identifier
    ID string `json:"id"`
    
    // Route path
    Path string `json:"path"`
    
    // HTTP method
    Method string `json:"method"`
    
    // Route purpose/type
    Type CommunicationRouteType `json:"type"`
    
    // Description
    Description string `json:"description,omitempty"`
    
    // Request schema (JSON Schema, OpenAPI schema, etc.)
    RequestSchema interface{} `json:"request_schema,omitempty"`
    
    // Response schema
    ResponseSchema interface{} `json:"response_schema,omitempty"`
    
    // Authentication required
    AuthRequired bool `json:"auth_required"`
    
    // Idempotent operation
    Idempotent bool `json:"idempotent"`
    
    // Expected timeout
    Timeout string `json:"timeout,omitempty"`
}

type CommunicationRouteType string

const (
    // Control plane operations
    RouteTypeControl         CommunicationRouteType = "control"
    RouteTypeAdmin           CommunicationRouteType = "admin"
    RouteTypeManagement      CommunicationRouteType = "management"
    
    // Lifecycle hooks
    RouteTypeLifecycleStart  CommunicationRouteType = "lifecycle.start"
    RouteTypeLifecycleStop   CommunicationRouteType = "lifecycle.stop"
    RouteTypeLifecycleReload CommunicationRouteType = "lifecycle.reload"
    
    // Configuration
    RouteTypeConfigUpdate    CommunicationRouteType = "config.update"
    RouteTypeConfigQuery     CommunicationRouteType = "config.query"
    
    // Events (polling-based)
    RouteTypeEventPoll       CommunicationRouteType = "event.poll"
    RouteTypeEventAck        CommunicationRouteType = "event.ack"
    
    // Health & Status
    RouteTypeHealthCheck     CommunicationRouteType = "health.check"
    RouteTypeStatusQuery     CommunicationRouteType = "status.query"
    
    // Schema operations
    RouteTypeSchemaQuery     CommunicationRouteType = "schema.query"
    RouteTypeSchemaValidate  CommunicationRouteType = "schema.validate"
    
    // Metrics & Observability
    RouteTypeMetricsQuery    CommunicationRouteType = "metrics.query"
    RouteTypeTracingExport   CommunicationRouteType = "tracing.export"
    
    // Custom
    RouteTypeCustom          CommunicationRouteType = "custom"
)

type PollingConfig struct {
    // Polling interval
    Interval string `json:"interval"` // Duration string
    
    // Polling timeout
    Timeout string `json:"timeout,omitempty"`
    
    // Long polling support
    LongPolling bool `json:"long_polling,omitempty"`
    
    // Long polling timeout
    LongPollingTimeout string `json:"long_polling_timeout,omitempty"`
}

type WebhookEventType string

const (
    // Service → Gateway events
    EventSchemaUpdated      WebhookEventType = "schema.updated"
    EventHealthChanged      WebhookEventType = "health.changed"
    EventInstanceScaling    WebhookEventType = "instance.scaling"
    EventMaintenanceMode    WebhookEventType = "maintenance.mode"
    
    // Gateway → Service events
    EventRateLimitChanged   WebhookEventType = "ratelimit.changed"
    EventCircuitBreakerOpen WebhookEventType = "circuit.breaker.open"
    EventCircuitBreakerClosed WebhookEventType = "circuit.breaker.closed"
    EventConfigUpdated      WebhookEventType = "config.updated"
    EventTrafficShift       WebhookEventType = "traffic.shift"
)

type RetryConfig struct {
    // Maximum retry attempts
    MaxAttempts int `json:"max_attempts"`
    
    // Initial retry delay
    InitialDelay string `json:"initial_delay"` // Duration string
    
    // Maximum retry delay
    MaxDelay string `json:"max_delay"`
    
    // Backoff multiplier
    Multiplier float64 `json:"multiplier"`
}
```

**Fields**:

- `service_webhook`: Webhook URL on service for receiving gateway notifications (push-based)
- `gateway_webhook`: Webhook URL on gateway for receiving service notifications (push-based)
- `secret`: Shared secret for HMAC-SHA256 signature verification
- `subscribe_events`: Events service wants to receive from gateway
- `publish_events`: Events service will send to gateway
- `retry`: Retry policy for failed webhook deliveries
- `http_routes`: HTTP communication routes (alternative/complement to push webhooks)
  - `service_routes`: Routes service exposes for gateway to call
  - `gateway_routes`: Routes gateway exposes for service to call
  - `polling`: Polling configuration for event polling pattern

### 4.8 SchemaCompatibility

```go
type SchemaCompatibility struct {
    // Compatibility mode
    Mode CompatibilityMode `json:"mode"`
    
    // Previous schema versions (for lineage tracking)
    PreviousVersions []string `json:"previous_versions,omitempty"`
    
    // Breaking changes from previous version
    BreakingChanges []BreakingChange `json:"breaking_changes,omitempty"`
    
    // Deprecation notices
    Deprecations []Deprecation `json:"deprecations,omitempty"`
}

type CompatibilityMode string

const (
    // New schema can read data written by old schema
    CompatibilityBackward CompatibilityMode = "backward"
    
    // Old schema can read data written by new schema
    CompatibilityForward CompatibilityMode = "forward"
    
    // Both backward and forward compatible
    CompatibilityFull CompatibilityMode = "full"
    
    // Breaking changes, no compatibility guaranteed
    CompatibilityNone CompatibilityMode = "none"
    
    // Transitive backward compatibility across N versions
    CompatibilityBackwardTransitive CompatibilityMode = "backward_transitive"
    
    // Transitive forward compatibility across N versions
    CompatibilityForwardTransitive CompatibilityMode = "forward_transitive"
)

type BreakingChange struct {
    // Type of breaking change
    Type ChangeType `json:"type"`
    
    // Path in schema (e.g., "/paths/users/get", "User.email")
    Path string `json:"path"`
    
    // Human-readable description
    Description string `json:"description"`
    
    // Severity level
    Severity ChangeSeverity `json:"severity"`
    
    // Migration instructions
    Migration string `json:"migration,omitempty"`
}

type ChangeType string

const (
    ChangeTypeFieldRemoved     ChangeType = "field_removed"
    ChangeTypeFieldTypeChanged ChangeType = "field_type_changed"
    ChangeTypeFieldRequired    ChangeType = "field_required"
    ChangeTypeEndpointRemoved  ChangeType = "endpoint_removed"
    ChangeTypeEndpointChanged  ChangeType = "endpoint_changed"
    ChangeTypeEnumValueRemoved ChangeType = "enum_value_removed"
    ChangeTypeMethodRemoved    ChangeType = "method_removed"
)

type ChangeSeverity string

const (
    SeverityCritical ChangeSeverity = "critical" // Immediate breakage
    SeverityHigh     ChangeSeverity = "high"     // Likely breakage
    SeverityMedium   ChangeSeverity = "medium"   // Possible breakage
    SeverityLow      ChangeSeverity = "low"      // Minimal risk
)

type Deprecation struct {
    // Path in schema
    Path string `json:"path"`
    
    // Deprecation date (ISO 8601)
    DeprecatedAt string `json:"deprecated_at"`
    
    // Planned removal date (ISO 8601)
    RemovalDate string `json:"removal_date,omitempty"`
    
    // Replacement recommendation
    Replacement string `json:"replacement,omitempty"`
    
    // Migration guide
    Migration string `json:"migration,omitempty"`
    
    // Reason for deprecation
    Reason string `json:"reason,omitempty"`
}
```

**Fields**:

- `mode`: Compatibility guarantee level
- `previous_versions`: Schema version lineage for compatibility tracking
- `breaking_changes`: List of breaking changes from previous version
- `deprecations`: Deprecated fields/endpoints with migration guidance

### 4.9 ProtocolMetadata

```go
type ProtocolMetadata struct {
    // GraphQL-specific metadata
    GraphQL *GraphQLMetadata `json:"graphql,omitempty"`
    
    // gRPC-specific metadata
    GRPC *GRPCMetadata `json:"grpc,omitempty"`
    
    // OpenAPI-specific metadata
    OpenAPI *OpenAPIMetadata `json:"openapi,omitempty"`
    
    // AsyncAPI-specific metadata
    AsyncAPI *AsyncAPIMetadata `json:"asyncapi,omitempty"`
    
    // oRPC-specific metadata
    ORPC *ORPCMetadata `json:"orpc,omitempty"`
}

type GraphQLMetadata struct {
    // Federation configuration
    Federation *GraphQLFederation `json:"federation,omitempty"`
    
    // Subscription support
    SubscriptionsEnabled bool `json:"subscriptions_enabled,omitempty"`
    
    // Subscription protocol
    SubscriptionProtocol string `json:"subscription_protocol,omitempty"` // "graphql-ws", "graphql-transport-ws"
    
    // Query complexity limits
    ComplexityLimit int `json:"complexity_limit,omitempty"`
    
    // Query depth limits
    DepthLimit int `json:"depth_limit,omitempty"`
}

type GraphQLFederation struct {
    // Federation version
    Version string `json:"version"` // "v1", "v2"
    
    // Subgraph name
    SubgraphName string `json:"subgraph_name"`
    
    // Entity types owned by this service
    Entities []FederatedEntity `json:"entities,omitempty"`
    
    // External types from other services
    Extends []string `json:"extends,omitempty"`
    
    // Provides relationships
    Provides []ProvidesRelation `json:"provides,omitempty"`
    
    // Requires relationships
    Requires []RequiresRelation `json:"requires,omitempty"`
}

type FederatedEntity struct {
    // Type name
    TypeName string `json:"type_name"`
    
    // Key fields for entity resolution
    KeyFields []string `json:"key_fields"`
    
    // Fields owned by this service
    Fields []string `json:"fields"`
    
    // Resolvable via this subgraph
    Resolvable bool `json:"resolvable"`
}

type ProvidesRelation struct {
    // Field that provides the relation
    Field string `json:"field"`
    
    // Fields provided
    Fields []string `json:"fields"`
}

type RequiresRelation struct {
    // Field that requires data
    Field string `json:"field"`
    
    // Required fields
    Fields []string `json:"fields"`
}

type GRPCMetadata struct {
    // Reflection enabled
    ReflectionEnabled bool `json:"reflection_enabled"`
    
    // Package names
    Packages []string `json:"packages"`
    
    // Service names
    Services []string `json:"services"`
    
    // gRPC-Web support
    GRPCWebEnabled bool `json:"grpc_web_enabled,omitempty"`
    
    // gRPC-Web protocol
    GRPCWebProtocol string `json:"grpc_web_protocol,omitempty"` // "grpc-web", "grpc-web-text"
    
    // Server streaming support
    ServerStreamingEnabled bool `json:"server_streaming_enabled,omitempty"`
    
    // Client streaming support
    ClientStreamingEnabled bool `json:"client_streaming_enabled,omitempty"`
    
    // Bidirectional streaming support
    BidirectionalStreamingEnabled bool `json:"bidirectional_streaming_enabled,omitempty"`
}

type OpenAPIMetadata struct {
    // x-extension fields to preserve
    Extensions map[string]interface{} `json:"extensions,omitempty"`
    
    // Server variables for URL templating
    ServerVariables map[string]ServerVariable `json:"server_variables,omitempty"`
    
    // Default security schemes
    DefaultSecurity []string `json:"default_security,omitempty"`
    
    // Composition settings for schema merging
    Composition *CompositionConfig `json:"composition,omitempty"`
}

type CompositionConfig struct {
    // Include this schema in merged/federated API documentation
    IncludeInMerged bool `json:"include_in_merged"`
    
    // Prefix for component schemas to avoid naming conflicts
    // If empty, defaults to service name
    ComponentPrefix string `json:"component_prefix,omitempty"`
    
    // Tag prefix for operation tags
    TagPrefix string `json:"tag_prefix,omitempty"`
    
    // Operation ID prefix to avoid conflicts
    OperationIDPrefix string `json:"operation_id_prefix,omitempty"`
    
    // Conflict resolution strategy when paths/components collide
    ConflictStrategy ConflictStrategy `json:"conflict_strategy"`
    
    // Whether to preserve x-extensions from this schema
    PreserveExtensions bool `json:"preserve_extensions"`
    
    // Custom servers to use in merged spec (overrides service defaults)
    CustomServers []OpenAPIServer `json:"custom_servers,omitempty"`
}

// ConflictStrategy defines how to handle conflicts during composition
type ConflictStrategy string

const (
    // ConflictStrategyPrefix adds service prefix to conflicting items
    ConflictStrategyPrefix ConflictStrategy = "prefix"
    
    // ConflictStrategyError fails composition on conflicts
    ConflictStrategyError ConflictStrategy = "error"
    
    // ConflictStrategySkip skips conflicting items from this service
    ConflictStrategySkip ConflictStrategy = "skip"
    
    // ConflictStrategyOverwrite overwrites existing with this service's version
    ConflictStrategyOverwrite ConflictStrategy = "overwrite"
    
    // ConflictStrategyMerge attempts to merge conflicting schemas
    ConflictStrategyMerge ConflictStrategy = "merge"
)

type OpenAPIServer struct {
    // Server URL
    URL string `json:"url"`
    
    // Server description
    Description string `json:"description,omitempty"`
    
    // Server variables
    Variables map[string]ServerVariable `json:"variables,omitempty"`
}

type ServerVariable struct {
    // Default value
    Default string `json:"default"`
    
    // Enum values
    Enum []string `json:"enum,omitempty"`
    
    // Description
    Description string `json:"description,omitempty"`
}

type AsyncAPIMetadata struct {
    // Message broker type
    Protocol string `json:"protocol"` // "kafka", "amqp", "mqtt", "ws"
    
    // Channel bindings
    ChannelBindings map[string]interface{} `json:"channel_bindings,omitempty"`
    
    // Message bindings
    MessageBindings map[string]interface{} `json:"message_bindings,omitempty"`
}

type ORPCMetadata struct {
    // Batch operations supported
    BatchEnabled bool `json:"batch_enabled,omitempty"`
    
    // Streaming procedures
    StreamingProcedures []string `json:"streaming_procedures,omitempty"`
}
```

**Fields**:

- Protocol-specific metadata for intelligent gateway configuration
- Each protocol has specialized metadata for federation, streaming, etc.

### 4.10 ServiceHints

```go
type ServiceHints struct {
    // Recommended timeout for operations
    RecommendedTimeout string `json:"recommended_timeout,omitempty"`
    
    // Expected latency profile
    ExpectedLatency *LatencyProfile `json:"expected_latency,omitempty"`
    
    // Scaling characteristics
    Scaling *ScalingProfile `json:"scaling,omitempty"`
    
    // Dependencies on other services
    Dependencies []ServiceDependency `json:"dependencies,omitempty"`
}

type LatencyProfile struct {
    // Median latency
    P50 string `json:"p50,omitempty"` // e.g., "10ms"
    
    // 95th percentile
    P95 string `json:"p95,omitempty"` // e.g., "50ms"
    
    // 99th percentile
    P99 string `json:"p99,omitempty"` // e.g., "100ms"
    
    // 99.9th percentile
    P999 string `json:"p999,omitempty"` // e.g., "500ms"
}

type ScalingProfile struct {
    // Auto-scaling enabled
    AutoScale bool `json:"auto_scale"`
    
    // Minimum instances
    MinInstances int `json:"min_instances,omitempty"`
    
    // Maximum instances
    MaxInstances int `json:"max_instances,omitempty"`
    
    // Target CPU utilization
    TargetCPU float64 `json:"target_cpu,omitempty"` // 0.0-1.0
    
    // Target memory utilization
    TargetMemory float64 `json:"target_memory,omitempty"` // 0.0-1.0
}

type ServiceDependency struct {
    // Service name
    ServiceName string `json:"service_name"`
    
    // Schema type
    SchemaType SchemaType `json:"schema_type"`
    
    // Version requirement (semver range)
    VersionRange string `json:"version_range,omitempty"`
    
    // Is dependency critical for operation?
    Critical bool `json:"critical"`
    
    // Types/operations used from dependency
    UsedOperations []string `json:"used_operations,omitempty"`
}
```

**Fields**:

- `recommended_timeout`: Service's recommended timeout (gateway may override)
- `expected_latency`: Expected latency percentiles for capacity planning
- `scaling`: Scaling characteristics for gateway load balancing
- `dependencies`: Other services this service depends on

### 4.11 InstanceMetadata

```go
type InstanceMetadata struct {
    // Instance address (host:port)
    Address string `json:"address"`
    
    // Instance region/zone/datacenter
    Region string `json:"region,omitempty"`
    Zone   string `json:"zone,omitempty"`
    
    // Instance labels for selection
    Labels map[string]string `json:"labels,omitempty"`
    
    // Instance weight for load balancing (0-100, default: 100)
    Weight int `json:"weight,omitempty"`
    
    // Instance status
    Status InstanceStatus `json:"status"`
    
    // Instance role in deployment
    Role InstanceRole `json:"role,omitempty"`
    
    // Canary/blue-green deployment metadata
    Deployment *DeploymentMetadata `json:"deployment,omitempty"`
    
    // Instance start time
    StartedAt int64 `json:"started_at"` // Unix timestamp
    
    // Expected schema checksum (for validation)
    ExpectedSchemaChecksum string `json:"expected_schema_checksum,omitempty"`
}

type InstanceStatus string

const (
    InstanceStatusStarting InstanceStatus = "starting"
    InstanceStatusHealthy  InstanceStatus = "healthy"
    InstanceStatusDegraded InstanceStatus = "degraded"
    InstanceStatusUnhealthy InstanceStatus = "unhealthy"
    InstanceStatusDraining InstanceStatus = "draining"
    InstanceStatusStopping InstanceStatus = "stopping"
)

type InstanceRole string

const (
    InstanceRolePrimary InstanceRole = "primary"
    InstanceRoleCanary  InstanceRole = "canary"
    InstanceRoleBlue    InstanceRole = "blue"
    InstanceRoleGreen   InstanceRole = "green"
    InstanceRoleShadow  InstanceRole = "shadow"
)

type DeploymentMetadata struct {
    // Deployment ID
    DeploymentID string `json:"deployment_id"`
    
    // Deployment strategy
    Strategy DeploymentStrategy `json:"strategy"`
    
    // Traffic percentage (0-100)
    TrafficPercent int `json:"traffic_percent,omitempty"`
    
    // Deployment stage
    Stage string `json:"stage,omitempty"` // "canary", "rollout", "stable"
    
    // Deployment time
    DeployedAt int64 `json:"deployed_at"`
}

type DeploymentStrategy string

const (
    DeploymentStrategyRolling    DeploymentStrategy = "rolling"
    DeploymentStrategyCanary     DeploymentStrategy = "canary"
    DeploymentStrategyBlueGreen  DeploymentStrategy = "blue_green"
    DeploymentStrategyShadow     DeploymentStrategy = "shadow"
    DeploymentStrategyRecreate   DeploymentStrategy = "recreate"
)
```

**Fields**:

- `address`: Instance network address (host:port)
- `region`/`zone`: Geographic location for region-aware routing
- `labels`: Key-value labels for instance selection (e.g., `{"env": "prod", "team": "payments"}`)
- `weight`: Load balancing weight (0=no traffic, 100=full weight)
- `status`: Current instance health status
- `role`: Instance role in deployment (primary, canary, blue, green, shadow)
- `deployment`: Deployment metadata for canary/blue-green strategies
- `started_at`: Instance start timestamp
- `expected_schema_checksum`: Expected schema checksum for validation across instances

### 4.12 RouteMetadata

```go
type RouteMetadata struct {
    // Operation/route identifier
    OperationID string `json:"operation_id"`
    
    // Path pattern
    Path string `json:"path"`
    
    // HTTP method (for REST/OpenAPI)
    Method string `json:"method,omitempty"`
    
    // Is operation idempotent?
    Idempotent bool `json:"idempotent"`
    
    // Recommended timeout for this operation
    TimeoutHint string `json:"timeout_hint,omitempty"`
    
    // Operation cost/complexity (1-10 scale)
    Cost int `json:"cost,omitempty"`
    
    // Is result cacheable?
    Cacheable bool `json:"cacheable,omitempty"`
    
    // Cache TTL hint
    CacheTTL string `json:"cache_ttl,omitempty"`
    
    // Data sensitivity level
    Sensitivity DataSensitivity `json:"sensitivity,omitempty"`
    
    // Expected response size
    ResponseSize SizeHint `json:"response_size,omitempty"`
    
    // Rate limit hint (requests per second)
    RateLimitHint int `json:"rate_limit_hint,omitempty"`
}

type DataSensitivity string

const (
    SensitivityPublic       DataSensitivity = "public"
    SensitivityInternal     DataSensitivity = "internal"
    SensitivityConfidential DataSensitivity = "confidential"
    SensitivityPII          DataSensitivity = "pii"
    SensitivityPHI          DataSensitivity = "phi"
    SensitivityPCI          DataSensitivity = "pci"
)

type SizeHint string

const (
    SizeSmall  SizeHint = "small"  // < 1KB
    SizeMedium SizeHint = "medium" // 1KB - 100KB
    SizeLarge  SizeHint = "large"  // 100KB - 1MB
    SizeXLarge SizeHint = "xlarge" // > 1MB
)
```

**Fields**:

- Per-operation metadata for gateway route configuration
- Hints for caching, timeouts, rate limiting, compliance

---

## 5. Schema Types

### 5.1 Supported Types

```go
type SchemaType string

const (
    SchemaTypeOpenAPI   SchemaType = "openapi"
    SchemaTypeAsyncAPI  SchemaType = "asyncapi"
    SchemaTypeGRPC      SchemaType = "grpc"
    SchemaTypeGraphQL   SchemaType = "graphql"
    SchemaTypeORPC      SchemaType = "orpc"      // OpenAPI-based RPC
    SchemaTypeThrift    SchemaType = "thrift"    // Future
    SchemaTypeAvro      SchemaType = "avro"      // Future
    SchemaTypeCustom    SchemaType = "custom"    // Extensibility
)
```

### 5.2 OpenAPI

- **Type**: `"openapi"`
- **Spec Versions**: `"3.0.0"`, `"3.0.1"`, `"3.1.0"` (recommended)
- **Content Type**: `"application/json"` or `"application/yaml"`
- **Use Case**: REST APIs with request/response schemas
- **Provider**: Forge router auto-generates OpenAPI 3.1.0

### 5.3 AsyncAPI

- **Type**: `"asyncapi"`
- **Spec Versions**: `"2.6.0"`, `"3.0.0"` (recommended)
- **Content Type**: `"application/json"` or `"application/yaml"`
- **Use Case**: WebSocket, SSE, message queues, event-driven APIs
- **Provider**: Forge router auto-generates AsyncAPI 3.0.0 for streaming routes

### 5.4 gRPC

- **Type**: `"grpc"`
- **Spec Versions**: Protocol Buffer version (e.g., `"proto3"`)
- **Content Type**: `"application/x-protobuf"` or `"application/json"` (for FileDescriptorSet)
- **Use Case**: High-performance RPC services
- **Provider**: Extract from `.proto` files or gRPC server reflection

### 5.5 GraphQL

- **Type**: `"graphql"`
- **Spec Versions**: GraphQL spec version (e.g., `"2021"`, `"2018"`)
- **Content Type**: `"application/graphql"` (SDL format) or `"application/json"` (introspection query result)
- **Use Case**: Query-based APIs with flexible data fetching
- **Provider**: Extract GraphQL SDL via introspection query or parse SDL files
- **Formats**:
  - **SDL (Schema Definition Language)**: Human-readable text format
  - **Introspection**: JSON format from GraphQL introspection query

**Example SDL Format**:
```json
{
  "format": "SDL",
  "sdl": "type Query { user(id: ID!): User }",
  "spec_version": "2021"
}
```

**Example Introspection Format**:
```json
{
  "format": "introspection",
  "data": {
    "__schema": {
      "types": [...],
      "queryType": {...}
    }
  },
  "spec_version": "2021"
}
```

### 5.6 oRPC

- **Type**: `"orpc"`
- **Spec Versions**: oRPC specification version (e.g., `"1.0.0"`)
- **Content Type**: `"application/json"`
- **Use Case**: RPC-style APIs with OpenAPI conventions
- **Provider**: Auto-generate from RPC handlers or extract from application routes
- **Features**:
  - Procedure-based API design
  - Typed input/output schemas
  - Built-in error handling conventions
  - Batch operation support
  - Transport configuration (HTTP, encoding)

**Example oRPC Schema**:
```json
{
  "orpc": "1.0.0",
  "info": {
    "title": "User Service",
    "version": "1.0.0"
  },
  "procedures": {
    "getUser": {
      "summary": "Get user by ID",
      "input": {
        "type": "object",
        "properties": {
          "id": { "type": "string" }
        },
        "required": ["id"]
      },
      "output": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "name": { "type": "string" }
        }
      }
    }
  },
  "transport": {
    "protocol": "http",
    "endpoint": "/rpc",
    "encoding": "json"
  }
}
```

### 5.7 Thrift

- **Type**: `"thrift"`
- **Spec Versions**: Thrift version (e.g., `"0.19.0"`, `"0.18.0"`)
- **Content Type**: `"application/json"` (JSON representation of Thrift IDL)
- **Use Case**: Cross-language RPC services with IDL-based contracts
- **Provider**: Parse .thrift IDL files or generate from application services
- **Features**:
  - Strongly-typed service definitions
  - Multiple protocol support (binary, compact, JSON)
  - Code generation for multiple languages
  - Support for structs, enums, exceptions, services

**Example Thrift Schema**:
```json
{
  "thrift_version": "0.19.0",
  "format": "idl",
  "namespaces": {
    "go": "user_service",
    "java": "com.example.userservice"
  },
  "services": [
    {
      "name": "UserService",
      "functions": [
        {
          "name": "getUser",
          "return_type": "User",
          "arguments": [
            { "id": 1, "name": "userId", "type": "string" }
          ]
        }
      ]
    }
  ],
  "structs": [
    {
      "name": "User",
      "fields": [
        { "id": 1, "name": "id", "type": "string", "required": true },
        { "id": 2, "name": "name", "type": "string", "required": true },
        { "id": 3, "name": "email", "type": "string", "required": false }
      ]
    }
  ],
  "enums": []
}
```

### 5.8 Avro

- **Type**: `"avro"`
- **Spec Versions**: Avro specification version (e.g., `"1.11.1"`, `"1.10.0"`)
- **Content Type**: `"application/json"` (Avro Protocol or Schema)
- **Use Case**: Data serialization for streaming systems, Kafka, Hadoop
- **Provider**: Parse .avsc schema files or generate from application data structures
- **Features**:
  - Compact binary serialization
  - Schema evolution support
  - Rich data types (records, arrays, maps, unions)
  - Integration with big data ecosystems

**Example Avro Schema**:
```json
{
  "avro_version": "1.11.1",
  "protocol": "UserProtocol",
  "namespace": "com.example.user",
  "doc": "Avro protocol for user service",
  "types": [
    {
      "type": "record",
      "name": "User",
      "doc": "User record",
      "fields": [
        { "name": "id", "type": "string" },
        { "name": "name", "type": "string" },
        { "name": "email", "type": ["null", "string"], "default": null },
        { "name": "created_at", "type": "long" }
      ]
    }
  ],
  "messages": {
    "getUser": {
      "doc": "Get user by ID",
      "request": [
        { "name": "userId", "type": "string" }
      ],
      "response": "User"
    }
  }
}
```

### 5.9 Custom

- **Type**: `"custom"`
- **Spec Versions**: Implementation-defined
- **Content Type**: Implementation-defined
- **Use Case**: Proprietary or emerging protocols
- **Provider**: Custom implementation

---

## 6. Location Strategies

### 6.1 HTTP

**Description**: Gateway fetches schema via HTTP GET request.

**Advantages**:
- Simple, no backend storage required
- Service controls schema freshness
- Works with any HTTP server

**Disadvantages**:
- Requires gateway to access service network
- Adds latency to route discovery
- Service must remain available for schema fetching

**Example**:

```json
{
  "type": "http",
  "url": "http://user-service:8080/openapi.json",
  "headers": {
    "Authorization": "Bearer gateway-token"
  }
}
```

### 6.2 Registry

**Description**: Schema stored in backend KV store (Consul, etcd).

**Advantages**:
- Decoupled from service availability
- Centralized schema storage
- Faster gateway startup (no service polling)

**Disadvantages**:
- Requires backend storage setup
- Schema size limits (typically 512KB in Consul)
- Additional write operations on schema updates

**Example**:

```json
{
  "type": "registry",
  "registry_path": "/schemas/user-service/v1/openapi"
}
```

**Storage Format** (Consul KV):

```
Key:   /schemas/user-service/v1/openapi
Value: <OpenAPI JSON content>
```

### 6.3 Inline

**Description**: Schema embedded directly in manifest.

**Advantages**:
- Single fetch operation
- No separate storage or HTTP calls
- Fastest gateway startup

**Disadvantages**:
- Bloats manifest size
- Only suitable for small schemas (< 100KB recommended)
- Increases network bandwidth for large schemas

**Example**:

```json
{
  "type": "inline",
  "inline_schema": {
    "openapi": "3.1.0",
    "info": {
      "title": "User Service",
      "version": "1.0.0"
    },
    "paths": { ... }
  }
}
```

---

## 7. Route Mounting Strategies

### 7.1 Overview

When a gateway federates multiple service APIs into a single unified API, it must decide how to mount each service's routes. FARP provides flexible mounting strategies to handle different architectural patterns.

### 7.2 Strategy Types

#### 7.2.1 Root Mounting (`root`)

**Description**: Service routes merged directly to gateway root with no prefix.

**Use Case**:
- Single service per gateway
- BFF (Backend for Frontend) pattern
- API composition where services own distinct path spaces

**Pros**:
- Clean URLs for clients
- No path rewriting needed

**Cons**:
- Path conflicts between services
- Requires careful route planning

**Example**:
```json
{
  "routing": {
    "strategy": "root",
    "priority": 50
  }
}
```

**Result**:
```
Service Path: /users
Gateway Path: /users

Service Path: /users/:id
Gateway Path: /users/:id
```

#### 7.2.2 Instance Mounting (`instance`) **[DEFAULT]**

**Description**: Mount under instance ID prefix.

**Use Case**:
- Testing/debugging specific instances
- Canary deployments
- Multi-tenancy with instance isolation

**Pros**:
- No conflicts (instance IDs are unique)
- Direct instance targeting
- Automatic conflict resolution

**Cons**:
- URLs include random instance IDs
- Not user-friendly
- Clients need service discovery

**Example**:
```json
{
  "routing": {
    "strategy": "instance"
  }
}
```

**Result**:
```
Service Path: /users
Gateway Path: /user-service-abc123/users

Service Path: /users/:id
Gateway Path: /user-service-abc123/users/:id
```

#### 7.2.3 Service Mounting (`service`)

**Description**: Mount under service name prefix.

**Use Case**:
- Multi-service gateways
- Service-oriented APIs
- Microservice federations

**Pros**:
- Stable, predictable URLs
- Service-level load balancing
- Clear service boundaries

**Cons**:
- Additional path segment
- Gateway handles load balancing across instances

**Example**:
```json
{
  "routing": {
    "strategy": "service"
  }
}
```

**Result**:
```
Service Path: /users
Gateway Path: /user-service/users

Service Path: /users/:id
Gateway Path: /user-service/users/:id
```

#### 7.2.4 Versioned Mounting (`versioned`)

**Description**: Mount under service name + version prefix.

**Use Case**:
- API versioning in URL
- Multiple API versions simultaneously
- Zero-downtime migrations

**Pros**:
- Explicit versioning
- Side-by-side version support
- Gradual migration

**Cons**:
- Longer URLs
- Version proliferation

**Example**:
```json
{
  "routing": {
    "strategy": "versioned"
  }
}
```

**Result**:
```
Service Path: /users
Gateway Path: /user-service/v1/users

Service Path: /users/:id
Gateway Path: /user-service/v1/users/:id
```

#### 7.2.5 Custom Mounting (`custom`)

**Description**: Mount under custom base path.

**Use Case**:
- Custom API structures
- Domain-specific routing
- Legacy path compatibility

**Pros**:
- Full control
- Business-aligned URLs
- Legacy system compatibility

**Cons**:
- Manual configuration required
- Potential conflicts

**Example**:
```json
{
  "routing": {
    "strategy": "custom",
    "base_path": "/api/v2/identity"
  }
}
```

**Result**:
```
Service Path: /users
Gateway Path: /api/v2/identity/users

Service Path: /users/:id
Gateway Path: /api/v2/identity/users/:id
```

#### 7.2.6 Subdomain Mounting (`subdomain`)

**Description**: Mount on separate subdomain.

**Use Case**:
- Large-scale federations
- Service isolation
- DNS-based routing

**Pros**:
- Clean path space per service
- No prefix required
- Scales to many services

**Cons**:
- DNS configuration required
- TLS certificate management
- Clients need subdomain support

**Example**:
```json
{
  "routing": {
    "strategy": "subdomain",
    "subdomain": "users"
  }
}
```

**Result**:
```
Service Path: /users
Gateway Path: users.gateway.com/users

Service Path: /users/:id
Gateway Path: users.gateway.com/users/:id
```

### 7.3 Path Rewriting

Services can define path transformation rules:

```json
{
  "routing": {
    "strategy": "service",
    "rewrite": [
      {
        "pattern": "^/v1/(.*)$",
        "replacement": "/$1"
      },
      {
        "pattern": "^/internal/(.*)$",
        "replacement": "/api/$1"
      }
    ],
    "strip_prefix": true
  }
}
```

**Behavior**:
1. Gateway receives: `/user-service/v1/users/123`
2. Apply rewrite: `/user-service/users/123`
3. Strip prefix: `/users/123`
4. Forward to service: `GET /users/123`

### 7.4 Conflict Resolution

When multiple services define overlapping routes, the gateway uses priority-based resolution:

```json
// Service A: Low priority
{
  "service_name": "legacy-api",
  "routing": {
    "strategy": "root",
    "priority": 30
  }
}

// Service B: High priority (wins conflicts)
{
  "service_name": "new-api",
  "routing": {
    "strategy": "root",
    "priority": 70
  }
}
```

**Conflict Resolution Rules**:
1. Higher priority wins (0-100 scale, default: 50)
2. If priority equal, service version wins (semver comparison)
3. If versions equal, first registered wins
4. Gateway logs all conflicts for audit

### 7.5 Route Tagging

Services can tag routes for gateway filtering:

```json
{
  "routing": {
    "strategy": "service",
    "tags": ["public", "rest", "v2"]
  }
}
```

**Gateway can filter**:
- Only public routes
- Only specific versions
- Only certain protocols

### 7.6 Implementation Example

**Service-Side Configuration**:
```go
manifest := &farp.SchemaManifest{
    ServiceName:    "user-service",
    ServiceVersion: "v1.2.0",
    InstanceID:     generateInstanceID(),
    Routing: farp.RoutingConfig{
        Strategy:    farp.MountStrategyService,
        StripPrefix: true,
        Priority:    60,
        Tags:        []string{"public", "rest"},
        Rewrite: []farp.PathRewrite{
            {Pattern: "^/api/(.*)$", Replacement: "/$1"},
        },
    },
}
```

**Gateway-Side Processing**:
```go
func (g *Gateway) mountService(manifest *farp.SchemaManifest) error {
    basePath := g.calculateBasePath(manifest.Routing)
    
    for _, schema := range manifest.Schemas {
        routes := g.extractRoutes(schema)
        
        for _, route := range routes {
            // Apply rewrite rules
            transformedPath := g.applyRewrites(route.Path, manifest.Routing.Rewrite)
            
            // Calculate gateway path
            gatewayPath := path.Join(basePath, transformedPath)
            
            // Check for conflicts
            if existing := g.routes.Get(gatewayPath); existing != nil {
                if manifest.Routing.Priority <= existing.Priority {
                    logger.Warn("route conflict, skipping",
                        "path", gatewayPath,
                        "service", manifest.ServiceName,
                        "priority", manifest.Routing.Priority,
                    )
                    continue
                }
            }
            
            // Register route
            g.routes.Register(gatewayPath, &Route{
                ServiceName: manifest.ServiceName,
                InstanceID:  manifest.InstanceID,
                OriginalPath: route.Path,
                StripPrefix:  manifest.Routing.StripPrefix,
                Priority:     manifest.Routing.Priority,
                Handler:      g.createProxyHandler(manifest, route),
            })
        }
    }
    
    return nil
}
```

---

## 8. Authentication & Authorization

### 8.1 Overview

FARP defines standard mechanisms for authentication and authorization between:
- **Client → Gateway**: End-user authentication
- **Gateway → Service**: Service-to-service authentication
- **Schema Access**: Who can fetch schemas/manifests

### 8.2 Authentication Schemes

#### 8.2.1 Bearer Token (JWT)

**Most Common**: Token-based authentication with JWT.

**Configuration**:
```json
{
  "auth": {
    "schemes": [
      {
        "type": "bearer",
        "config": {
          "format": "jwt",
          "jwks_url": "https://auth.example.com/.well-known/jwks.json",
          "issuer": "https://auth.example.com",
          "audience": "user-service-api"
        }
      }
    ],
    "required_scopes": ["users:read", "users:write"]
  }
}
```

**Gateway Behavior**:
1. Extract `Authorization: Bearer <token>` header
2. Validate JWT signature using JWKS
3. Verify issuer, audience, expiration
4. Extract scopes/claims
5. Check against `required_scopes`
6. Forward token to service (or exchange for service token)

#### 8.2.2 API Key

**Configuration**:
```json
{
  "auth": {
    "schemes": [
      {
        "type": "apikey",
        "config": {
          "in": "header",
          "name": "X-API-Key",
          "validation_url": "https://auth.example.com/validate"
        }
      }
    ]
  }
}
```

**Gateway Behavior**:
1. Extract API key from header/query
2. Validate via `validation_url` or local cache
3. Attach client identity to request context
4. Forward or transform key for service

#### 8.2.3 Mutual TLS (mTLS)

**Configuration**:
```json
{
  "auth": {
    "schemes": [
      {
        "type": "mtls",
        "config": {
          "ca_cert": "/etc/pki/ca.crt",
          "verify_client": true,
          "allowed_dns_names": ["*.internal.example.com"]
        }
      }
    ]
  }
}
```

**Gateway Behavior**:
1. Establish TLS connection with client cert verification
2. Extract client identity from certificate CN/SAN
3. Verify against allowed DNS names
4. Forward client cert info to service

#### 8.2.4 OAuth 2.0 / OIDC

**Configuration**:
```json
{
  "auth": {
    "schemes": [
      {
        "type": "oidc",
        "config": {
          "issuer": "https://auth.example.com",
          "client_id": "user-service",
          "discovery_url": "https://auth.example.com/.well-known/openid-configuration"
        }
      }
    ],
    "required_scopes": ["openid", "profile", "email"]
  }
}
```

### 8.3 Access Control Rules

**Fine-grained authorization**:

```json
{
  "auth": {
    "schemes": [{"type": "bearer", "config": {"format": "jwt"}}],
    "access_control": [
      {
        "path": "/users",
        "methods": ["GET"],
        "roles": ["user", "admin"],
        "allow_anonymous": false
      },
      {
        "path": "/users/*",
        "methods": ["POST", "PUT", "DELETE"],
        "roles": ["admin"],
        "permissions": ["users:write"]
      },
      {
        "path": "/health",
        "methods": ["GET"],
        "allow_anonymous": true
      }
    ],
    "public_routes": ["/health", "/metrics", "/openapi.json"]
  }
}
```

**Gateway Enforcement**:
```go
func (g *Gateway) enforceAccessControl(req *http.Request, rule *AccessRule) error {
    // Allow anonymous if configured
    if rule.AllowAnonymous {
        return nil
    }
    
    // Extract claims from token
    claims := req.Context().Value("jwt_claims").(map[string]interface{})
    
    // Check roles
    if len(rule.Roles) > 0 {
        userRoles := claims["roles"].([]string)
        if !hasAnyRole(userRoles, rule.Roles) {
            return ErrForbidden
        }
    }
    
    // Check permissions
    if len(rule.Permissions) > 0 {
        userPerms := claims["permissions"].([]string)
        if !hasAllPermissions(userPerms, rule.Permissions) {
            return ErrForbidden
        }
    }
    
    return nil
}
```

### 8.4 Token Validation

Services can provide token validation endpoint for custom logic:

```json
{
  "auth": {
    "token_validation_url": "https://user-service:8080/validate-token"
  }
}
```

**Request**:
```http
POST /validate-token HTTP/1.1
Content-Type: application/json

{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "method": "GET",
  "path": "/users/123"
}
```

**Response**:
```json
{
  "valid": true,
  "claims": {
    "sub": "user-123",
    "roles": ["user"],
    "permissions": ["users:read"]
  }
}
```

### 8.5 Gateway → Service Authentication

Gateway authenticates to services using separate credentials:

```json
{
  "auth": {
    "gateway_to_service": {
      "type": "mtls",
      "config": {
        "client_cert": "/etc/gateway/client.crt",
        "client_key": "/etc/gateway/client.key"
      }
    }
  }
}
```

**Pattern: Token Exchange**:
1. Client sends token to gateway
2. Gateway validates client token
3. Gateway exchanges for service token (OAuth2 token exchange RFC 8693)
4. Gateway calls service with service token
5. Service validates service token

### 8.6 Schema Access Control

Control who can fetch schemas and manifests:

**Backend ACL (Consul)**:
```hcl
# Services can write their own schemas
service "user-service" {
  policy = "write"
}

key_prefix "/services/user-service/schemas/" {
  policy = "write"
}

# Gateways can read all schemas
key_prefix "/services/" {
  policy = "read"
}
```

**HTTP Schema Endpoints**:
```go
// Protect schema endpoints with authentication
func (s *Service) registerSchemaEndpoints() {
    s.router.GET("/openapi.json", s.authMiddleware("schema:read"), s.handleOpenAPI)
    s.router.GET("/_farp/manifest", s.authMiddleware("schema:read"), s.handleManifest)
}
```

### 8.7 Security Best Practices

1. **Always use TLS**: HTTPS for HTTP schemas, TLS for service discovery
2. **Rotate secrets**: Regular rotation of API keys, certificates
3. **Least privilege**: Grant minimum required permissions
4. **Audit logging**: Log all auth failures and schema access
5. **Rate limiting**: Prevent brute force attacks
6. **Token expiration**: Short-lived tokens (15 min recommended)
7. **mTLS when possible**: Strongest authentication for service mesh

---

## 9. Bidirectional Communication

### 9.1 Overview

FARP supports bidirectional push notifications between gateway and service for real-time updates, eliminating polling overhead.

### 9.2 Communication Patterns

#### 9.2.1 Service → Gateway (Push)

**Use Cases**:
- Schema updated (hot reload)
- Health status changed
- Instance scaling events
- Maintenance mode activation

**Configuration**:
```json
{
  "webhook": {
    "gateway_webhook": "https://gateway.example.com/farp/webhook",
    "secret": "shared-hmac-secret",
    "publish_events": [
      "schema.updated",
      "health.changed",
      "maintenance.mode"
    ],
    "retry": {
      "max_attempts": 3,
      "initial_delay": "1s",
      "max_delay": "30s",
      "multiplier": 2.0
    }
  }
}
```

#### 9.2.2 Gateway → Service (Push)

**Use Cases**:
- Rate limit changes
- Circuit breaker state
- Configuration updates
- Traffic shift notifications

**Configuration**:
```json
{
  "webhook": {
    "service_webhook": "https://user-service:8080/farp/webhook",
    "secret": "shared-hmac-secret",
    "subscribe_events": [
      "ratelimit.changed",
      "circuit.breaker.open",
      "circuit.breaker.closed",
      "traffic.shift"
    ]
  }
}
```

### 9.3 Webhook Protocol

#### 9.3.1 Request Format

```http
POST /farp/webhook HTTP/1.1
Host: gateway.example.com
Content-Type: application/json
X-FARP-Signature: sha256=a1b2c3d4...
X-FARP-Event: schema.updated
X-FARP-Delivery: uuid-1234-5678

{
  "event": "schema.updated",
  "timestamp": 1698768000,
  "source": {
    "service_name": "user-service",
    "instance_id": "user-service-abc123"
  },
  "data": {
    "schema_type": "openapi",
    "checksum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "manifest_url": "https://user-service:8080/_farp/manifest"
  }
}
```

#### 9.3.2 Signature Verification

**HMAC-SHA256 Signature**:

```go
func verifyWebhookSignature(payload []byte, signature string, secret string) bool {
    // Signature format: "sha256=<hex>"
    if !strings.HasPrefix(signature, "sha256=") {
        return false
    }
    
    expectedSig := strings.TrimPrefix(signature, "sha256=")
    
    // Calculate HMAC
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(payload)
    actualSig := hex.EncodeToString(mac.Sum(nil))
    
    // Constant-time comparison
    return hmac.Equal([]byte(expectedSig), []byte(actualSig))
}
```

**Gateway Webhook Handler**:
```go
func (g *Gateway) handleFARPWebhook(w http.ResponseWriter, r *http.Request) {
    // Read body
    body, _ := io.ReadAll(r.Body)
    
    // Get service instance
    instanceID := r.Header.Get("X-FARP-Source-Instance")
    manifest, _ := g.registry.GetManifest(r.Context(), instanceID)
    
    // Verify signature
    signature := r.Header.Get("X-FARP-Signature")
    if !verifyWebhookSignature(body, signature, manifest.Webhook.Secret) {
        http.Error(w, "invalid signature", http.StatusUnauthorized)
        return
    }
    
    // Parse event
    var event WebhookEvent
    json.Unmarshal(body, &event)
    
    // Process event
    switch event.Event {
    case "schema.updated":
        g.handleSchemaUpdate(event)
    case "health.changed":
        g.handleHealthChange(event)
    case "maintenance.mode":
        g.handleMaintenanceMode(event)
    }
    
    w.WriteHeader(http.StatusOK)
}
```

### 9.4 Event Types

#### 9.4.1 schema.updated

**Direction**: Service → Gateway

**Data**:
```json
{
  "event": "schema.updated",
  "timestamp": 1698768000,
  "source": {
    "service_name": "user-service",
    "instance_id": "abc123"
  },
  "data": {
    "schema_type": "openapi",
    "checksum": "new-checksum",
    "manifest_url": "https://service/manifest"
  }
}
```

**Gateway Action**: Fetch updated manifest and reconfigure routes

#### 9.4.2 health.changed

**Direction**: Service → Gateway

**Data**:
```json
{
  "event": "health.changed",
  "timestamp": 1698768000,
  "source": {
    "service_name": "user-service",
    "instance_id": "abc123"
  },
  "data": {
    "status": "unhealthy",
    "reason": "database_connection_failed"
  }
}
```

**Gateway Action**: Remove instance from load balancer

#### 9.4.3 circuit.breaker.open

**Direction**: Gateway → Service

**Data**:
```json
{
  "event": "circuit.breaker.open",
  "timestamp": 1698768000,
  "source": {
    "gateway_id": "gateway-xyz"
  },
  "data": {
    "reason": "error_rate_threshold_exceeded",
    "error_rate": 0.52,
    "threshold": 0.50,
    "window": "5m"
  }
}
```

**Service Action**: Log event, potentially reduce load, alert ops

#### 9.4.4 ratelimit.changed

**Direction**: Gateway → Service

**Data**:
```json
{
  "event": "ratelimit.changed",
  "timestamp": 1698768000,
  "source": {
    "gateway_id": "gateway-xyz"
  },
  "data": {
    "new_limit": 1000,
    "old_limit": 5000,
    "reason": "high_load_shedding"
  }
}
```

**Service Action**: Adjust internal rate limiters, back off non-critical operations

#### 9.4.5 traffic.shift

**Direction**: Gateway → Service

**Data**:
```json
{
  "event": "traffic.shift",
  "timestamp": 1698768000,
  "source": {
    "gateway_id": "gateway-xyz"
  },
  "data": {
    "instances": {
      "abc123": 0.90,
      "xyz789": 0.10
    },
    "reason": "canary_deployment"
  }
}
```

**Service Action**: Prepare for increased/decreased load

### 9.5 Retry Logic

**Exponential Backoff**:

```go
func (s *Service) sendWebhook(event *WebhookEvent, config WebhookConfig) error {
    var lastErr error
    
    for attempt := 0; attempt < config.Retry.MaxAttempts; attempt++ {
        // Calculate delay
        delay := calculateBackoff(
            attempt,
            config.Retry.InitialDelay,
            config.Retry.MaxDelay,
            config.Retry.Multiplier,
        )
        
        if attempt > 0 {
            time.Sleep(delay)
        }
        
        // Send webhook
        err := s.doSendWebhook(event, config)
        if err == nil {
            return nil // Success
        }
        
        // Check if retryable
        if !isRetryable(err) {
            return err
        }
        
        lastErr = err
        logger.Warn("webhook delivery failed, retrying",
            "attempt", attempt+1,
            "error", err,
        )
    }
    
    return fmt.Errorf("webhook delivery failed after %d attempts: %w",
        config.Retry.MaxAttempts, lastErr)
}

func calculateBackoff(attempt int, initial, max time.Duration, multiplier float64) time.Duration {
    delay := float64(initial) * math.Pow(multiplier, float64(attempt))
    if delay > float64(max) {
        delay = float64(max)
    }
    
    // Add jitter (±20%)
    jitter := delay * 0.2 * (rand.Float64()*2 - 1)
    return time.Duration(delay + jitter)
}
```

### 9.6 Delivery Guarantees

**At-Least-Once Delivery**:
- Sender retries on failure
- Receiver must handle duplicates (idempotency)
- Use `X-FARP-Delivery` header for deduplication

**Idempotency**:
```go
func (g *Gateway) handleFARPWebhook(w http.ResponseWriter, r *http.Request) {
    deliveryID := r.Header.Get("X-FARP-Delivery")
    
    // Check if already processed
    if g.processedDeliveries.Contains(deliveryID) {
        logger.Info("duplicate webhook delivery", "id", deliveryID)
        w.WriteHeader(http.StatusOK) // Acknowledge duplicate
        return
    }
    
    // Process event
    // ...
    
    // Mark as processed (with TTL)
    g.processedDeliveries.Add(deliveryID, time.Hour)
}
```

### 9.7 Webhook Security

1. **HMAC Signature**: Always verify webhook signatures
2. **HTTPS Only**: Require TLS for webhook endpoints
3. **IP Allowlisting**: Restrict webhook sources
4. **Rate Limiting**: Prevent webhook flooding
5. **Timeout**: Set reasonable timeouts (5-10s)
6. **Secret Rotation**: Rotate webhook secrets regularly

### 9.8 HTTP-Based Communication Routes

**Alternative to Push Webhooks**: Services can expose HTTP routes for gateway to poll or call for various purposes.

#### 9.8.1 Service Routes

**Service exposes routes for gateway to call**:

```json
{
  "webhook": {
    "http_routes": {
      "service_routes": [
        {
          "id": "event-poll",
          "path": "/farp/events",
          "method": "GET",
          "type": "event.poll",
          "description": "Poll for pending events",
          "response_schema": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "event_id": {"type": "string"},
                "event": {"type": "string"},
                "data": {"type": "object"}
              }
            }
          },
          "auth_required": true,
          "idempotent": true,
          "timeout": "5s"
        },
        {
          "id": "event-ack",
          "path": "/farp/events/{event_id}/ack",
          "method": "POST",
          "type": "event.ack",
          "description": "Acknowledge event processing",
          "auth_required": true,
          "idempotent": true,
          "timeout": "2s"
        },
        {
          "id": "lifecycle-reload",
          "path": "/admin/reload",
          "method": "POST",
          "type": "lifecycle.reload",
          "description": "Trigger configuration reload",
          "auth_required": true,
          "idempotent": true,
          "timeout": "10s"
        },
        {
          "id": "config-update",
          "path": "/admin/config",
          "method": "PUT",
          "type": "config.update",
          "description": "Update service configuration",
          "request_schema": {
            "type": "object",
            "properties": {
              "config": {"type": "object"}
            }
          },
          "auth_required": true,
          "idempotent": true,
          "timeout": "5s"
        }
      ],
      "polling": {
        "interval": "30s",
        "timeout": "5s",
        "long_polling": true,
        "long_polling_timeout": "60s"
      }
    }
  }
}
```

#### 9.8.2 Gateway Routes

**Gateway exposes routes for service to call**:

```json
{
  "webhook": {
    "http_routes": {
      "gateway_routes": [
        {
          "id": "schema-validate",
          "path": "/farp/schema/validate",
          "method": "POST",
          "type": "schema.validate",
          "description": "Validate schema before publishing",
          "request_schema": {
            "type": "object",
            "properties": {
              "schema_type": {"type": "string"},
              "schema": {"type": "object"}
            }
          },
          "response_schema": {
            "type": "object",
            "properties": {
              "valid": {"type": "boolean"},
              "errors": {"type": "array"}
            }
          },
          "auth_required": true,
          "idempotent": true,
          "timeout": "10s"
        },
        {
          "id": "config-query",
          "path": "/farp/config",
          "method": "GET",
          "type": "config.query",
          "description": "Query gateway configuration for this service",
          "auth_required": true,
          "idempotent": true,
          "timeout": "5s"
        }
      ]
    }
  }
}
```

#### 9.8.3 Event Polling Pattern

**Long Polling for Events**:

```go
// Gateway polls service for events
func (g *Gateway) pollServiceEvents(ctx context.Context, service *ServiceManifest) {
    route := findRoute(service.Webhook.HTTPRoutes.ServiceRoutes, "event.poll")
    if route == nil {
        return
    }
    
    for {
        // Long polling request
        ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
        defer cancel()
        
        req, _ := http.NewRequestWithContext(ctx, route.Method, 
            service.Address + route.Path, nil)
        
        // Add auth
        req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.serviceToken))
        
        resp, err := g.httpClient.Do(req)
        if err != nil {
            time.Sleep(30 * time.Second)
            continue
        }
        
        if resp.StatusCode == 200 {
            var events []Event
            json.NewDecoder(resp.Body).Decode(&events)
            
            // Process events
            for _, event := range events {
                g.processEvent(event)
                
                // Acknowledge event
                g.acknowledgeEvent(service, event.EventID)
            }
        }
        
        resp.Body.Close()
    }
}

func (g *Gateway) acknowledgeEvent(service *ServiceManifest, eventID string) {
    route := findRoute(service.Webhook.HTTPRoutes.ServiceRoutes, "event.ack")
    if route == nil {
        return
    }
    
    path := strings.Replace(route.Path, "{event_id}", eventID, 1)
    req, _ := http.NewRequest(route.Method, service.Address + path, nil)
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.serviceToken))
    
    g.httpClient.Do(req)
}
```

#### 9.8.4 Lifecycle Hooks

**Gateway can trigger service lifecycle operations**:

```go
// Gateway triggers service reload
func (g *Gateway) reloadService(service *ServiceManifest) error {
    route := findRoute(service.Webhook.HTTPRoutes.ServiceRoutes, "lifecycle.reload")
    if route == nil {
        return errors.New("reload not supported")
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(ctx, route.Method, 
        service.Address + route.Path, nil)
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.serviceToken))
    
    resp, err := g.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("reload failed: %d", resp.StatusCode)
    }
    
    return nil
}
```

#### 9.8.5 Configuration Management

**Gateway pushes configuration updates to service**:

```go
// Gateway updates service configuration
func (g *Gateway) updateServiceConfig(service *ServiceManifest, config map[string]interface{}) error {
    route := findRoute(service.Webhook.HTTPRoutes.ServiceRoutes, "config.update")
    if route == nil {
        return errors.New("config update not supported")
    }
    
    body, _ := json.Marshal(map[string]interface{}{
        "config": config,
    })
    
    req, _ := http.NewRequest(route.Method, 
        service.Address + route.Path, 
        bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.serviceToken))
    
    resp, err := g.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        return fmt.Errorf("config update failed: %d", resp.StatusCode)
    }
    
    return nil
}
```

#### 9.8.6 Schema Validation

**Service validates schema with gateway before publishing**:

```go
// Service validates schema before publishing
func (s *Service) validateSchemaWithGateway(schema interface{}) error {
    route := findRoute(s.manifest.Webhook.HTTPRoutes.GatewayRoutes, "schema.validate")
    if route == nil {
        return nil // Gateway doesn't support validation
    }
    
    body, _ := json.Marshal(map[string]interface{}{
        "schema_type": "openapi",
        "schema": schema,
    })
    
    req, _ := http.NewRequest(route.Method, 
        s.gatewayAddress + route.Path,
        bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.gatewayToken))
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    var result struct {
        Valid  bool     `json:"valid"`
        Errors []string `json:"errors"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    
    if !result.Valid {
        return fmt.Errorf("schema validation failed: %v", result.Errors)
    }
    
    return nil
}
```

### 9.9 Communication Pattern Comparison

| Pattern | Push Webhooks | HTTP Polling | HTTP Routes |
|---------|--------------|--------------|-------------|
| **Latency** | Low (instant) | Medium (polling interval) | Low (on-demand) |
| **Complexity** | Medium | Low | Medium |
| **Firewall Friendly** | No (requires inbound) | Yes (outbound only) | Yes (outbound only) |
| **Reliability** | Retry required | Inherent retry | Request/response |
| **Use Case** | Real-time events | Periodic checks | Request/response operations |
| **Best For** | Schema updates, health changes | Environments with firewall restrictions | Configuration, validation, control |

**Recommendation**:
- **Webhooks**: Use for real-time event notifications (schema updates, health changes)
- **HTTP Polling**: Use when webhooks are blocked by network/firewall
- **HTTP Routes**: Use for control plane operations (config updates, lifecycle, validation)
- **Hybrid**: Combine all three for maximum flexibility

### 9.10 Fallback Strategy

```go
func (g *Gateway) watchService(serviceName string) {
    manifest, _ := g.registry.GetManifest(ctx, serviceName)
    
    // Try webhooks first
    if manifest.Webhook.GatewayWebhook != "" {
        err := g.subscribeWebhook(manifest)
        if err == nil {
            logger.Info("webhook enabled for service", "service", serviceName)
            return
        }
        logger.Warn("webhook subscription failed, falling back to polling", "error", err)
    }
    
    // Fall back to HTTP polling
    if manifest.Webhook.HTTPRoutes != nil && 
       len(manifest.Webhook.HTTPRoutes.ServiceRoutes) > 0 {
        
        pollConfig := manifest.Webhook.HTTPRoutes.Polling
        if pollConfig != nil {
            interval, _ := time.ParseDuration(pollConfig.Interval)
            
            go func() {
                ticker := time.NewTicker(interval)
                defer ticker.Stop()
                
                for range ticker.C {
                    g.pollServiceEvents(ctx, manifest)
                }
            }()
            
            logger.Info("HTTP polling enabled for service", 
                "service", serviceName,
                "interval", pollConfig.Interval)
            return
        }
    }
    
    // Last resort: periodic manifest refresh
    logger.Warn("no bidirectional communication available, using periodic refresh",
        "service", serviceName)
    go g.periodicManifestRefresh(serviceName)
}
```

---

## 10. Multi-Instance Management

### 10.1 Overview

Production services typically run multiple instances for high availability, scalability, and zero-downtime deployments. FARP provides comprehensive metadata and patterns for managing multiple instances of the same service.

### 10.2 Instance Identity

#### 10.2.1 Instance ID Generation

**Requirements**:
- **Globally unique** across all instances
- **Stable** across restarts (for stateful services)
- **Deterministic** or **random** based on use case

**Recommended Formats**:

```go
// Format 1: service-name + hostname + random
instanceID := fmt.Sprintf("%s-%s-%s", 
    serviceName, 
    hostname, 
    randomString(8))
// Example: user-service-ip-10-0-1-5-a1b2c3d4

// Format 2: service-name + UUID
instanceID := fmt.Sprintf("%s-%s",
    serviceName,
    uuid.New().String())
// Example: user-service-550e8400-e29b-41d4-a716-446655440000

// Format 3: Kubernetes pod name (stable)
instanceID := os.Getenv("POD_NAME")
// Example: user-service-deployment-7d8f9b6c5-xk2p7
```

#### 10.2.2 Instance Metadata

**Complete Example**:

```json
{
  "service_name": "user-service",
  "service_version": "v1.2.3",
  "instance_id": "user-service-abc123",
  "instance": {
    "address": "10.0.1.5:8080",
    "region": "us-east-1",
    "zone": "us-east-1a",
    "labels": {
      "env": "production",
      "team": "platform",
      "datacenter": "aws"
    },
    "weight": 100,
    "status": "healthy",
    "role": "primary",
    "started_at": 1698768000
  }
}
```

### 10.3 Instance Discovery

#### 10.3.1 Discovery Pattern

**Gateway discovers all instances**:

```go
func (g *Gateway) discoverServiceInstances(serviceName string) ([]*SchemaManifest, error) {
    // List all instances of the service
    manifests, err := g.registry.ListManifests(ctx, serviceName)
    if err != nil {
        return nil, err
    }
    
    // Filter by health status
    var healthy []*SchemaManifest
    for _, manifest := range manifests {
        if manifest.Instance != nil && 
           manifest.Instance.Status == InstanceStatusHealthy {
            healthy = append(healthy, manifest)
        }
    }
    
    return healthy, nil
}
```

#### 10.3.2 Instance Selection

**Label-based selection**:

```go
func (g *Gateway) selectInstances(manifests []*SchemaManifest, selector map[string]string) []*SchemaManifest {
    var selected []*SchemaManifest
    
    for _, manifest := range manifests {
        if matchesLabels(manifest.Instance.Labels, selector) {
            selected = append(selected, manifest)
        }
    }
    
    return selected
}

// Example: Select only production instances
instances := g.selectInstances(allInstances, map[string]string{
    "env": "production",
})

// Example: Select specific region
instances := g.selectInstances(allInstances, map[string]string{
    "region": "us-west-2",
})
```

### 10.4 Load Balancing Strategies

#### 10.4.1 Weighted Round Robin

```go
type WeightedRoundRobin struct {
    instances []*SchemaManifest
    current   int
    mu        sync.Mutex
}

func (w *WeightedRoundRobin) Next() *SchemaManifest {
    w.mu.Lock()
    defer w.mu.Unlock()
    
    if len(w.instances) == 0 {
        return nil
    }
    
    // Build weighted list
    weighted := make([]*SchemaManifest, 0)
    for _, instance := range w.instances {
        weight := instance.Instance.Weight
        if weight == 0 {
            weight = 100 // default
        }
        
        // Add instance N times based on weight
        for i := 0; i < weight; i++ {
            weighted = append(weighted, instance)
        }
    }
    
    instance := weighted[w.current % len(weighted)]
    w.current++
    
    return instance
}
```

#### 10.4.2 Least Connections

```go
type LeastConnections struct {
    instances   []*SchemaManifest
    connections map[string]int // instance_id -> active connections
    mu          sync.Mutex
}

func (l *LeastConnections) Next() *SchemaManifest {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    var selected *SchemaManifest
    minConns := int(^uint(0) >> 1) // max int
    
    for _, instance := range l.instances {
        conns := l.connections[instance.InstanceID]
        
        // Consider weight
        weight := instance.Instance.Weight
        if weight == 0 {
            weight = 100
        }
        
        // Normalize: connections / weight
        normalized := (conns * 100) / weight
        
        if normalized < minConns {
            minConns = normalized
            selected = instance
        }
    }
    
    return selected
}
```

#### 10.4.3 Region-Aware Routing

```go
func (g *Gateway) selectInstanceByRegion(instances []*SchemaManifest, clientRegion string) *SchemaManifest {
    // Prefer same-region instances
    for _, instance := range instances {
        if instance.Instance.Region == clientRegion {
            return instance
        }
    }
    
    // Fall back to any healthy instance
    return instances[rand.Intn(len(instances))]
}
```

#### 10.4.4 Consistent Hashing

```go
type ConsistentHash struct {
    ring     map[uint32]*SchemaManifest
    keys     []uint32
    replicas int
}

func (c *ConsistentHash) Get(key string) *SchemaManifest {
    if len(c.keys) == 0 {
        return nil
    }
    
    hash := c.hashKey(key)
    
    // Binary search for the right node
    idx := sort.Search(len(c.keys), func(i int) bool {
        return c.keys[i] >= hash
    })
    
    if idx == len(c.keys) {
        idx = 0
    }
    
    return c.ring[c.keys[idx]]
}

// Use case: Sticky sessions, caching
instance := consistentHash.Get(userID)
```

### 10.5 Schema Consistency

#### 10.5.1 Schema Validation Across Instances

**Gateway validates schema consistency**:

```go
func (g *Gateway) validateSchemaConsistency(serviceName string) error {
    manifests, _ := g.registry.ListManifests(ctx, serviceName)
    
    if len(manifests) == 0 {
        return nil
    }
    
    // Compare schema checksums
    expectedChecksum := manifests[0].Checksum
    
    var inconsistent []string
    for _, manifest := range manifests[1:] {
        if manifest.Checksum != expectedChecksum {
            inconsistent = append(inconsistent, manifest.InstanceID)
        }
    }
    
    if len(inconsistent) > 0 {
        return fmt.Errorf("schema inconsistency detected: instances %v have different schemas",
            inconsistent)
    }
    
    return nil
}
```

#### 10.5.2 Rolling Updates

**Zero-downtime schema updates**:

```go
// Instance registers with expected schema checksum
func (s *Service) registerWithSchemaVersion() error {
    manifest := &SchemaManifest{
        ServiceName:    "user-service",
        ServiceVersion: "v1.2.3",
        InstanceID:     s.instanceID,
        Instance: &InstanceMetadata{
            Status: InstanceStatusStarting,
            ExpectedSchemaChecksum: s.schemaChecksum,
        },
        Schemas: s.schemas,
        Checksum: s.schemaChecksum,
    }
    
    // Register
    err := s.registry.RegisterManifest(ctx, manifest)
    if err != nil {
        return err
    }
    
    // Wait for gateway to acknowledge
    time.Sleep(5 * time.Second)
    
    // Mark as healthy
    manifest.Instance.Status = InstanceStatusHealthy
    return s.registry.UpdateManifest(ctx, manifest)
}
```

### 10.6 Health Aggregation

#### 10.6.1 Service-Level Health

**Gateway aggregates instance health**:

```go
type ServiceHealth struct {
    ServiceName      string
    TotalInstances   int
    HealthyInstances int
    Status           string
    Instances        map[string]InstanceStatus
}

func (g *Gateway) getServiceHealth(serviceName string) *ServiceHealth {
    manifests, _ := g.registry.ListManifests(ctx, serviceName)
    
    health := &ServiceHealth{
        ServiceName:  serviceName,
        TotalInstances: len(manifests),
        Instances:    make(map[string]InstanceStatus),
    }
    
    for _, manifest := range manifests {
        if manifest.Instance != nil {
            health.Instances[manifest.InstanceID] = manifest.Instance.Status
            
            if manifest.Instance.Status == InstanceStatusHealthy {
                health.HealthyInstances++
            }
        }
    }
    
    // Determine overall status
    ratio := float64(health.HealthyInstances) / float64(health.TotalInstances)
    switch {
    case ratio == 1.0:
        health.Status = "healthy"
    case ratio >= 0.5:
        health.Status = "degraded"
    default:
        health.Status = "unhealthy"
    }
    
    return health
}
```

### 10.7 Canary Deployments

#### 10.7.1 Canary Configuration

```json
{
  "service_name": "user-service",
  "service_version": "v1.3.0",
  "instance_id": "user-service-canary-xyz",
  "instance": {
    "address": "10.0.1.10:8080",
    "status": "healthy",
    "role": "canary",
    "weight": 10,
    "deployment": {
      "deployment_id": "deploy-20241115-001",
      "strategy": "canary",
      "traffic_percent": 10,
      "stage": "canary",
      "deployed_at": 1700000000
    }
  }
}
```

#### 10.7.2 Canary Routing

```go
func (g *Gateway) selectInstanceWithCanary(instances []*SchemaManifest) *SchemaManifest {
    var primary []*SchemaManifest
    var canary []*SchemaManifest
    
    for _, instance := range instances {
        if instance.Instance.Role == InstanceRoleCanary {
            canary = append(canary, instance)
        } else {
            primary = append(primary, instance)
        }
    }
    
    // Route based on canary traffic percentage
    if len(canary) > 0 {
        trafficPercent := canary[0].Instance.Deployment.TrafficPercent
        
        if rand.Intn(100) < trafficPercent {
            return canary[rand.Intn(len(canary))]
        }
    }
    
    // Route to primary
    if len(primary) > 0 {
        return primary[rand.Intn(len(primary))]
    }
    
    return nil
}
```

### 10.8 Blue-Green Deployments

#### 10.8.1 Blue-Green Configuration

```json
// Blue (current production)
{
  "instance": {
    "role": "blue",
    "weight": 100,
    "deployment": {
      "strategy": "blue_green",
      "traffic_percent": 100
    }
  }
}

// Green (new version, ready for switch)
{
  "instance": {
    "role": "green",
    "weight": 0,
    "deployment": {
      "strategy": "blue_green",
      "traffic_percent": 0
    }
  }
}
```

#### 10.8.2 Traffic Switch

```go
func (g *Gateway) switchBlueGreen(serviceName string, toColor InstanceRole) error {
    manifests, _ := g.registry.ListManifests(ctx, serviceName)
    
    for _, manifest := range manifests {
        if manifest.Instance.Role == toColor {
            // Switch traffic to green
            manifest.Instance.Weight = 100
            manifest.Instance.Deployment.TrafficPercent = 100
        } else {
            // Remove traffic from blue
            manifest.Instance.Weight = 0
            manifest.Instance.Deployment.TrafficPercent = 0
        }
        
        g.registry.UpdateManifest(ctx, manifest)
    }
    
    logger.Info("blue-green traffic switched",
        "service", serviceName,
        "to", toColor)
    
    return nil
}
```

### 10.9 Shadow Traffic

#### 10.9.1 Shadow Instance Configuration

```json
{
  "instance": {
    "role": "shadow",
    "weight": 0,
    "deployment": {
      "strategy": "shadow",
      "traffic_percent": 0
    }
  }
}
```

#### 10.9.2 Shadow Routing

```go
func (g *Gateway) routeWithShadow(req *http.Request, instances []*SchemaManifest) {
    // Find primary and shadow instances
    var primary, shadow *SchemaManifest
    
    for _, instance := range instances {
        if instance.Instance.Role == InstanceRoleShadow {
            shadow = instance
        } else if instance.Instance.Status == InstanceStatusHealthy {
            primary = instance
        }
    }
    
    // Route to primary
    primaryResp := g.forward(req, primary)
    
    // Duplicate to shadow (fire and forget)
    if shadow != nil {
        go func() {
            shadowReq := req.Clone(context.Background())
            shadowResp := g.forward(shadowReq, shadow)
            
            // Compare responses for validation
            g.compareShadowResponse(primaryResp, shadowResp)
        }()
    }
    
    return primaryResp
}
```

### 10.10 Instance Draining

#### 10.10.1 Graceful Shutdown

```go
func (s *Service) drain() error {
    // Mark as draining
    s.manifest.Instance.Status = InstanceStatusDraining
    s.registry.UpdateManifest(ctx, s.manifest)
    
    logger.Info("instance draining, gateway will stop routing new requests")
    
    // Wait for existing connections to finish
    timeout := 30 * time.Second
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    s.server.Shutdown(ctx)
    
    // Deregister
    s.registry.DeleteManifest(ctx, s.manifest.InstanceID)
    
    return nil
}
```

#### 10.10.2 Gateway Behavior

```go
func (g *Gateway) selectHealthyInstances(instances []*SchemaManifest) []*SchemaManifest {
    var healthy []*SchemaManifest
    
    for _, instance := range instances {
        // Exclude draining and stopping instances
        if instance.Instance.Status == InstanceStatusHealthy ||
           instance.Instance.Status == InstanceStatusDegraded {
            healthy = append(healthy, instance)
        }
    }
    
    return healthy
}
```

### 10.11 Best Practices

**1. Instance Metadata**:
- Always include `address`, `status`, `started_at`
- Use labels for flexible instance selection
- Set appropriate weights for traffic shaping

**2. Schema Consistency**:
- Validate schema checksums across instances
- Use `expected_schema_checksum` for rolling updates
- Deploy new version gradually

**3. Health Management**:
- Use `draining` status before shutdown
- Allow 30-60s drain period
- Monitor instance health continuously

**4. Deployment Strategies**:
- **Canary**: 10% traffic → validate → 100%
- **Blue-Green**: Test green → switch traffic → keep blue for rollback
- **Shadow**: Compare responses → validate → promote
- **Rolling**: Update instances one by one

**5. Load Balancing**:
- Use weighted round-robin for simple cases
- Use least connections for long-lived connections
- Use consistent hashing for sticky sessions
- Use region-aware routing for global services

---

## 11. Schema Compatibility

### 11.1 Overview

Schema compatibility ensures safe schema evolution without breaking existing consumers. FARP provides metadata for services to declare compatibility guarantees and breaking changes.

### 10.2 Compatibility Modes

#### 10.2.1 Backward Compatible (`backward`)

**Definition**: New schema can process data/requests from old schema.

**Use Case**: Adding optional fields, new endpoints

**Example**:
```json
{
  "compatibility": {
    "mode": "backward",
    "previous_versions": ["v1.1.0"]
  }
}
```

**Safe Changes**:
- Add optional fields
- Add new endpoints
- Add enum values
- Relax validation rules

**Breaking Changes**:
- Remove fields
- Change field types
- Make fields required
- Remove endpoints

#### 10.2.2 Forward Compatible (`forward`)

**Definition**: Old schema can process data/requests from new schema.

**Use Case**: Deprecating features gradually

**Safe Changes**:
- Remove optional fields
- Remove endpoints (if clients don't use them)

#### 10.2.3 Full Compatible (`full`)

**Definition**: Both backward and forward compatible.

**Use Case**: Strict compatibility requirements

**Safe Changes**: Intersection of backward and forward
- Add optional fields with defaults
- Add new endpoints

#### 10.2.4 Transitive Compatibility

**Definition**: Compatibility across N schema versions.

**Use Case**: Long-term compatibility guarantees

```json
{
  "compatibility": {
    "mode": "backward_transitive",
    "previous_versions": ["v1.2.0", "v1.1.0", "v1.0.0"]
  }
}
```

### 10.3 Breaking Change Detection

Services should declare breaking changes explicitly:

```json
{
  "compatibility": {
    "mode": "none",
    "breaking_changes": [
      {
        "type": "field_removed",
        "path": "/components/schemas/User/properties/email",
        "description": "Email field removed from User schema",
        "severity": "critical",
        "migration": "Use /users/{id}/contact endpoint instead"
      },
      {
        "type": "endpoint_changed",
        "path": "/paths/users/get",
        "description": "Changed response format from array to paginated object",
        "severity": "high",
        "migration": "Access users via response.data instead of root array"
      }
    ]
  }
}
```

### 10.4 Deprecation Tracking

```json
{
  "compatibility": {
    "deprecations": [
      {
        "path": "/paths/users/v1/list",
        "deprecated_at": "2024-01-15T00:00:00Z",
        "removal_date": "2024-07-15T00:00:00Z",
        "replacement": "/users?page=1",
        "migration": "Migrate to paginated /users endpoint",
        "reason": "Replaced with paginated endpoint for better performance"
      }
    ]
  }
}
```

### 10.5 Gateway Behavior

**Gateway should**:
1. Check compatibility mode before auto-updating routes
2. Warn about `mode: none` (breaking changes)
3. Delay updates with breaking changes (require manual approval)
4. Log all deprecations
5. Track deprecation removal dates
6. Alert when deprecated endpoints are used

**Example Gateway Logic**:
```go
func (g *Gateway) handleSchemaUpdate(old, new *SchemaManifest) error {
    for _, newSchema := range new.Schemas {
        oldSchema := findSchema(old.Schemas, newSchema.Type)
        
        // Check compatibility
        if newSchema.Compatibility == nil {
            // No compatibility info, assume breaking
            return g.requireManualApproval(new)
        }
        
        switch newSchema.Compatibility.Mode {
        case CompatibilityBackward, CompatibilityForward, CompatibilityFull:
            // Safe to auto-update
            g.updateRoutes(newSchema)
            
        case CompatibilityNone:
            // Breaking changes
            if len(newSchema.Compatibility.BreakingChanges) > 0 {
                g.logBreakingChanges(newSchema.Compatibility.BreakingChanges)
                return g.requireManualApproval(new)
            }
            
        case CompatibilityBackwardTransitive:
            // Check if old version in lineage
            if !contains(newSchema.Compatibility.PreviousVersions, old.ServiceVersion) {
                return g.requireManualApproval(new)
            }
        }
        
        // Log deprecations
        for _, dep := range newSchema.Compatibility.Deprecations {
            g.trackDeprecation(dep)
        }
    }
    
    return nil
}
```

---

## 11. Route Metadata

### 11.1 Overview

Route metadata provides per-operation hints for intelligent gateway configuration. Unlike schema content, this metadata describes operational characteristics.

### 11.2 Idempotency

**Critical for retry logic**:

```json
{
  "operation_id": "getUser",
  "path": "/users/{id}",
  "method": "GET",
  "idempotent": true
}
```

**Gateway Behavior**:
- `idempotent: true` → Safe to retry on network errors
- `idempotent: false` → Only retry on known retryable errors

**By HTTP Method**:
- GET, HEAD, OPTIONS, TRACE → Always idempotent
- PUT, DELETE → Usually idempotent
- POST, PATCH → Usually NOT idempotent (unless explicitly marked)

### 11.3 Timeout Hints

Services can suggest operation-specific timeouts:

```json
{
  "operation_id": "exportReport",
  "path": "/reports/export",
  "method": "POST",
  "timeout_hint": "5m",
  "idempotent": false
}
```

**Gateway Behavior**:
- Use hint as starting point
- Override based on observed latency
- Apply global maximum timeout

### 11.4 Cacheability

```json
{
  "operation_id": "getUser",
  "path": "/users/{id}",
  "method": "GET",
  "cacheable": true,
  "cache_ttl": "5m",
  "idempotent": true
}
```

**Gateway Behavior**:
- Cache responses with `cacheable: true`
- Use `cache_ttl` for TTL
- Invalidate on mutations to same resource

### 11.5 Data Sensitivity

**Critical for compliance**:

```json
{
  "operation_id": "getUserProfile",
  "path": "/users/{id}/profile",
  "method": "GET",
  "sensitivity": "pii",
  "idempotent": true
}
```

**Gateway Behavior**:
- `pii`/`phi`/`pci` → Enable audit logging
- `pii`/`phi`/`pci` → Redact from general logs
- `confidential` → Restrict to authenticated users
- `public` → Allow caching in CDN

### 11.6 Cost-Based Routing

```json
{
  "operation_id": "generateMLPrediction",
  "path": "/ml/predict",
  "method": "POST",
  "cost": 8,
  "timeout_hint": "30s",
  "idempotent": true
}
```

**Cost Scale**: 1 (cheap) - 10 (expensive)

**Gateway Behavior**:
- Apply stricter rate limits to high-cost operations
- Route to dedicated instance pools
- Prioritize low-cost operations under load

### 11.7 Complete Example

```json
{
  "routes": [
    {
      "operation_id": "listUsers",
      "path": "/users",
      "method": "GET",
      "idempotent": true,
      "timeout_hint": "1s",
      "cost": 2,
      "cacheable": true,
      "cache_ttl": "1m",
      "sensitivity": "internal",
      "response_size": "medium",
      "rate_limit_hint": 1000
    },
    {
      "operation_id": "createUser",
      "path": "/users",
      "method": "POST",
      "idempotent": false,
      "timeout_hint": "3s",
      "cost": 4,
      "cacheable": false,
      "sensitivity": "pii",
      "response_size": "small",
      "rate_limit_hint": 100
    },
    {
      "operation_id": "processPayment",
      "path": "/payments",
      "method": "POST",
      "idempotent": true,
      "timeout_hint": "10s",
      "cost": 9,
      "cacheable": false,
      "sensitivity": "pci",
      "response_size": "small",
      "rate_limit_hint": 10
    }
  ]
}
```

---

## 12. Protocol-Specific Metadata

### 12.1 Overview

Each protocol has unique characteristics requiring specialized metadata for gateway configuration.

### 12.2 GraphQL Federation

**Purpose**: Enable Apollo Federation v1/v2 composition

**Example**:
```json
{
  "type": "graphql",
  "metadata": {
    "graphql": {
      "federation": {
        "version": "v2",
        "subgraph_name": "users",
        "entities": [
          {
            "type_name": "User",
            "key_fields": ["id"],
            "fields": ["id", "email", "name", "createdAt"],
            "resolvable": true
          }
        ],
        "extends": ["Product"],
        "provides": [
          {
            "field": "User.orders",
            "fields": ["id", "total"]
          }
        ],
        "requires": [
          {
            "field": "User.reviewCount",
            "fields": ["id"]
          }
        ]
      },
      "subscriptions_enabled": true,
      "subscription_protocol": "graphql-transport-ws",
      "complexity_limit": 1000,
      "depth_limit": 10
    }
  }
}
```

**Gateway Behavior**:
- Compose federated schema from all subgraphs
- Route entity resolution queries
- Apply complexity/depth limits
- Handle subscriptions via websocket

### 12.3 gRPC Metadata

**Purpose**: Enable gRPC proxying, transcoding, and gRPC-Web

**Example**:
```json
{
  "type": "grpc",
  "metadata": {
    "grpc": {
      "reflection_enabled": true,
      "packages": ["user.v1", "auth.v1"],
      "services": ["UserService", "AuthService"],
      "grpc_web_enabled": true,
      "grpc_web_protocol": "grpc-web",
      "server_streaming_enabled": true,
      "client_streaming_enabled": false,
      "bidirectional_streaming_enabled": false
    }
  }
}
```

**Gateway Behavior**:
- Use reflection for schema discovery
- Enable gRPC-Web for browser clients
- Configure streaming support
- Generate REST endpoints via transcoding

### 12.4 OpenAPI Extensions

**Purpose**: Preserve vendor extensions and server variables

**Example**:
```json
{
  "type": "openapi",
  "metadata": {
    "openapi": {
      "extensions": {
        "x-api-id": "user-api-v2",
        "x-visibility": "public"
      },
      "server_variables": {
        "environment": {
          "default": "production",
          "enum": ["production", "staging", "development"],
          "description": "Environment for this API"
        },
        "region": {
          "default": "us-east-1",
          "enum": ["us-east-1", "us-west-2", "eu-west-1"]
        }
      },
      "default_security": ["bearer", "apikey"]
    }
  }
}
```

### 12.5 AsyncAPI Bindings

**Purpose**: Configure message broker specifics

**Example**:
```json
{
  "type": "asyncapi",
  "metadata": {
    "asyncapi": {
      "protocol": "kafka",
      "channel_bindings": {
        "user.events": {
          "kafka": {
            "topic": "user-events-v2",
            "partitions": 12,
            "replicas": 3
          }
        }
      },
      "message_bindings": {
        "user.created": {
          "kafka": {
            "key": {
              "type": "string",
              "enum": ["user_id"]
            },
            "schemaIdLocation": "header"
          }
        }
      }
    }
  }
}
```

### 12.6 oRPC Batch Support

**Purpose**: Enable batch operations

**Example**:
```json
{
  "type": "orpc",
  "metadata": {
    "orpc": {
      "batch_enabled": true,
      "streaming_procedures": ["watchUsers", "subscribeEvents"]
    }
  }
}
```

**Gateway Behavior**:
- Batch multiple RPC calls into single HTTP request
- Route streaming procedures to WebSocket/SSE

### 12.7 OpenAPI Schema Composition

**Purpose**: Enable automatic merging of multiple OpenAPI schemas into a unified API specification

**Overview**:
When multiple services register OpenAPI schemas, gateways can automatically compose them into a single unified API specification. This enables:
- Unified API documentation for all services
- Automatic conflict resolution when paths or components collide
- Service-specific prefixing to avoid naming conflicts
- Flexible routing strategies for composed endpoints

**Example**:
```json
{
  "type": "openapi",
  "metadata": {
    "openapi": {
      "composition": {
        "include_in_merged": true,
        "component_prefix": "user_svc",
        "tag_prefix": "Users",
        "operation_id_prefix": "UserAPI",
        "conflict_strategy": "prefix",
        "preserve_extensions": true,
        "custom_servers": [
          {
            "url": "https://api.example.com/v2",
            "description": "Production server",
            "variables": {
              "environment": {
                "default": "production",
                "enum": ["production", "staging"]
              }
            }
          }
        ]
      }
    }
  }
}
```

**Composition Fields**:

- **`include_in_merged`** (boolean): Whether to include this schema in unified API spec
  - `true`: Include in merged OpenAPI documentation
  - `false`: Skip (useful for internal/admin APIs)

- **`component_prefix`** (string): Prefix for component schema names
  - Defaults to service name if not specified
  - Example: `User` schema becomes `user_svc_User`
  - Prevents naming conflicts across services

- **`tag_prefix`** (string): Prefix for operation tags
  - Groups operations by service in documentation
  - Example: `users` tag becomes `Users_users`

- **`operation_id_prefix`** (string): Prefix for operation IDs
  - Ensures unique operation IDs across services
  - Example: `getUser` becomes `UserAPI_getUser`

- **`conflict_strategy`** (enum): How to handle conflicts
  - `prefix`: Add service prefix to conflicting items (default)
  - `error`: Fail composition on conflicts
  - `skip`: Skip conflicting items from this service
  - `overwrite`: Use this service's version
  - `merge`: Attempt to merge conflicting items

- **`preserve_extensions`** (boolean): Whether to preserve x-* extensions
  - Maintains vendor-specific extensions in merged spec

- **`custom_servers`** (array): Override servers in merged spec
  - Useful when services use different base URLs
  - Variables support templating for environments

**Gateway Behavior**:

1. **Schema Collection**: Gateway collects all registered OpenAPI schemas
2. **Filtering**: Only includes schemas with `include_in_merged: true`
3. **Path Routing**: Applies routing strategy (instance, service, versioned, etc.)
4. **Conflict Detection**: Identifies colliding paths, components, tags, operation IDs
5. **Resolution**: Applies conflict strategy to resolve collisions
6. **Composition**: Merges paths, components, tags, and servers
7. **Output**: Produces unified OpenAPI 3.x specification

**Composition Example**:

Service A (User Service):
```json
{
  "openapi": "3.1.0",
  "paths": {
    "/users": { "get": { "operationId": "getUsers", "tags": ["users"] } }
  },
  "components": {
    "schemas": {
      "User": { "type": "object", "properties": { "id": { "type": "string" } } }
    }
  }
}
```

Service B (Order Service):
```json
{
  "openapi": "3.1.0",
  "paths": {
    "/orders": { "get": { "operationId": "getOrders", "tags": ["orders"] } }
  },
  "components": {
    "schemas": {
      "Order": { "type": "object", "properties": { "id": { "type": "string" } } }
    }
  }
}
```

Merged Result (with instance routing and prefix strategy):
```json
{
  "openapi": "3.1.0",
  "info": {
    "title": "Federated API",
    "version": "1.0.0"
  },
  "paths": {
    "/user-service/users": {
      "get": {
        "operationId": "user-service_getUsers",
        "tags": ["user-service_users"]
      }
    },
    "/order-service/orders": {
      "get": {
        "operationId": "order-service_getOrders",
        "tags": ["order-service_orders"]
      }
    }
  },
  "components": {
    "schemas": {
      "user-service_User": {
        "type": "object",
        "properties": { "id": { "type": "string" } }
      },
      "order-service_Order": {
        "type": "object",
        "properties": { "id": { "type": "string" } }
      }
    }
  }
}
```

**Conflict Resolution Examples**:

*Conflict: Both services define `/health` path*

- **prefix**: Routes become `/service-a/health` and `/service-b/health`
- **error**: Composition fails with error
- **skip**: Only first service's `/health` is included
- **overwrite**: Last service's `/health` overwrites previous
- **merge**: Attempts to merge operations (GET from A, POST from B)

*Conflict: Both services define `Error` component*

- **prefix**: Becomes `service-a_Error` and `service-b_Error`
- **skip**: Only first service's component included
- **overwrite**: Last service's component used

**Implementation**: 
The FARP Go library provides a `merger` package that handles:
- Parsing OpenAPI schemas from service manifests
- Applying routing strategies to paths
- Detecting and resolving conflicts
- Prefixing component names, tags, and operation IDs
- Generating unified OpenAPI specification
- Tracking composition conflicts and warnings

**Use Cases**:
- API aggregation gateways exposing unified API
- Multi-tenant systems with per-service schemas
- Microservices documentation portals
- API versioning with side-by-side deployments
- Gradual service migration (old + new in single spec)

---

## 13. Service Hints

### 13.1 Overview

Service hints are **non-binding suggestions** from the service to the gateway. Gateways may use these for better decisions but are not required to honor them.

### 13.2 Recommended Timeouts

```json
{
  "hints": {
    "recommended_timeout": "5s"
  }
}
```

**Gateway Behavior** (optional):
- Use as default timeout
- Override based on observed latency
- Apply per-route overrides

### 13.3 Expected Latency

```json
{
  "hints": {
    "expected_latency": {
      "p50": "10ms",
      "p95": "50ms",
      "p99": "100ms",
      "p999": "500ms"
    }
  }
}
```

**Gateway Behavior** (optional):
- Set SLO alerting thresholds
- Capacity planning
- Anomaly detection

### 13.4 Scaling Profile

```json
{
  "hints": {
    "scaling": {
      "auto_scale": true,
      "min_instances": 2,
      "max_instances": 20,
      "target_cpu": 0.70,
      "target_memory": 0.80
    }
  }
}
```

**Gateway Behavior** (optional):
- Adjust connection pool sizes
- Pre-warm connections
- Load balancing weights

### 13.5 Service Dependencies

**Purpose**: Dependency graph visualization and impact analysis

```json
{
  "hints": {
    "dependencies": [
      {
        "service_name": "auth-service",
        "schema_type": "grpc",
        "version_range": ">=v2.0.0",
        "critical": true,
        "used_operations": ["ValidateToken", "GetUser"]
      },
      {
        "service_name": "notification-service",
        "schema_type": "asyncapi",
        "version_range": ">=v1.5.0",
        "critical": false,
        "used_operations": ["SendEmail"]
      }
    ]
  }
}
```

**Gateway Behavior** (optional):
- Build service dependency graph
- Impact analysis for outages
- Health check propagation
- Validate schema compatibility across dependencies

### 13.6 Complete Example

```json
{
  "service_name": "user-service",
  "service_version": "v2.1.0",
  "hints": {
    "recommended_timeout": "3s",
    "expected_latency": {
      "p50": "15ms",
      "p95": "75ms",
      "p99": "150ms",
      "p999": "500ms"
    },
    "scaling": {
      "auto_scale": true,
      "min_instances": 3,
      "max_instances": 50,
      "target_cpu": 0.65,
      "target_memory": 0.75
    },
    "dependencies": [
      {
        "service_name": "auth-service",
        "schema_type": "grpc",
        "version_range": ">=v2.0.0 <v3.0.0",
        "critical": true,
        "used_operations": ["ValidateToken"]
      },
      {
        "service_name": "profile-service",
        "schema_type": "openapi",
        "version_range": ">=v1.8.0",
        "critical": false,
        "used_operations": ["/profiles/{id}"]
      }
    ]
  }
}
```

---

## 14. Registration Flow

### 14.1 Service Startup Sequence

```
1. Application starts
   ↓
2. Router initialized with routes defined
   ↓
3. Schema providers generate schemas
   - OpenAPIProvider → openapi.json
   - AsyncAPIProvider → asyncapi.json
   ↓
4. SchemaManifest created
   - Calculate checksums for each schema
   - Calculate global checksum
   - Set updated_at timestamp
   ↓
5. Publish schemas (if strategy == Registry)
   - backend.Put("/schemas/service/v1/openapi", openapi_json)
   - backend.Put("/schemas/service/v1/asyncapi", asyncapi_json)
   ↓
6. Register ServiceInstance with manifest
   - instance.Metadata["schema_manifest"] = manifest_json
   - backend.Register(instance)
   ↓
7. Start health check heartbeat
   ↓
8. Application ready to serve traffic
```

### 10.2 Gateway Discovery Sequence

```
1. Gateway starts
   ↓
2. Subscribe to service registrations
   - backend.Watch("user-service", onChange)
   ↓
3. Service registered → onChange triggered
   ↓
4. Fetch SchemaManifest
   - manifest = instance.Metadata["schema_manifest"]
   ↓
5. For each schema in manifest:
   a. Check if schema already cached (compare hash)
   b. If new or changed:
      - Fetch schema via location strategy
      - Validate schema format
      - Store in local cache
   ↓
6. Convert schemas to gateway routes
   - OpenAPI paths → HTTP routes
   - AsyncAPI channels → WebSocket routes
   ↓
7. Configure gateway
   - Add/update routes
   - Configure health checks
   - Set up load balancing
   ↓
8. Start routing traffic to service
```

### 10.3 Hot Reload / Schema Update

```
1. Service code updated (routes added/modified)
   ↓
2. Router hot reloads
   ↓
3. Schema providers regenerate schemas
   ↓
4. Calculate new checksums
   ↓
5. Compare with previous checksums
   - If unchanged → skip update
   - If changed → proceed
   ↓
6. Update manifest
   - Set new checksum
   - Update updated_at timestamp
   ↓
7. Publish updated schemas (if strategy == Registry)
   ↓
8. Update ServiceInstance metadata
   - backend.UpdateMetadata(instance_id, manifest)
   ↓
9. Trigger change notification
   ↓
10. Gateway detects change → reconfigure routes
```

---

## 8. Schema Provider Interface

### 8.1 Interface Definition

```go
// SchemaProvider generates schemas from application code
type SchemaProvider interface {
    // Type returns the schema type this provider generates
    Type() SchemaType
    
    // Generate generates a schema from the application
    // Returns the schema as interface{} (typically map[string]interface{} or struct)
    Generate(ctx context.Context, app Application) (interface{}, error)
    
    // Validate validates a generated schema
    Validate(schema interface{}) error
    
    // Hash calculates the SHA256 hash of a schema
    Hash(schema interface{}) (string, error)
    
    // Serialize converts schema to bytes for storage/transmission
    Serialize(schema interface{}) ([]byte, error)
    
    // Endpoint returns the HTTP endpoint where the schema is served
    // Returns empty string if not served via HTTP
    Endpoint() string
}
```

### 8.2 Built-in Providers

#### OpenAPIProvider

```go
type OpenAPIProvider struct {
    router Router
    config OpenAPIConfig
}

func (p *OpenAPIProvider) Type() SchemaType {
    return SchemaTypeOpenAPI
}

func (p *OpenAPIProvider) Generate(ctx context.Context, app Application) (interface{}, error) {
    // Generate OpenAPI 3.1.0 spec from router
    spec := generateOpenAPISpec(p.router, p.config)
    return spec, nil
}

func (p *OpenAPIProvider) Endpoint() string {
    return "/openapi.json"
}
```

#### AsyncAPIProvider

```go
type AsyncAPIProvider struct {
    router Router
    config AsyncAPIConfig
}

func (p *AsyncAPIProvider) Type() SchemaType {
    return SchemaTypeAsyncAPI
}

func (p *AsyncAPIProvider) Generate(ctx context.Context, app Application) (interface{}, error) {
    // Generate AsyncAPI 3.0.0 spec from streaming routes
    spec := generateAsyncAPISpec(p.router, p.config)
    return spec, nil
}

func (p *AsyncAPIProvider) Endpoint() string {
    return "/asyncapi.json"
}
```

### 8.3 Custom Provider Implementation

```go
// Example: gRPC schema provider
type GRPCSchemaProvider struct {
    protoFiles []string
}

func (p *GRPCSchemaProvider) Type() SchemaType {
    return SchemaTypeGRPC
}

func (p *GRPCSchemaProvider) Generate(ctx context.Context, app Application) (interface{}, error) {
    // Parse .proto files and extract FileDescriptorSet
    descriptors := parseProtoFiles(p.protoFiles)
    return descriptors, nil
}

func (p *GRPCSchemaProvider) Validate(schema interface{}) error {
    // Validate FileDescriptorSet format
    _, ok := schema.(*descriptor.FileDescriptorSet)
    if !ok {
        return errors.New("invalid gRPC schema format")
    }
    return nil
}
```

---

## 9. Registry Interface

### 9.1 Interface Definition

```go
// SchemaRegistry manages schema manifests and schemas
type SchemaRegistry interface {
    // Manifest operations
    RegisterManifest(ctx context.Context, manifest *SchemaManifest) error
    GetManifest(ctx context.Context, instanceID string) (*SchemaManifest, error)
    UpdateManifest(ctx context.Context, manifest *SchemaManifest) error
    DeleteManifest(ctx context.Context, instanceID string) error
    ListManifests(ctx context.Context, serviceName string) ([]*SchemaManifest, error)
    
    // Schema operations
    PublishSchema(ctx context.Context, path string, schema interface{}) error
    FetchSchema(ctx context.Context, path string) (interface{}, error)
    DeleteSchema(ctx context.Context, path string) error
    
    // Watch operations
    WatchManifests(ctx context.Context, serviceName string, onChange func(*SchemaManifest)) error
    WatchSchemas(ctx context.Context, path string, onChange func(interface{})) error
    
    // Lifecycle
    Close() error
}
```

### 9.2 Backend Implementations

#### Consul Backend

```go
type ConsulSchemaRegistry struct {
    client *consul.Client
    config ConsulConfig
}

func (r *ConsulSchemaRegistry) RegisterManifest(ctx context.Context, manifest *SchemaManifest) error {
    // Serialize manifest to JSON
    data, _ := json.Marshal(manifest)
    
    // Store in Consul KV
    key := fmt.Sprintf("/services/%s/instances/%s/manifest", 
        manifest.ServiceName, manifest.InstanceID)
    
    return r.client.KV().Put(&consul.KVPair{
        Key:   key,
        Value: data,
    }, nil)
}

func (r *ConsulSchemaRegistry) PublishSchema(ctx context.Context, path string, schema interface{}) error {
    // Serialize schema
    data, _ := json.Marshal(schema)
    
    // Store in Consul KV
    return r.client.KV().Put(&consul.KVPair{
        Key:   path,
        Value: data,
    }, nil)
}

func (r *ConsulSchemaRegistry) WatchManifests(ctx context.Context, serviceName string, onChange func(*SchemaManifest)) error {
    // Watch for manifest changes in Consul
    prefix := fmt.Sprintf("/services/%s/instances/", serviceName)
    
    plan, err := watch.Parse(map[string]interface{}{
        "type":   "keyprefix",
        "prefix": prefix,
    })
    if err != nil {
        return err
    }
    
    plan.Handler = func(idx uint64, data interface{}) {
        // Parse and invoke onChange callback
        // ...
    }
    
    return plan.RunWithClientAndHclog(r.client, nil)
}
```

#### etcd Backend

```go
type EtcdSchemaRegistry struct {
    client *clientv3.Client
    config EtcdConfig
}

func (r *EtcdSchemaRegistry) RegisterManifest(ctx context.Context, manifest *SchemaManifest) error {
    data, _ := json.Marshal(manifest)
    
    key := fmt.Sprintf("/forge/services/%s/instances/%s/manifest",
        manifest.ServiceName, manifest.InstanceID)
    
    _, err := r.client.Put(ctx, key, string(data))
    return err
}

func (r *EtcdSchemaRegistry) WatchManifests(ctx context.Context, serviceName string, onChange func(*SchemaManifest)) error {
    prefix := fmt.Sprintf("/forge/services/%s/instances/", serviceName)
    
    watchChan := r.client.Watch(ctx, prefix, clientv3.WithPrefix())
    
    go func() {
        for watchResp := range watchChan {
            for _, event := range watchResp.Events {
                // Parse manifest and invoke onChange
                // ...
            }
        }
    }()
    
    return nil
}
```

---

## 10. Service Discovery Integration

### 10.1 Integration Pattern

FARP is designed to **extend** existing service discovery systems, not replace them. The integration pattern is:

```
1. Service registers with discovery backend (normal flow)
2. Service adds FARP metadata to ServiceInstance.Metadata
3. Discovery backend stores/propagates metadata (backend-specific)
4. Gateway discovers service via existing discovery mechanism
5. Gateway extracts FARP metadata from service instance
6. Gateway fetches schemas using FARP metadata URLs
```

### 10.2 Metadata Injection

Services inject FARP metadata into the discovery backend's native metadata mechanism:

**Key-Value Metadata (Consul, etcd, Eureka)**:
```json
{
  "service_id": "user-service-abc123",
  "service_name": "user-service",
  "address": "10.0.1.5",
  "port": 8080,
  "metadata": {
    "farp.enabled": "true",
    "farp.manifest": "http://10.0.1.5:8080/_farp/manifest",
    "farp.openapi": "http://10.0.1.5:8080/openapi.json",
    "farp.openapi.path": "/openapi.json",
    "farp.asyncapi": "http://10.0.1.5:8080/asyncapi.json",
    "farp.asyncapi.path": "/asyncapi.json",
    "farp.capabilities": "[rest websocket]",
    "farp.strategy": "hybrid"
  }
}
```

**DNS TXT Records (mDNS/Bonjour)**:
```
_user-service._tcp.local. IN TXT (
  "version=1.0.0"
  "farp.enabled=true"
  "farp.manifest=http://192.168.1.100:8080/_farp/manifest"
  "farp.openapi=http://192.168.1.100:8080/openapi.json"
  "farp.openapi.path=/openapi.json"
  "farp.capabilities=[rest websocket]"
  "mdns.service_type=_user-service._tcp"
)
```

**Note**: Services SHOULD include `mdns.service_type` in TXT records for gateway filtering.

**Kubernetes Annotations**:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: user-service
  annotations:
    farp.enabled: "true"
    farp.manifest: "http://user-service.default.svc.cluster.local:8080/_farp/manifest"
    farp.openapi: "http://user-service.default.svc.cluster.local:8080/openapi.json"
    farp.capabilities: "[rest websocket]"
```

### 10.3 Standard Metadata Keys

FARP defines standard metadata keys for cross-backend compatibility:

| Key | Type | Description | Example |
|-----|------|-------------|---------|
| `farp.enabled` | boolean | FARP availability | `"true"` |
| `farp.manifest` | URL | Full URL to manifest endpoint | `"http://host:port/_farp/manifest"` |
| `farp.openapi` | URL | Full URL to OpenAPI schema | `"http://host:port/openapi.json"` |
| `farp.openapi.path` | path | Path-only version | `"/openapi.json"` |
| `farp.asyncapi` | URL | Full URL to AsyncAPI schema | `"http://host:port/asyncapi.json"` |
| `farp.asyncapi.path` | path | Path-only version | `"/asyncapi.json"` |
| `farp.graphql` | URL | Full URL to GraphQL endpoint | `"http://host:port/graphql"` |
| `farp.graphql.path` | path | Path-only version | `"/graphql"` |
| `farp.grpc.reflection` | boolean | gRPC reflection enabled | `"true"` |
| `farp.capabilities` | array | Service capabilities | `"[rest websocket grpc]"` |
| `farp.strategy` | string | Distribution strategy | `"push"`, `"pull"`, or `"hybrid"` |
| `mdns.service_type` | string | mDNS service type for filtering | `"_octopus._tcp"` |
| `mdns.domain` | string | mDNS domain | `"local."` |

**Note**: All metadata values are strings. Arrays/objects are JSON-encoded strings.

**mDNS-Specific Keys**:
- `mdns.service_type`: MUST be included for mDNS-based discovery to enable gateway service type filtering
- `mdns.domain`: Optional, defaults to `"local."` for standard mDNS discovery

### 10.4 Backend-Specific Patterns

#### Consul
```go
// Inject FARP metadata into ServiceInstance
instance := &api.AgentServiceRegistration{
    ID:      "user-service-abc123",
    Name:    "user-service",
    Address: "10.0.1.5",
    Port:    8080,
    Meta: map[string]string{
        "farp.enabled":  "true",
        "farp.manifest": "http://10.0.1.5:8080/_farp/manifest",
        "farp.openapi":  "http://10.0.1.5:8080/openapi.json",
    },
}
consulClient.Agent().ServiceRegister(instance)
```

#### mDNS/Bonjour
```go
// Inject FARP metadata into TXT records
txt := []string{
    "version=1.0.0",
    "farp.enabled=true",
    "farp.manifest=http://192.168.1.100:8080/_farp/manifest",
    "farp.openapi=http://192.168.1.100:8080/openapi.json",
    "mdns.service_type=_octopus._tcp",  // Include service type for filtering
}
server, _ := zeroconf.Register(
    "user-service-abc123",
    "_octopus._tcp",  // Custom service type
    "local.",
    8080,
    txt,
    nil,
)
```

**mDNS Service Types**: Services SHOULD advertise using well-defined service types:
- `_farp._tcp` - Generic FARP-enabled services
- `_http._tcp` - HTTP-based APIs  
- `_octopus._tcp` - Custom application-specific types
- Service name-based types (e.g., `_user-service._tcp`) for backwards compatibility

**Gateway Multi-Type Discovery**:
```go
// Gateway discovers multiple service types
resolver, _ := zeroconf.NewResolver(nil)

serviceTypes := []string{"_farp._tcp", "_octopus._tcp", "_http._tcp"}
for _, serviceType := range serviceTypes {
    entries := make(chan *zeroconf.ServiceEntry)
    
    go func() {
        for entry := range entries {
            // Extract FARP metadata from TXT records
            if hasFARPMetadata(entry.Text) {
                fetchSchemas(entry.Text)
            }
        }
    }()
    
    resolver.Browse(ctx, serviceType, "local.", entries)
}
```

#### Kubernetes
```go
// Inject FARP metadata into Service annotations
service := &corev1.Service{
    ObjectMeta: metav1.ObjectMeta{
        Name: "user-service",
        Annotations: map[string]string{
            "farp.enabled":  "true",
            "farp.manifest": "http://user-service:8080/_farp/manifest",
            "farp.openapi":  "http://user-service:8080/openapi.json",
        },
    },
}
k8sClient.CoreV1().Services("default").Create(ctx, service, metav1.CreateOptions{})
```

### 10.5 Gateway Discovery Flow

```
1. Gateway watches service discovery backend
   ↓
2. New service registered → event triggered
   ↓
3. Gateway retrieves ServiceInstance/Service object
   ↓
4. Gateway checks for farp.enabled in metadata/annotations
   ↓
5. If farp.enabled == "true":
   a. Extract farp.manifest URL from metadata
   b. Fetch SchemaManifest via HTTP
   c. For each schema in manifest:
      - Fetch schema from location (registry or HTTP)
      - Validate checksum
      - Convert to gateway routes
   d. Configure gateway with routes
```

### 10.6 Implementation Guidelines

**For Service Developers**:
1. Generate FARP metadata from SchemaManifest
2. Inject metadata into ServiceInstance before registration
3. Ensure HTTP endpoints are accessible at advertised URLs
4. Use consistent base URL (address:port from service instance)

**For Discovery Backend Integrations**:
1. Support arbitrary string metadata/annotations
2. Preserve metadata across service updates
3. Include metadata in watch/query responses
4. No special FARP-specific logic required

**For Gateway Developers**:
1. Check for `farp.enabled` in service metadata
2. Fetch manifest from `farp.manifest` URL
3. Use fallback: try `farp.openapi`, `farp.asyncapi`, etc. if manifest fetch fails
4. Cache schemas by checksum to avoid redundant fetches
5. Handle both full URLs and path-only metadata gracefully

---

## 11. Storage Backend

### 11.1 Key Structure

**Consul**:

```
/services/{service_name}/
  instances/{instance_id}/
    metadata              # ServiceInstance JSON
    manifest              # SchemaManifest JSON
  schemas/{version}/
    openapi               # OpenAPI JSON
    asyncapi              # AsyncAPI JSON
    grpc                  # gRPC FileDescriptorSet (protobuf)
```

**etcd**:

```
/forge/services/{service_name}/
  instances/{instance_id}/
    metadata
    manifest
  schemas/{version}/
    openapi
    asyncapi
    grpc
```

**Kubernetes ConfigMap**:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: user-service-schemas-v1
  namespace: default
data:
  openapi.json: |
    { "openapi": "3.1.0", ... }
  asyncapi.json: |
    { "asyncapi": "3.0.0", ... }
```

### 10.2 Storage Best Practices

1. **Size Limits**:
   - Consul KV: 512KB per key (use HTTP location for large schemas)
   - etcd: 1.5MB default (configurable with `--max-request-bytes`)
   - Kubernetes ConfigMap: 1MB

2. **TTL**:
   - Set TTL equal to service TTL
   - Clean up schemas when service deregisters
   - Use TTL=0 for long-lived schemas

3. **Compression**:
   - Compress schemas > 100KB (gzip)
   - Store compressed in registry
   - Decompress on fetch

4. **Versioning**:
   - Include service version in schema path
   - Support multiple versions simultaneously
   - Clean up old versions after grace period

---

## 12. Change Detection

### 12.1 Checksum Calculation

**Schema Checksum**:

```go
func CalculateSchemaChecksum(schema interface{}) (string, error) {
    // Serialize to canonical JSON (sorted keys)
    data, err := json.Marshal(schema)
    if err != nil {
        return "", err
    }
    
    // Calculate SHA256
    hash := sha256.Sum256(data)
    return hex.EncodeToString(hash[:]), nil
}
```

**Manifest Checksum** (combines all schema checksums):

```go
func CalculateManifestChecksum(manifest *SchemaManifest) (string, error) {
    // Sort schemas by type for deterministic hashing
    sort.Slice(manifest.Schemas, func(i, j int) bool {
        return manifest.Schemas[i].Type < manifest.Schemas[j].Type
    })
    
    // Concatenate all schema hashes
    var combined string
    for _, schema := range manifest.Schemas {
        combined += schema.Hash
    }
    
    // Calculate SHA256 of combined hashes
    hash := sha256.Sum256([]byte(combined))
    return hex.EncodeToString(hash[:]), nil
}
```

### 12.2 Change Detection Flow

```go
// Gateway-side change detection
func (g *Gateway) handleManifestUpdate(newManifest *SchemaManifest) {
    // Get cached manifest
    cached, exists := g.manifestCache.Get(newManifest.InstanceID)
    if !exists {
        // New service, fetch all schemas
        g.fetchAndRegisterSchemas(newManifest)
        return
    }
    
    // Compare checksums
    if cached.Checksum == newManifest.Checksum {
        // No changes, skip
        return
    }
    
    // Find changed schemas
    for _, newSchema := range newManifest.Schemas {
        cachedSchema := findSchema(cached.Schemas, newSchema.Type)
        if cachedSchema == nil || cachedSchema.Hash != newSchema.Hash {
            // Schema changed, refetch
            g.fetchSchema(newSchema)
        }
    }
    
    // Reconfigure routes
    g.reconfigureRoutes(newManifest)
}
```

### 12.3 Zero-Downtime Updates

**Blue-Green Schema Deployment**:

```go
// Register new version without removing old version
manifest := &SchemaManifest{
    ServiceVersion: "v2.0.0",
    Schemas: []SchemaDescriptor{
        {Type: SchemaTypeOpenAPI, Location: {RegistryPath: "/schemas/user-service/v2/openapi"}},
    },
}

// Gateway supports both v1 and v2 routes during migration
gateway.AddRoutes(v1Routes, trafficSplit: 90%)
gateway.AddRoutes(v2Routes, trafficSplit: 10%)

// After validation, shift traffic to v2
gateway.UpdateTrafficSplit(v1Routes, 0%)
gateway.UpdateTrafficSplit(v2Routes, 100%)

// Deregister v1 after grace period
gateway.RemoveRoutes(v1Routes)
```

---

## 13. Security Considerations

### 13.1 Schema Access Control

**Backend ACLs**:

```hcl
# Consul ACL policy
service "user-service" {
  policy = "write"
}

key_prefix "/schemas/user-service/" {
  policy = "write"
}

key_prefix "/schemas/" {
  policy = "read"  # Gateways can read all schemas
}
```

### 13.2 Schema Validation

Always validate schemas before publishing:

```go
func (r *Registry) PublishSchema(ctx context.Context, path string, schema interface{}) error {
    // Validate schema format
    if err := validateSchemaFormat(schema); err != nil {
        return fmt.Errorf("invalid schema format: %w", err)
    }
    
    // Check schema size
    data, _ := json.Marshal(schema)
    if len(data) > r.maxSchemaSize {
        return fmt.Errorf("schema too large: %d bytes (max %d)", len(data), r.maxSchemaSize)
    }
    
    // Verify checksum
    expectedHash := calculateChecksum(schema)
    // ...
    
    return r.backend.Put(ctx, path, data)
}
```

### 13.3 TLS for HTTP Schemas

When using HTTP location strategy, enforce HTTPS:

```go
func (g *Gateway) fetchSchemaHTTP(location SchemaLocation) (interface{}, error) {
    // Enforce HTTPS
    u, err := url.Parse(location.URL)
    if err != nil {
        return nil, err
    }
    
    if u.Scheme != "https" {
        return nil, errors.New("schema URL must use HTTPS")
    }
    
    // Use TLS client with certificate verification
    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                MinVersion: tls.VersionTLS12,
            },
        },
    }
    
    // Fetch with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    req, _ := http.NewRequestWithContext(ctx, "GET", location.URL, nil)
    // Add auth headers from location.Headers
    // ...
}
```

### 13.4 Rate Limiting

Prevent DoS via excessive schema updates:

```go
type RateLimiter struct {
    mu         sync.Mutex
    updates    map[string]*tokenBucket
    maxRate    int           // Updates per minute
    bucketSize int
}

func (r *Registry) UpdateManifest(ctx context.Context, manifest *SchemaManifest) error {
    // Check rate limit
    if !r.rateLimiter.Allow(manifest.InstanceID) {
        return errors.New("rate limit exceeded for schema updates")
    }
    
    return r.backend.UpdateManifest(ctx, manifest)
}
```

### 13.5 Audit Logging

Log all schema operations:

```go
func (r *Registry) RegisterManifest(ctx context.Context, manifest *SchemaManifest) error {
    logger.Info("schema manifest registered",
        "service", manifest.ServiceName,
        "version", manifest.ServiceVersion,
        "instance_id", manifest.InstanceID,
        "checksum", manifest.Checksum,
        "schemas", len(manifest.Schemas),
    )
    
    return r.backend.RegisterManifest(ctx, manifest)
}
```

---

## 14. Error Handling

### 14.1 Error Types

```go
// Error types
var (
    ErrManifestNotFound  = errors.New("schema manifest not found")
    ErrSchemaNotFound    = errors.New("schema not found")
    ErrInvalidManifest   = errors.New("invalid manifest format")
    ErrInvalidSchema     = errors.New("invalid schema format")
    ErrSchemaToLarge     = errors.New("schema exceeds size limit")
    ErrChecksumMismatch  = errors.New("schema checksum mismatch")
    ErrUnsupportedType   = errors.New("unsupported schema type")
    ErrBackendUnavailable = errors.New("backend unavailable")
)
```

### 14.2 Error Handling Best Practices

**Service-Side**:

```go
func (p *Publisher) PublishManifest(ctx context.Context) error {
    manifest, err := p.generateManifest(ctx)
    if err != nil {
        // Log but don't fail service startup
        logger.Warn("failed to generate schema manifest", "error", err)
        return nil  // Service can still start
    }
    
    if err := p.registry.RegisterManifest(ctx, manifest); err != nil {
        // Retry with exponential backoff
        return retry.Do(
            func() error {
                return p.registry.RegisterManifest(ctx, manifest)
            },
            retry.Attempts(3),
            retry.Delay(time.Second),
        )
    }
    
    return nil
}
```

**Gateway-Side**:

```go
func (g *Gateway) fetchSchema(descriptor SchemaDescriptor) error {
    // Try primary location
    schema, err := g.fetchSchemaFromLocation(descriptor.Location)
    if err != nil {
        // Try fallback: fetch from HTTP endpoint
        if fallbackURL := g.getFallbackURL(descriptor); fallbackURL != "" {
            schema, err = g.fetchSchemaHTTP(fallbackURL)
            if err == nil {
                return nil
            }
        }
        
        // Use cached schema if available
        if cached, ok := g.schemaCache.Get(descriptor.Hash); ok {
            logger.Warn("using cached schema due to fetch failure", "error", err)
            return nil
        }
        
        return fmt.Errorf("failed to fetch schema: %w", err)
    }
    
    // Validate checksum
    actualHash := calculateChecksum(schema)
    if actualHash != descriptor.Hash {
        return ErrChecksumMismatch
    }
    
    return nil
}
```

---

## 15. Versioning

### 15.1 Protocol Versioning

FARP uses semantic versioning:

- **Major**: Breaking changes to manifest format or interfaces
- **Minor**: Backward-compatible additions (new schema types, fields)
- **Patch**: Bug fixes, documentation updates

Current version: **1.0.0**

### 15.2 Version Negotiation

Services include protocol version in manifest:

```json
{
  "version": "1.0.0",
  ...
}
```

Gateways check version compatibility:

```go
func (g *Gateway) isCompatible(manifest *SchemaManifest) bool {
    manifestVersion := semver.MustParse(manifest.Version)
    gatewayVersion := semver.MustParse(g.protocolVersion)
    
    // Major version must match
    if manifestVersion.Major != gatewayVersion.Major {
        return false
    }
    
    // Gateway must support manifest's minor version or higher
    return gatewayVersion.Minor >= manifestVersion.Minor
}
```

### 15.3 Deprecation Policy

- Deprecated features marked in docs with version
- Removed after 2 major versions
- Example: Feature deprecated in v1.5.0, removed in v3.0.0

---

## 16. Extensibility

### 16.1 Custom Schema Types

Register custom schema providers:

```go
// Define custom schema type
const SchemaTypeCustom SchemaType = "myprotocol"

// Implement provider
type MyProtocolProvider struct{}

func (p *MyProtocolProvider) Type() SchemaType {
    return SchemaTypeCustom
}

func (p *MyProtocolProvider) Generate(ctx context.Context, app Application) (interface{}, error) {
    // Generate custom schema
    return mySchema, nil
}

// Register provider
farp.RegisterSchemaProvider(&MyProtocolProvider{})
```

### 16.2 Custom Metadata

Add custom fields to manifest metadata:

```json
{
  "version": "1.0.0",
  "service_name": "user-service",
  ...
  "x-custom-metadata": {
    "team": "platform",
    "cost-center": "engineering",
    "compliance": "pci-dss"
  }
}
```

### 16.3 Gateway Extensions

Gateways can define custom conversion logic:

```go
type CustomConverter struct{}

func (c *CustomConverter) ConvertToRoutes(manifest *SchemaManifest) ([]Route, error) {
    // Custom route conversion logic
    return routes, nil
}

// Register converter
gateway.RegisterConverter(SchemaTypeCustom, &CustomConverter{})
```

---

## Appendix A: Complete Examples

### A.1 Full Manifest Example

```json
{
  "version": "1.0.0",
  "service_name": "user-service",
  "service_version": "v1.2.3",
  "instance_id": "user-service-abc123-xyz789",
  "schemas": [
    {
      "type": "openapi",
      "spec_version": "3.1.0",
      "location": {
        "type": "registry",
        "registry_path": "/schemas/user-service/v1/openapi"
      },
      "content_type": "application/json",
      "hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "size": 45678,
      "compatibility": {
        "mode": "backward",
        "previous_versions": ["v1.1.0", "v1.0.0"],
        "deprecations": [
          {
            "path": "/paths/users/v1/list",
            "deprecated_at": "2024-01-15T00:00:00Z",
            "removal_date": "2024-07-15T00:00:00Z",
            "replacement": "/users",
            "migration": "Use paginated /users endpoint",
            "reason": "Replaced with paginated API for better performance"
          }
        ]
      },
      "metadata": {
        "openapi": {
          "extensions": {
            "x-api-id": "user-api-v2"
          },
          "default_security": ["bearer", "apikey"]
        }
      }
    },
    {
      "type": "asyncapi",
      "spec_version": "3.0.0",
      "location": {
        "type": "http",
        "url": "https://user-service:8080/asyncapi.json"
      },
      "content_type": "application/json",
      "hash": "d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2d2",
      "size": 12345
    },
    {
      "type": "grpc",
      "spec_version": "proto3",
      "location": {
        "type": "inline"
      },
      "content_type": "application/json",
      "hash": "c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3",
      "size": 8765,
      "inline_schema": {
        "file": [
          {
            "name": "user_service.proto",
            "package": "user",
            "syntax": "proto3"
          }
        ]
      }
    },
    {
      "type": "graphql",
      "spec_version": "2021",
      "location": {
        "type": "http",
        "url": "https://user-service:8080/graphql"
      },
      "content_type": "application/graphql",
      "hash": "f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4",
      "size": 15432,
      "compatibility": {
        "mode": "backward"
      },
      "metadata": {
        "graphql": {
          "federation": {
            "version": "v2",
            "subgraph_name": "users",
            "entities": [
              {
                "type_name": "User",
                "key_fields": ["id"],
                "fields": ["id", "email", "name", "createdAt"],
                "resolvable": true
              }
            ],
            "extends": ["Product"]
          },
          "subscriptions_enabled": true,
          "subscription_protocol": "graphql-transport-ws",
          "complexity_limit": 1000,
          "depth_limit": 10
        }
      }
    },
    {
      "type": "orpc",
      "spec_version": "1.0.0",
      "location": {
        "type": "http",
        "url": "https://user-service:8080/orpc.json"
      },
      "content_type": "application/json",
      "hash": "a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5",
      "size": 9876
    }
  ],
  "capabilities": ["rest", "grpc", "graphql", "websocket", "sse", "rpc"],
  "endpoints": {
    "health": "/health",
    "metrics": "/metrics",
    "openapi": "/openapi.json",
    "asyncapi": "/asyncapi.json",
    "graphql": "/graphql",
    "grpc_reflection": true
  },
  "routing": {
    "strategy": "service",
    "strip_prefix": true,
    "priority": 60,
    "tags": ["public", "rest", "v1"],
    "rewrite": [
      {
        "pattern": "^/api/v1/(.*)$",
        "replacement": "/$1"
      }
    ]
  },
  "auth": {
    "schemes": [
      {
        "type": "bearer",
        "config": {
          "format": "jwt",
          "jwks_url": "https://auth.example.com/.well-known/jwks.json",
          "issuer": "https://auth.example.com",
          "audience": "user-service-api"
        }
      },
      {
        "type": "apikey",
        "config": {
          "in": "header",
          "name": "X-API-Key"
        }
      }
    ],
    "required_scopes": ["users:read", "users:write"],
    "access_control": [
      {
        "path": "/users",
        "methods": ["GET"],
        "roles": ["user", "admin"],
        "allow_anonymous": false
      },
      {
        "path": "/users/*",
        "methods": ["POST", "PUT", "DELETE"],
        "roles": ["admin"],
        "permissions": ["users:write"]
      },
      {
        "path": "/health",
        "methods": ["GET"],
        "allow_anonymous": true
      }
    ],
    "public_routes": ["/health", "/metrics", "/openapi.json"],
    "token_validation_url": "https://user-service:8080/validate-token"
  },
  "webhook": {
    "gateway_webhook": "https://gateway.example.com/farp/webhook",
    "service_webhook": "https://user-service:8080/farp/webhook",
    "secret": "shared-hmac-secret-change-in-production",
    "publish_events": [
      "schema.updated",
      "health.changed",
      "maintenance.mode"
    ],
    "subscribe_events": [
      "ratelimit.changed",
      "circuit.breaker.open",
      "circuit.breaker.closed",
      "traffic.shift"
    ],
    "retry": {
      "max_attempts": 3,
      "initial_delay": "1s",
      "max_delay": "30s",
      "multiplier": 2.0
    },
    "http_routes": {
      "service_routes": [
        {
          "id": "event-poll",
          "path": "/farp/events",
          "method": "GET",
          "type": "event.poll",
          "description": "Poll for pending events (fallback if webhooks fail)",
          "auth_required": true,
          "idempotent": true,
          "timeout": "5s"
        },
        {
          "id": "lifecycle-reload",
          "path": "/admin/reload",
          "method": "POST",
          "type": "lifecycle.reload",
          "description": "Trigger configuration reload",
          "auth_required": true,
          "idempotent": true,
          "timeout": "10s"
        },
        {
          "id": "config-update",
          "path": "/admin/config",
          "method": "PUT",
          "type": "config.update",
          "description": "Update service configuration from gateway",
          "auth_required": true,
          "idempotent": true,
          "timeout": "5s"
        }
      ],
      "gateway_routes": [
        {
          "id": "schema-validate",
          "path": "/farp/schema/validate",
          "method": "POST",
          "type": "schema.validate",
          "description": "Validate schema before publishing",
          "auth_required": true,
          "idempotent": true,
          "timeout": "10s"
        },
        {
          "id": "config-query",
          "path": "/farp/config",
          "method": "GET",
          "type": "config.query",
          "description": "Query gateway configuration for this service",
          "auth_required": true,
          "idempotent": true,
          "timeout": "5s"
        }
      ],
      "polling": {
        "interval": "30s",
        "timeout": "5s",
        "long_polling": true,
        "long_polling_timeout": "60s"
      }
    }
  },
  "hints": {
    "recommended_timeout": "3s",
    "expected_latency": {
      "p50": "15ms",
      "p95": "75ms",
      "p99": "150ms",
      "p999": "500ms"
    },
    "scaling": {
      "auto_scale": true,
      "min_instances": 3,
      "max_instances": 50,
      "target_cpu": 0.65,
      "target_memory": 0.75
    },
    "dependencies": [
      {
        "service_name": "auth-service",
        "schema_type": "grpc",
        "version_range": ">=v2.0.0 <v3.0.0",
        "critical": true,
        "used_operations": ["ValidateToken"]
      }
    ]
  },
  "updated_at": 1698768000,
  "checksum": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
}
```

---

## Appendix B: Backend Comparison

| Feature | Consul | etcd | Kubernetes | mDNS/Bonjour | Redis | Memory |
|---------|--------|------|------------|--------------|-------|--------|
| **Schema Storage** | KV Store | KV Store | ConfigMaps | TXT Records (metadata only) | Strings | In-memory |
| **Watch Support** | ✅ Blocking queries | ✅ Watch API | ✅ Watch API | ✅ Multicast | ✅ Pub/Sub | ✅ Callbacks |
| **Size Limit** | 512 KB | 1.5 MB | 1 MB | ~200 bytes (TXT) | 512 MB | RAM limit |
| **TTL Support** | ✅ | ✅ | ❌ (manual) | ✅ | ✅ | ✅ |
| **ACL** | ✅ | ✅ RBAC | ✅ RBAC | ❌ | ✅ ACL | ❌ |
| **Network Scope** | WAN | WAN | Cluster | Local subnet | WAN | Local |
| **Best For** | Multi-DC | High consistency | K8s-native | Local dev/IoT | High throughput | Development |

---

## Appendix C: Migration Guide

### From Custom Service Discovery

1. Keep existing service registration
2. Add FARP manifest to metadata
3. Publish schemas to backend
4. Update gateway to watch manifests
5. Gradually migrate routes to schema-based configuration

### From Manual Gateway Configuration

1. Generate schemas from existing routes
2. Create FARP manifests
3. Deploy gateway watcher
4. Run in shadow mode (validate but don't apply)
5. Switch to auto-configuration
6. Remove manual route configs

---

**End of Specification v1.0.0**

