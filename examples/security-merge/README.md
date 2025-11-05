# Security/Auth Merging Example

This example demonstrates how FARP's OpenAPI merger handles authentication and authorization when composing multiple service APIs with different security schemes.

## Security Schemes Demonstrated

### 1. **API Key Authentication**
- **Service**: User Service
- **Type**: `apiKey`
- **Location**: Header (`X-API-Key`)
- **Use case**: Simple API authentication

### 2. **Bearer Token (JWT)**
- **Service**: Order Service
- **Type**: `http` with `bearer` scheme
- **Format**: JWT
- **Use case**: Token-based authentication

### 3. **OAuth 2.0**
- **Services**: Payment Service, Billing Service
- **Type**: `oauth2`
- **Flow**: Authorization Code
- **Conflict**: Both services use "oauth" name → Demonstrates conflict resolution

## Running the Example

```bash
cd examples/security-merge
go run main.go
```

## Expected Output

```
FARP Security/Auth Merging Example
===================================

1. User Service (API Key auth)
2. Order Service (Bearer token auth)
3. Payment Service (OAuth2)
4. Billing Service (OAuth2 - CONFLICTING NAME)

5. Merging all services...

✓ Successfully merged 4 services

Merged Security Schemes:
───────────────────────────────────────────────────────────
  • apiKey                    (type: apiKey, in: header)
  • bearerAuth                (type: http, scheme: bearer)
  • oauth                     (type: oauth2, desc: Payment OAuth)
  • billing-service_oauth     (type: oauth2, desc: Billing OAuth)

📋 Security Conflicts Resolved:
───────────────────────────────────────────────────────────
  1. Scheme: oauth
     Services: [payment-service billing-service]
     Strategy: prefix
     Resolution: Prefixed to billing-service_oauth

📊 Security Summary:
───────────────────────────────────────────────────────────
  Total security schemes: 4
  By type:
    - apiKey: 1
    - http: 1
    - oauth2: 2

✓ Security merging complete!
```

## How It Works

### Conflict Detection

The merger automatically detects when multiple services define security schemes with the same name:

```go
// Payment Service defines "oauth"
payment-service/securitySchemes/oauth

// Billing Service also defines "oauth" (CONFLICT!)
billing-service/securitySchemes/oauth
```

### Conflict Resolution Strategies

#### 1. **Prefix Strategy** (Recommended)
```go
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategyPrefix,
}

// Result: billing-service_oauth
```

#### 2. **Overwrite Strategy**
```go
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategyOverwrite,
}

// Result: Last service's definition wins
```

#### 3. **Error Strategy**
```go
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategyError,
}

// Result: Merge fails with error
```

#### 4. **Skip Strategy**
```go
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategySkip,
}

// Result: Second service's scheme is skipped
```

#### 5. **Merge Strategy**
```go
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategyMerge,
}

// Result: Overwrites with warning
```

### Supported Security Types

#### API Key
```json
{
  "type": "apiKey",
  "name": "X-API-Key",
  "in": "header"
}
```

#### HTTP (Basic, Bearer, Digest)
```json
{
  "type": "http",
  "scheme": "bearer",
  "bearerFormat": "JWT"
}
```

#### OAuth 2.0
```json
{
  "type": "oauth2",
  "flows": {
    "authorizationCode": {
      "authorizationUrl": "https://example.com/oauth/authorize",
      "tokenUrl": "https://example.com/oauth/token",
      "scopes": {
        "read": "Read access",
        "write": "Write access"
      }
    }
  }
}
```

#### OpenID Connect
```json
{
  "type": "openIdConnect",
  "openIdConnectUrl": "https://example.com/.well-known/openid-configuration"
}
```

## Advanced Security Features

### Operation-Level Security

Operations can override global security:

```json
{
  "paths": {
    "/public": {
      "get": {
        "security": []  // No auth required
      }
    },
    "/private": {
      "get": {
        "security": [
          { "bearerAuth": [] }  // Requires bearer token
        ]
      }
    }
  }
}
```

### Security Scopes

OAuth2 operations can require specific scopes:

```json
{
  "security": [
    {
      "oauth": ["read:users", "write:users"]
    }
  ]
}
```

### Multiple Security Options (OR Logic)

```json
{
  "security": [
    { "apiKey": [] },      // Option 1: API Key
    { "bearerAuth": [] }   // OR Option 2: Bearer Token
  ]
}
```

### Combined Security (AND Logic)

```json
{
  "security": [
    {
      "apiKey": [],        // BOTH required
      "bearerAuth": []
    }
  ]
}
```

## Best Practices

### 1. **Use Descriptive Names**
```go
// Good
{Name: "jwt_bearer", Type: "http", Scheme: "bearer"}

// Avoid
{Name: "auth", Type: "http", Scheme: "bearer"}
```

### 2. **Prefix Service-Specific Schemes**
```go
// For service-specific auth
{Name: "user_service_api_key", Type: "apiKey"}
{Name: "order_service_oauth", Type: "oauth2"}
```

### 3. **Share Common Schemes**
```go
// For shared auth across services (use same name)
{Name: "platform_jwt", Type: "http", Scheme: "bearer"}
```

### 4. **Document Security Requirements**
```go
{
    Name: "oauth",
    Type: "oauth2",
    Description: "OAuth 2.0 with authorization code flow. " +
                 "Required scopes: read, write"
}
```

### 5. **Handle Conflicts Explicitly**
```go
// Set conflict strategy per service
Composition: &farp.CompositionConfig{
    ConflictStrategy: farp.ConflictStrategyPrefix, // Explicit
}
```

## Production Considerations

### Security Scheme Validation

The merger validates security schemes:
- Ensures required fields are present
- Checks `in` values for apiKey (header, query, cookie)
- Validates OAuth2 flow configurations
- Verifies OpenID Connect URLs

### Conflict Tracking

All security conflicts are tracked:
```go
for _, conflict := range result.Conflicts {
    if conflict.Type == merger.ConflictTypeSecurityScheme {
        log.Printf("Security conflict: %s between %v",
            conflict.Item, conflict.Services)
    }
}
```

### Multi-Protocol Security

While this example focuses on OpenAPI, security handling is also implemented for:
- **AsyncAPI**: Authentication for message brokers
- **gRPC**: Service-level authentication metadata
- **oRPC**: Procedure-level security requirements

## Related Documentation

- [FARP Specification - Authentication & Authorization](../../docs/SPECIFICATION.md#8-authentication--authorization)
- [OpenAPI Security Schemes](https://spec.openapis.org/oas/v3.1.0#security-scheme-object)
- [Merger Package Documentation](../../merger/)

## Security Types Reference

| Type | Description | Use Case |
|------|-------------|----------|
| `apiKey` | API key authentication | Simple APIs, internal services |
| `http` (basic) | HTTP Basic Auth | Legacy systems |
| `http` (bearer) | Bearer tokens (JWT) | Modern REST APIs |
| `http` (digest) | HTTP Digest Auth | Enhanced security over Basic |
| `oauth2` | OAuth 2.0 flows | Third-party integrations |
| `openIdConnect` | OpenID Connect | Enterprise SSO |
| `mutualTLS` | mTLS | High-security environments |

## Troubleshooting

### Conflict: Same Security Scheme Name

**Problem**: Multiple services define the same security scheme name.

**Solution**: Use `ConflictStrategyPrefix` to automatically prefix scheme names:
```go
ConflictStrategy: farp.ConflictStrategyPrefix
```

### Global vs Operation Security

**Problem**: Operations don't inherit global security.

**Solution**: OpenAPI requires explicit security at operation level. The merger preserves both global and operation-level security.

### Missing Required Fields

**Problem**: Security scheme validation fails.

**Solution**: Ensure all required fields are present:
- apiKey: `name`, `in`
- http: `scheme`
- oauth2: `flows`
- openIdConnect: `openIdConnectUrl`

## Summary

The FARP merger provides comprehensive security/auth handling:

✅ **Automatic conflict detection** for security schemes  
✅ **Flexible resolution strategies** (prefix, skip, error, overwrite, merge)  
✅ **All OpenAPI security types** supported  
✅ **Operation-level security** preserved  
✅ **Validation** of security scheme definitions  
✅ **Conflict tracking** for transparency  
✅ **Multi-protocol support** (OpenAPI, AsyncAPI, gRPC, oRPC)  

This enables secure composition of microservices with different authentication mechanisms into a unified API gateway.

