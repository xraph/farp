package discovery

import (
	"net/http"
	"time"

	"github.com/xraph/farp"
	"github.com/xraph/farp/gateway"
)

// ServiceNodeConfig configures a ServiceNode.
type ServiceNodeConfig struct {
	// Required: Service identity
	ServiceName    string
	ServiceVersion string
	InstanceID     string // auto-generated UUID if empty
	Address        string // host:port this service listens on

	// Discovery mode (use one):
	// Option A: Registry-based — register in external backend
	Discovery ServiceDiscovery
	// Option B: Push-based — push manifest directly to gateway
	GatewayURL string // e.g., "http://gateway:9090/_farp/v1"

	// Optional: SchemaRegistry for publishing manifest to KV store
	Registry farp.SchemaRegistry

	// Optional: Schema providers for auto-generating schemas
	Providers []farp.SchemaProvider

	// Health/TTL configuration
	HealthInterval time.Duration // default: 10s
	TTL            time.Duration // default: 30s
	MaxRetries     int           // default: 5
	RetryBackoff   time.Duration // default: 2s

	// Tags for service instance filtering/labeling
	Tags []string

	// Metadata is additional key-value metadata to include on the registered
	// ServiceInstance. These are merged with the auto-generated FARP metadata
	// keys (farp.enabled, farp.openapi, etc.) — user-provided keys take precedence.
	Metadata map[string]string

	// Endpoints configures the service's introspection endpoints.
	// These flow into the manifest and are also advertised as farp.* metadata
	// keys on the registered ServiceInstance (e.g., farp.openapi, farp.health).
	Endpoints farp.SchemaEndpoints

	// Routing configuration (flows into manifest)
	MountStrategy farp.MountStrategy
	BasePath      string
	PathRules     []farp.PathRule

	// Routes provides route information for OpenAPI schema generation.
	// Can be []farp.RouteDescriptor, map[string]any (OpenAPI paths), or any
	// type that the configured schema provider understands.
	// If nil, the provider's Generate() may fail or produce empty paths.
	Routes any

	// Service hints (flows into manifest)
	Hints *farp.ServiceHints

	// HTTP client for push mode (optional, uses default if nil)
	HTTPClient *http.Client
}

func (c *ServiceNodeConfig) setDefaults() {
	if c.HealthInterval == 0 {
		c.HealthInterval = 10 * time.Second
	}

	if c.TTL == 0 {
		c.TTL = 30 * time.Second
	}

	if c.MaxRetries == 0 {
		c.MaxRetries = 5
	}

	if c.RetryBackoff == 0 {
		c.RetryBackoff = 2 * time.Second
	}

	if c.MountStrategy == "" {
		c.MountStrategy = farp.MountStrategyService
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
}

// GatewayNodeConfig configures a GatewayNode.
type GatewayNodeConfig struct {
	// Discovery mode (use one or both):
	// Option A: Registry-based — watch external backend for services
	Discovery ServiceDiscovery
	// Option B: Push-based — accept manifest pushes from services
	EnablePush bool

	// Optional: SchemaRegistry for storing discovered manifests.
	// If nil, uses an in-memory registry internally.
	Registry farp.SchemaRegistry

	// Route change callback — called when routes change.
	// Use this for simple integrations.
	OnRoutesChanged func(routes []gateway.ServiceRoute)

	// OnServiceEvent is called when a service discovery event occurs
	// (add, update, remove). Called before manifest processing, so the
	// gateway can track service lifecycle independently of routes.
	OnServiceEvent func(event DiscoveryEvent)

	// Or use atomic swap handler for zero-downtime route updates.
	RouteHandler farp.RouteUpdateHandler

	// Which services to watch (empty = all services)
	ServiceNames []string

	// HTTP client for fetching manifests from services (registry mode)
	HTTPClient *http.Client

	// ManifestFetcher for custom manifest fetching logic.
	// If nil, uses HTTPManifestFetcher (GET /_farp/manifest).
	Fetcher ManifestFetcher

	// How often to poll for health (for backends that don't push health)
	HealthPollInterval time.Duration // default: 15s

	// Push mode: how long before an instance with no heartbeat is removed
	HeartbeatTimeout time.Duration // default: 30s
}

func (c *GatewayNodeConfig) setDefaults() {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	if c.HealthPollInterval == 0 {
		c.HealthPollInterval = 15 * time.Second
	}

	if c.HeartbeatTimeout == 0 {
		c.HeartbeatTimeout = 30 * time.Second
	}
}
