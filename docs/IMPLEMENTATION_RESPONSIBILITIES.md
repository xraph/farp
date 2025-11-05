# FARP Implementation Responsibilities

This document clarifies **exactly** what FARP provides versus what you must implement in your service framework or gateway.

---

## Quick Answer

**FARP is a protocol spec + tooling library**, like `protobuf` or `openapi-generator`. It defines data formats and provides schema generation, but **you must implement the HTTP transport layer**.

---

## For Service Developers

### What FARP Gives You

```go
import (
    "github.com/xraph/farp"
    "github.com/xraph/farp/providers/openapi"
    "github.com/xraph/farp/providers/asyncapi"
)

// 1. Generate schemas from your app
openapiProvider := openapi.NewProvider("3.1.0", "/openapi.json")
openapiSchema, _ := openapiProvider.Generate(ctx, yourApp)

// 2. Create a manifest
manifest := &farp.SchemaManifest{
    ServiceName:    "user-service",
    ServiceVersion: "v1.0.0",
    InstanceID:     "user-service-abc123",
    Schemas: []farp.SchemaDescriptor{
        {
            Type:        farp.SchemaTypeOpenAPI,
            SpecVersion: "3.1.0",
            Location: farp.SchemaLocation{
                Type: farp.LocationTypeHTTP,
                URL:  "http://localhost:8080/openapi.json",
            },
        },
    },
    Endpoints: farp.SchemaEndpoints{
        Health:  "/health",
        Metrics: "/metrics",
        OpenAPI: "/openapi.json",
    },
}

// 3. Validate it
if err := manifest.Validate(); err != nil {
    log.Fatal(err)
}

// 4. Serialize it
manifestJSON, _ := json.Marshal(manifest)
```

### What YOU Must Implement

#### 1. HTTP Endpoints

```go
// In your HTTP router/server:

// Serve the FARP manifest
http.HandleFunc("/_farp/manifest", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write(manifestJSON)
})

// Serve the OpenAPI schema
http.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write(openapiSchema) // From provider
})

// Health endpoint (your existing implementation)
http.HandleFunc("/health", yourHealthHandler)
```

#### 2. Service Discovery Registration

**With Consul:**

```go
import "github.com/hashicorp/consul/api"

// Register service with FARP metadata
client, _ := api.NewClient(api.DefaultConfig())
client.Agent().ServiceRegister(&api.AgentServiceRegistration{
    ID:      "user-service-abc123",
    Name:    "user-service",
    Address: "10.0.1.5",
    Port:    8080,
    Meta: map[string]string{
        "farp.enabled":  "true",
        "farp.manifest": "http://10.0.1.5:8080/_farp/manifest",
        "farp.openapi":  "http://10.0.1.5:8080/openapi.json",
    },
})
```

**With mDNS/Bonjour:**

```go
import "github.com/grandcat/zeroconf"

// Register with FARP metadata in TXT records
txt := []string{
    "version=1.0.0",
    "farp.enabled=true",
    "farp.manifest=http://192.168.1.100:8080/_farp/manifest",
    "farp.openapi=http://192.168.1.100:8080/openapi.json",
}

server, _ := zeroconf.Register(
    "user-service-abc123",
    "_farp._tcp",
    "local.",
    8080,
    txt,
    nil,
)
defer server.Shutdown()
```

**With Kubernetes:**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: user-service
  annotations:
    farp.enabled: "true"
    farp.manifest: "http://user-service:8080/_farp/manifest"
    farp.openapi: "http://user-service:8080/openapi.json"
spec:
  selector:
    app: user-service
  ports:
    - port: 8080
```

#### 3. Optional: Store Schemas in Registry

If using `LocationTypeRegistry`:

```go
// Example with Consul KV store
client, _ := api.NewClient(api.DefaultConfig())

// Store OpenAPI schema
client.KV().Put(&api.KVPair{
    Key:   "/schemas/user-service/v1/openapi",
    Value: openapiSchema,
}, nil)

// Update manifest to point to registry
manifest.Schemas[0].Location = farp.SchemaLocation{
    Type:         farp.LocationTypeRegistry,
    RegistryPath: "/schemas/user-service/v1/openapi",
}
```

---

## For Gateway Developers

### What FARP Gives You

```go
import (
    "github.com/xraph/farp"
    "github.com/xraph/farp/merger"
)

// 1. Parse a manifest
var manifest farp.SchemaManifest
json.Unmarshal(manifestJSON, &manifest)

// 2. Validate it
if err := manifest.Validate(); err != nil {
    // Handle invalid manifest
}

// 3. Merge multiple service schemas (optional)
mergerClient := merger.NewMerger(merger.DefaultMergerConfig())
mergedOpenAPI, _ := mergerClient.MergeOpenAPI([]merger.ServiceSchema{...})
```

### What YOU Must Implement

#### 1. Service Discovery Watcher

**With Consul:**

```go
import "github.com/hashicorp/consul/api"

client, _ := api.NewClient(api.DefaultConfig())

// Watch for service changes
plan, _ := watch.Parse(map[string]interface{}{
    "type":    "service",
    "service": "user-service",
})

plan.Handler = func(idx uint64, data interface{}) {
    services := data.([]*api.ServiceEntry)
    
    for _, svc := range services {
        // Extract FARP manifest URL from metadata
        manifestURL := svc.Service.Meta["farp.manifest"]
        
        // Fetch manifest (see next step)
        manifest := fetchManifest(manifestURL)
        
        // Configure routes (see step 3)
        configureRoutes(manifest)
    }
}

plan.Run(client)
```

**With mDNS:**

```go
import "github.com/grandcat/zeroconf"

resolver, _ := zeroconf.NewResolver(nil)
entries := make(chan *zeroconf.ServiceEntry)

go func() {
    for entry := range entries {
        // Extract FARP manifest URL from TXT records
        for _, txt := range entry.Text {
            if strings.HasPrefix(txt, "farp.manifest=") {
                manifestURL := strings.TrimPrefix(txt, "farp.manifest=")
                
                // Fetch manifest
                manifest := fetchManifest(manifestURL)
                
                // Configure routes
                configureRoutes(manifest)
            }
        }
    }
}()

resolver.Browse(context.Background(), "_farp._tcp", "local.", entries)
```

#### 2. HTTP Client to Fetch Schemas

```go
func fetchManifest(url string) (*farp.SchemaManifest, error) {
    client := &http.Client{Timeout: 5 * time.Second}
    
    resp, err := client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    var manifest farp.SchemaManifest
    if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
        return nil, err
    }
    
    return &manifest, nil
}

func fetchSchema(descriptor farp.SchemaDescriptor) ([]byte, error) {
    switch descriptor.Location.Type {
    case farp.LocationTypeHTTP:
        return fetchHTTPSchema(descriptor.Location.URL)
        
    case farp.LocationTypeRegistry:
        return fetchRegistrySchema(descriptor.Location.RegistryPath)
        
    case farp.LocationTypeInline:
        return json.Marshal(descriptor.InlineSchema)
        
    default:
        return nil, fmt.Errorf("unsupported location type")
    }
}

func fetchHTTPSchema(url string) ([]byte, error) {
    client := &http.Client{Timeout: 10 * time.Second}
    
    resp, err := client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    return io.ReadAll(resp.Body)
}
```

#### 3. Convert Schemas to Routes

```go
func configureRoutes(manifest *farp.SchemaManifest) error {
    for _, descriptor := range manifest.Schemas {
        schema, err := fetchSchema(descriptor)
        if err != nil {
            return err
        }
        
        switch descriptor.Type {
        case farp.SchemaTypeOpenAPI:
            routes := convertOpenAPIToRoutes(manifest, schema)
            applyRoutes(routes)
            
        case farp.SchemaTypeAsyncAPI:
            routes := convertAsyncAPIToRoutes(manifest, schema)
            applyRoutes(routes)
            
        case farp.SchemaTypeGRPC:
            // gRPC-specific handling
            configureGRPCRoutes(manifest, schema)
        }
    }
    
    return nil
}

func convertOpenAPIToRoutes(manifest *farp.SchemaManifest, schemaJSON []byte) []Route {
    var openapi map[string]interface{}
    json.Unmarshal(schemaJSON, &openapi)
    
    paths := openapi["paths"].(map[string]interface{})
    routes := []Route{}
    
    // Get service instance address
    serviceAddr := manifest.Instance.Address
    servicePort := manifest.Instance.Port
    baseURL := fmt.Sprintf("http://%s:%d", serviceAddr, servicePort)
    
    // Apply routing strategy
    basePath := ""
    switch manifest.Routing.Strategy {
    case farp.MountStrategyRoot:
        basePath = "/"
    case farp.MountStrategyService:
        basePath = "/" + manifest.ServiceName
    case farp.MountStrategyCustom:
        basePath = manifest.Routing.BasePath
    }
    
    for path, methods := range paths {
        routes = append(routes, Route{
            Path:      basePath + path,
            TargetURL: baseURL + path,
            Methods:   extractMethods(methods),
        })
    }
    
    return routes
}

func applyRoutes(routes []Route) {
    // Apply to YOUR gateway's routing table
    // This is gateway-specific (Kong, Traefik, Envoy, custom, etc.)
    
    for _, route := range routes {
        yourGateway.AddRoute(route)
    }
}
```

#### 4. Health Monitoring (Optional but Recommended)

```go
func monitorHealth(manifest *farp.SchemaManifest) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    healthURL := fmt.Sprintf("http://%s:%d%s",
        manifest.Instance.Address,
        manifest.Instance.Port,
        manifest.Endpoints.Health,
    )
    
    for range ticker.C {
        resp, err := http.Get(healthURL)
        if err != nil || resp.StatusCode != 200 {
            // Mark service as unhealthy
            yourGateway.MarkUnhealthy(manifest.InstanceID)
        } else {
            // Mark service as healthy
            yourGateway.MarkHealthy(manifest.InstanceID)
        }
    }
}
```

---

## Summary: FARP's Scope

### ✅ FARP Provides

| Component | Package | Purpose |
|-----------|---------|---------|
| **Type definitions** | `types.go` | Data structures for manifests, routing, auth, etc. |
| **Schema providers** | `providers/*` | Generate schemas from code (OpenAPI, gRPC, etc.) |
| **Validation** | `manifest.go` | Ensure manifests are valid |
| **Schema merging** | `merger/*` | Combine multiple schemas into unified docs |
| **Registry interface** | `registry.go` | Abstract storage operations |

### ❌ FARP Does NOT Provide

| Component | Who Implements | Where |
|-----------|----------------|-------|
| **HTTP servers** | Service framework | Your service code |
| **HTTP clients** | Gateway | Your gateway code |
| **Service discovery integration** | Both | Your code + discovery backend |
| **Registry backend implementations** | External | Use Consul, etcd, K8s client libraries |
| **Route configuration** | Gateway | Your gateway's routing logic |
| **Health monitoring** | Gateway | Your gateway's health check system |

---

## Real-World Examples

### Example: Forge Framework (Service)

```go
// In Forge framework's startup code:

// 1. Use FARP providers
openapiProvider := openapi.NewForgeProvider(forgeApp)
asyncapiProvider := asyncapi.NewForgeProvider(forgeApp)

openapi := openapiProvider.Generate(ctx, forgeApp)
asyncapi := asyncapiProvider.Generate(ctx, forgeApp)

// 2. Create manifest
manifest := farp.NewManifest(
    "user-service",
    "v1.0.0",
    "user-service-abc123",
)

manifest.AddSchema(farp.SchemaTypeOpenAPI, "3.1.0", farp.SchemaLocation{
    Type: farp.LocationTypeHTTP,
    URL:  "http://localhost:8080/openapi.json",
})

// 3. Expose via HTTP
forgeApp.GET("/_farp/manifest", func(c *forge.Context) error {
    return c.JSON(manifest)
})

forgeApp.GET("/openapi.json", func(c *forge.Context) error {
    return c.JSON(openapi)
})

// 4. Register with mDNS
mdnsBackend.Register(ctx, manifest)
```

### Example: octopus-gateway (Gateway)

```rust
// In octopus-gateway's startup code:

// 1. Watch mDNS for services
let mut browser = mdns::Browser::new("_farp._tcp")?;

browser.on_service_discovered(|entry| {
    // Extract manifest URL
    let manifest_url = entry.txt_records
        .iter()
        .find(|txt| txt.starts_with("farp.manifest="))
        .map(|txt| txt.strip_prefix("farp.manifest=").unwrap());
    
    // Fetch manifest
    let manifest = fetch_manifest(manifest_url)?;
    
    // Fetch schemas
    for descriptor in manifest.schemas {
        let schema = fetch_schema(&descriptor)?;
        
        // Convert to routes
        let routes = convert_to_routes(&manifest, &schema);
        
        // Apply to gateway
        routing_table.apply(routes);
    }
});

browser.start()?;
```

---

## Decision Tree: What Should I Implement?

### I'm Building a Service Framework

**You should implement:**
1. ✅ HTTP endpoints to serve `/_farp/manifest` and schemas
2. ✅ Integration with discovery backend (Consul, mDNS, K8s)
3. ✅ Call FARP providers to generate schemas
4. ✅ Health and metrics endpoints

**You should use from FARP:**
- Type definitions (`SchemaManifest`, etc.)
- Schema providers (`providers/openapi`, etc.)
- Validation (`manifest.Validate()`)

### I'm Building an API Gateway

**You should implement:**
1. ✅ HTTP client to fetch manifests and schemas
2. ✅ Watcher for discovery backend (Consul, mDNS, K8s)
3. ✅ Schema-to-route conversion logic
4. ✅ Gateway-specific route application
5. ✅ Health monitoring

**You should use from FARP:**
- Type definitions (`SchemaManifest`, etc.)
- Schema merging (`merger/*`) for unified docs
- Validation (`manifest.Validate()`)

### I'm Building Both (Monorepo)

**Service side:**
- Implement HTTP endpoints
- Use FARP providers

**Gateway side:**
- Implement HTTP client
- Implement route conversion
- Use FARP merger (optional)

---

## FAQs

### Q: Why doesn't FARP include HTTP client/server?

**A:** FARP is a protocol spec, not a framework. Including HTTP transport would:
- Create unnecessary dependencies
- Limit flexibility (what if you want gRPC transport?)
- Increase complexity
- Violate single responsibility principle

Think of it like **Protocol Buffers**: protobuf defines the format, but you choose the transport (gRPC, HTTP, message queues, etc.).

### Q: Is `gateway/client.go` production-ready?

**A:** No, it's a **reference implementation** showing how to integrate FARP. Real gateways should implement their own logic tailored to their architecture, error handling, and performance requirements.

### Q: Should I use `LocationTypeHTTP` or `LocationTypeRegistry`?

**A:** 
- **HTTP** - Simpler, no backend storage needed, service controls freshness
- **Registry** - More reliable, schemas persist even if service dies, faster gateway startup
- **Both (Hybrid)** - Best for production, high availability

### Q: Can I use FARP with non-Go services?

**A:** Yes! FARP is just a JSON format. Any language can:
1. Generate a `SchemaManifest` JSON document
2. Expose it via HTTP
3. Register with discovery backend

The Go library is a reference implementation.

---

## Next Steps

1. **For Services**: Read [Forge FARP Integration Guide](../extensions/discovery/FORGE_INTEGRATION.md)
2. **For Gateways**: Read [Gateway Discovery Examples](GATEWAY_DISCOVERY_EXAMPLES.md)
3. **For Both**: Read [FARP Specification](SPECIFICATION.md)

