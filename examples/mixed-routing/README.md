# Mixed Routing Strategies Example

This example demonstrates how different services can use different routing strategies when being composed into a unified OpenAPI specification.

## Routing Strategies Demonstrated

### 1. **Root Mounting** (`MountStrategyRoot`)
- **Service**: Public API
- **Original path**: `/health`
- **Merged path**: `/health` (no prefix)
- **Use case**: Public-facing endpoints that should be at the root

### 2. **Service-Based Mounting** (`MountStrategyService`)
- **Service**: Admin Service
- **Original path**: `/users`
- **Merged path**: `/admin-service/users`
- **Use case**: Group endpoints by service name

### 3. **Custom Path Mounting** (`MountStrategyCustom`)
- **Service**: Legacy API
- **Original path**: `/orders`
- **Merged path**: `/api/legacy/orders`
- **Use case**: Maintain specific URL structure for legacy systems

### 4. **Versioned Mounting** (`MountStrategyVersioned`)
- **Service**: User API v3.1.0
- **Original path**: `/users`
- **Merged path**: `/user-api/v3.1.0/users`
- **Use case**: API versioning with version in URL

### 5. **Instance-Based Mounting** (`MountStrategyInstance`)
- **Service**: Cache Service (instance: cache-east-1)
- **Original path**: `/get`
- **Merged path**: `/cache-east-1/get`
- **Use case**: Multi-instance deployments (regional, datacenter-specific)

## Running the Example

```bash
cd examples/mixed-routing
go run main.go
```

## Expected Output

```
FARP Mixed Routing Strategies Example
======================================

1. Public API Service (Root mounting)
2. Admin Service (Service mounting)
3. Legacy Service (Custom path mounting)
4. Versioned Service (Versioned mounting)
5. Multi-Instance Service (Instance mounting)

6. Merging all services...

✓ Successfully merged 5 services

Merged API Endpoints (showing routing strategies):
───────────────────────────────────────────────────────────

📍 Root Mounted (/):
  /health
  /version

📍 Service Mounted (/service-name/):
  /admin-service/users
  /admin-service/config

📍 Custom Path Mounted (/api/legacy/):
  /api/legacy/orders
  /api/legacy/products

📍 Versioned Mounted (/service/version/):
  /user-api/v3.1.0/users
  /user-api/v3.1.0/profile

📍 Instance Mounted (/instance-id/):
  /cache-east-1/get
  /cache-east-1/set

───────────────────────────────────────────────────────────

Total Endpoints: 10

✓ All routing strategies successfully applied!
```

## Configuring Routing Strategy

Each service configures its routing strategy in the manifest:

```go
// Root mounting
manifest.Routing.Strategy = farp.MountStrategyRoot

// Service mounting
manifest.Routing.Strategy = farp.MountStrategyService

// Custom path mounting
manifest.Routing.Strategy = farp.MountStrategyCustom
manifest.Routing.BasePath = "/api/legacy"

// Versioned mounting
manifest.Routing.Strategy = farp.MountStrategyVersioned

// Instance mounting
manifest.Routing.Strategy = farp.MountStrategyInstance
```

## Use Cases

### Root Mounting
- Health checks
- Status endpoints
- Public APIs
- Documentation endpoints

### Service Mounting
- Microservices architecture
- Clear service boundaries
- Easy service identification

### Custom Path Mounting
- Legacy API compatibility
- Specific URL requirements
- Marketing/branding URLs
- API gateway transformations

### Versioned Mounting
- API versioning
- Side-by-side deployments
- Gradual migration
- Breaking changes management

### Instance Mounting
- Multi-region deployments
- Load balancing
- A/B testing
- Canary deployments
- Geographic routing

## Mixing Strategies

The merger **automatically applies each service's routing strategy** before composition, allowing you to:

1. Have public APIs at root `/`
2. Group internal APIs under `/admin-service/`
3. Maintain legacy paths at `/api/legacy/`
4. Version new APIs at `/user-api/v3.1.0/`
5. Route to specific instances at `/cache-east-1/`

All in a **single unified OpenAPI specification**!

## Conflict Resolution

When using root mounting for multiple services, you may encounter path conflicts. The merger handles this with the configured conflict strategy:

```go
// In the manifest's composition config
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategyPrefix, // or error, skip, overwrite, merge
}
```

## Benefits

1. **Flexibility**: Each service chooses its own routing strategy
2. **Compatibility**: Maintain legacy URLs while introducing new patterns
3. **Organization**: Clear separation of concerns via paths
4. **Versioning**: Support multiple API versions simultaneously
5. **Multi-Region**: Route to specific instances based on location
6. **Zero Conflicts**: Automatic conflict resolution with prefixing

## Related Documentation

- [FARP Specification - Route Mounting Strategies](../../docs/SPECIFICATION.md#7-route-mounting-strategies)
- [FARP Specification - OpenAPI Composition](../../docs/SPECIFICATION.md#127-openapi-schema-composition)
- [Merger Package](../../merger/)

