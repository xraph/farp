package openapi

import (
	"context"
	"testing"

	"github.com/xraph/farp"
)

// Mock application for testing (separate from forge integration mock).
type testApp struct {
	name    string
	version string
	routes  any
}

func (m *testApp) Name() string    { return m.name }
func (m *testApp) Version() string { return m.version }
func (m *testApp) Routes() any     { return m.routes }

func TestNewProvider(t *testing.T) {
	// Test with defaults
	p := NewProvider("", "")
	if p.specVersion != "3.1.0" {
		t.Errorf("expected default spec version '3.1.0', got '%s'", p.specVersion)
	}

	if p.endpoint != "/openapi.json" {
		t.Errorf("expected default endpoint '/openapi.json', got '%s'", p.endpoint)
	}

	// Test with custom values
	p = NewProvider("3.0.0", "/custom/openapi.json")
	if p.specVersion != "3.0.0" {
		t.Errorf("expected spec version '3.0.0', got '%s'", p.specVersion)
	}

	if p.endpoint != "/custom/openapi.json" {
		t.Errorf("expected endpoint '/custom/openapi.json', got '%s'", p.endpoint)
	}
}

func TestProvider_Type(t *testing.T) {
	p := NewProvider("", "")
	if p.Type() != farp.SchemaTypeOpenAPI {
		t.Errorf("expected type OpenAPI, got %v", p.Type())
	}
}

func TestProvider_SpecVersion(t *testing.T) {
	p := NewProvider("3.0.1", "")
	if p.SpecVersion() != "3.0.1" {
		t.Errorf("expected spec version '3.0.1', got '%s'", p.SpecVersion())
	}
}

func TestProvider_ContentType(t *testing.T) {
	p := NewProvider("", "")
	if p.ContentType() != "application/json" {
		t.Errorf("expected content type 'application/json', got '%s'", p.ContentType())
	}
}

func TestProvider_Endpoint(t *testing.T) {
	p := NewProvider("", "/custom.json")
	if p.Endpoint() != "/custom.json" {
		t.Errorf("expected endpoint '/custom.json', got '%s'", p.Endpoint())
	}
}

func TestProvider_Generate(t *testing.T) {
	p := NewProvider("3.1.0", "")
	ctx := context.Background()

	// Test successful generation
	app := &testApp{
		name:    "test-service",
		version: "v1.0.0",
		routes:  []string{"route1", "route2"},
	}

	schema, err := p.Generate(ctx, app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	schemaMap, ok := schema.(map[string]any)
	if !ok {
		t.Fatal("schema should be map[string]interface{}")
	}

	if schemaMap["openapi"] != "3.1.0" {
		t.Errorf("expected openapi version '3.1.0', got %v", schemaMap["openapi"])
	}

	info, ok := schemaMap["info"].(map[string]any)
	if !ok {
		t.Fatal("info should be map[string]interface{}")
	}

	if info["title"] != "test-service" {
		t.Errorf("expected title 'test-service', got %v", info["title"])
	}

	if info["version"] != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %v", info["version"])
	}

	// Test with nil routes
	appNoRoutes := &testApp{
		name:    "test-service",
		version: "v1.0.0",
		routes:  nil,
	}

	_, err = p.Generate(ctx, appNoRoutes)
	if err == nil {
		t.Error("expected error for app with nil routes")
	}
}

func TestProvider_Validate(t *testing.T) {
	p := NewProvider("", "")

	tests := []struct {
		name    string
		schema  any
		wantErr bool
	}{
		{
			name: "valid schema",
			schema: map[string]any{
				"openapi": "3.1.0",
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"paths": map[string]any{},
			},
			wantErr: false,
		},
		{
			name:    "not a map",
			schema:  "invalid",
			wantErr: true,
		},
		{
			name: "missing openapi field",
			schema: map[string]any{
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"paths": map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "missing info field",
			schema: map[string]any{
				"openapi": "3.1.0",
				"paths":   map[string]any{},
			},
			wantErr: true,
		},
		{
			name: "missing paths field",
			schema: map[string]any{
				"openapi": "3.1.0",
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := p.Validate(tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProvider_HashAndSerialize(t *testing.T) {
	p := NewProvider("", "")

	schema := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
	}

	// Test Hash
	hash, err := p.Hash(schema)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Test Serialize
	data, err := p.Serialize(schema)
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty serialized data")
	}
}

func TestProvider_GenerateDescriptor(t *testing.T) {
	p := NewProvider("3.1.0", "/openapi.json")
	ctx := context.Background()

	app := &testApp{
		name:    "test-service",
		version: "v1.0.0",
		routes:  []string{"route1"},
	}

	tests := []struct {
		name           string
		locationType   farp.LocationType
		locationConfig map[string]string
		wantErr        bool
	}{
		{
			name:           "inline location",
			locationType:   farp.LocationTypeInline,
			locationConfig: map[string]string{},
			wantErr:        false,
		},
		{
			name:         "HTTP location",
			locationType: farp.LocationTypeHTTP,
			locationConfig: map[string]string{
				"url": "http://test.com/openapi.json",
			},
			wantErr: false,
		},
		{
			name:           "HTTP location without URL",
			locationType:   farp.LocationTypeHTTP,
			locationConfig: map[string]string{},
			wantErr:        true,
		},
		{
			name:         "registry location",
			locationType: farp.LocationTypeRegistry,
			locationConfig: map[string]string{
				"registry_path": "/schemas/test/v1/openapi",
			},
			wantErr: false,
		},
		{
			name:           "registry location without path",
			locationType:   farp.LocationTypeRegistry,
			locationConfig: map[string]string{},
			wantErr:        true,
		},
		{
			name:         "HTTP with headers",
			locationType: farp.LocationTypeHTTP,
			locationConfig: map[string]string{
				"url":     "http://test.com/openapi.json",
				"headers": `{"Authorization":"Bearer token"}`,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			descriptor, err := p.GenerateDescriptor(ctx, app, tt.locationType, tt.locationConfig)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateDescriptor() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if err != nil {
				return
			}

			if descriptor.Type != farp.SchemaTypeOpenAPI {
				t.Errorf("expected type OpenAPI, got %v", descriptor.Type)
			}

			if descriptor.SpecVersion != "3.1.0" {
				t.Errorf("expected spec version '3.1.0', got '%s'", descriptor.SpecVersion)
			}

			if descriptor.ContentType != "application/json" {
				t.Errorf("expected content type 'application/json', got '%s'", descriptor.ContentType)
			}

			if descriptor.Hash == "" {
				t.Error("expected non-empty hash")
			}

			if descriptor.Size == 0 {
				t.Error("expected non-zero size")
			}

			// Check inline schema
			if tt.locationType == farp.LocationTypeInline {
				if descriptor.InlineSchema == nil {
					t.Error("expected inline schema for inline location type")
				}
			}

			// Check URL
			if tt.locationType == farp.LocationTypeHTTP && tt.locationConfig["url"] != "" {
				if descriptor.Location.URL != tt.locationConfig["url"] {
					t.Errorf("expected URL '%s', got '%s'", tt.locationConfig["url"], descriptor.Location.URL)
				}
			}

			// Check registry path
			if tt.locationType == farp.LocationTypeRegistry && tt.locationConfig["registry_path"] != "" {
				if descriptor.Location.RegistryPath != tt.locationConfig["registry_path"] {
					t.Errorf(
						"expected registry path '%s', got '%s'",
						tt.locationConfig["registry_path"],
						descriptor.Location.RegistryPath,
					)
				}
			}
		})
	}
}

func TestProvider_GenerateDescriptor_AppError(t *testing.T) {
	p := NewProvider("3.1.0", "/openapi.json")
	ctx := context.Background()

	// App with nil routes should cause Generate to fail
	appNoRoutes := &testApp{
		name:    "test-service",
		version: "v1.0.0",
		routes:  nil,
	}

	_, err := p.GenerateDescriptor(ctx, appNoRoutes, farp.LocationTypeInline, map[string]string{})
	if err == nil {
		t.Error("expected error when app provides nil routes")
	}
}

func TestProvider_Generate_WithRouteDescriptors(t *testing.T) {
	p := NewProvider("3.1.0", "")
	ctx := context.Background()

	app := &testApp{
		name:    "user-service",
		version: "v2.0.0",
		routes: []farp.RouteDescriptor{
			{
				Path:        "/users",
				Methods:     []string{"GET", "POST"},
				OperationID: "users",
				Metadata: map[string]any{
					"summary": "User operations",
					"tags":    []string{"users"},
				},
			},
			{
				Path:        "/users/{id}",
				Methods:     []string{"GET", "PUT", "DELETE"},
				OperationID: "user_by_id",
				Metadata: map[string]any{
					"summary":     "Single user operations",
					"description": "CRUD operations on a single user",
				},
			},
			{
				Path: "/health",
				// No methods specified — should default to GET
			},
		},
	}

	schema, err := p.Generate(ctx, app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	schemaMap, ok := schema.(map[string]any)
	if !ok {
		t.Fatal("schema should be map[string]any")
	}

	// Verify top-level structure
	if schemaMap["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", schemaMap["openapi"])
	}

	paths, ok := schemaMap["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths should be map[string]any")
	}

	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}

	// Verify /users path
	usersPath, ok := paths["/users"].(map[string]any)
	if !ok {
		t.Fatal("/users path item should be map[string]any")
	}

	getOp, ok := usersPath["get"].(map[string]any)
	if !ok {
		t.Fatal("/users should have a GET operation")
	}

	if getOp["operationId"] != "get_users" {
		t.Errorf("GET /users operationId = %v, want get_users", getOp["operationId"])
	}

	if getOp["summary"] != "User operations" {
		t.Errorf("GET /users summary = %v, want 'User operations'", getOp["summary"])
	}

	if _, ok := usersPath["post"]; !ok {
		t.Error("/users should have a POST operation")
	}

	// Verify /users/{id} path
	userByIDPath, ok := paths["/users/{id}"].(map[string]any)
	if !ok {
		t.Fatal("/users/{id} path item should be map[string]any")
	}

	for _, method := range []string{"get", "put", "delete"} {
		op, ok := userByIDPath[method].(map[string]any)
		if !ok {
			t.Errorf("/users/{id} should have a %s operation", method)
			continue
		}

		if op["description"] != "CRUD operations on a single user" {
			t.Errorf("%s /users/{id} description = %v, want 'CRUD operations on a single user'", method, op["description"])
		}
	}

	// Verify /health defaults to GET when no methods specified
	healthPath, ok := paths["/health"].(map[string]any)
	if !ok {
		t.Fatal("/health path item should be map[string]any")
	}

	if _, ok := healthPath["get"]; !ok {
		t.Error("/health should default to GET when no methods specified")
	}

	// Validate the schema passes provider validation
	if err := p.Validate(schema); err != nil {
		t.Errorf("generated schema should pass validation: %v", err)
	}
}

func TestProvider_Generate_WithRouteDescriptors_MethodMerging(t *testing.T) {
	p := NewProvider("3.1.0", "")
	ctx := context.Background()

	// Two descriptors for the same path with different methods
	app := &testApp{
		name:    "test-service",
		version: "v1.0.0",
		routes: []farp.RouteDescriptor{
			{
				Path:        "/items",
				Methods:     []string{"GET"},
				OperationID: "list_items",
			},
			{
				Path:        "/items",
				Methods:     []string{"POST"},
				OperationID: "create_item",
			},
		},
	}

	schema, err := p.Generate(ctx, app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	schemaMap := schema.(map[string]any)
	paths := schemaMap["paths"].(map[string]any)

	if len(paths) != 1 {
		t.Fatalf("expected 1 merged path, got %d", len(paths))
	}

	itemsPath, ok := paths["/items"].(map[string]any)
	if !ok {
		t.Fatal("/items path item should be map[string]any")
	}

	if _, ok := itemsPath["get"]; !ok {
		t.Error("/items should have GET after merge")
	}

	if _, ok := itemsPath["post"]; !ok {
		t.Error("/items should have POST after merge")
	}
}

// testAppWithOpenAPISchema implements both Application and OpenAPISchemaProvider.
type testAppWithOpenAPISchema struct {
	testApp
	baseSchema map[string]any
}

func (a *testAppWithOpenAPISchema) OpenAPISchema() map[string]any {
	return a.baseSchema
}

func TestProvider_Generate_WithOpenAPISchemaProvider(t *testing.T) {
	p := NewProvider("3.1.0", "")
	ctx := context.Background()

	app := &testAppWithOpenAPISchema{
		testApp: testApp{
			name:    "user-service",
			version: "v1.0.0",
			routes: []farp.RouteDescriptor{
				{
					Path:    "/users",
					Methods: []string{"GET"},
				},
			},
		},
		baseSchema: map[string]any{
			"openapi": "3.1.0",
			"info": map[string]any{
				"title":       "User Service API",
				"description": "Manages users",
			},
			"servers": []any{
				map[string]any{"url": "https://api.example.com"},
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
			"paths": map[string]any{
				"/admin": map[string]any{
					"get": map[string]any{
						"summary": "Admin endpoint from base",
					},
				},
			},
		},
	}

	schema, err := p.Generate(ctx, app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	schemaMap := schema.(map[string]any)

	// Base info should take precedence (description preserved)
	info := schemaMap["info"].(map[string]any)
	if info["description"] != "Manages users" {
		t.Errorf("base schema info.description should be preserved, got %v", info["description"])
	}

	// Title from base takes precedence
	if info["title"] != "User Service API" {
		t.Errorf("base schema info.title should take precedence, got %v", info["title"])
	}

	// Servers from base should be present
	servers, ok := schemaMap["servers"].([]any)
	if !ok || len(servers) == 0 {
		t.Error("servers from base schema should be preserved")
	}

	// Components from base should be present
	components, ok := schemaMap["components"].(map[string]any)
	if !ok {
		t.Fatal("components from base schema should be preserved")
	}

	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("components.schemas should be preserved")
	}

	if _, ok := schemas["User"]; !ok {
		t.Error("User schema component should be preserved from base")
	}

	// Both base paths and generated paths should be present
	paths := schemaMap["paths"].(map[string]any)
	if _, ok := paths["/users"]; !ok {
		t.Error("generated /users path should be present")
	}

	if _, ok := paths["/admin"]; !ok {
		t.Error("base /admin path should be present")
	}

	// Validate the merged schema
	if err := p.Validate(schema); err != nil {
		t.Errorf("merged schema should pass validation: %v", err)
	}
}

func TestProvider_Generate_WithMapRoutes(t *testing.T) {
	p := NewProvider("3.1.0", "")
	ctx := context.Background()

	// Routes provided as a direct OpenAPI paths map
	app := &testApp{
		name:    "test-service",
		version: "v1.0.0",
		routes: map[string]any{
			"/orders": map[string]any{
				"get": map[string]any{
					"summary":     "List orders",
					"operationId": "listOrders",
				},
				"post": map[string]any{
					"summary":     "Create order",
					"operationId": "createOrder",
				},
			},
		},
	}

	schema, err := p.Generate(ctx, app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	schemaMap := schema.(map[string]any)
	paths := schemaMap["paths"].(map[string]any)

	ordersPath, ok := paths["/orders"].(map[string]any)
	if !ok {
		t.Fatal("/orders should exist in paths")
	}

	getOp, ok := ordersPath["get"].(map[string]any)
	if !ok {
		t.Fatal("/orders should have GET operation")
	}

	if getOp["operationId"] != "listOrders" {
		t.Errorf("GET /orders operationId = %v, want listOrders", getOp["operationId"])
	}
}
