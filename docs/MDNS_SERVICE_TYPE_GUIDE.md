# mDNS Service Type Configuration for FARP

## Overview

mDNS (Multicast DNS) and Bonjour use **service types** to categorize and filter services on the network. For FARP-enabled services using mDNS/Bonjour discovery, configuring the correct service type is critical for gateway discovery and filtering.

## Why Service Types Matter

### Problem Without Service Types

Without configurable service types, services register as:
```
_<service-name>._tcp.local.
```

For example:
- `_user-service._tcp.local.`
- `_payment-service._tcp.local.`
- `_inventory-service._tcp.local.`

**Issues**:
- ❌ Gateways must know every service name in advance
- ❌ No way to discover "all API services" generically
- ❌ Can't filter by service category (e.g., all HTTP APIs)
- ❌ Doesn't follow mDNS conventions for well-known service types

### Solution With Service Types

With configurable service types, services register as:
```
_<type>._tcp.local.
```

For example:
- `_octopus._tcp.local.` - Custom application services
- `_farp._tcp.local.` - Generic FARP-enabled services
- `_http._tcp.local.` - Standard HTTP services

**Benefits**:
- ✅ Gateways discover by type: "Give me all `_octopus._tcp` services"
- ✅ Service filtering: Only discover relevant service categories
- ✅ Standard mDNS conventions
- ✅ Easier multi-service discovery

---

## Service Type Standards

### Recommended Service Types

| Service Type | Purpose | When to Use |
|-------------|---------|-------------|
| `_farp._tcp` | Generic FARP services | Default for FARP-enabled APIs |
| `_http._tcp` | HTTP-based services | Standard web services and REST APIs |
| `_octopus._tcp` | Custom application type | Application-specific service mesh |
| `_<app>._tcp` | Application-specific | Custom service types for your ecosystem |

### Service Type Format

mDNS service types follow the format:
```
_<service-type>._<protocol>
```

Examples:
- `_octopus._tcp` - TCP-based Octopus services
- `_farp._tcp` - TCP-based FARP services
- `_http._tcp` - TCP-based HTTP services

**Rules**:
- MUST start with underscore (`_`)
- SHOULD use lowercase
- SHOULD use hyphens for multi-word types (e.g., `_my-service._tcp`)
- MUST include protocol (usually `._tcp`)

---

## Configuration

### Service Registration (Forge)

**Default Behavior** (Service name-based):
```go
discovery.New(
    discovery.WithBackend("mdns"),
    // No ServiceType specified → uses "_<service-name>._tcp"
)
```

Result: Service registers as `_kineta._tcp.local.`

**Custom Service Type**:
```go
discovery.New(
    discovery.WithBackend("mdns"),
    discovery.WithMDNS(backends.MDNSConfig{
        ServiceType: "_octopus._tcp",  // Custom type
        Domain:      "local.",
    }),
)
```

Result: Service registers as `_octopus._tcp.local.`

**With TXT Metadata**:
```go
txt := []string{
    "version=1.0.0",
    "farp.enabled=true",
    "farp.manifest=http://192.168.1.100:8080/_farp/manifest",
    "mdns.service_type=_octopus._tcp",  // Included for gateway filtering
}

zeroconf.Register(
    "kineta-instance-123",
    "_octopus._tcp",  // Service type
    "local.",
    8080,
    txt,
    nil,
)
```

### Gateway Discovery (Multi-Type)

**Discover Multiple Service Types**:
```go
backend, _ := backends.NewMDNSBackend(backends.MDNSConfig{
    Domain: "local.",
    ServiceTypes: []string{
        "_octopus._tcp",  // Custom services
        "_farp._tcp",     // FARP services
        "_http._tcp",     // Standard HTTP
    },
    BrowseTimeout: 3 * time.Second,
})

// Discover all configured types
services, _ := backend.DiscoverAllTypes(ctx)

for _, svc := range services {
    // Extract service type from metadata
    serviceType := svc.Metadata["mdns.service_type"]
    
    // Process based on type
    if serviceType == "_octopus._tcp" {
        // Handle Octopus services
    }
}
```

**Single Service Type Discovery**:
```go
backend, _ := backends.NewMDNSBackend(backends.MDNSConfig{
    Domain:      "local.",
    ServiceType: "_octopus._tcp",
})

// Discovers only _octopus._tcp services
services, _ := backend.Discover(ctx, "octopus")
```

---

## Gateway Configuration Examples

### Rust Gateway (octopus-gateway)

```yaml
backends:
  # mDNS/Bonjour discovery for local development
  - type: mdns
    enabled: true
    config:
      # Service types to discover
      service_type: "_octopus._tcp"    # Single type mode
      
      # OR multi-type mode:
      service_types:
        - "_octopus._tcp"   # Custom services
        - "_farp._tcp"      # FARP-enabled services  
        - "_http._tcp"      # Standard HTTP APIs
      
      domain: "local."              # mDNS domain
      watch_interval: 30s           # Poll interval
      query_timeout: 5s             # Query timeout
      enable_ipv6: true             # IPv6 support
```

### YAML Configuration (Forge)

```yaml
extensions:
  discovery:
    enabled: true
    backend: mdns
    
    mdns:
      domain: "local."
      service_type: "_octopus._tcp"  # Register with this type
      ipv6: true
      browse_timeout: 3s
      ttl: 120
    
    farp:
      enabled: true
      # FARP metadata automatically added to TXT records
```

---

## FARP Metadata Integration

### TXT Record Structure

When FARP is enabled, services include:

```
_octopus._tcp.local. IN TXT (
  "version=1.0.0"
  "farp.enabled=true"
  "farp.manifest=http://192.168.1.100:8080/_farp/manifest"
  "farp.openapi=http://192.168.1.100:8080/openapi.json"
  "farp.capabilities=[rest websocket]"
  "mdns.service_type=_octopus._tcp"   ← Service type for filtering
  "mdns.domain=local."
)
```

### Gateway Discovery Flow

```
1. Gateway configured with service_types: ["_octopus._tcp", "_farp._tcp"]
   ↓
2. Gateway browses mDNS for each type
   ↓
3. Receives service entries with TXT records
   ↓
4. Filters by "farp.enabled=true"
   ↓
5. Extracts FARP manifest URL
   ↓
6. Fetches schemas from manifest endpoint
   ↓
7. Configures routes based on schemas
```

---

## Use Cases

### Use Case 1: Application-Specific Service Mesh

**Scenario**: All microservices in the "Octopus" application use `_octopus._tcp`

**Benefits**:
- Gateway only discovers Octopus services
- Ignores other services on the network
- Consistent service type across the mesh

**Configuration**:
```go
// Service
discovery.WithMDNS(backends.MDNSConfig{
    ServiceType: "_octopus._tcp",
})

// Gateway
backend.DiscoverAllTypes(ctx) // with ServiceTypes: ["_octopus._tcp"]
```

### Use Case 2: Multi-Application Gateway

**Scenario**: Gateway handles multiple applications (Octopus, other APIs)

**Benefits**:
- Discovers services from multiple applications
- Filters by service type
- Routes to appropriate upstream

**Configuration**:
```go
// Gateway
backend, _ := backends.NewMDNSBackend(backends.MDNSConfig{
    ServiceTypes: []string{
        "_octopus._tcp",
        "_payment._tcp",
        "_inventory._tcp",
    },
})

services, _ := backend.DiscoverAllTypes(ctx)
```

### Use Case 3: FARP-Only Discovery

**Scenario**: Gateway only cares about FARP-enabled services

**Benefits**:
- Discovers any FARP service regardless of application
- Generic FARP gateway

**Configuration**:
```go
// All FARP services register with _farp._tcp
backend, _ := backends.NewMDNSBackend(backends.MDNSConfig{
    ServiceType: "_farp._tcp",
})

services, _ := backend.Discover(ctx, "farp")
```

---

## Best Practices

### For Services

1. ✅ **Choose a consistent service type** for your application ecosystem
2. ✅ **Include `mdns.service_type` in TXT records** for gateway filtering
3. ✅ **Use `_farp._tcp` for generic FARP services** if no custom type is needed
4. ✅ **Document your service types** in your API gateway configuration

### For Gateways

1. ✅ **Support multiple service types** for flexibility
2. ✅ **Filter by `farp.enabled` TXT record** to identify FARP services
3. ✅ **Use `mdns.service_type` metadata** for routing decisions
4. ✅ **Set reasonable browse timeouts** (3-5 seconds)
5. ✅ **Poll periodically** (30-60 seconds) for service changes

### For FARP Spec Compliance

1. ✅ **Services MUST include `mdns.service_type` in metadata**
2. ✅ **Gateways SHOULD support `ServiceTypes` configuration**
3. ✅ **Default to `_farp._tcp` for generic FARP services**
4. ✅ **Allow custom service types for application-specific meshes**

---

## Troubleshooting

### Service Not Discovered

**Check**:
1. Is the service registered with the correct type?
2. Is the gateway configured to discover that type?
3. Are both on the same network and domain?

**Debug**:
```bash
# List all mDNS services
dns-sd -B _octopus._tcp local.

# Resolve specific service
dns-sd -L "kineta-instance-123" _octopus._tcp local.
```

### Multiple Service Types

If you need to register with multiple types, register separate mDNS instances:

```go
// Register with _octopus._tcp
backend1, _ := backends.NewMDNSBackend(backends.MDNSConfig{
    ServiceType: "_octopus._tcp",
})
backend1.Register(ctx, instance)

// Also register with _farp._tcp
backend2, _ := backends.NewMDNSBackend(backends.MDNSConfig{
    ServiceType: "_farp._tcp",
})
backend2.Register(ctx, instance)
```

---

## Summary

- **Service Type** is the mDNS mechanism for categorizing and filtering services
- **Configurable** via `MDNSConfig.ServiceType` (single) or `ServiceTypes` (multi)
- **Gateway Discovery** uses `ServiceTypes` to browse multiple service categories
- **FARP Integration** requires `mdns.service_type` in TXT records for proper filtering
- **Standard Types**: `_farp._tcp` (generic), `_http._tcp` (HTTP), `_octopus._tcp` (custom)
- **Backend Agnostic**: FARP spec supports service types across all backends

**Recommendation**: Use `_octopus._tcp` for your application-specific service mesh, or `_farp._tcp` for generic FARP services.

