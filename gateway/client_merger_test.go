package gateway

import (
	"context"
	"testing"

	"github.com/xraph/farp"
	"github.com/xraph/farp/merger"
)

func TestClient_GenerateMergedOpenAPI(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	// Register two services with OpenAPI schemas
	manifest1 := createTestManifestWithOpenAPI("user-service", "v1.0.0", "instance-1")
	manifest2 := createTestManifestWithOpenAPI("order-service", "v1.0.0", "instance-2")

	if err := registry.RegisterManifest(ctx, manifest1); err != nil {
		t.Fatalf("Failed to register manifest1: %v", err)
	}
	if err := registry.RegisterManifest(ctx, manifest2); err != nil {
		t.Fatalf("Failed to register manifest2: %v", err)
	}

	// Store schemas
	schema1 := createTestOpenAPISchema("user-service", "/users")
	schema2 := createTestOpenAPISchema("order-service", "/orders")

	hash1 := manifest1.Schemas[0].Hash
	hash2 := manifest2.Schemas[0].Hash

	registry.schemas[hash1] = schema1
	registry.schemas[hash2] = schema2

	// Load manifests into client
	client.mu.Lock()
	client.manifestCache[manifest1.InstanceID] = manifest1
	client.manifestCache[manifest2.InstanceID] = manifest2
	client.schemaCache[hash1] = schema1
	client.schemaCache[hash2] = schema2
	client.mu.Unlock()

	// Test merging
	result, err := client.GenerateMergedOpenAPI(ctx, "")
	if err != nil {
		t.Fatalf("GenerateMergedOpenAPI failed: %v", err)
	}

	if len(result.IncludedServices) != 2 {
		t.Errorf("Expected 2 included services, got %d", len(result.IncludedServices))
	}

	if result.Spec == nil {
		t.Error("Expected merged spec to be non-nil")
	}

	if len(result.Spec.Paths) == 0 {
		t.Error("Expected merged spec to have paths")
	}
}

func TestClient_GenerateMergedOpenAPI_SingleService(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	// Register single service
	manifest := createTestManifestWithOpenAPI("user-service", "v1.0.0", "instance-1")
	if err := registry.RegisterManifest(ctx, manifest); err != nil {
		t.Fatalf("Failed to register manifest: %v", err)
	}

	schema := createTestOpenAPISchema("user-service", "/users")
	hash := manifest.Schemas[0].Hash
	registry.schemas[hash] = schema

	client.mu.Lock()
	client.manifestCache[manifest.InstanceID] = manifest
	client.schemaCache[hash] = schema
	client.mu.Unlock()

	// Test merging single service
	result, err := client.GenerateMergedOpenAPI(ctx, "user-service")
	if err != nil {
		t.Fatalf("GenerateMergedOpenAPI failed: %v", err)
	}

	if len(result.IncludedServices) != 1 {
		t.Errorf("Expected 1 included service, got %d", len(result.IncludedServices))
	}

	if result.IncludedServices[0] != "user-service" {
		t.Errorf("Expected user-service to be included, got %s", result.IncludedServices[0])
	}
}

func TestClient_GenerateMergedOpenAPI_NoServices(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	result, err := client.GenerateMergedOpenAPI(ctx, "")
	if err != nil {
		t.Fatalf("GenerateMergedOpenAPI failed: %v", err)
	}

	if len(result.IncludedServices) != 0 {
		t.Errorf("Expected 0 included services, got %d", len(result.IncludedServices))
	}
}

func TestClient_GenerateMergedOpenAPI_WithCustomConfig(t *testing.T) {
	registry := newMockRegistry()

	config := merger.MergerConfig{
		DefaultConflictStrategy: farp.ConflictStrategyError,
		MergedTitle:             "Custom API",
		MergedDescription:       "Custom description",
		MergedVersion:           "2.0.0",
		IncludeServiceTags:      false,
		SortOutput:              false,
	}

	client := NewClientWithConfig(registry, config)

	ctx := context.Background()

	// Register service
	manifest := createTestManifestWithOpenAPI("user-service", "v1.0.0", "instance-1")
	if err := registry.RegisterManifest(ctx, manifest); err != nil {
		t.Fatalf("Failed to register manifest: %v", err)
	}

	schema := createTestOpenAPISchema("user-service", "/users")
	hash := manifest.Schemas[0].Hash
	registry.schemas[hash] = schema

	client.mu.Lock()
	client.manifestCache[manifest.InstanceID] = manifest
	client.schemaCache[hash] = schema
	client.mu.Unlock()

	result, err := client.GenerateMergedOpenAPI(ctx, "")
	if err != nil {
		t.Fatalf("GenerateMergedOpenAPI failed: %v", err)
	}

	if result.Spec.Info.Title != "Custom API" {
		t.Errorf("Expected title 'Custom API', got '%s'", result.Spec.Info.Title)
	}

	if result.Spec.Info.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", result.Spec.Info.Version)
	}
}

func TestClient_GetMergedOpenAPIJSON(t *testing.T) {
	registry := newMockRegistry()
	client := NewClient(registry)

	ctx := context.Background()

	// Register service
	manifest := createTestManifestWithOpenAPI("user-service", "v1.0.0", "instance-1")
	if err := registry.RegisterManifest(ctx, manifest); err != nil {
		t.Fatalf("Failed to register manifest: %v", err)
	}

	schema := createTestOpenAPISchema("user-service", "/users")
	hash := manifest.Schemas[0].Hash
	registry.schemas[hash] = schema

	client.mu.Lock()
	client.manifestCache[manifest.InstanceID] = manifest
	client.schemaCache[hash] = schema
	client.mu.Unlock()

	json, err := client.GetMergedOpenAPIJSON(ctx, "")
	if err != nil {
		t.Fatalf("GetMergedOpenAPIJSON failed: %v", err)
	}

	if len(json) == 0 {
		t.Error("Expected JSON output to be non-empty")
	}
}

// Helper functions

func createTestManifestWithOpenAPI(serviceName, version, instanceID string) *farp.SchemaManifest {
	manifest := farp.NewManifest(serviceName, version, instanceID)
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        serviceName + "-hash-123",
		Size:        1024,
		Metadata: &farp.ProtocolMetadata{
			OpenAPI: &farp.OpenAPIMetadata{
				Composition: &farp.CompositionConfig{
					IncludeInMerged: true,
				},
			},
		},
	})
	return manifest
}

func createTestOpenAPISchema(serviceName, path string) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":   serviceName,
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			path: map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": serviceName + "_get",
					"summary":     "Get " + serviceName,
					"tags":        []interface{}{serviceName},
				},
			},
		},
	}
}
