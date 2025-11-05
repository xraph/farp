// Package gateway provides a REFERENCE IMPLEMENTATION for API gateway integration with FARP.
//
// ⚠️ IMPORTANT: This is NOT production-ready code. It serves as an example/helper
// to demonstrate how gateways should integrate with FARP.
//
// # What This Package Provides
//
// - Example of how to watch for manifest changes
// - Example of how to convert schemas to routes
// - Example of how to cache schemas
// - Integration with the merger package
//
// # What This Package Does NOT Provide
//
// - Complete HTTP client (HTTP schema fetch implemented)
// - Production-ready error handling
// - Gateway-specific route application
// - Health monitoring
// - Load balancing logic
// - Retry logic with exponential backoff
// - Circuit breaker patterns
//
// # For Production Gateways
//
// Real gateway implementations (Kong, Traefik, Envoy, custom) should:
//
// 1. Implement their own HTTP client to fetch schemas
// 2. Implement their own service discovery watchers (Consul, etcd, K8s, mDNS)
// 3. Implement gateway-specific route configuration
// 4. Add production-ready error handling and observability
// 5. Use this package as a reference/inspiration, not a dependency
//
// See docs/IMPLEMENTATION_RESPONSIBILITIES.md for complete guidance.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/xraph/farp"
	"github.com/xraph/farp/merger"
)

// Client is a reference implementation for API gateway integration.
// It demonstrates how to watch for service schema changes and provides conversion utilities.
//
// For production use, gateways should implement their own logic tailored to their
// specific architecture, error handling, and performance requirements.
type Client struct {
	registry      farp.SchemaRegistry
	manifestCache map[string]*farp.SchemaManifest // key: instanceID
	schemaCache   map[string]any                  // key: hash
	merger        *merger.Merger
	httpClient    *http.Client
	mu            sync.RWMutex
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client for schema fetching.
// This is useful when you need custom authentication, TLS configuration, or other HTTP client settings.
//
// Example:
//
//	client := http.Client{
//		Timeout: 60 * time.Second,
//		Transport: &http.Transport{
//			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
//		},
//	}
//	gatewayClient := gateway.NewClient(registry, gateway.WithHTTPClient(&client))
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// NewClient creates a new gateway client with default merger config.
// Options can be provided to customize the client behavior.
//
// Example:
//
//	client := gateway.NewClient(registry, gateway.WithHTTPClient(customHTTPClient))
func NewClient(registry farp.SchemaRegistry, opts ...ClientOption) *Client {
	return NewClientWithConfig(registry, merger.DefaultMergerConfig(), opts...)
}

// NewClientWithConfig creates a new gateway client with custom merger config.
// Options can be provided to customize the client behavior.
//
// Example:
//
//	config := merger.DefaultMergerConfig()
//	config.Timeout = 60 * time.Second
//	client := gateway.NewClientWithConfig(registry, config, gateway.WithHTTPClient(customHTTPClient))
func NewClientWithConfig(registry farp.SchemaRegistry, mergerConfig merger.MergerConfig, opts ...ClientOption) *Client {
	c := &Client{
		registry:      registry,
		manifestCache: make(map[string]*farp.SchemaManifest),
		schemaCache:   make(map[string]any),
		merger:        merger.NewMerger(mergerConfig),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WatchServices watches for service registrations and schema updates
// onChange is called whenever services are added, updated, or removed.
func (c *Client) WatchServices(ctx context.Context, serviceName string, onChange func([]ServiceRoute)) error {
	// Initial load
	manifests, err := c.registry.ListManifests(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to list initial manifests: %w", err)
	}

	// Convert initial manifests to routes
	routes := c.ConvertToRoutes(manifests)
	onChange(routes)

	// Watch for changes
	return c.registry.WatchManifests(ctx, serviceName, func(event *farp.ManifestEvent) {
		// Update manifest cache
		c.mu.Lock()

		switch event.Type {
		case farp.EventTypeAdded, farp.EventTypeUpdated:
			c.manifestCache[event.Manifest.InstanceID] = event.Manifest
		case farp.EventTypeRemoved:
			delete(c.manifestCache, event.Manifest.InstanceID)
		}

		// Get all cached manifests
		manifests := make([]*farp.SchemaManifest, 0, len(c.manifestCache))
		for _, m := range c.manifestCache {
			manifests = append(manifests, m)
		}

		c.mu.Unlock()

		// Convert to routes and notify
		routes := c.ConvertToRoutes(manifests)
		onChange(routes)
	})
}

// ConvertToRoutes converts service manifests to gateway routes
// This is a reference implementation - actual gateways should customize this.
func (c *Client) ConvertToRoutes(manifests []*farp.SchemaManifest) []ServiceRoute {
	var routes []ServiceRoute

	for _, manifest := range manifests {
		// Fetch schemas for this manifest
		for _, schemaDesc := range manifest.Schemas {
			var (
				schema any
				err    error
			)

			// Check cache first

			if cached, ok := c.getSchemaFromCache(schemaDesc.Hash); ok {
				schema = cached
			} else {
				// Fetch schema based on location type
				schema, err = c.fetchSchema(context.Background(), &schemaDesc)
				if err != nil {
					// Log error and skip
					continue
				}

				// Cache the schema
				c.cacheSchema(schemaDesc.Hash, schema)
			}

			// Convert schema to routes based on type
			switch schemaDesc.Type {
			case farp.SchemaTypeOpenAPI:
				routes = append(routes, c.convertOpenAPIToRoutes(manifest, schema, &schemaDesc)...)
			case farp.SchemaTypeAsyncAPI:
				routes = append(routes, c.convertAsyncAPIToRoutes(manifest, schema)...)
			case farp.SchemaTypeGraphQL:
				routes = append(routes, c.convertGraphQLToRoutes(manifest, schema)...)
			}
		}
	}

	return routes
}

// ServiceRoute represents a route configuration for the gateway.
type ServiceRoute struct {
	// Path is the route path pattern
	Path string

	// Methods are HTTP methods for this route (e.g., ["GET", "POST"])
	Methods []string

	// TargetURL is the backend service URL
	TargetURL string

	// HealthURL is the health check URL
	HealthURL string

	// Middleware are middleware names to apply
	Middleware []string

	// Metadata contains additional route information
	Metadata map[string]any

	// ServiceName is the name of the backend service
	ServiceName string

	// ServiceVersion is the version of the backend service
	ServiceVersion string
}

// fetchSchema fetches a schema based on its location.
func (c *Client) fetchSchema(ctx context.Context, descriptor *farp.SchemaDescriptor) (any, error) {
	switch descriptor.Location.Type {
	case farp.LocationTypeInline:
		return descriptor.InlineSchema, nil

	case farp.LocationTypeRegistry:
		return c.registry.FetchSchema(ctx, descriptor.Location.RegistryPath)

	case farp.LocationTypeHTTP:
		if descriptor.Location.URL == "" {
			return nil, errors.New("URL is required for HTTP location type")
		}

		// Create HTTP request with context
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, descriptor.Location.URL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %w", err)
		}

		// Set headers if provided
		for key, value := range descriptor.Location.Headers {
			req.Header.Set(key, value)
		}

		// Set Accept header for JSON schemas
		if req.Header.Get("Accept") == "" {
			req.Header.Set("Accept", "application/json")
		}

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch schema from URL %s: %w", descriptor.Location.URL, err)
		}
		defer resp.Body.Close()

		// Check status code
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, resp.Status)
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Parse JSON response
		var schema any
		if err := json.Unmarshal(body, &schema); err != nil {
			return nil, fmt.Errorf("failed to parse JSON schema from %s: %w", descriptor.Location.URL, err)
		}

		return schema, nil

	default:
		return nil, fmt.Errorf("%w: %s", farp.ErrInvalidLocation, descriptor.Location.Type)
	}
}

// getBaseURL extracts the base URL for a service from multiple sources.
// Priority order:
// 1. OpenAPI schema's servers array (first server URL)
// 2. Schema descriptor's Location.URL (extract base URL from full URL)
// 3. manifest.Instance.Address (convert to http://host:port)
func (c *Client) getBaseURL(manifest *farp.SchemaManifest, schemaMap map[string]any, schemaDesc *farp.SchemaDescriptor) string {
	// Try OpenAPI schema's servers array
	if servers, ok := schemaMap["servers"].([]any); ok && len(servers) > 0 {
		if server, ok := servers[0].(map[string]any); ok {
			if serverURL, ok := server["url"].(string); ok && serverURL != "" {
				// Normalize URL (remove trailing slash)
				return strings.TrimSuffix(serverURL, "/")
			}
		}
	}

	// Try schema descriptor's Location.URL
	if schemaDesc != nil && schemaDesc.Location.Type == farp.LocationTypeHTTP && schemaDesc.Location.URL != "" {
		// Extract base URL from full URL
		parsedURL, err := url.Parse(schemaDesc.Location.URL)
		if err == nil {
			// Reconstruct base URL (scheme + host + port)
			baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
			if parsedURL.Scheme == "" {
				// If no scheme, default to http
				baseURL = "http://" + parsedURL.Host
			}

			return baseURL
		}
	}

	// Try manifest Instance.Address
	if manifest.Instance != nil && manifest.Instance.Address != "" {
		address := manifest.Instance.Address
		// If address doesn't have a scheme, default to http
		if !strings.Contains(address, "://") {
			return "http://" + address
		}

		return address
	}

	// Fallback: construct default URL from service name (backward compatibility)
	// This maintains compatibility with tests and cases where no URL is provided
	return fmt.Sprintf("http://%s:8080", manifest.ServiceName)
}

// convertOpenAPIToRoutes converts an OpenAPI schema to gateway routes.
func (c *Client) convertOpenAPIToRoutes(manifest *farp.SchemaManifest, schema any, schemaDesc *farp.SchemaDescriptor) []ServiceRoute {
	var routes []ServiceRoute

	// Parse OpenAPI schema
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return routes
	}

	paths, ok := schemaMap["paths"].(map[string]any)
	if !ok {
		return routes
	}

	// Base URL for the service - try multiple sources
	baseURL := c.getBaseURL(manifest, schemaMap, schemaDesc)

	// Convert each path to a route
	for path, pathItem := range paths {
		pathItemMap, ok := pathItem.(map[string]any)
		if !ok {
			continue
		}

		// Get methods for this path
		methods := []string{}

		for method := range pathItemMap {
			switch method {
			case "get", "post", "put", "delete", "patch", "options", "head":
				methods = append(methods, method)
			}
		}

		if len(methods) > 0 {
			routes = append(routes, ServiceRoute{
				Path:           path,
				Methods:        methods,
				TargetURL:      baseURL + path,
				HealthURL:      baseURL + manifest.Endpoints.Health,
				ServiceName:    manifest.ServiceName,
				ServiceVersion: manifest.ServiceVersion,
				Metadata: map[string]any{
					"schema_type": "openapi",
				},
			})
		}
	}

	return routes
}

// convertAsyncAPIToRoutes converts an AsyncAPI schema to gateway routes (WebSocket, SSE).
func (c *Client) convertAsyncAPIToRoutes(manifest *farp.SchemaManifest, schema any) []ServiceRoute {
	var routes []ServiceRoute

	// Parse AsyncAPI schema
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return routes
	}

	channels, ok := schemaMap["channels"].(map[string]any)
	if !ok {
		return routes
	}

	// Base URL for the service - try multiple sources
	baseURL := c.getBaseURL(manifest, schemaMap, nil)

	// Convert each channel to a route
	for channelPath := range channels {
		routes = append(routes, ServiceRoute{
			Path:           channelPath,
			Methods:        []string{"WEBSOCKET"}, // Special method for WebSocket
			TargetURL:      baseURL + channelPath,
			HealthURL:      baseURL + manifest.Endpoints.Health,
			ServiceName:    manifest.ServiceName,
			ServiceVersion: manifest.ServiceVersion,
			Metadata: map[string]any{
				"schema_type": "asyncapi",
				"protocol":    "websocket",
			},
		})
	}

	return routes
}

// convertGraphQLToRoutes converts a GraphQL schema to a gateway route.
func (c *Client) convertGraphQLToRoutes(manifest *farp.SchemaManifest, schema any) []ServiceRoute {
	var routes []ServiceRoute

	// GraphQL typically has a single endpoint
	// Parse schema if it's a map (for consistency)
	schemaMap, _ := schema.(map[string]any)
	if schemaMap == nil {
		schemaMap = make(map[string]any)
	}

	// Base URL for the service - try multiple sources
	baseURL := c.getBaseURL(manifest, schemaMap, nil)

	graphqlPath := manifest.Endpoints.GraphQL
	if graphqlPath == "" {
		graphqlPath = "/graphql"
	}

	routes = append(routes, ServiceRoute{
		Path:           graphqlPath,
		Methods:        []string{"POST", "GET"},
		TargetURL:      baseURL + graphqlPath,
		HealthURL:      baseURL + manifest.Endpoints.Health,
		ServiceName:    manifest.ServiceName,
		ServiceVersion: manifest.ServiceVersion,
		Metadata: map[string]any{
			"schema_type": "graphql",
		},
	})

	return routes
}

// GenerateMergedSchemas generates unified specs for all protocol types from registered services.
func (c *Client) GenerateMergedSchemas(ctx context.Context, serviceName string) (*merger.MultiProtocolResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get all manifests for the service
	var manifests []*farp.SchemaManifest

	if serviceName == "" {
		// Get all services
		for _, manifest := range c.manifestCache {
			manifests = append(manifests, manifest)
		}
	} else {
		// Get specific service
		for _, manifest := range c.manifestCache {
			if manifest.ServiceName == serviceName {
				manifests = append(manifests, manifest)
			}
		}
	}

	// Create schema fetcher
	schemaFetcher := func(hash string) (any, error) {
		if cached, ok := c.getSchemaFromCache(hash); ok {
			return cached, nil
		}
		// In production, fetch from registry here
		return nil, fmt.Errorf("schema not in cache: %s", hash)
	}

	// Create multi-protocol merger
	multiMerger := merger.NewMultiProtocolMerger(merger.DefaultMergerConfig())

	// Merge all protocols
	return multiMerger.MergeAll(manifests, schemaFetcher)
}

// GenerateMergedOpenAPI generates a unified OpenAPI spec from all registered services

// Deprecated: Use GenerateMergedSchemas for multi-protocol support.
func (c *Client) GenerateMergedOpenAPI(ctx context.Context, serviceName string) (*merger.MergeResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get all manifests for the service
	var manifests []*farp.SchemaManifest

	if serviceName == "" {
		// Get all services
		for _, manifest := range c.manifestCache {
			manifests = append(manifests, manifest)
		}
	} else {
		// Get specific service
		for _, manifest := range c.manifestCache {
			if manifest.ServiceName == serviceName {
				manifests = append(manifests, manifest)
			}
		}
	}

	// Build service schemas
	serviceSchemas := make([]merger.ServiceSchema, 0, len(manifests))
	for _, manifest := range manifests {
		// Find OpenAPI schema
		for _, schemaDesc := range manifest.Schemas {
			if schemaDesc.Type != farp.SchemaTypeOpenAPI {
				continue
			}

			// Fetch the schema
			var (
				schema any
				err    error
			)

			if cached, ok := c.getSchemaFromCache(schemaDesc.Hash); ok {
				schema = cached
			} else {
				schema, err = c.fetchSchema(ctx, &schemaDesc)
				if err != nil {
					continue
				}

				c.cacheSchema(schemaDesc.Hash, schema)
			}

			serviceSchemas = append(serviceSchemas, merger.ServiceSchema{
				Manifest: manifest,
				Schema:   schema,
			})

			break // Only one OpenAPI schema per manifest
		}
	}

	// Merge schemas
	return c.merger.Merge(serviceSchemas)
}

// GetMergedOpenAPIJSON returns the merged OpenAPI spec as JSON.
func (c *Client) GetMergedOpenAPIJSON(ctx context.Context, serviceName string) ([]byte, error) {
	result, err := c.GenerateMergedOpenAPI(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// Convert to JSON
	// Note: In production, you'd use a proper JSON marshaler
	// For now, return a placeholder
	return fmt.Appendf(nil, `{
		"openapi": "%s",
		"info": {
			"title": "%s",
			"description": "%s",
			"version": "%s"
		},
		"paths": {},
		"components": {}
	}`, result.Spec.OpenAPI, result.Spec.Info.Title, result.Spec.Info.Description, result.Spec.Info.Version), nil
}

// getSchemaFromCache retrieves a cached schema by hash.
func (c *Client) getSchemaFromCache(hash string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	schema, ok := c.schemaCache[hash]

	return schema, ok
}

// cacheSchema stores a schema in cache.
func (c *Client) cacheSchema(hash string, schema any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.schemaCache[hash] = schema
}

// ClearCache clears the schema cache.
func (c *Client) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.schemaCache = make(map[string]any)
}

// GetManifest retrieves a cached manifest by instance ID.
func (c *Client) GetManifest(instanceID string) (*farp.SchemaManifest, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	manifest, ok := c.manifestCache[instanceID]

	return manifest, ok
}
