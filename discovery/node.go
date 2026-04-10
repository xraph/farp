package discovery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xraph/farp"
	"github.com/xraph/farp/gateway"
	"github.com/xraph/farp/registry/memory"
)

// ServiceNode manages the full FARP lifecycle for a service:
// schema generation, manifest building, HTTP endpoint serving,
// discovery registration, health reporting, and graceful shutdown.
type ServiceNode struct {
	config   ServiceNodeConfig
	manifest *farp.SchemaManifest
	handler  *FARPHandler
	schemas  map[farp.SchemaType]any
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.RWMutex
}

// NewServiceNode creates a new ServiceNode.
// Either config.Discovery or config.GatewayURL must be set.
func NewServiceNode(config ServiceNodeConfig) (*ServiceNode, error) {
	config.setDefaults()

	if config.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	if config.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	if config.Discovery == nil && config.GatewayURL == "" {
		return nil, fmt.Errorf("either Discovery or GatewayURL must be set")
	}

	if config.InstanceID == "" {
		config.InstanceID = generateInstanceID()
	}

	// Build manifest
	manifest := farp.NewManifest(config.ServiceName, config.ServiceVersion, config.InstanceID)
	manifest.Routing.Strategy = config.MountStrategy
	manifest.Routing.BasePath = config.BasePath

	// Wire endpoints from config, falling back to FARP defaults
	manifest.Endpoints.Health = "/_farp/health"
	if config.Endpoints.Health != "" {
		manifest.Endpoints.Health = config.Endpoints.Health
	}
	if config.Endpoints.OpenAPI != "" {
		manifest.Endpoints.OpenAPI = config.Endpoints.OpenAPI
	}
	if config.Endpoints.AsyncAPI != "" {
		manifest.Endpoints.AsyncAPI = config.Endpoints.AsyncAPI
	}
	if config.Endpoints.GraphQL != "" {
		manifest.Endpoints.GraphQL = config.Endpoints.GraphQL
	}
	if config.Endpoints.Metrics != "" {
		manifest.Endpoints.Metrics = config.Endpoints.Metrics
	}
	manifest.Endpoints.GRPCReflection = config.Endpoints.GRPCReflection

	if config.Hints != nil {
		manifest.Hints = config.Hints
	}

	manifest.Instance = &farp.InstanceMetadata{
		Address:   config.Address,
		Status:    farp.InstanceStatusStarting,
		StartedAt: time.Now().Unix(),
	}

	schemas := make(map[farp.SchemaType]any)

	handler := NewFARPHandler(manifest, schemas)

	return &ServiceNode{
		config:   config,
		manifest: manifest,
		handler:  handler,
		schemas:  schemas,
		done:     make(chan struct{}),
	}, nil
}

// Start registers the service and begins the health reporting loop.
// It generates schemas from providers (if configured), builds the manifest,
// registers in the discovery backend (or pushes to gateway), and starts
// a background goroutine for health reporting and TTL renewal.
func (n *ServiceNode) Start(ctx context.Context) error {
	// Generate schemas from providers
	if err := n.generateSchemas(ctx); err != nil {
		return fmt.Errorf("failed to generate schemas: %w", err)
	}

	// Update checksums
	if err := n.manifest.UpdateChecksum(); err != nil {
		return fmt.Errorf("failed to update checksum: %w", err)
	}

	_ = n.manifest.UpdateRoutesChecksum()

	// Mark as healthy
	n.manifest.Instance.Status = farp.InstanceStatusHealthy
	n.handler.SetHealth(farp.InstanceStatusHealthy)
	n.handler.UpdateManifest(n.manifest)

	// Register in discovery backend
	instance := n.buildInstance()

	discovery := n.config.Discovery
	if discovery == nil && n.config.GatewayURL != "" {
		// Push mode: create a PushDiscovery
		discovery = NewPushDiscovery(n.config.GatewayURL, n.config.HTTPClient)
	}

	if err := discovery.Register(ctx, instance); err != nil {
		return fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	// Publish manifest to SchemaRegistry if configured
	if n.config.Registry != nil {
		if err := n.config.Registry.RegisterManifest(ctx, n.manifest); err != nil {
			// Non-fatal: registry publish is optional
			_ = err
		}
	}

	// Start background health loop
	childCtx, cancel := context.WithCancel(ctx)
	n.cancel = cancel

	go n.healthLoop(childCtx, discovery)

	return nil
}

// Stop gracefully deregisters the service and stops the health loop.
func (n *ServiceNode) Stop(ctx context.Context) error {
	// Cancel health loop
	if n.cancel != nil {
		n.cancel()
		<-n.done
	}

	// Mark as stopping
	n.handler.SetHealth(farp.InstanceStatusStopping)

	// Deregister from discovery
	discovery := n.config.Discovery
	if discovery == nil && n.config.GatewayURL != "" {
		discovery = NewPushDiscovery(n.config.GatewayURL, n.config.HTTPClient)
	}

	if discovery != nil {
		if err := discovery.Deregister(ctx, n.config.InstanceID); err != nil {
			return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
		}
	}

	// Remove from SchemaRegistry
	if n.config.Registry != nil {
		_ = n.config.Registry.DeleteManifest(ctx, n.config.InstanceID)
	}

	return nil
}

// Manifest returns the current SchemaManifest.
func (n *ServiceNode) Manifest() *farp.SchemaManifest {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.manifest
}

// HTTPHandler returns an http.Handler that serves FARP endpoints.
func (n *ServiceNode) HTTPHandler() http.Handler {
	return n.handler
}

// UpdateSchema triggers re-generation of schemas from providers
// and pushes the updated manifest to the discovery/registry backend.
func (n *ServiceNode) UpdateSchema(ctx context.Context) error {
	if err := n.generateSchemas(ctx); err != nil {
		return err
	}

	if err := n.manifest.UpdateChecksum(); err != nil {
		return err
	}

	_ = n.manifest.UpdateRoutesChecksum()
	n.manifest.UpdatedAt = time.Now().Unix()
	n.handler.UpdateManifest(n.manifest)

	// Push update to registry
	if n.config.Registry != nil {
		_ = n.config.Registry.UpdateManifest(ctx, n.manifest)
	}

	// Re-register to propagate changes
	discovery := n.config.Discovery
	if discovery == nil && n.config.GatewayURL != "" {
		discovery = NewPushDiscovery(n.config.GatewayURL, n.config.HTTPClient)
	}

	if discovery != nil {
		instance := n.buildInstance()
		_ = discovery.Register(ctx, instance)
	}

	return nil
}

// serviceApp wraps ServiceNodeConfig to implement farp.Application.
type serviceApp struct {
	config *ServiceNodeConfig
}

func (a *serviceApp) Name() string    { return a.config.ServiceName }
func (a *serviceApp) Version() string { return a.config.ServiceVersion }
func (a *serviceApp) Routes() any     { return nil }

func (n *ServiceNode) generateSchemas(ctx context.Context) error {
	app := &serviceApp{config: &n.config}

	for _, provider := range n.config.Providers {
		schema, err := provider.Generate(ctx, app)
		if err != nil {
			return fmt.Errorf("provider %s failed: %w", provider.Type(), err)
		}

		hash, err := farp.CalculateSchemaChecksum(schema)
		if err != nil {
			return err
		}

		n.mu.Lock()
		n.schemas[provider.Type()] = schema
		n.mu.Unlock()

		n.handler.UpdateSchema(provider.Type(), schema)

		// Update or add schema descriptor in manifest
		found := false

		for i := range n.manifest.Schemas {
			if n.manifest.Schemas[i].Type == provider.Type() {
				n.manifest.Schemas[i].Hash = hash
				found = true

				break
			}
		}

		if !found {
			n.manifest.AddSchema(farp.SchemaDescriptor{
				Type:        provider.Type(),
				SpecVersion: provider.SpecVersion(),
				Location: farp.SchemaLocation{
					Type: farp.LocationTypeHTTP,
					URL:  fmt.Sprintf("http://%s/_farp/schemas/%s", n.config.Address, provider.Type()),
				},
				ContentType: provider.ContentType(),
				Hash:        hash,
			})
			n.manifest.AddCapability(string(provider.Type()))

			// Auto-populate manifest endpoint paths from providers when not
			// already set by config. This ensures the manifest and metadata
			// advertise spec endpoints even for auto-detected schemas.
			switch provider.Type() {
			case farp.SchemaTypeOpenAPI:
				if n.manifest.Endpoints.OpenAPI == "" {
					n.manifest.Endpoints.OpenAPI = provider.Endpoint()
				}
			case farp.SchemaTypeAsyncAPI:
				if n.manifest.Endpoints.AsyncAPI == "" {
					n.manifest.Endpoints.AsyncAPI = provider.Endpoint()
				}
			case farp.SchemaTypeGraphQL:
				if n.manifest.Endpoints.GraphQL == "" {
					n.manifest.Endpoints.GraphQL = provider.Endpoint()
				}
			}
		}
	}

	return nil
}

func (n *ServiceNode) buildInstance() ServiceInstance {
	baseURL := "http://" + n.config.Address

	// Build metadata from the manifest (which is fully populated by the time
	// Start() calls buildInstance, after generateSchemas has run).
	metadata := map[string]string{
		"farp.enabled":  "true",
		"farp.version":  farp.ProtocolVersion,
		"farp.manifest": baseURL + "/_farp/manifest",
	}

	// Advertise endpoints from the manifest so that gateways can discover
	// spec URLs without fetching the full manifest.
	eps := n.manifest.Endpoints
	if eps.Health != "" {
		metadata["farp.health"] = eps.Health
		metadata["farp.health.url"] = baseURL + eps.Health
	}
	if eps.OpenAPI != "" {
		metadata["farp.openapi"] = baseURL + eps.OpenAPI
		metadata["farp.openapi.path"] = eps.OpenAPI
	}
	if eps.AsyncAPI != "" {
		metadata["farp.asyncapi"] = baseURL + eps.AsyncAPI
		metadata["farp.asyncapi.path"] = eps.AsyncAPI
	}
	if eps.GraphQL != "" {
		metadata["farp.graphql"] = baseURL + eps.GraphQL
		metadata["farp.graphql.path"] = eps.GraphQL
	}
	if eps.GRPCReflection {
		metadata["farp.grpc.reflection"] = "true"
	}

	// Advertise capabilities from the manifest.
	if len(n.manifest.Capabilities) > 0 {
		metadata["farp.capabilities"] = strings.Join(n.manifest.Capabilities, ",")
	}

	// Merge user-provided metadata last so it can override auto-generated keys.
	for k, v := range n.config.Metadata {
		metadata[k] = v
	}

	return ServiceInstance{
		ID:              n.config.InstanceID,
		ServiceName:     n.config.ServiceName,
		Version:         n.config.ServiceVersion,
		Address:         n.config.Address,
		Status:          farp.InstanceStatusHealthy,
		Tags:            n.config.Tags,
		Metadata:        metadata,
		RegisteredAt:    time.Now(),
		LastHealthCheck: time.Now(),
	}
}

func (n *ServiceNode) healthLoop(ctx context.Context, discovery ServiceDiscovery) {
	defer close(n.done)

	ticker := time.NewTicker(n.config.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// In push mode, use checksum reconciliation (§17.4.1)
			if pushDisc, ok := discovery.(*PushDiscovery); ok {
				n.pushHealthCheck(ctx, pushDisc)
			} else {
				if err := discovery.ReportHealth(ctx, n.config.InstanceID, farp.InstanceStatusHealthy); err != nil {
					n.retryRegister(ctx, discovery)
				}
			}
		}
	}
}

// pushHealthCheck sends a heartbeat with routes_checksum and handles
// reconciliation if the gateway's state doesn't match (§17.4.1).
func (n *ServiceNode) pushHealthCheck(ctx context.Context, pushDisc *PushDiscovery) {
	n.mu.RLock()
	expectedChecksum := n.manifest.RoutesChecksum
	n.mu.RUnlock()

	resp, err := pushDisc.ReportHealthWithChecksum(
		ctx, n.config.InstanceID,
		farp.InstanceStatusHealthy,
		expectedChecksum,
	)
	if err != nil {
		// Gateway unreachable — retry registration
		n.retryRegister(ctx, pushDisc)
		return
	}

	// Reconciliation: if gateway checksum doesn't match, re-register WITH
	// manifest so the gateway can apply schemas without fetching.
	if resp.RoutesChecksum != expectedChecksum {
		n.mu.RLock()
		manifest := n.manifest
		n.mu.RUnlock()

		instance := n.buildInstance()
		if _, err := pushDisc.RegisterWithManifest(ctx, instance, manifest); err != nil {
			// Non-fatal: will retry on next heartbeat
			_ = err
		}
	}
}

// retryRegister attempts re-registration with exponential backoff.
func (n *ServiceNode) retryRegister(ctx context.Context, discovery ServiceDiscovery) {
	for attempt := range n.config.MaxRetries {
		time.Sleep(n.config.RetryBackoff * time.Duration(attempt+1))

		instance := n.buildInstance()
		if err := discovery.Register(ctx, instance); err == nil {
			break
		}
	}
}

// =============================================================================
// GatewayNode
// =============================================================================

// GatewayNode manages the full FARP lifecycle for a gateway:
// service discovery, manifest fetching, schema registration, route management,
// and health tracking — all automatic.
type GatewayNode struct {
	config      GatewayNodeConfig
	registry    farp.SchemaRegistry
	gwClient    *gateway.Client
	manifests   map[string]*farp.SchemaManifest // key: instanceID
	pushHandler *PushHandler
	cancel      context.CancelFunc
	done        chan struct{}
	mu          sync.RWMutex
}

// NewGatewayNode creates a new GatewayNode.
func NewGatewayNode(config GatewayNodeConfig) (*GatewayNode, error) {
	config.setDefaults()

	if config.Discovery == nil && !config.EnablePush {
		return nil, fmt.Errorf("either Discovery or EnablePush must be set")
	}

	// Use provided registry or create in-memory
	reg := config.Registry
	if reg == nil {
		reg = memory.NewRegistry()
	}

	gwClient := gateway.NewClient(reg)

	var pushHandler *PushHandler
	if config.EnablePush {
		pushHandler = NewPushHandler(config.HeartbeatTimeout)
	}

	fetcher := config.Fetcher
	if fetcher == nil {
		fetcher = NewHTTPManifestFetcher(config.HTTPClient)
	}

	return &GatewayNode{
		config:      config,
		registry:    reg,
		gwClient:    gwClient,
		manifests:   make(map[string]*farp.SchemaManifest),
		pushHandler: pushHandler,
		done:        make(chan struct{}),
	}, nil
}

// Start begins watching for services and auto-managing routes.
func (n *GatewayNode) Start(ctx context.Context) error {
	childCtx, cancel := context.WithCancel(ctx)
	n.cancel = cancel

	// Watch registry-based discovery
	if n.config.Discovery != nil {
		go n.watchDiscovery(childCtx, n.config.Discovery)
	}

	// Watch push-based registrations
	if n.pushHandler != nil {
		go n.watchPush(childCtx)
	}

	return nil
}

// Stop stops watching and cleans up.
func (n *GatewayNode) Stop(_ context.Context) error {
	if n.cancel != nil {
		n.cancel()
		<-n.done
	}

	return nil
}

// Services returns all currently known services and their manifests.
func (n *GatewayNode) Services() map[string]*farp.SchemaManifest {
	n.mu.RLock()
	defer n.mu.RUnlock()

	result := make(map[string]*farp.SchemaManifest, len(n.manifests))
	for k, v := range n.manifests {
		result[k] = v
	}

	return result
}

// Routes returns the current computed route table.
func (n *GatewayNode) Routes() []gateway.ServiceRoute {
	n.mu.RLock()
	manifests := make([]*farp.SchemaManifest, 0, len(n.manifests))
	for _, m := range n.manifests {
		manifests = append(manifests, m)
	}
	n.mu.RUnlock()

	return n.gwClient.ConvertToRoutes(manifests)
}

// Registry returns the underlying SchemaRegistry.
func (n *GatewayNode) Registry() farp.SchemaRegistry {
	return n.registry
}

// GatewayClient returns the underlying gateway.Client.
func (n *GatewayNode) GatewayClient() *gateway.Client {
	return n.gwClient
}

// PushHandler returns the HTTP handler for push-based registration.
// Returns nil if EnablePush is false.
func (n *GatewayNode) PushHandler() http.Handler {
	if n.pushHandler == nil {
		return http.NotFoundHandler()
	}

	return n.pushHandler
}

func (n *GatewayNode) watchDiscovery(ctx context.Context, disc ServiceDiscovery) {
	defer func() {
		select {
		case <-n.done:
		default:
			close(n.done)
		}
	}()

	// Watch service names or all
	serviceName := ""
	if len(n.config.ServiceNames) == 1 {
		serviceName = n.config.ServiceNames[0]
	}

	_ = disc.Watch(ctx, serviceName, func(event DiscoveryEvent) {
		n.handleDiscoveryEvent(ctx, event)
	})
}

func (n *GatewayNode) watchPush(ctx context.Context) {
	if n.pushHandler == nil {
		return
	}

	n.pushHandler.Watch(ctx, "", func(event DiscoveryEvent) {
		n.handleDiscoveryEvent(ctx, event)
	})
}

// ListServiceNames returns distinct service names from all known manifests.
func (n *GatewayNode) ListServiceNames() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	seen := make(map[string]struct{})
	for _, m := range n.manifests {
		seen[m.ServiceName] = struct{}{}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}

	return names
}

func (n *GatewayNode) handleDiscoveryEvent(ctx context.Context, event DiscoveryEvent) {
	// Notify service event listener before processing
	if n.config.OnServiceEvent != nil {
		n.config.OnServiceEvent(event)
	}

	switch event.Type {
	case farp.EventTypeAdded, farp.EventTypeUpdated:
		manifest := event.Manifest
		if manifest == nil {
			// Fetch manifest from service
			fetcher := n.config.Fetcher
			if fetcher == nil {
				fetcher = NewHTTPManifestFetcher(n.config.HTTPClient)
			}

			var err error

			manifest, err = fetcher.FetchManifest(ctx, event.Instance)
			if err != nil {
				return // Skip this event
			}
		}

		n.mu.Lock()
		n.manifests[event.Instance.ID] = manifest
		n.mu.Unlock()

		// Register in SchemaRegistry
		if event.Type == farp.EventTypeAdded {
			_ = n.registry.RegisterManifest(ctx, manifest)
		} else {
			_ = n.registry.UpdateManifest(ctx, manifest)
		}

		// Notify route changes
		n.notifyRouteChange()

	case farp.EventTypeRemoved:
		n.mu.Lock()
		delete(n.manifests, event.Instance.ID)
		n.mu.Unlock()

		_ = n.registry.DeleteManifest(ctx, event.Instance.ID)

		// Notify route changes
		n.notifyRouteChange()
	}
}

func (n *GatewayNode) notifyRouteChange() {
	if n.config.OnRoutesChanged != nil {
		routes := n.Routes()
		n.config.OnRoutesChanged(routes)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func generateInstanceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)

	return "i-" + hex.EncodeToString(b)
}
