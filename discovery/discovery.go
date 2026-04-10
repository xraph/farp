// Package discovery provides pluggable service discovery for FARP.
//
// It supports three discovery modes:
//
//   - Registry-based (pull): Services register in Consul/etcd/K8s/Redis/mDNS,
//     gateways watch the registry for changes.
//   - Push-based (reverse): Services push their manifest directly to the gateway
//     via HTTP. No external registry needed.
//   - Hybrid: Registry for discovery + direct push for fast propagation.
//
// # Service Side
//
// Use ServiceNode to auto-register, serve FARP HTTP endpoints, and manage
// the full lifecycle:
//
//	node, _ := discovery.NewServiceNode(discovery.ServiceNodeConfig{
//	    ServiceName: "user-service",
//	    Address:     "10.0.0.5:8080",
//	    Discovery:   consulBackend,
//	})
//	node.Start(ctx)
//	defer node.Stop(ctx)
//	http.Handle("/_farp/", node.HTTPHandler())
//
// # Gateway Side
//
// Use GatewayNode to auto-discover services, fetch manifests, and manage routes:
//
//	gw, _ := discovery.NewGatewayNode(discovery.GatewayNodeConfig{
//	    Discovery:       consulBackend,
//	    OnRoutesChanged: updateRoutes,
//	})
//	gw.Start(ctx)
//	defer gw.Stop(ctx)
package discovery

import (
	"context"
	"time"

	"github.com/xraph/farp"
)

// ServiceInstance represents a discovered service instance in infrastructure.
// This is the network-level presence of a service, distinct from SchemaManifest
// which describes the API contracts.
type ServiceInstance struct {
	// Unique ID for this instance (maps to SchemaManifest.InstanceID)
	ID string `json:"id"`

	// Service name (maps to SchemaManifest.ServiceName)
	ServiceName string `json:"service_name"`

	// Service version (maps to SchemaManifest.ServiceVersion)
	Version string `json:"version,omitempty"`

	// Network address (host or host:port)
	Address string `json:"address"`

	// Port number
	Port int `json:"port"`

	// Health status
	Status farp.InstanceStatus `json:"status"`

	// Tags for filtering and labeling
	Tags []string `json:"tags,omitempty"`

	// Metadata tags (key-value pairs for filtering, labeling)
	Metadata map[string]string `json:"metadata,omitempty"`

	// When this instance was registered
	RegisteredAt time.Time `json:"registered_at"`

	// When health was last reported
	LastHealthCheck time.Time `json:"last_health_check"`
}

// DiscoveryEvent represents a change in service discovery.
type DiscoveryEvent struct {
	Type      farp.EventType  `json:"type"`
	Instance  ServiceInstance `json:"instance"`
	Timestamp time.Time       `json:"timestamp"`

	// Manifest is populated when available (e.g., push mode includes it directly).
	// For registry-based discovery, this may be nil and the gateway fetches it separately.
	Manifest *farp.SchemaManifest `json:"manifest,omitempty"`
}

// DiscoveryEventHandler is called when a service discovery event occurs.
type DiscoveryEventHandler func(event DiscoveryEvent)

// ServiceDiscovery provides pluggable service discovery operations.
// Implementations wrap infrastructure-specific discovery mechanisms
// (Consul, etcd, Kubernetes, Redis, mDNS, or push-based).
type ServiceDiscovery interface {
	// Discover returns all known instances of a service.
	// If serviceName is empty, returns all instances across all services.
	Discover(ctx context.Context, serviceName string) ([]ServiceInstance, error)

	// Watch watches for changes to instances of a service.
	// The handler is called when instances are added, removed, or change health status.
	// If serviceName is empty, watches all services.
	// The watch runs until the context is cancelled.
	Watch(ctx context.Context, serviceName string, handler DiscoveryEventHandler) error

	// Register registers a service instance in the discovery backend.
	Register(ctx context.Context, instance ServiceInstance) error

	// Deregister removes a service instance from the discovery backend.
	Deregister(ctx context.Context, instanceID string) error

	// ReportHealth reports the health status of a registered instance.
	ReportHealth(ctx context.Context, instanceID string, status farp.InstanceStatus) error

	// Close closes the discovery backend connection and releases resources.
	Close() error

	// Health returns nil if the discovery backend itself is reachable.
	Health(ctx context.Context) error
}

// ManifestFetcher fetches a FARP manifest from a live service instance.
// Used by GatewayNode when discovery events don't include the manifest directly.
type ManifestFetcher interface {
	// FetchManifest fetches the SchemaManifest from a service instance.
	FetchManifest(ctx context.Context, instance ServiceInstance) (*farp.SchemaManifest, error)
}

// NamedDiscovery is an optional interface that discovery backends can implement
// to provide identity and initialization. This is useful for frameworks (like Forge)
// that need to manage backend lifecycle.
type NamedDiscovery interface {
	// Name returns the backend name (e.g., "consul", "etcd", "mdns").
	Name() string

	// Initialize performs any deferred initialization.
	Initialize(ctx context.Context) error
}

// TagDiscovery is an optional interface for backends that support tag-based filtering.
type TagDiscovery interface {
	// DiscoverWithTags returns instances matching all given tags.
	DiscoverWithTags(ctx context.Context, serviceName string, tags []string) ([]ServiceInstance, error)
}

// ListableDiscovery is an optional interface for backends that can list all service names.
type ListableDiscovery interface {
	// ListServices returns all known service names.
	ListServices(ctx context.Context) ([]string, error)
}
