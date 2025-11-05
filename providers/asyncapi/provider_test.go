package asyncapi

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
	if p.specVersion != "3.0.0" {
		t.Errorf("expected default spec version '3.0.0', got '%s'", p.specVersion)
	}

	if p.endpoint != "/asyncapi.json" {
		t.Errorf("expected default endpoint '/asyncapi.json', got '%s'", p.endpoint)
	}

	// Test with custom values
	p = NewProvider("2.6.0", "/custom/asyncapi.json")
	if p.specVersion != "2.6.0" {
		t.Errorf("expected spec version '2.6.0', got '%s'", p.specVersion)
	}

	if p.endpoint != "/custom/asyncapi.json" {
		t.Errorf("expected endpoint '/custom/asyncapi.json', got '%s'", p.endpoint)
	}
}

func TestProvider_Type(t *testing.T) {
	p := NewProvider("", "")
	if p.Type() != farp.SchemaTypeAsyncAPI {
		t.Errorf("expected type AsyncAPI, got %v", p.Type())
	}
}

func TestProvider_SpecVersion(t *testing.T) {
	p := NewProvider("2.6.0", "")
	if p.SpecVersion() != "2.6.0" {
		t.Errorf("expected spec version '2.6.0', got '%s'", p.SpecVersion())
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
	p := NewProvider("3.0.0", "")
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

	if schemaMap["asyncapi"] != "3.0.0" {
		t.Errorf("expected asyncapi version '3.0.0', got %v", schemaMap["asyncapi"])
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
	tests := []struct {
		name        string
		specVersion string
		schema      any
		wantErr     bool
	}{
		{
			name:        "valid AsyncAPI 3.0 schema",
			specVersion: "3.0.0",
			schema: map[string]any{
				"asyncapi": "3.0.0",
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"channels":   map[string]any{},
				"operations": map[string]any{},
			},
			wantErr: false,
		},
		{
			name:        "valid AsyncAPI 2.6 schema",
			specVersion: "2.6.0",
			schema: map[string]any{
				"asyncapi": "2.6.0",
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
			},
			wantErr: false,
		},
		{
			name:        "not a map",
			specVersion: "3.0.0",
			schema:      "invalid",
			wantErr:     true,
		},
		{
			name:        "missing asyncapi field",
			specVersion: "3.0.0",
			schema: map[string]any{
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"channels":   map[string]any{},
				"operations": map[string]any{},
			},
			wantErr: true,
		},
		{
			name:        "missing info field",
			specVersion: "3.0.0",
			schema: map[string]any{
				"asyncapi":   "3.0.0",
				"channels":   map[string]any{},
				"operations": map[string]any{},
			},
			wantErr: true,
		},
		{
			name:        "AsyncAPI 3.0 missing channels",
			specVersion: "3.0.0",
			schema: map[string]any{
				"asyncapi": "3.0.0",
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"operations": map[string]any{},
			},
			wantErr: true,
		},
		{
			name:        "AsyncAPI 3.0 missing operations",
			specVersion: "3.0.0",
			schema: map[string]any{
				"asyncapi": "3.0.0",
				"info": map[string]any{
					"title":   "Test API",
					"version": "1.0.0",
				},
				"channels": map[string]any{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider(tt.specVersion, "")

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
		"asyncapi": "3.0.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"channels":   map[string]any{},
		"operations": map[string]any{},
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
	p := NewProvider("3.0.0", "/asyncapi.json")
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
				"url": "http://test.com/asyncapi.json",
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
				"registry_path": "/schemas/test/v1/asyncapi",
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
				"url":     "http://test.com/asyncapi.json",
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

			if descriptor.Type != farp.SchemaTypeAsyncAPI {
				t.Errorf("expected type AsyncAPI, got %v", descriptor.Type)
			}

			if descriptor.SpecVersion != "3.0.0" {
				t.Errorf("expected spec version '3.0.0', got '%s'", descriptor.SpecVersion)
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
	p := NewProvider("3.0.0", "/asyncapi.json")
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
