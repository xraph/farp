package merger

import (
	"testing"

	"github.com/xraph/farp"
)

func TestMerger_Merge_SingleService(t *testing.T) {
	merger := NewMerger(DefaultMergerConfig())

	manifest := createTestManifest("user-service", "v1.0.0", "instance-123")
	schema := createTestOpenAPISchema("user-service", "/users")

	schemas := []ServiceSchema{
		{
			Manifest: manifest,
			Schema:   schema,
		},
	}

	result, err := merger.Merge(schemas)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(result.IncludedServices) != 1 {
		t.Errorf("Expected 1 included service, got %d", len(result.IncludedServices))
	}

	if len(result.Spec.Paths) == 0 {
		t.Error("Expected paths to be merged")
	}
}

func TestMerger_Merge_MultipleServices(t *testing.T) {
	merger := NewMerger(DefaultMergerConfig())

	manifest1 := createTestManifest("user-service", "v1.0.0", "instance-1")
	schema1 := createTestOpenAPISchema("user-service", "/users")

	manifest2 := createTestManifest("order-service", "v1.0.0", "instance-2")
	schema2 := createTestOpenAPISchema("order-service", "/orders")

	schemas := []ServiceSchema{
		{Manifest: manifest1, Schema: schema1},
		{Manifest: manifest2, Schema: schema2},
	}

	result, err := merger.Merge(schemas)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(result.IncludedServices) != 2 {
		t.Errorf("Expected 2 included services, got %d", len(result.IncludedServices))
	}

	// Check that paths from both services are present
	// Default routing strategy is MountStrategyInstance
	foundUsers, foundOrders := false, false
	for path := range result.Spec.Paths {
		// Paths are prefixed with instance ID by default
		if path == "/instance-1/users" {
			foundUsers = true
		}
		if path == "/instance-2/orders" {
			foundOrders = true
		}
	}

	if !foundUsers || !foundOrders {
		t.Errorf("Expected paths from both services to be present. Got paths: %v", result.Spec.Paths)
		for path := range result.Spec.Paths {
			t.Logf("  Path: %s", path)
		}
	}
}

func TestMerger_ConflictResolution_Prefix(t *testing.T) {
	config := DefaultMergerConfig()
	config.DefaultConflictStrategy = farp.ConflictStrategyPrefix
	merger := NewMerger(config)

	// Both services define /users path
	manifest1 := createTestManifest("service-a", "v1.0.0", "instance-1")
	manifest1.Routing.Strategy = farp.MountStrategyRoot
	schema1 := createTestOpenAPISchema("service-a", "/users")

	manifest2 := createTestManifest("service-b", "v1.0.0", "instance-2")
	manifest2.Routing.Strategy = farp.MountStrategyRoot
	schema2 := createTestOpenAPISchema("service-b", "/users")

	schemas := []ServiceSchema{
		{Manifest: manifest1, Schema: schema1},
		{Manifest: manifest2, Schema: schema2},
	}

	result, err := merger.Merge(schemas)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Should have conflict
	if len(result.Conflicts) == 0 {
		t.Error("Expected conflicts to be reported")
	}

	// Check that conflict was resolved with prefix
	foundPrefixed := false
	for _, conflict := range result.Conflicts {
		if conflict.Strategy == farp.ConflictStrategyPrefix {
			foundPrefixed = true
			break
		}
	}

	if !foundPrefixed {
		t.Error("Expected conflict to be resolved with prefix strategy")
	}
}

func TestMerger_ConflictResolution_Skip(t *testing.T) {
	config := DefaultMergerConfig()
	config.DefaultConflictStrategy = farp.ConflictStrategySkip
	merger := NewMerger(config)

	manifest1 := createTestManifest("service-a", "v1.0.0", "instance-1")
	manifest1.Routing.Strategy = farp.MountStrategyRoot
	schema1 := createTestOpenAPISchema("service-a", "/users")

	manifest2 := createTestManifest("service-b", "v1.0.0", "instance-2")
	manifest2.Routing.Strategy = farp.MountStrategyRoot
	schema2 := createTestOpenAPISchema("service-b", "/users")

	schemas := []ServiceSchema{
		{Manifest: manifest1, Schema: schema1},
		{Manifest: manifest2, Schema: schema2},
	}

	result, err := merger.Merge(schemas)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Should only have one /users path (first one wins)
	count := 0
	for path := range result.Spec.Paths {
		if path == "/users" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("Expected exactly 1 /users path, got %d", count)
	}
}

func TestMerger_ComponentPrefixing(t *testing.T) {
	merger := NewMerger(DefaultMergerConfig())

	manifest := createTestManifest("user-service", "v1.0.0", "instance-123")
	schema := createTestOpenAPIWithComponents("user-service")

	schemas := []ServiceSchema{
		{Manifest: manifest, Schema: schema},
	}

	result, err := merger.Merge(schemas)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Check that component names are prefixed
	foundPrefixed := false
	for name := range result.Spec.Components.Schemas {
		if name == "user-service_User" {
			foundPrefixed = true
			break
		}
	}

	if !foundPrefixed {
		t.Error("Expected component names to be prefixed with service name")
	}
}

func TestMerger_RoutingStrategies(t *testing.T) {
	tests := []struct {
		name           string
		strategy       farp.MountStrategy
		expectedPrefix string
	}{
		{"Root", farp.MountStrategyRoot, ""},
		{"Service", farp.MountStrategyService, "user-service"},
		{"Versioned", farp.MountStrategyVersioned, "user-service/v1.0.0"},
		{"Instance", farp.MountStrategyInstance, "instance-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := NewMerger(DefaultMergerConfig())

			manifest := createTestManifest("user-service", "v1.0.0", "instance-123")
			manifest.Routing.Strategy = tt.strategy
			schema := createTestOpenAPISchema("user-service", "/users")

			schemas := []ServiceSchema{
				{Manifest: manifest, Schema: schema},
			}

			result, err := merger.Merge(schemas)
			if err != nil {
				t.Fatalf("Merge failed: %v", err)
			}

			// Check path prefix
			found := false
			for path := range result.Spec.Paths {
				if tt.expectedPrefix == "" {
					if path == "/users" {
						found = true
					}
				} else if path == "/"+tt.expectedPrefix+"/users" {
					found = true
				}
			}

			if !found {
				t.Errorf("Expected path with prefix '%s' not found", tt.expectedPrefix)
			}
		})
	}
}

func TestMerger_ExcludeFromMerge(t *testing.T) {
	merger := NewMerger(DefaultMergerConfig())

	// Service with IncludeInMerged = false
	manifest := createTestManifest("internal-service", "v1.0.0", "instance-123")
	schema := createTestOpenAPISchema("internal-service", "/internal")

	// Set composition config to exclude
	manifest.Schemas[0].Metadata = &farp.ProtocolMetadata{
		OpenAPI: &farp.OpenAPIMetadata{
			Composition: &farp.CompositionConfig{
				IncludeInMerged: false,
			},
		},
	}

	schemas := []ServiceSchema{
		{Manifest: manifest, Schema: schema},
	}

	result, err := merger.Merge(schemas)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(result.IncludedServices) != 0 {
		t.Error("Expected service to be excluded from merge")
	}

	if len(result.ExcludedServices) != 1 {
		t.Errorf("Expected 1 excluded service, got %d", len(result.ExcludedServices))
	}
}

// Helper functions

func createTestManifest(serviceName, version, instanceID string) *farp.SchemaManifest {
	manifest := farp.NewManifest(serviceName, version, instanceID)
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
					"operationId": serviceName + "_getUsers",
					"summary":     "Get users",
					"tags":        []interface{}{"users"},
				},
			},
		},
	}
}

func createTestOpenAPIWithComponents(serviceName string) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":   serviceName,
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUsers",
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":   map[string]interface{}{"type": "string"},
						"name": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
}

