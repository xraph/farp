package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xraph/farp"
)

// Mock registry for testing
type mockRegistry struct {
	manifests       map[string]*farp.SchemaManifest
	schemas         map[string]interface{}
	watchHandler    farp.ManifestChangeHandler
	schemaWatchHandler farp.SchemaChangeHandler
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		manifests: make(map[string]*farp.SchemaManifest),
		schemas:   make(map[string]interface{}),
	}
}

func (m *mockRegistry) RegisterManifest(ctx context.Context, manifest *farp.SchemaManifest) error {
	m.manifests[manifest.InstanceID] = manifest
	return nil
}

func (m *mockRegistry) GetManifest(ctx context.Context, instanceID string) (*farp.SchemaManifest, error) {
	manifest, ok := m.manifests[instanceID]
	if !ok {
		return nil, farp.ErrManifestNotFound
	}
	return manifest, nil
}

func (m *mockRegistry) UpdateManifest(ctx context.Context, manifest *farp.SchemaManifest) error {
	m.manifests[manifest.InstanceID] = manifest
	return nil
}

func (m *mockRegistry) DeleteManifest(ctx context.Context, instanceID string) error {
	delete(m.manifests, instanceID)
	return nil
}

func (m *mockRegistry) ListManifests(ctx context.Context, serviceName string) ([]*farp.SchemaManifest, error) {
	var result []*farp.SchemaManifest
	for _, manifest := range m.manifests {
		if serviceName == "" || manifest.ServiceName == serviceName {
			result = append(result, manifest)
		}
	}
	return result, nil
}

func (m *mockRegistry) PublishSchema(ctx context.Context, path string, schema interface{}) error {
	m.schemas[path] = schema
	return nil
}

func (m *mockRegistry) FetchSchema(ctx context.Context, path string) (interface{}, error) {
	schema, ok := m.schemas[path]
	if !ok {
		return nil, farp.ErrSchemaNotFound
	}
	return schema, nil
}

func (m *mockRegistry) DeleteSchema(ctx context.Context, path string) error {
	delete(m.schemas, path)
	return nil
}

func (m *mockRegistry) WatchManifests(ctx context.Context, serviceName string, onChange farp.ManifestChangeHandler) error {
	m.watchHandler = onChange
	<-ctx.Done()
	return nil
}

func (m *mockRegistry) WatchSchemas(ctx context.Context, path string, onChange farp.SchemaChangeHandler) error {
	m.schemaWatchHandler = onChange
	<-ctx.Done()
	return nil
}

func (m *mockRegistry) Close() error {
	return nil
}

func (m *mockRegistry) Health(ctx context.Context) error {
	return nil
}

// Trigger a manifest change event
func (m *mockRegistry) triggerManifestEvent(eventType farp.EventType, manifest *farp.SchemaManifest) {
	if m.watchHandler != nil {
		m.watchHandler(&farp.ManifestEvent{
			Type:      eventType,
			Manifest:  manifest,
			Timestamp: time.Now().Unix(),
		})
	}
}

func TestNewClient(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.registry != registry {
		t.Error("Client has wrong registry")
	}

	if client.manifestCache == nil {
		t.Error("manifestCache should be initialized")
	}

	if client.schemaCache == nil {
		t.Error("schemaCache should be initialized")
	}
}

func TestClient_WatchServices(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Register a manifest
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType:  "application/json",
		Hash:         "abc123",
		Size:         1024,
		InlineSchema: map[string]interface{}{
			"openapi": "3.1.0",
			"paths": map[string]interface{}{
				"/test": map[string]interface{}{
					"get": map[string]interface{}{},
				},
			},
		},
	})
	registry.RegisterManifest(context.Background(), manifest)

	// Watch for changes
	changeCallCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := client.WatchServices(ctx, "test-service", func(routes []ServiceRoute) {
			changeCallCount++
			if changeCallCount == 1 {
				// First call should be initial load
				if len(routes) < 1 {
					t.Errorf("expected at least 1 route, got %d", len(routes))
				}
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("WatchServices() error = %v", err)
		}
	}()

	// Give it time to process initial load
	time.Sleep(50 * time.Millisecond)

	// Trigger an update event
	registry.triggerManifestEvent(farp.EventTypeUpdated, manifest)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	if changeCallCount < 2 {
		t.Errorf("expected at least 2 change callbacks, got %d", changeCallCount)
	}
}

func TestClient_ConvertToRoutes_OpenAPI(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]interface{}{
			"openapi": "3.1.0",
			"paths": map[string]interface{}{
				"/users": map[string]interface{}{
					"get":  map[string]interface{}{},
					"post": map[string]interface{}{},
				},
				"/posts": map[string]interface{}{
					"get": map[string]interface{}{},
				},
			},
		},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}

	// Verify route properties
	for _, route := range routes {
		if route.ServiceName != "test-service" {
			t.Errorf("route.ServiceName = %v, want test-service", route.ServiceName)
		}
		if route.ServiceVersion != "v1.0.0" {
			t.Errorf("route.ServiceVersion = %v, want v1.0.0", route.ServiceVersion)
		}
		if len(route.Methods) == 0 {
			t.Error("route should have methods")
		}
	}
}

func TestClient_ConvertToRoutes_AsyncAPI(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeAsyncAPI,
		SpecVersion: "3.0.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]interface{}{
			"asyncapi": "3.0.0",
			"channels": map[string]interface{}{
				"/ws/notifications": map[string]interface{}{},
				"/ws/messages":      map[string]interface{}{},
			},
		},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}

	// Verify WebSocket routes
	for _, route := range routes {
		if len(route.Methods) != 1 || route.Methods[0] != "WEBSOCKET" {
			t.Errorf("AsyncAPI route should have WEBSOCKET method, got %v", route.Methods)
		}
	}
}

func TestClient_ConvertToRoutes_GraphQL(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.Endpoints.GraphQL = "/graphql"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeGraphQL,
		SpecVersion: "2021",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/graphql",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]interface{}{
			"schema": "type Query { hello: String }",
		},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.Path != "/graphql" {
		t.Errorf("route.Path = %v, want /graphql", route.Path)
	}

	hasPost := false
	hasGet := false
	for _, method := range route.Methods {
		if method == "POST" {
			hasPost = true
		}
		if method == "GET" {
			hasGet = true
		}
	}

	if !hasPost || !hasGet {
		t.Error("GraphQL route should support both POST and GET")
	}
}

func TestClient_ConvertToRoutes_RegistryLocation(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Store schema in registry
	schemaPath := "/schemas/test/v1/openapi"
	schema := map[string]interface{}{
		"openapi": "3.1.0",
		"paths": map[string]interface{}{
			"/api": map[string]interface{}{
				"get": map[string]interface{}{},
			},
		},
	}
	registry.PublishSchema(context.Background(), schemaPath, schema)

	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type:         farp.LocationTypeRegistry,
			RegistryPath: schemaPath,
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	if len(routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(routes))
	}
}

func TestClient_ClearCache(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Add something to cache
	client.cacheSchema("hash123", map[string]interface{}{"test": "data"})

	if len(client.schemaCache) == 0 {
		t.Error("cache should not be empty after adding")
	}

	// Clear cache
	client.ClearCache()

	if len(client.schemaCache) != 0 {
		t.Error("cache should be empty after clearing")
	}
}

func TestClient_GetManifest(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"

	// Add to cache
	client.manifestCache["instance-123"] = manifest

	// Get manifest
	retrieved, ok := client.GetManifest("instance-123")
	if !ok {
		t.Error("GetManifest() should find cached manifest")
	}

	if retrieved.InstanceID != "instance-123" {
		t.Errorf("retrieved.InstanceID = %v, want instance-123", retrieved.InstanceID)
	}

	// Try non-existent
	_, ok = client.GetManifest("nonexistent")
	if ok {
		t.Error("GetManifest() should not find non-existent manifest")
	}
}

func TestClient_SchemaCache(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	hash := "test-hash-123"
	schema := map[string]interface{}{"test": "data"}

	// Should not be in cache initially
	_, ok := client.getSchemaFromCache(hash)
	if ok {
		t.Error("schema should not be in cache initially")
	}

	// Add to cache
	client.cacheSchema(hash, schema)

	// Should be in cache now
	cached, ok := client.getSchemaFromCache(hash)
	if !ok {
		t.Error("schema should be in cache after caching")
	}

	cachedMap, ok := cached.(map[string]interface{})
	if !ok {
		t.Fatal("cached schema is not map[string]interface{}")
	}

	if cachedMap["test"] != "data" {
		t.Error("cached data doesn't match original")
	}
}

func TestClient_ConvertToRoutes_InvalidSchema(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Test with invalid schema structure
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType:  "application/json",
		Hash:         "abc123",
		Size:         1024,
		InlineSchema: "invalid-not-a-map", // Invalid schema
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	// Should handle invalid schema gracefully and return empty routes
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for invalid schema, got %d", len(routes))
	}
}

func TestClient_ConvertToRoutes_SchemaNotFound(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Schema references non-existent registry path
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type:         farp.LocationTypeRegistry,
			RegistryPath: "/nonexistent/path",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	// Should handle missing schema gracefully
	if len(routes) != 0 {
		t.Errorf("expected 0 routes when schema not found, got %d", len(routes))
	}
}

func TestClient_ConvertToRoutes_HTTPLocation(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Schema with HTTP location (not yet implemented)
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeHTTP,
			URL:  "http://test.com/openapi.json",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	// Should handle unimplemented HTTP fetch gracefully
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for HTTP location (not implemented), got %d", len(routes))
	}
}

func TestClient_ConvertToRoutes_UnknownSchemaType(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Schema with unknown type
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeCustom, // Not handled by conversion
		SpecVersion: "1.0.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType:  "application/json",
		Hash:         "abc123",
		Size:         1024,
		InlineSchema: map[string]interface{}{"test": "data"},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	// Should skip unknown schema types
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for unknown schema type, got %d", len(routes))
	}
}

func TestClient_ConvertToRoutes_OpenAPI_NoPaths(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// OpenAPI schema without paths
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]interface{}{
			"openapi": "3.1.0",
			// No paths
		},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	// Should handle missing paths gracefully
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for OpenAPI without paths, got %d", len(routes))
	}
}

func TestClient_ConvertToRoutes_AsyncAPI_NoChannels(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// AsyncAPI schema without channels
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeAsyncAPI,
		SpecVersion: "3.0.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]interface{}{
			"asyncapi": "3.0.0",
			// No channels
		},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	// Should handle missing channels gracefully
	if len(routes) != 0 {
		t.Errorf("expected 0 routes for AsyncAPI without channels, got %d", len(routes))
	}
}

func TestClient_ConvertToRoutes_GraphQL_DefaultPath(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// GraphQL without explicit endpoint
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	// No GraphQL endpoint specified
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeGraphQL,
		SpecVersion: "2021",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/graphql",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]interface{}{
			"schema": "type Query { hello: String }",
		},
	})

	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	// Should use default path
	if routes[0].Path != "/graphql" {
		t.Errorf("expected default path /graphql, got %s", routes[0].Path)
	}
}

func TestClient_ConvertToRoutes_CacheHit(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	hash := "abc123"
	schema := map[string]interface{}{
		"openapi": "3.1.0",
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{},
			},
		},
	}

	// Pre-cache the schema
	client.cacheSchema(hash, schema)

	// Create manifest with cached schema
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType:  "application/json",
		Hash:         hash,
		Size:         1024,
		InlineSchema: schema,
	})

	// Convert should use cached schema
	manifests := []*farp.SchemaManifest{manifest}
	routes := client.ConvertToRoutes(manifests)

	if len(routes) != 1 {
		t.Errorf("expected 1 route from cached schema, got %d", len(routes))
	}
}

func TestClient_WatchServices_ListError(t *testing.T) {
	// Create a custom registry for testing error paths
	errorRegistry := &errorMockRegistry{error: errors.New("list error")}
	errorClient := NewClient(errorRegistry)

	err := errorClient.WatchServices(context.Background(), "test-service", func(routes []ServiceRoute) {})

	if err == nil {
		t.Error("expected error from WatchServices when ListManifests fails")
	}
}

// errorMockRegistry for testing error paths
type errorMockRegistry struct {
	error error
}

func (m *errorMockRegistry) RegisterManifest(ctx context.Context, manifest *farp.SchemaManifest) error {
	return m.error
}

func (m *errorMockRegistry) GetManifest(ctx context.Context, instanceID string) (*farp.SchemaManifest, error) {
	return nil, m.error
}

func (m *errorMockRegistry) UpdateManifest(ctx context.Context, manifest *farp.SchemaManifest) error {
	return m.error
}

func (m *errorMockRegistry) DeleteManifest(ctx context.Context, instanceID string) error {
	return m.error
}

func (m *errorMockRegistry) ListManifests(ctx context.Context, serviceName string) ([]*farp.SchemaManifest, error) {
	return nil, m.error
}

func (m *errorMockRegistry) PublishSchema(ctx context.Context, path string, schema interface{}) error {
	return m.error
}

func (m *errorMockRegistry) FetchSchema(ctx context.Context, path string) (interface{}, error) {
	return nil, m.error
}

func (m *errorMockRegistry) DeleteSchema(ctx context.Context, path string) error {
	return m.error
}

func (m *errorMockRegistry) WatchManifests(ctx context.Context, serviceName string, onChange farp.ManifestChangeHandler) error {
	return m.error
}

func (m *errorMockRegistry) WatchSchemas(ctx context.Context, path string, onChange farp.SchemaChangeHandler) error {
	return m.error
}

func (m *errorMockRegistry) Close() error {
	return m.error
}

func (m *errorMockRegistry) Health(ctx context.Context) error {
	return m.error
}

