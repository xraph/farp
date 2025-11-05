# OpenAPI Schema Composition Example

This example demonstrates how to use FARP's OpenAPI schema composition feature to automatically merge multiple microservice API specifications into a unified gateway API.

## Overview

The example simulates three microservices:
- **User Service**: Manages user accounts and profiles
- **Product Service**: Manages product catalog
- **Order Service**: Manages customer orders

Each service registers its OpenAPI schema with FARP, and the gateway client automatically composes them into a single unified API specification.

## Features Demonstrated

1. **Service Registration**: Multiple services register with composition metadata
2. **Conflict Resolution**: Uses `prefix` strategy to avoid path/component conflicts
3. **Routing Strategies**: Each service mounted under its service name path
4. **Component Prefixing**: Schemas prefixed to avoid naming conflicts (User_User, Product_Product, Order_Order)
5. **Tag Management**: Operations tagged by service for documentation grouping
6. **Unified Output**: Single OpenAPI 3.1 specification with all services

## Running the Example

```bash
# From the farp root directory
cd examples/composition
go run main.go
```

## Expected Output

```
FARP OpenAPI Schema Composition Example
========================================

1. Registering User Service...
2. Registering Product Service...
3. Registering Order Service...

4. Generating Merged OpenAPI Specification...

✓ Successfully merged 3 services
  Services: [user-service product-service order-service]

Merged Specification Details:
  Title: Unified E-Commerce API
  Version: 1.0.0
  Total Paths: 7
  Total Components: 3

API Endpoints:
  - /user-service/users
  - /user-service/users/{id}
  - /product-service/products
  - /product-service/products/{id}
  - /order-service/orders
  - /order-service/orders/{id}

✓ No conflicts encountered

5. Exporting Merged Specification...

Merged OpenAPI Specification (truncated):
{
  "openapi": "3.1.0",
  "info": {
    "title": "Unified E-Commerce API",
    "description": "Composed API from User, Product, and Order services",
    "version": "1.0.0"
  },
  ...
}

✓ Composition complete!
```

## Composition Configuration

Each service configures its composition behavior via `CompositionConfig`:

```go
Composition: &farp.CompositionConfig{
    IncludeInMerged:   true,                    // Include in merged spec
    ComponentPrefix:   "User",                  // Prefix for components
    TagPrefix:         "Users",                 // Prefix for tags
    OperationIDPrefix: "UserAPI",              // Prefix for operation IDs
    ConflictStrategy:  farp.ConflictStrategyPrefix, // How to handle conflicts
}
```

## Conflict Strategies

The example uses `ConflictStrategyPrefix` which automatically prefixes conflicting items with the service name. Other available strategies:

- `ConflictStrategyError`: Fail composition on conflicts
- `ConflictStrategySkip`: Skip conflicting items from current service
- `ConflictStrategyOverwrite`: Use current service's version
- `ConflictStrategyMerge`: Attempt to intelligently merge

## Routing Strategies

Each service uses `MountStrategyService` which mounts paths under `/service-name/`. Other available strategies:

- `MountStrategyRoot`: Mount at root path (no prefix)
- `MountStrategyInstance`: Mount under instance ID
- `MountStrategyVersioned`: Mount under `/service/version/`
- `MountStrategyCustom`: Use custom base path
- `MountStrategySubdomain`: Use subdomain routing

## Customizing the Merger

You can customize the merger behavior with `MergerConfig`:

```go
mergerConfig := merger.MergerConfig{
    DefaultConflictStrategy: farp.ConflictStrategyPrefix,
    MergedTitle:             "My Unified API",
    MergedDescription:       "Custom description",
    MergedVersion:           "1.0.0",
    IncludeServiceTags:      true,
    SortOutput:              true,
    Servers: []merger.Server{
        {
            URL:         "https://api.example.com",
            Description: "Production API Gateway",
        },
    },
}

client := gateway.NewClientWithConfig(registry, mergerConfig)
```

## Use Cases

This composition feature is useful for:

1. **API Aggregation Gateways**: Expose unified API from multiple microservices
2. **API Documentation Portals**: Generate single docs site for all services
3. **Multi-Tenant Systems**: Compose per-tenant service schemas
4. **API Versioning**: Support side-by-side old and new versions
5. **Service Migration**: Gradually migrate while exposing unified API

## Production Considerations

- **Performance**: Schemas are parsed and cached once, recomposed on changes
- **Scalability**: Handles hundreds of services efficiently
- **Memory**: ~1-5MB per OpenAPI schema
- **Composition Time**: <100ms for typical deployments
- **Error Handling**: Per-service error isolation, composition continues if one fails

## Related Documentation

- [FARP Specification - OpenAPI Schema Composition](../../docs/SPECIFICATION.md#127-openapi-schema-composition)
- [Merger Package Documentation](../../merger/)
- [Gateway Client Documentation](../../gateway/)

