package gateway

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xraph/farp"
)

// Mock registry for testing.
type mockRegistry struct {
	mu                 sync.RWMutex
	manifests          map[string]*farp.SchemaManifest
	schemas            map[string]any
	watchHandler       farp.ManifestChangeHandler
	schemaWatchHandler farp.SchemaChangeHandler
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		manifests: make(map[string]*farp.SchemaManifest),
		schemas:   make(map[string]any),
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

func (m *mockRegistry) PublishSchema(ctx context.Context, path string, schema any) error {
	m.schemas[path] = schema

	return nil
}

func (m *mockRegistry) FetchSchema(ctx context.Context, path string) (any, error) {
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
	m.mu.Lock()
	m.watchHandler = onChange
	m.mu.Unlock()
	<-ctx.Done()

	return nil
}

func (m *mockRegistry) WatchSchemas(ctx context.Context, path string, onChange farp.SchemaChangeHandler) error {
	m.mu.Lock()
	m.schemaWatchHandler = onChange
	m.mu.Unlock()
	<-ctx.Done()

	return nil
}

func (m *mockRegistry) Close() error {
	return nil
}

func (m *mockRegistry) Health(ctx context.Context) error {
	return nil
}

// Trigger a manifest change event.
func (m *mockRegistry) triggerManifestEvent(eventType farp.EventType, manifest *farp.SchemaManifest) {
	m.mu.RLock()
	handler := m.watchHandler
	m.mu.RUnlock()

	if handler != nil {
		handler(&farp.ManifestEvent{
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
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/test": map[string]any{
					"get": map[string]any{},
				},
			},
		},
	})
	registry.RegisterManifest(context.Background(), manifest)

	// Watch for changes
	var changeCallCount int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := client.WatchServices(ctx, "test-service", func(routes []ServiceRoute) {
			count := atomic.AddInt32(&changeCallCount, 1)
			if count == 1 {
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

	// Trigger an update event with a DIFFERENT route to ensure onChange fires.
	// Note: WatchServices now skips onChange when routes haven't changed
	// (this prevents intermittent 404s from unnecessary remounts).
	updatedManifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	updatedManifest.Endpoints.Health = "/health"
	updatedManifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "def456",
		Size:        2048,
		InlineSchema: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/test": map[string]any{
					"get": map[string]any{},
				},
				"/new-route": map[string]any{
					"post": map[string]any{},
				},
			},
		},
	})
	registry.triggerManifestEvent(farp.EventTypeUpdated, updatedManifest)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&changeCallCount) < 2 {
		t.Errorf("expected at least 2 change callbacks, got %d", atomic.LoadInt32(&changeCallCount))
	}
}

func TestClient_WatchServices_SkipsUnchangedRoutes(t *testing.T) {
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
		InlineSchema: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/test": map[string]any{
					"get": map[string]any{},
				},
			},
		},
	})
	registry.RegisterManifest(context.Background(), manifest)

	var changeCallCount int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := client.WatchServices(ctx, "test-service", func(routes []ServiceRoute) {
			atomic.AddInt32(&changeCallCount, 1)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("WatchServices() error = %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Trigger update with SAME routes — should be skipped
	registry.triggerManifestEvent(farp.EventTypeUpdated, manifest)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	// Should only have 1 callback (initial load), the update should be skipped
	if count := atomic.LoadInt32(&changeCallCount); count != 1 {
		t.Errorf("expected exactly 1 callback (unchanged routes should be skipped), got %d", count)
	}
}

func TestClient_WatchServices_RoutesChecksumFastPath(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.RoutesChecksum = "hash-v1"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
		InlineSchema: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/test": map[string]any{
					"get": map[string]any{},
				},
			},
		},
	})
	registry.RegisterManifest(context.Background(), manifest)

	var changeCallCount int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := client.WatchServices(ctx, "test-service", func(routes []ServiceRoute) {
			atomic.AddInt32(&changeCallCount, 1)
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("WatchServices() error = %v", err)
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Update with same RoutesChecksum — fast path should skip
	updatedManifest := manifest.Clone()
	updatedManifest.Schemas[0].Hash = "different-schema-hash" // Schema changed
	updatedManifest.RoutesChecksum = "hash-v1"                // But routes unchanged
	registry.triggerManifestEvent(farp.EventTypeUpdated, updatedManifest)
	time.Sleep(50 * time.Millisecond)

	cancel()
	time.Sleep(50 * time.Millisecond)

	if count := atomic.LoadInt32(&changeCallCount); count != 1 {
		t.Errorf("expected 1 callback (fast path should skip unchanged routes), got %d", count)
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
		InlineSchema: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/users": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
				"/posts": map[string]any{
					"get": map[string]any{},
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
		InlineSchema: map[string]any{
			"asyncapi": "3.0.0",
			"channels": map[string]any{
				"/ws/notifications": map[string]any{},
				"/ws/messages":      map[string]any{},
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
		InlineSchema: map[string]any{
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
	schema := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/api": map[string]any{
				"get": map[string]any{},
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
	client.cacheSchema("hash123", map[string]any{"test": "data"})

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
	schema := map[string]any{"test": "data"}

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

	cachedMap, ok := cached.(map[string]any)
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
		InlineSchema: map[string]any{"test": "data"},
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
		InlineSchema: map[string]any{
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
		InlineSchema: map[string]any{
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
		InlineSchema: map[string]any{
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
	schema := map[string]any{
		"openapi": "3.1.0",
		"paths": map[string]any{
			"/test": map[string]any{
				"get": map[string]any{},
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

func TestClient_OpenAPISchemaToRoutes_EndToEnd(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Simulate what a service does: generate OpenAPI schema from RouteDescriptors,
	// embed it in a manifest, then verify the gateway can extract correct routes.
	openAPISchema := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Order Service",
			"version": "v2.0.0",
		},
		"servers": []any{
			map[string]any{"url": "https://orders.internal:8443"},
		},
		"paths": map[string]any{
			"/orders": map[string]any{
				"get": map[string]any{
					"operationId": "listOrders",
					"summary":     "List all orders",
				},
				"post": map[string]any{
					"operationId": "createOrder",
					"summary":     "Create an order",
				},
			},
			"/orders/{id}": map[string]any{
				"get": map[string]any{
					"operationId": "getOrder",
				},
				"put": map[string]any{
					"operationId": "updateOrder",
				},
				"delete": map[string]any{
					"operationId": "deleteOrder",
				},
			},
			"/orders/{id}/items": map[string]any{
				"get": map[string]any{
					"operationId": "listOrderItems",
				},
			},
		},
	}

	manifest := farp.NewManifest("order-service", "v2.0.0", "order-inst-1")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:         farp.SchemaTypeOpenAPI,
		SpecVersion:  "3.1.0",
		Location:     farp.SchemaLocation{Type: farp.LocationTypeInline},
		ContentType:  "application/json",
		Hash:         "openapi-hash-001",
		Size:         2048,
		InlineSchema: openAPISchema,
	})

	routes := client.ConvertToRoutes([]*farp.SchemaManifest{manifest})

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}

	// Build a lookup by path for deterministic assertions
	routeByPath := make(map[string]ServiceRoute)
	for _, r := range routes {
		routeByPath[r.Path] = r
	}

	// Verify /orders route
	ordersRoute, ok := routeByPath["/orders"]
	if !ok {
		t.Fatal("expected route for /orders")
	}

	if len(ordersRoute.Methods) != 2 {
		t.Errorf("/orders should have 2 methods, got %d", len(ordersRoute.Methods))
	}

	// Base URL should come from servers array
	if ordersRoute.TargetURL != "https://orders.internal:8443/orders" {
		t.Errorf("/orders TargetURL = %v, want https://orders.internal:8443/orders", ordersRoute.TargetURL)
	}

	if ordersRoute.HealthURL != "https://orders.internal:8443/health" {
		t.Errorf("/orders HealthURL = %v, want https://orders.internal:8443/health", ordersRoute.HealthURL)
	}

	if ordersRoute.ServiceName != "order-service" {
		t.Errorf("ServiceName = %v, want order-service", ordersRoute.ServiceName)
	}

	if ordersRoute.ServiceVersion != "v2.0.0" {
		t.Errorf("ServiceVersion = %v, want v2.0.0", ordersRoute.ServiceVersion)
	}

	if ordersRoute.InstanceID != "order-inst-1" {
		t.Errorf("InstanceID = %v, want order-inst-1", ordersRoute.InstanceID)
	}

	if ordersRoute.Metadata["schema_type"] != "openapi" {
		t.Errorf("metadata schema_type = %v, want openapi", ordersRoute.Metadata["schema_type"])
	}

	// Verify /orders/{id} route has 3 methods
	orderByIDRoute, ok := routeByPath["/orders/{id}"]
	if !ok {
		t.Fatal("expected route for /orders/{id}")
	}

	if len(orderByIDRoute.Methods) != 3 {
		t.Errorf("/orders/{id} should have 3 methods, got %d", len(orderByIDRoute.Methods))
	}

	// Verify /orders/{id}/items route
	itemsRoute, ok := routeByPath["/orders/{id}/items"]
	if !ok {
		t.Fatal("expected route for /orders/{id}/items")
	}

	if len(itemsRoute.Methods) != 1 {
		t.Errorf("/orders/{id}/items should have 1 method, got %d", len(itemsRoute.Methods))
	}
}

func TestClient_AsyncAPISchemaToRoutes_EndToEnd(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Simulate an AsyncAPI service with channels (WebSocket endpoints)
	asyncAPISchema := map[string]any{
		"asyncapi": "3.0.0",
		"info": map[string]any{
			"title":   "Chat Service",
			"version": "v1.0.0",
		},
		"channels": map[string]any{
			"/ws/chat": map[string]any{
				"description": "Real-time chat messages",
				"messages": map[string]any{
					"chatMessage": map[string]any{
						"payload": map[string]any{"type": "object"},
					},
				},
			},
			"/ws/presence": map[string]any{
				"description": "User presence updates",
			},
			"/ws/typing": map[string]any{
				"description": "Typing indicators",
			},
		},
		"operations": map[string]any{
			"onChat": map[string]any{
				"action":  "receive",
				"channel": map[string]any{"$ref": "#/channels/~1ws~1chat"},
			},
		},
	}

	manifest := farp.NewManifest("chat-service", "v1.0.0", "chat-inst-1")
	manifest.Endpoints.Health = "/health"
	manifest.Instance = &farp.InstanceMetadata{
		Address: "chat-service.internal:9090",
	}
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:         farp.SchemaTypeAsyncAPI,
		SpecVersion:  "3.0.0",
		Location:     farp.SchemaLocation{Type: farp.LocationTypeInline},
		ContentType:  "application/json",
		Hash:         "asyncapi-hash-001",
		Size:         1024,
		InlineSchema: asyncAPISchema,
	})

	routes := client.ConvertToRoutes([]*farp.SchemaManifest{manifest})

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes (one per channel), got %d", len(routes))
	}

	// All AsyncAPI routes should be WEBSOCKET
	for _, route := range routes {
		if len(route.Methods) != 1 || route.Methods[0] != "WEBSOCKET" {
			t.Errorf("route %s should have WEBSOCKET method, got %v", route.Path, route.Methods)
		}

		if route.Metadata["schema_type"] != "asyncapi" {
			t.Errorf("route %s metadata.schema_type = %v, want asyncapi", route.Path, route.Metadata["schema_type"])
		}

		if route.Metadata["protocol"] != "websocket" {
			t.Errorf("route %s metadata.protocol = %v, want websocket", route.Path, route.Metadata["protocol"])
		}

		if route.ServiceName != "chat-service" {
			t.Errorf("route %s ServiceName = %v, want chat-service", route.Path, route.ServiceName)
		}

		// Base URL should come from instance address
		expectedBase := "http://chat-service.internal:9090"
		if route.HealthURL != expectedBase+"/health" {
			t.Errorf("route %s HealthURL = %v, want %s/health", route.Path, route.HealthURL, expectedBase)
		}
	}
}

func TestClient_OpenAPIRoutes_BaseURL_Priority(t *testing.T) {
	registry := newMockRegistry()

	tests := []struct {
		name        string
		schema      map[string]any
		schemaDesc  farp.SchemaDescriptor
		instance    *farp.InstanceMetadata
		wantBaseURL string
	}{
		{
			name: "servers array takes priority",
			schema: map[string]any{
				"openapi": "3.1.0",
				"servers": []any{
					map[string]any{"url": "https://api.example.com"},
				},
				"paths": map[string]any{"/test": map[string]any{"get": map[string]any{}}},
			},
			schemaDesc: farp.SchemaDescriptor{
				Type:        farp.SchemaTypeOpenAPI,
				SpecVersion: "3.1.0",
				Location:    farp.SchemaLocation{Type: farp.LocationTypeInline},
				ContentType: "application/json",
				Hash:        "hash1",
				Size:        100,
			},
			instance:    &farp.InstanceMetadata{Address: "instance.local:8080"},
			wantBaseURL: "https://api.example.com",
		},
		{
			name: "instance address used when no servers",
			schema: map[string]any{
				"openapi": "3.1.0",
				"paths":   map[string]any{"/test": map[string]any{"get": map[string]any{}}},
			},
			schemaDesc: farp.SchemaDescriptor{
				Type:        farp.SchemaTypeOpenAPI,
				SpecVersion: "3.1.0",
				Location:    farp.SchemaLocation{Type: farp.LocationTypeInline},
				ContentType: "application/json",
				Hash:        "hash2",
				Size:        100,
			},
			instance:    &farp.InstanceMetadata{Address: "instance.local:9090"},
			wantBaseURL: "http://instance.local:9090",
		},
		{
			name: "instance address used when no servers or location URL",
			schema: map[string]any{
				"openapi": "3.1.0",
				"paths":   map[string]any{"/test": map[string]any{"get": map[string]any{}}},
			},
			schemaDesc: farp.SchemaDescriptor{
				Type:         farp.SchemaTypeOpenAPI,
				SpecVersion:  "3.1.0",
				Location:     farp.SchemaLocation{Type: farp.LocationTypeInline},
				ContentType:  "application/json",
				Hash:         "hash3",
				Size:         100,
				InlineSchema: nil, // Will be set below
			},
			instance:    &farp.InstanceMetadata{Address: "my-service.local:3000"},
			wantBaseURL: "http://my-service.local:3000",
		},
		{
			name: "fallback to service name",
			schema: map[string]any{
				"openapi": "3.1.0",
				"paths":   map[string]any{"/test": map[string]any{"get": map[string]any{}}},
			},
			schemaDesc: farp.SchemaDescriptor{
				Type:         farp.SchemaTypeOpenAPI,
				SpecVersion:  "3.1.0",
				Location:     farp.SchemaLocation{Type: farp.LocationTypeInline},
				ContentType:  "application/json",
				Hash:         "hash4",
				Size:         100,
				InlineSchema: nil,
			},
			instance:    nil,
			wantBaseURL: "http://priority-svc:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(registry)

			manifest := farp.NewManifest("priority-svc", "v1.0.0", "inst-"+tt.name)
			manifest.Endpoints.Health = "/health"
			manifest.Instance = tt.instance

			desc := tt.schemaDesc
			desc.InlineSchema = tt.schema

			manifest.AddSchema(desc)

			routes := client.ConvertToRoutes([]*farp.SchemaManifest{manifest})
			if len(routes) != 1 {
				t.Fatalf("expected 1 route, got %d", len(routes))
			}

			expectedTarget := tt.wantBaseURL + "/test"
			if routes[0].TargetURL != expectedTarget {
				t.Errorf("TargetURL = %v, want %v", routes[0].TargetURL, expectedTarget)
			}
		})
	}
}

func TestClient_MultiSchemaManifest(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	// Manifest with both OpenAPI and AsyncAPI schemas
	manifest := farp.NewManifest("multi-service", "v1.0.0", "multi-inst-1")
	manifest.Endpoints.Health = "/health"

	// OpenAPI schema (REST endpoints)
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location:    farp.SchemaLocation{Type: farp.LocationTypeInline},
		ContentType: "application/json",
		Hash:        "openapi-multi-hash",
		Size:        512,
		InlineSchema: map[string]any{
			"openapi": "3.1.0",
			"paths": map[string]any{
				"/api/users": map[string]any{
					"get":  map[string]any{},
					"post": map[string]any{},
				},
			},
		},
	})

	// AsyncAPI schema (WebSocket channels)
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeAsyncAPI,
		SpecVersion: "3.0.0",
		Location:    farp.SchemaLocation{Type: farp.LocationTypeInline},
		ContentType: "application/json",
		Hash:        "asyncapi-multi-hash",
		Size:        512,
		InlineSchema: map[string]any{
			"asyncapi": "3.0.0",
			"channels": map[string]any{
				"/ws/events": map[string]any{},
			},
		},
	})

	routes := client.ConvertToRoutes([]*farp.SchemaManifest{manifest})

	if len(routes) != 2 {
		t.Fatalf("expected 2 routes (1 REST + 1 WebSocket), got %d", len(routes))
	}

	var hasREST, hasWebSocket bool

	for _, route := range routes {
		schemaType, _ := route.Metadata["schema_type"].(string)

		switch schemaType {
		case "openapi":
			hasREST = true

			if route.Path != "/api/users" {
				t.Errorf("REST route path = %v, want /api/users", route.Path)
			}

			if len(route.Methods) != 2 {
				t.Errorf("REST route should have 2 methods, got %d", len(route.Methods))
			}
		case "asyncapi":
			hasWebSocket = true

			if route.Path != "/ws/events" {
				t.Errorf("WebSocket route path = %v, want /ws/events", route.Path)
			}

			if len(route.Methods) != 1 || route.Methods[0] != "WEBSOCKET" {
				t.Errorf("WebSocket route methods = %v, want [WEBSOCKET]", route.Methods)
			}
		}
	}

	if !hasREST {
		t.Error("expected a REST route from OpenAPI schema")
	}

	if !hasWebSocket {
		t.Error("expected a WebSocket route from AsyncAPI schema")
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

// errorMockRegistry for testing error paths.
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

func (m *errorMockRegistry) PublishSchema(ctx context.Context, path string, schema any) error {
	return m.error
}

func (m *errorMockRegistry) FetchSchema(ctx context.Context, path string) (any, error) {
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
