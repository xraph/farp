# FARP Gateway Discovery Examples

## Overview

This document provides practical examples for API gateways and service meshes to discover and consume FARP-enabled services across different discovery backends.

---

## mDNS/Bonjour Discovery (Local Development)

### Go Gateway Example

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/xraph/forge/extensions/discovery/backends"
    "github.com/xraph/farp"
)

// GatewayDiscovery handles service discovery for the gateway
type GatewayDiscovery struct {
    backend *backends.MDNSBackend
    routes  map[string]*farp.SchemaManifest
}

func NewGateway() (*GatewayDiscovery, error) {
    // Configure mDNS backend to discover multiple service types
    backend, err := backends.NewMDNSBackend(backends.MDNSConfig{
        Domain: "local.",
        ServiceTypes: []string{
            "_octopus._tcp",  // Custom application services
            "_farp._tcp",     // Generic FARP services
            "_http._tcp",     // Standard HTTP APIs
        },
        BrowseTimeout: 3 * time.Second,
        WatchInterval: 30 * time.Second,
        IPv6:          true,
    })
    if err != nil {
        return nil, err
    }

    if err := backend.Initialize(context.Background()); err != nil {
        return nil, err
    }

    return &GatewayDiscovery{
        backend: backend,
        routes:  make(map[string]*farp.SchemaManifest),
    }, nil
}

// DiscoverServices finds all FARP-enabled services
func (g *GatewayDiscovery) DiscoverServices(ctx context.Context) error {
    // Discover all configured service types
    services, err := g.backend.DiscoverAllTypes(ctx)
    if err != nil {
        return fmt.Errorf("failed to discover services: %w", err)
    }

    fmt.Printf("Found %d services\n", len(services))

    for _, svc := range services {
        // Check if FARP is enabled
        if svc.Metadata["farp.enabled"] != "true" {
            continue
        }

        // Get manifest URL
        manifestURL := svc.Metadata["farp.manifest"]
        if manifestURL == "" {
            fmt.Printf("Service %s has FARP enabled but no manifest URL\n", svc.Name)
            continue
        }

        // Fetch and parse manifest
        manifest, err := g.fetchManifest(ctx, manifestURL)
        if err != nil {
            fmt.Printf("Failed to fetch manifest for %s: %v\n", svc.Name, err)
            continue
        }

        // Store routes
        g.routes[svc.ID] = manifest

        fmt.Printf("✓ Registered routes for %s (service type: %s)\n", 
            svc.Name, 
            svc.Metadata["mdns.service_type"])
    }

    return nil
}

// fetchManifest retrieves the FARP schema manifest
func (g *GatewayDiscovery) fetchManifest(ctx context.Context, url string) (*farp.SchemaManifest, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var manifest farp.SchemaManifest
    if err := json.Unmarshal(body, &manifest); err != nil {
        return nil, err
    }

    return &manifest, nil
}

// WatchForChanges continuously watches for service changes
func (g *GatewayDiscovery) WatchForChanges(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := g.DiscoverServices(ctx); err != nil {
                fmt.Printf("Error discovering services: %v\n", err)
            }
        }
    }
}

func main() {
    gateway, err := NewGateway()
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // Initial discovery
    if err := gateway.DiscoverServices(ctx); err != nil {
        panic(err)
    }

    // Watch for changes
    go gateway.WatchForChanges(ctx)

    // Start gateway server...
    select {}
}
```

---

## Rust Gateway Example (octopus-gateway)

### Configuration

```yaml
# config/gateway.yaml
backends:
  # mDNS/Bonjour discovery for local development
  - type: mdns
    enabled: true
    config:
      # Service types to discover
      service_types:
        - "_octopus._tcp"   # Custom application services
        - "_farp._tcp"      # FARP-enabled services
        - "_http._tcp"      # Standard HTTP APIs
      
      domain: "local."              # mDNS domain
      watch_interval: 30s           # How often to poll
      query_timeout: 5s             # Query timeout
      enable_ipv6: true             # IPv6 support

  # Consul for production
  - type: consul
    enabled: true
    config:
      address: "consul.service.consul:8500"
      datacenter: "dc1"
```

### Implementation

```rust
use std::time::Duration;
use mdns::{Record, RecordKind};
use tokio::time;

pub struct GatewayDiscovery {
    service_types: Vec<String>,
    domain: String,
    watch_interval: Duration,
}

impl GatewayDiscovery {
    pub fn new(config: MdnsConfig) -> Self {
        Self {
            service_types: config.service_types,
            domain: config.domain,
            watch_interval: Duration::from_secs(config.watch_interval),
        }
    }

    /// Discover FARP-enabled services across all configured types
    pub async fn discover_services(&self) -> Result<Vec<ServiceInstance>> {
        let mut all_services = Vec::new();

        for service_type in &self.service_types {
            let services = self.discover_by_type(service_type).await?;
            all_services.extend(services);
        }

        // Filter for FARP-enabled services
        let farp_services: Vec<_> = all_services
            .into_iter()
            .filter(|svc| svc.has_farp_metadata())
            .collect();

        println!("Discovered {} FARP-enabled services", farp_services.len());

        Ok(farp_services)
    }

    /// Discover services by specific mDNS service type
    async fn discover_by_type(&self, service_type: &str) -> Result<Vec<ServiceInstance>> {
        let query = format!("{}.{}", service_type, self.domain);
        
        // Browse for services
        let services = mdns::discover::all(&query, Duration::from_secs(3))?
            .map(|response| {
                let service = ServiceInstance::from_mdns(response);
                service
            })
            .collect::<Vec<_>>()
            .await;

        Ok(services)
    }

    /// Watch for service changes continuously
    pub async fn watch_for_changes(
        &self,
        callback: impl Fn(Vec<ServiceInstance>),
    ) {
        let mut interval = time::interval(self.watch_interval);

        loop {
            interval.tick().await;

            match self.discover_services().await {
                Ok(services) => {
                    callback(services);
                }
                Err(e) => {
                    eprintln!("Discovery error: {}", e);
                }
            }
        }
    }
}

#[derive(Debug, Clone)]
pub struct ServiceInstance {
    pub id: String,
    pub name: String,
    pub address: String,
    pub port: u16,
    pub metadata: HashMap<String, String>,
}

impl ServiceInstance {
    fn from_mdns(response: mdns::Response) -> Self {
        let mut metadata = HashMap::new();

        // Parse TXT records
        for record in response.records() {
            if let RecordKind::TXT(txt) = record.kind {
                for entry in txt {
                    if let Some((key, value)) = entry.split_once('=') {
                        metadata.insert(key.to_string(), value.to_string());
                    }
                }
            }
        }

        Self {
            id: response.instance_name().to_string(),
            name: response.service_name().to_string(),
            address: response.address().to_string(),
            port: response.port(),
            metadata,
        }
    }

    fn has_farp_metadata(&self) -> bool {
        self.metadata.get("farp.enabled") == Some(&"true".to_string())
    }

    pub fn manifest_url(&self) -> Option<&str> {
        self.metadata.get("farp.manifest").map(|s| s.as_str())
    }
}

// Usage
#[tokio::main]
async fn main() {
    let config = MdnsConfig {
        service_types: vec![
            "_octopus._tcp".to_string(),
            "_farp._tcp".to_string(),
        ],
        domain: "local.".to_string(),
        watch_interval: 30,
    };

    let discovery = GatewayDiscovery::new(config);

    // Initial discovery
    let services = discovery.discover_services().await.unwrap();
    
    for svc in &services {
        println!("Service: {} at {}:{}", svc.name, svc.address, svc.port);
        
        if let Some(manifest_url) = svc.manifest_url() {
            // Fetch and process schema manifest
            let manifest = fetch_manifest(manifest_url).await.unwrap();
            configure_routes(&manifest).await;
        }
    }

    // Watch for changes
    discovery.watch_for_changes(|services| {
        println!("Services updated: {} services", services.len());
        // Update routes...
    }).await;
}
```

---

## Consul Discovery (Production)

### Go Gateway Example

```go
package main

import (
    "context"
    "fmt"

    consul "github.com/hashicorp/consul/api"
    "github.com/xraph/farp"
)

type ConsulGateway struct {
    client *consul.Client
}

func NewConsulGateway(address string) (*ConsulGateway, error) {
    config := consul.DefaultConfig()
    config.Address = address

    client, err := consul.NewClient(config)
    if err != nil {
        return nil, err
    }

    return &ConsulGateway{client: client}, nil
}

// DiscoverFARPServices discovers all FARP-enabled services in Consul
func (g *ConsulGateway) DiscoverFARPServices(ctx context.Context) ([]*ServiceInstance, error) {
    // Get all services
    services, _, err := g.client.Catalog().Services(nil)
    if err != nil {
        return nil, err
    }

    var farpServices []*ServiceInstance

    for serviceName := range services {
        // Get service instances
        instances, _, err := g.client.Health().Service(serviceName, "", true, nil)
        if err != nil {
            continue
        }

        for _, instance := range instances {
            // Check for FARP metadata
            if instance.Service.Meta["farp.enabled"] == "true" {
                svc := &ServiceInstance{
                    ID:       instance.Service.ID,
                    Name:     instance.Service.Service,
                    Address:  instance.Service.Address,
                    Port:     instance.Service.Port,
                    Metadata: instance.Service.Meta,
                }
                farpServices = append(farpServices, svc)

                // Fetch manifest
                if manifestURL := svc.Metadata["farp.manifest"]; manifestURL != "" {
                    manifest, _ := fetchManifest(ctx, manifestURL)
                    // Configure routes from manifest...
                    _ = manifest
                }
            }
        }
    }

    return farpServices, nil
}

// WatchServices watches for service changes
func (g *ConsulGateway) WatchServices(ctx context.Context) error {
    // Use Consul's blocking queries for real-time updates
    var waitIndex uint64

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Blocking query with wait index
        services, meta, err := g.client.Catalog().Services(&consul.QueryOptions{
            WaitIndex: waitIndex,
            WaitTime:  5 * time.Minute,
        })
        if err != nil {
            return err
        }

        waitIndex = meta.LastIndex

        // Process service changes
        for serviceName := range services {
            // Check for FARP metadata and update routes...
        }
    }
}
```

---

## Kubernetes Discovery (Production)

### Go Gateway Example

```go
package main

import (
    "context"
    "fmt"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

type K8sGateway struct {
    clientset *kubernetes.Clientset
    namespace string
}

func NewK8sGateway(namespace string) (*K8sGateway, error) {
    // In-cluster config
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, err
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }

    return &K8sGateway{
        clientset: clientset,
        namespace: namespace,
    }, nil
}

// DiscoverFARPServices discovers FARP-enabled services via annotations
func (g *K8sGateway) DiscoverFARPServices(ctx context.Context) error {
    // List services with FARP annotation
    services, err := g.clientset.CoreV1().Services(g.namespace).List(ctx, metav1.ListOptions{
        LabelSelector: "farp.enabled=true",
    })
    if err != nil {
        return err
    }

    for _, svc := range services.Items {
        // Check for FARP annotations
        annotations := svc.Annotations
        if annotations["farp.enabled"] != "true" {
            continue
        }

        manifestURL := annotations["farp.manifest"]
        if manifestURL == "" {
            continue
        }

        // Fetch manifest
        manifest, err := fetchManifest(ctx, manifestURL)
        if err != nil {
            fmt.Printf("Failed to fetch manifest for %s: %v\n", svc.Name, err)
            continue
        }

        // Configure routes from manifest
        fmt.Printf("✓ Configured routes for %s\n", svc.Name)
        _ = manifest
    }

    return nil
}

// WatchServices watches for service changes using Kubernetes Watch API
func (g *K8sGateway) WatchServices(ctx context.Context) error {
    watcher, err := g.clientset.CoreV1().Services(g.namespace).Watch(ctx, metav1.ListOptions{
        LabelSelector: "farp.enabled=true",
    })
    if err != nil {
        return err
    }
    defer watcher.Stop()

    for event := range watcher.ResultChan() {
        svc := event.Object.(*corev1.Service)

        switch event.Type {
        case "ADDED", "MODIFIED":
            // Service added or updated
            if manifestURL := svc.Annotations["farp.manifest"]; manifestURL != "" {
                manifest, _ := fetchManifest(ctx, manifestURL)
                // Update routes...
                _ = manifest
            }

        case "DELETED":
            // Service deleted - remove routes
            fmt.Printf("Service %s deleted\n", svc.Name)
        }
    }

    return nil
}
```

---

## Multi-Backend Gateway (Hybrid)

### Example: Local + Production

```go
package main

import (
    "context"
    "fmt"

    "github.com/xraph/forge/extensions/discovery/backends"
)

type HybridGateway struct {
    mdnsBackend   *backends.MDNSBackend
    consulBackend *backends.ConsulBackend
}

func NewHybridGateway() (*HybridGateway, error) {
    // mDNS for local development
    mdnsBackend, err := backends.NewMDNSBackend(backends.MDNSConfig{
        Domain:       "local.",
        ServiceTypes: []string{"_octopus._tcp", "_farp._tcp"},
    })
    if err != nil {
        return nil, err
    }

    // Consul for production
    consulBackend, err := backends.NewConsulBackend(backends.ConsulConfig{
        Address: "consul.service.consul:8500",
    })
    if err != nil {
        return nil, err
    }

    return &HybridGateway{
        mdnsBackend:   mdnsBackend,
        consulBackend: consulBackend,
    }, nil
}

func (g *HybridGateway) DiscoverAllServices(ctx context.Context) ([]*ServiceInstance, error) {
    var allServices []*ServiceInstance

    // Discover from mDNS
    mdnsServices, err := g.mdnsBackend.DiscoverAllTypes(ctx)
    if err == nil {
        allServices = append(allServices, mdnsServices...)
        fmt.Printf("Found %d services via mDNS\n", len(mdnsServices))
    }

    // Discover from Consul
    consulServices, err := g.consulBackend.ListServices(ctx)
    if err == nil {
        allServices = append(allServices, consulServices...)
        fmt.Printf("Found %d services via Consul\n", len(consulServices))
    }

    // Deduplicate by service ID
    uniqueServices := deduplicateServices(allServices)

    return uniqueServices, nil
}
```

---

## Summary

### Key Takeaways

1. **mDNS Discovery**: Use `ServiceTypes` for multi-type discovery
2. **FARP Filtering**: Check `farp.enabled` metadata
3. **Manifest Fetching**: Use `farp.manifest` URL to retrieve schemas
4. **Watch for Changes**: Poll (mDNS) or use blocking queries (Consul) or Watch API (K8s)
5. **Multi-Backend**: Combine multiple backends for hybrid deployments

### Configuration Patterns

| Environment | Backend | Discovery Method | Watch Strategy |
|------------|---------|------------------|----------------|
| Local Dev | mDNS | Multi-type browse | Poll (30s) |
| Staging | Consul | Catalog API | Blocking queries |
| Production | Kubernetes | Service annotations | Watch API |
| Hybrid | mDNS + Consul | Both | Both |

### FARP Metadata Keys for Gateways

- `farp.enabled` - Check if FARP is available
- `farp.manifest` - Fetch full schema manifest
- `mdns.service_type` - Filter by service type (mDNS only)
- `farp.capabilities` - Determine service capabilities

---

For more information:
- [FARP Specification](SPECIFICATION.md)
- [mDNS Service Type Guide](MDNS_SERVICE_TYPE_GUIDE.md)
- [Provider Implementation Guide](../PROVIDERS_IMPLEMENTATION.md)

