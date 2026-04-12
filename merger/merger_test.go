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
	// Default routing strategy is MountStrategyService
	foundUsers, foundOrders := false, false

	for path := range result.Spec.Paths {
		// Paths are prefixed with service name by default
		if path == "/user-service/users" {
			foundUsers = true
		}

		if path == "/order-service/orders" {
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

func createTestManifest(serviceName string, version string, instanceID string) *farp.SchemaManifest {
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

func TestMerge_RefRewriting(t *testing.T) {
	merger := NewMerger(DefaultMergerConfig())

	manifest := createTestManifest("user-service", "v1.0.0", "inst-1")
	schema := map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "User Service", "version": "1.0.0"},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"operationId": "listUsers",
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/UserList",
									},
								},
							},
						},
					},
				},
				"post": map[string]any{
					"operationId": "createUser",
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/CreateUserRequest",
								},
							},
						},
					},
					"responses": map[string]any{
						"201": map[string]any{
							"description": "Created",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"$ref": "#/components/schemas/User",
									},
								},
							},
						},
					},
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "string"},
						"name": map[string]any{"type": "string"},
					},
				},
				"UserList": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"$ref": "#/components/schemas/User",
							},
						},
					},
				},
				"CreateUserRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	result, err := merger.Merge([]ServiceSchema{
		{Manifest: manifest, Schema: schema},
	})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Component names should be prefixed
	if _, ok := result.Spec.Components.Schemas["user-service_User"]; !ok {
		t.Error("expected prefixed component 'user-service_User'")
	}

	if _, ok := result.Spec.Components.Schemas["User"]; ok {
		t.Error("unprefixed component 'User' should not exist")
	}

	// $ref inside components should be rewritten (UserList references User)
	userList, ok := result.Spec.Components.Schemas["user-service_UserList"]
	if !ok {
		t.Fatal("expected prefixed component 'user-service_UserList'")
	}

	items, _ := userList["properties"].(map[string]any)
	itemsArr, _ := items["items"].(map[string]any)
	itemsItems, _ := itemsArr["items"].(map[string]any)
	ref, _ := itemsItems["$ref"].(string)

	if ref != "#/components/schemas/user-service_User" {
		t.Errorf("$ref in component schema not rewritten: got %q, want %q",
			ref, "#/components/schemas/user-service_User")
	}

	// $ref in path operations should be rewritten
	for path, pathItem := range result.Spec.Paths {
		if pathItem.Get != nil {
			for _, resp := range pathItem.Get.Responses {
				for _, media := range resp.Content {
					if refVal, ok := media.Schema["$ref"]; ok {
						refStr, _ := refVal.(string)
						if refStr == "#/components/schemas/UserList" {
							t.Errorf("$ref in GET %s response not rewritten: still %q", path, refStr)
						}
					}
				}
			}
		}

		if pathItem.Post != nil && pathItem.Post.RequestBody != nil {
			for _, media := range pathItem.Post.RequestBody.Content {
				if refVal, ok := media.Schema["$ref"]; ok {
					refStr, _ := refVal.(string)
					if refStr == "#/components/schemas/CreateUserRequest" {
						t.Errorf("$ref in POST %s requestBody not rewritten: still %q", path, refStr)
					}
				}
			}
		}
	}
}

func createTestOpenAPISchema(serviceName, path string) map[string]any {
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   serviceName,
			"version": "1.0.0",
		},
		"paths": map[string]any{
			path: map[string]any{
				"get": map[string]any{
					"operationId": serviceName + "_getUsers",
					"summary":     "Get users",
					"tags":        []any{"users"},
				},
			},
		},
	}
}

func TestMerger_Merge_CollapseServiceTags(t *testing.T) {
	config := DefaultMergerConfig()
	config.CollapseServiceTags = true
	m := NewMerger(config)

	// Create a schema with multiple tags (simulating different modules)
	manifest := createTestManifest("TwinOS", "v1.0.0", "inst-1")
	schema := map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "TwinOS", "version": "1.0.0"},
		"paths": map[string]any{
			"/api/v1/agents": map[string]any{
				"get": map[string]any{
					"operationId": "listAgents",
					"tags":        []any{"Agents"},
					"summary":     "List agents",
				},
			},
			"/api/v1/query/execute": map[string]any{
				"post": map[string]any{
					"operationId": "executeQuery",
					"tags":        []any{"Query"},
					"summary":     "Execute query",
				},
			},
			"/api/v1/projects": map[string]any{
				"get": map[string]any{
					"operationId": "listProjects",
					"tags":        []any{"Projects"},
					"summary":     "List projects",
				},
			},
		},
		"tags": []any{
			map[string]any{"name": "Agents", "description": "Agent management"},
			map[string]any{"name": "Query", "description": "Query execution"},
			map[string]any{"name": "Projects", "description": "Project management"},
		},
	}

	result, err := m.Merge([]ServiceSchema{
		{Manifest: manifest, Schema: schema},
	})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Should have exactly 1 tag: "TwinOS"
	if len(result.Spec.Tags) != 1 {
		t.Errorf("expected 1 collapsed tag, got %d", len(result.Spec.Tags))

		for _, tag := range result.Spec.Tags {
			t.Logf("  tag: %s", tag.Name)
		}
	}

	if result.Spec.Tags[0].Name != "TwinOS" {
		t.Errorf("expected collapsed tag name 'TwinOS', got '%s'", result.Spec.Tags[0].Name)
	}

	// All operations should have tag "TwinOS"
	for path, pathItem := range result.Spec.Paths {
		ops := []*Operation{pathItem.Get, pathItem.Post, pathItem.Put, pathItem.Delete}
		for _, op := range ops {
			if op == nil {
				continue
			}

			if len(op.Tags) != 1 || op.Tags[0] != "TwinOS" {
				t.Errorf("operation at %s should have tags [TwinOS], got %v", path, op.Tags)
			}
		}
	}
}

func TestMerger_Merge_CollapseServiceTags_MultipleServices(t *testing.T) {
	config := DefaultMergerConfig()
	config.CollapseServiceTags = true
	m := NewMerger(config)

	manifest1 := createTestManifest("TwinOS", "v1.0.0", "inst-1")
	schema1 := map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "TwinOS", "version": "1.0.0"},
		"paths": map[string]any{
			"/api/agents": map[string]any{
				"get": map[string]any{"operationId": "listAgents", "tags": []any{"Agents"}},
			},
		},
		"tags": []any{map[string]any{"name": "Agents"}},
	}

	manifest2 := createTestManifest("Portal", "v1.0.0", "inst-2")
	manifest2.ServiceName = "Portal"
	schema2 := map[string]any{
		"openapi": "3.1.0",
		"info":    map[string]any{"title": "Portal", "version": "1.0.0"},
		"paths": map[string]any{
			"/api/users": map[string]any{
				"get": map[string]any{"operationId": "listUsers", "tags": []any{"Users"}},
			},
		},
		"tags": []any{map[string]any{"name": "Users"}},
	}

	result, err := m.Merge([]ServiceSchema{
		{Manifest: manifest1, Schema: schema1},
		{Manifest: manifest2, Schema: schema2},
	})
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Should have exactly 2 tags: "TwinOS" and "Portal"
	if len(result.Spec.Tags) != 2 {
		t.Errorf("expected 2 collapsed tags, got %d", len(result.Spec.Tags))

		for _, tag := range result.Spec.Tags {
			t.Logf("  tag: %s", tag.Name)
		}
	}

	tagNames := make(map[string]bool)
	for _, tag := range result.Spec.Tags {
		tagNames[tag.Name] = true
	}

	if !tagNames["TwinOS"] {
		t.Error("expected 'TwinOS' tag")
	}

	if !tagNames["Portal"] {
		t.Error("expected 'Portal' tag")
	}
}

func createTestOpenAPIWithComponents(serviceName string) map[string]any {
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   serviceName,
			"version": "1.0.0",
		},
		"paths": map[string]any{
			"/users": map[string]any{
				"get": map[string]any{
					"operationId": "getUsers",
				},
			},
		},
		"components": map[string]any{
			"schemas": map[string]any{
				"User": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "string"},
						"name": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}
