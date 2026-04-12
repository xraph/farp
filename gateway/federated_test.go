package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xraph/farp"
	"github.com/xraph/farp/merger"
)

// setupFederatedTest creates a client with two OpenAPI services pre-loaded.
func setupFederatedTest(t *testing.T) *Client {
	t.Helper()

	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	manifest1 := createTestManifestWithOpenAPI("user-service", "v1.0.0", "instance-1")
	manifest2 := createTestManifestWithOpenAPI("order-service", "v1.0.0", "instance-2")

	if err := registry.RegisterManifest(ctx, manifest1); err != nil {
		t.Fatalf("Failed to register manifest1: %v", err)
	}

	if err := registry.RegisterManifest(ctx, manifest2); err != nil {
		t.Fatalf("Failed to register manifest2: %v", err)
	}

	schema1 := createTestOpenAPISchema("user-service", "/users")
	schema2 := createTestOpenAPISchema("order-service", "/orders")

	hash1 := manifest1.Schemas[0].Hash
	hash2 := manifest2.Schemas[0].Hash

	registry.schemas[hash1] = schema1
	registry.schemas[hash2] = schema2

	client.mu.Lock()
	client.manifestCache[manifest1.InstanceID] = manifest1
	client.manifestCache[manifest2.InstanceID] = manifest2
	client.schemaCache[hash1] = schema1
	client.schemaCache[hash2] = schema2
	client.mu.Unlock()

	return client
}

// createTestManifestWithAsyncAPI creates a manifest with an AsyncAPI schema descriptor.
func createTestManifestWithAsyncAPI(serviceName, version, instanceID string) *farp.SchemaManifest {
	manifest := farp.NewManifest(serviceName, version, instanceID)
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeAsyncAPI,
		SpecVersion: "2.6.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        serviceName + "-asyncapi-hash-123",
		Size:        512,
		Metadata: &farp.ProtocolMetadata{
			AsyncAPI: &farp.AsyncAPIMetadata{
				Protocol: "websocket",
			},
		},
	})

	return manifest
}

// createTestAsyncAPISchema creates a test AsyncAPI schema with the given channel.
func createTestAsyncAPISchema(serviceName, channelPath string) map[string]any {
	return map[string]any{
		"asyncapi": "2.6.0",
		"info": map[string]any{
			"title":   serviceName,
			"version": "1.0.0",
		},
		"channels": map[string]any{
			channelPath: map[string]any{
				"description": "Channel for " + serviceName,
				"subscribe": map[string]any{
					"operationId": serviceName + "_subscribe",
					"summary":     "Subscribe to " + serviceName,
				},
			},
		},
	}
}

func TestFederatedHandler_OpenAPIEndpoint(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}

	if resp.Header.Get("X-Farp-Federated") != "true" {
		t.Error("Expected X-Farp-Federated header to be true")
	}

	// Verify it's valid OpenAPI JSON
	body, _ := io.ReadAll(resp.Body)

	var spec merger.OpenAPISpec
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to unmarshal OpenAPI JSON: %v", err)
	}

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("Expected openapi version 3.1.0, got %s", spec.OpenAPI)
	}

	if spec.Info.Title != "Federated API" {
		t.Errorf("Expected title 'Federated API', got '%s'", spec.Info.Title)
	}

	if len(spec.Paths) == 0 {
		t.Error("Expected merged spec to have paths")
	}
}

func TestFederatedHandler_AsyncAPIEndpoint(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	manifest := createTestManifestWithAsyncAPI("events-service", "v1.0.0", "instance-events")
	if err := registry.RegisterManifest(ctx, manifest); err != nil {
		t.Fatalf("Failed to register manifest: %v", err)
	}

	schema := createTestAsyncAPISchema("events-service", "events/created")
	hash := manifest.Schemas[0].Hash
	registry.schemas[hash] = schema

	client.mu.Lock()
	client.manifestCache[manifest.InstanceID] = manifest
	client.schemaCache[hash] = schema
	client.mu.Unlock()

	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/asyncapi.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)

	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to unmarshal AsyncAPI JSON: %v", err)
	}

	if v, ok := spec["asyncapi"].(string); !ok || v != "2.6.0" {
		t.Errorf("Expected asyncapi version 2.6.0, got %v", spec["asyncapi"])
	}

	channels, ok := spec["channels"].(map[string]any)
	if !ok || len(channels) == 0 {
		t.Error("Expected merged AsyncAPI spec to have channels")
	}
}

func TestFederatedHandler_EmptyProtocol(t *testing.T) {
	// Only OpenAPI services registered, request AsyncAPI
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/asyncapi.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for empty protocol, got %d", resp.StatusCode)
	}
}

func TestFederatedHandler_NotFound(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown path, got %d", resp.StatusCode)
	}
}

func TestFederatedHandler_MethodNotAllowed(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodPost, "/openapi.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405 for POST, got %d", resp.StatusCode)
	}
}

func TestFederatedHandler_Summary(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/summary", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)

	var summary federatedSummary
	if err := json.Unmarshal(body, &summary); err != nil {
		t.Fatalf("Failed to unmarshal summary: %v", err)
	}

	if !summary.HasOpenAPI {
		t.Error("Expected has_openapi to be true")
	}

	if summary.BuiltAt == "" {
		t.Error("Expected built_at to be set")
	}
}

func TestFederatedHandler_WithBasePath(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client, WithBasePath("/_farp/federated"))

	req := httptest.NewRequest(http.MethodGet, "/_farp/federated/openapi.json", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 with base path, got %d: %s", resp.StatusCode, string(body))
	}
}

func TestFederatedCache_StaleDetection(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	// First request builds cache
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatal("First request failed")
	}

	firstBuilt := handler.LastBuilt()

	// Add a new manifest (simulates a new service registering)
	manifest3 := createTestManifestWithOpenAPI("payment-service", "v1.0.0", "instance-3")
	schema3 := createTestOpenAPISchema("payment-service", "/payments")
	hash3 := manifest3.Schemas[0].Hash

	client.mu.Lock()
	client.manifestCache[manifest3.InstanceID] = manifest3
	client.schemaCache[hash3] = schema3
	client.mu.Unlock()

	// Second request should detect stale cache and rebuild
	req2 := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Result().StatusCode != http.StatusOK {
		t.Fatal("Second request failed")
	}

	secondBuilt := handler.LastBuilt()

	if !secondBuilt.After(firstBuilt) {
		t.Error("Expected cache to be rebuilt after manifest change")
	}
}

func TestFederatedCache_NoRebuildWhenUnchanged(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	// First request
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	firstBuilt := handler.LastBuilt()

	// Second request with same manifests
	req2 := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	secondBuilt := handler.LastBuilt()

	if !secondBuilt.Equal(firstBuilt) {
		t.Error("Expected cache NOT to be rebuilt when manifests unchanged")
	}
}

func TestFederatedCache_ExplicitInvalidate(t *testing.T) {
	client := setupFederatedTest(t)
	handler := NewFederatedSchemaHandler(client)

	// First request
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	firstBuilt := handler.LastBuilt()

	// Invalidate
	handler.Invalidate()

	// Second request should rebuild
	req2 := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	secondBuilt := handler.LastBuilt()

	if !secondBuilt.After(firstBuilt) {
		t.Error("Expected cache to be rebuilt after explicit invalidation")
	}
}

func TestGetMergedOpenAPIJSON_ProperFormat(t *testing.T) {
	client := setupFederatedTest(t)

	ctx := context.Background()

	data, err := client.GetMergedOpenAPIJSON(ctx, "")
	if err != nil {
		t.Fatalf("GetMergedOpenAPIJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty JSON")
	}

	// Must be valid JSON that deserializes to OpenAPISpec
	var spec merger.OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("JSON output is not valid OpenAPISpec: %v", err)
	}

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("Expected openapi 3.1.0, got %s", spec.OpenAPI)
	}

	if spec.Info.Title != "Federated API" {
		t.Errorf("Expected title 'Federated API', got '%s'", spec.Info.Title)
	}

	if len(spec.Paths) == 0 {
		t.Error("Expected paths in merged spec")
	}
}

func TestGetMergedAsyncAPIJSON_ProperFormat(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	manifest := createTestManifestWithAsyncAPI("events-service", "v1.0.0", "instance-events")
	if err := registry.RegisterManifest(ctx, manifest); err != nil {
		t.Fatalf("Failed to register manifest: %v", err)
	}

	schema := createTestAsyncAPISchema("events-service", "events/created")
	hash := manifest.Schemas[0].Hash
	registry.schemas[hash] = schema

	client.mu.Lock()
	client.manifestCache[manifest.InstanceID] = manifest
	client.schemaCache[hash] = schema
	client.mu.Unlock()

	data, err := client.GetMergedAsyncAPIJSON(ctx, "")
	if err != nil {
		t.Fatalf("GetMergedAsyncAPIJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty JSON")
	}

	var spec map[string]any
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	if v, ok := spec["asyncapi"].(string); !ok || v != "2.6.0" {
		t.Errorf("Expected asyncapi 2.6.0, got %v", spec["asyncapi"])
	}
}

func TestFederatedHandler_AfterRestart(t *testing.T) {
	// Simulate gateway restart: fresh client, populate registry, verify federated endpoints
	registry := newMockRegistry()

	ctx := context.Background()

	// Services are already registered (they survived the restart)
	manifest1 := createTestManifestWithOpenAPI("user-service", "v1.0.0", "instance-1")
	manifest2 := createTestManifestWithOpenAPI("order-service", "v1.0.0", "instance-2")

	registry.RegisterManifest(ctx, manifest1)
	registry.RegisterManifest(ctx, manifest2)

	schema1 := createTestOpenAPISchema("user-service", "/users")
	schema2 := createTestOpenAPISchema("order-service", "/orders")

	registry.schemas[manifest1.Schemas[0].Hash] = schema1
	registry.schemas[manifest2.Schemas[0].Hash] = schema2

	// Fresh client (simulates restart)
	client := NewClient(registry)

	// Simulate what WatchServices does on startup: load manifests and cache them
	manifests, err := registry.ListManifests(ctx, "")
	if err != nil {
		t.Fatalf("ListManifests failed: %v", err)
	}

	client.mu.Lock()

	for _, m := range manifests {
		client.manifestCache[m.InstanceID] = m
		for _, sd := range m.Schemas {
			if s, ok := registry.schemas[sd.Hash]; ok {
				client.schemaCache[sd.Hash] = s
			}
		}
	}

	client.mu.Unlock()

	// Mount federated handler
	handler := NewFederatedSchemaHandler(client)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 after restart, got %d: %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)

	var spec merger.OpenAPISpec
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to unmarshal OpenAPI JSON after restart: %v", err)
	}

	if len(spec.Paths) == 0 {
		t.Error("Expected paths in federated schema after restart")
	}
}

func TestGetManifestsHash_Deterministic(t *testing.T) {
	client := setupFederatedTest(t)

	hash1 := client.GetManifestsHash()
	hash2 := client.GetManifestsHash()

	if hash1 != hash2 {
		t.Error("Expected hash to be deterministic")
	}

	if hash1 == "" {
		t.Error("Expected non-empty hash")
	}
}

func TestGetManifestsHash_ChangesOnUpdate(t *testing.T) {
	client := setupFederatedTest(t)

	hash1 := client.GetManifestsHash()

	// Add a new manifest
	manifest := createTestManifestWithOpenAPI("new-service", "v1.0.0", "instance-new")

	client.mu.Lock()
	client.manifestCache[manifest.InstanceID] = manifest
	client.mu.Unlock()

	hash2 := client.GetManifestsHash()

	if hash1 == hash2 {
		t.Error("Expected hash to change after adding manifest")
	}
}
