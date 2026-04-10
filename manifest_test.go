package farp

import (
	"testing"
)

func TestNewManifest(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")

	if manifest.ServiceName != "test-service" {
		t.Errorf("expected service name 'test-service', got '%s'", manifest.ServiceName)
	}

	if manifest.ServiceVersion != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", manifest.ServiceVersion)
	}

	if manifest.InstanceID != "instance-123" {
		t.Errorf("expected instance ID 'instance-123', got '%s'", manifest.InstanceID)
	}

	if manifest.Version != ProtocolVersion {
		t.Errorf("expected protocol version '%s', got '%s'", ProtocolVersion, manifest.Version)
	}

	if len(manifest.Schemas) != 0 {
		t.Errorf("expected empty schemas, got %d", len(manifest.Schemas))
	}

	if manifest.UpdatedAt == 0 {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestManifest_AddSchema(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")

	schema := SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: SchemaLocation{
			Type: LocationTypeHTTP,
			URL:  "http://test.com/openapi.json",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	}

	manifest.AddSchema(schema)

	if len(manifest.Schemas) != 1 {
		t.Errorf("expected 1 schema, got %d", len(manifest.Schemas))
	}

	if manifest.Schemas[0].Type != SchemaTypeOpenAPI {
		t.Errorf("expected type OpenAPI, got %s", manifest.Schemas[0].Type)
	}
}

func TestManifest_AddCapability(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")

	manifest.AddCapability("rest")
	manifest.AddCapability("grpc")
	manifest.AddCapability("rest") // Duplicate

	if len(manifest.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(manifest.Capabilities))
	}

	if !manifest.HasCapability("rest") {
		t.Error("expected to have 'rest' capability")
	}

	if !manifest.HasCapability("grpc") {
		t.Error("expected to have 'grpc' capability")
	}

	if manifest.HasCapability("websocket") {
		t.Error("expected not to have 'websocket' capability")
	}
}

func TestManifest_UpdateChecksum(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")

	schema := SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: SchemaLocation{
			Type: LocationTypeHTTP,
			URL:  "http://test.com/openapi.json",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	}

	manifest.AddSchema(schema)

	err := manifest.UpdateChecksum()
	if err != nil {
		t.Fatalf("UpdateChecksum failed: %v", err)
	}

	if manifest.Checksum == "" {
		t.Error("expected checksum to be set")
	}

	if len(manifest.Checksum) != 64 {
		t.Errorf("expected checksum length 64, got %d", len(manifest.Checksum))
	}
}

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *SchemaManifest
		wantErr bool
	}{
		{
			name: "valid manifest",
			setup: func() *SchemaManifest {
				m := NewManifest("test-service", "v1.0.0", "instance-123")
				m.Endpoints.Health = "/health"
				m.AddSchema(SchemaDescriptor{
					Type:        SchemaTypeOpenAPI,
					SpecVersion: "3.1.0",
					Location: SchemaLocation{
						Type: LocationTypeHTTP,
						URL:  "http://test.com/openapi.json",
					},
					ContentType: "application/json",
					Hash:        "1234567890123456789012345678901234567890123456789012345678901234",
					Size:        1024,
				})
				m.UpdateChecksum()

				return m
			},
			wantErr: false,
		},
		{
			name: "missing service name",
			setup: func() *SchemaManifest {
				m := NewManifest("", "v1.0.0", "instance-123")
				m.Endpoints.Health = "/health"

				return m
			},
			wantErr: true,
		},
		{
			name: "missing instance ID",
			setup: func() *SchemaManifest {
				m := NewManifest("test-service", "v1.0.0", "")
				m.Endpoints.Health = "/health"

				return m
			},
			wantErr: true,
		},
		{
			name: "missing health endpoint",
			setup: func() *SchemaManifest {
				m := NewManifest("test-service", "v1.0.0", "instance-123")

				return m
			},
			wantErr: true,
		},
		{
			name: "invalid schema type",
			setup: func() *SchemaManifest {
				m := NewManifest("test-service", "v1.0.0", "instance-123")
				m.Endpoints.Health = "/health"
				m.AddSchema(SchemaDescriptor{
					Type:        SchemaType("invalid"),
					SpecVersion: "1.0.0",
					Location: SchemaLocation{
						Type: LocationTypeHTTP,
						URL:  "http://test.com/schema",
					},
					ContentType: "application/json",
					Hash:        "1234567890123456789012345678901234567890123456789012345678901234",
					Size:        1024,
				})

				return m
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := tt.setup()
			err := manifest.Validate()

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestManifest_GetSchema(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")

	openAPISchema := SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: SchemaLocation{
			Type: LocationTypeHTTP,
			URL:  "http://test.com/openapi.json",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	}

	manifest.AddSchema(openAPISchema)

	// Test found
	schema, found := manifest.GetSchema(SchemaTypeOpenAPI)
	if !found {
		t.Error("expected to find OpenAPI schema")
	}

	if schema.Type != SchemaTypeOpenAPI {
		t.Errorf("expected OpenAPI type, got %s", schema.Type)
	}

	// Test not found
	_, found = manifest.GetSchema(SchemaTypeAsyncAPI)
	if found {
		t.Error("expected not to find AsyncAPI schema")
	}
}

func TestManifest_Clone(t *testing.T) {
	original := NewManifest("test-service", "v1.0.0", "instance-123")
	original.AddCapability("rest")
	original.AddSchema(SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: SchemaLocation{
			Type: LocationTypeHTTP,
			URL:  "http://test.com/openapi.json",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	})

	original.Endpoints.Health = "/health"

	clone := original.Clone()

	// Verify deep copy
	if clone.ServiceName != original.ServiceName {
		t.Error("clone has different service name")
	}

	if len(clone.Schemas) != len(original.Schemas) {
		t.Error("clone has different number of schemas")
	}

	if len(clone.Capabilities) != len(original.Capabilities) {
		t.Error("clone has different number of capabilities")
	}

	// Modify clone and verify original is unchanged
	clone.AddCapability("grpc")

	if len(original.Capabilities) == len(clone.Capabilities) {
		t.Error("modifying clone affected original")
	}
}

func TestManifest_JSON(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.AddSchema(SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: SchemaLocation{
			Type: LocationTypeHTTP,
			URL:  "http://test.com/openapi.json",
		},
		ContentType: "application/json",
		Hash:        "abc123",
		Size:        1024,
	})

	// Test ToJSON
	data, err := manifest.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	// Test ToPrettyJSON
	prettyData, err := manifest.ToPrettyJSON()
	if err != nil {
		t.Fatalf("ToPrettyJSON failed: %v", err)
	}

	if len(prettyData) == 0 {
		t.Error("expected non-empty pretty JSON")
	}

	// Pretty JSON should be longer (has indentation)
	if len(prettyData) <= len(data) {
		t.Error("pretty JSON should be longer than compact JSON")
	}

	// Test FromJSON
	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	if parsed.ServiceName != manifest.ServiceName {
		t.Error("parsed manifest has different service name")
	}

	if len(parsed.Schemas) != len(manifest.Schemas) {
		t.Error("parsed manifest has different number of schemas")
	}
}

func TestFromJSON_Invalid(t *testing.T) {
	// Test with invalid JSON
	_, err := FromJSON([]byte("invalid json"))
	if err == nil {
		t.Error("FromJSON should return error for invalid JSON")
	}

	// Test with empty data
	_, err = FromJSON([]byte{})
	if err == nil {
		t.Error("FromJSON should return error for empty data")
	}
}

func TestCalculateSchemaChecksum(t *testing.T) {
	schema := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
	}

	checksum1, err := CalculateSchemaChecksum(schema)
	if err != nil {
		t.Fatalf("CalculateSchemaChecksum failed: %v", err)
	}

	if len(checksum1) != 64 {
		t.Errorf("expected checksum length 64, got %d", len(checksum1))
	}

	// Same schema should produce same checksum
	checksum2, err := CalculateSchemaChecksum(schema)
	if err != nil {
		t.Fatalf("CalculateSchemaChecksum failed: %v", err)
	}

	if checksum1 != checksum2 {
		t.Error("same schema produced different checksums")
	}

	// Different schema should produce different checksum
	schema["info"].(map[string]any)["version"] = "2.0.0"

	checksum3, err := CalculateSchemaChecksum(schema)
	if err != nil {
		t.Fatalf("CalculateSchemaChecksum failed: %v", err)
	}

	if checksum1 == checksum3 {
		t.Error("different schemas produced same checksum")
	}
}

func TestDiffManifests(t *testing.T) {
	// Create old manifest
	old := NewManifest("test-service", "v1.0.0", "instance-123")
	old.AddSchema(SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com/openapi.json"},
		ContentType: "application/json",
		Hash:        "hash1",
		Size:        1024,
	})
	old.AddCapability("rest")
	old.Endpoints.Health = "/health"

	// Create newPath manifest
	newPath := old.Clone()
	newPath.AddSchema(SchemaDescriptor{
		Type:        SchemaTypeAsyncAPI,
		SpecVersion: "3.0.0",
		Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com/asyncapi.json"},
		ContentType: "application/json",
		Hash:        "hash2",
		Size:        512,
	})
	newPath.AddCapability("websocket")

	// Change existing schema
	newPath.Schemas[0].Hash = "hash1-updated"

	diff := DiffManifests(old, newPath)

	if !diff.HasChanges() {
		t.Error("expected changes to be detected")
	}

	if len(diff.SchemasAdded) != 1 {
		t.Errorf("expected 1 schema added, got %d", len(diff.SchemasAdded))
	}

	if len(diff.SchemasChanged) != 1 {
		t.Errorf("expected 1 schema changed, got %d", len(diff.SchemasChanged))
	}

	if len(diff.CapabilitiesAdded) != 1 {
		t.Errorf("expected 1 capability added, got %d", len(diff.CapabilitiesAdded))
	}
}

func TestSchemaLocation_Validate(t *testing.T) {
	tests := []struct {
		name     string
		location SchemaLocation
		wantErr  bool
	}{
		{
			name: "valid HTTP location",
			location: SchemaLocation{
				Type: LocationTypeHTTP,
				URL:  "http://test.com/schema.json",
			},
			wantErr: false,
		},
		{
			name: "valid registry location",
			location: SchemaLocation{
				Type:         LocationTypeRegistry,
				RegistryPath: "/schemas/test/v1",
			},
			wantErr: false,
		},
		{
			name: "valid inline location",
			location: SchemaLocation{
				Type: LocationTypeInline,
			},
			wantErr: false,
		},
		{
			name: "HTTP without URL",
			location: SchemaLocation{
				Type: LocationTypeHTTP,
			},
			wantErr: true,
		},
		{
			name: "registry without path",
			location: SchemaLocation{
				Type: LocationTypeRegistry,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			location: SchemaLocation{
				Type: LocationType("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.location.Validate()

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateSchemaDescriptor_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		descriptor SchemaDescriptor
		wantErr    bool
	}{
		{
			name: "invalid hash length",
			descriptor: SchemaDescriptor{
				Type:        SchemaTypeOpenAPI,
				SpecVersion: "3.1.0",
				Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com"},
				ContentType: "application/json",
				Hash:        "tooshort",
				Size:        1024,
			},
			wantErr: true,
		},
		{
			name: "missing content type",
			descriptor: SchemaDescriptor{
				Type:        SchemaTypeOpenAPI,
				SpecVersion: "3.1.0",
				Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com"},
				Hash:        "1234567890123456789012345678901234567890123456789012345678901234",
				Size:        1024,
			},
			wantErr: true,
		},
		{
			name: "inline without schema",
			descriptor: SchemaDescriptor{
				Type:        SchemaTypeOpenAPI,
				SpecVersion: "3.1.0",
				Location:    SchemaLocation{Type: LocationTypeInline},
				ContentType: "application/json",
				Hash:        "1234567890123456789012345678901234567890123456789012345678901234",
				Size:        1024,
			},
			wantErr: true,
		},
		{
			name: "missing spec version",
			descriptor: SchemaDescriptor{
				Type:        SchemaTypeOpenAPI,
				Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com"},
				ContentType: "application/json",
				Hash:        "1234567890123456789012345678901234567890123456789012345678901234",
				Size:        1024,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSchemaDescriptor(&tt.descriptor)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

func TestCalculateManifestChecksum_Empty(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")

	checksum, err := CalculateManifestChecksum(manifest)
	if err != nil {
		t.Fatalf("CalculateManifestChecksum failed: %v", err)
	}

	// Empty manifest should have empty checksum
	if checksum != "" {
		t.Errorf("expected empty checksum for manifest with no schemas, got %s", checksum)
	}
}

func TestCalculateSchemaChecksum_Error(t *testing.T) {
	// Test with value that can't be marshaled to JSON
	invalidSchema := make(chan int) // channels can't be marshaled to JSON

	_, err := CalculateSchemaChecksum(invalidSchema)
	if err == nil {
		t.Error("CalculateSchemaChecksum should return error for invalid schema")
	}
}

func TestDiffManifests_NoChanges(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.AddSchema(SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com"},
		ContentType: "application/json",
		Hash:        "hash1",
		Size:        1024,
	})
	manifest.AddCapability("rest")
	manifest.Endpoints.Health = "/health"

	clone := manifest.Clone()

	diff := DiffManifests(manifest, clone)

	if diff.HasChanges() {
		t.Error("expected no changes between identical manifests")
	}
}

func TestDiffManifests_EndpointChanges(t *testing.T) {
	old := NewManifest("test-service", "v1.0.0", "instance-123")
	old.Endpoints.Health = "/health"

	newPath := old.Clone()
	newPath.Endpoints.Health = "/healthz"

	diff := DiffManifests(old, newPath)

	if !diff.EndpointsChanged {
		t.Error("expected endpoint changes to be detected")
	}

	if !diff.HasChanges() {
		t.Error("HasChanges should return true when endpoints changed")
	}
}

func BenchmarkManifest_UpdateChecksum(b *testing.B) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	for range 10 {
		manifest.AddSchema(SchemaDescriptor{
			Type:        SchemaTypeOpenAPI,
			SpecVersion: "3.1.0",
			Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com"},
			ContentType: "application/json",
			Hash:        "abc123",
			Size:        1024,
		})
	}

	b.ResetTimer()

	for range b.N {
		manifest.UpdateChecksum()
	}
}

func TestCalculateRoutesChecksum(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.Routing.Strategy = MountStrategyService
	manifest.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET", "POST"}, Protocol: "rest"},
		{Path: "/users/{id}", Methods: []string{"GET", "PUT", "DELETE"}, Protocol: "rest"},
	}

	checksum1, err := CalculateRoutesChecksum(manifest)
	if err != nil {
		t.Fatalf("CalculateRoutesChecksum() error = %v", err)
	}

	if checksum1 == "" {
		t.Fatal("expected non-empty checksum")
	}

	if len(checksum1) != 64 {
		t.Errorf("expected 64-char hex string, got %d chars", len(checksum1))
	}

	// Same manifest should produce same checksum
	checksum2, err := CalculateRoutesChecksum(manifest)
	if err != nil {
		t.Fatalf("CalculateRoutesChecksum() error = %v", err)
	}

	if checksum1 != checksum2 {
		t.Error("same manifest should produce same routes checksum")
	}
}

func TestCalculateRoutesChecksum_DifferentRoutes(t *testing.T) {
	manifest1 := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest1.Endpoints.Health = "/health"
	manifest1.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET"}, Protocol: "rest"},
	}

	manifest2 := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest2.Endpoints.Health = "/health"
	manifest2.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET"}, Protocol: "rest"},
		{Path: "/posts", Methods: []string{"GET"}, Protocol: "rest"},
	}

	checksum1, _ := CalculateRoutesChecksum(manifest1)
	checksum2, _ := CalculateRoutesChecksum(manifest2)

	if checksum1 == checksum2 {
		t.Error("different routes should produce different checksums")
	}
}

func TestCalculateRoutesChecksum_MethodOrderIndependent(t *testing.T) {
	manifest1 := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest1.Endpoints.Health = "/health"
	manifest1.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"POST", "GET"}, Protocol: "rest"},
	}

	manifest2 := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest2.Endpoints.Health = "/health"
	manifest2.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET", "POST"}, Protocol: "rest"},
	}

	checksum1, _ := CalculateRoutesChecksum(manifest1)
	checksum2, _ := CalculateRoutesChecksum(manifest2)

	if checksum1 != checksum2 {
		t.Error("method order should not affect checksum")
	}
}

func TestCalculateRoutesChecksum_StrategyChange(t *testing.T) {
	manifest1 := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest1.Endpoints.Health = "/health"
	manifest1.Routing.Strategy = MountStrategyService
	manifest1.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET"}, Protocol: "rest"},
	}

	manifest2 := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest2.Endpoints.Health = "/health"
	manifest2.Routing.Strategy = MountStrategyRoot
	manifest2.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET"}, Protocol: "rest"},
	}

	checksum1, _ := CalculateRoutesChecksum(manifest1)
	checksum2, _ := CalculateRoutesChecksum(manifest2)

	if checksum1 == checksum2 {
		t.Error("different mount strategy should produce different checksum")
	}
}

func TestUpdateRoutesChecksum(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET"}, Protocol: "rest"},
	}

	if manifest.RoutesChecksum != "" {
		t.Error("routes checksum should be empty before update")
	}

	err := manifest.UpdateRoutesChecksum()
	if err != nil {
		t.Fatalf("UpdateRoutesChecksum() error = %v", err)
	}

	if manifest.RoutesChecksum == "" {
		t.Error("routes checksum should be set after update")
	}
}

func TestDiffManifests_RoutingChanges(t *testing.T) {
	old := NewManifest("test-service", "v1.0.0", "instance-123")
	old.Endpoints.Health = "/health"
	old.Routing.Strategy = MountStrategyService

	newM := old.Clone()
	newM.Routing.Strategy = MountStrategyRoot

	diff := DiffManifests(old, newM)

	if !diff.RoutingChanged {
		t.Error("expected routing change to be detected")
	}

	if !diff.HasRouteChanges() {
		t.Error("HasRouteChanges should return true when routing config changed")
	}
}

func TestDiffManifests_RoutesChecksumChange(t *testing.T) {
	old := NewManifest("test-service", "v1.0.0", "instance-123")
	old.Endpoints.Health = "/health"
	old.RoutesChecksum = "abc123"

	newM := old.Clone()
	newM.RoutesChecksum = "def456"

	diff := DiffManifests(old, newM)

	if !diff.RoutesChecksumChanged {
		t.Error("expected routes checksum change to be detected")
	}

	if !diff.HasRouteChanges() {
		t.Error("HasRouteChanges should return true when routes checksum changed")
	}
}

func TestDiffManifests_SchemaDescriptionChangeNoRouteChange(t *testing.T) {
	// Schema content change (different hash) but same route structure.
	// HasRouteChanges should return false because SchemasChanged does not
	// mean routes changed (e.g., only a description or model field updated).
	old := NewManifest("test-service", "v1.0.0", "instance-123")
	old.Endpoints.Health = "/health"
	old.RoutesChecksum = "same-hash"
	old.AddSchema(SchemaDescriptor{
		Type:        SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location:    SchemaLocation{Type: LocationTypeHTTP, URL: "http://test.com"},
		ContentType: "application/json",
		Hash:        "hash1",
		Size:        1024,
	})

	newM := old.Clone()
	newM.Schemas[0].Hash = "hash2" // Schema content changed
	// But routes checksum stays the same

	diff := DiffManifests(old, newM)

	if !diff.HasChanges() {
		t.Error("HasChanges should detect schema content change")
	}

	// RoutesChecksum is unchanged, routing is unchanged, no schemas added/removed
	if diff.HasRouteChanges() {
		t.Error("HasRouteChanges should return false when only schema content changed but routes checksum is same")
	}
}

func TestManifest_Clone_IncludesRouteTable(t *testing.T) {
	manifest := NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/health"
	manifest.RoutesChecksum = "test-hash"
	manifest.RouteTable = []RouteDescriptor{
		{Path: "/users", Methods: []string{"GET"}, Protocol: "rest"},
	}
	manifest.Routing.Strategy = MountStrategyService

	clone := manifest.Clone()

	if clone.RoutesChecksum != manifest.RoutesChecksum {
		t.Error("clone should preserve RoutesChecksum")
	}

	if len(clone.RouteTable) != len(manifest.RouteTable) {
		t.Error("clone should preserve RouteTable")
	}

	if clone.Routing.Strategy != manifest.Routing.Strategy {
		t.Error("clone should preserve Routing config")
	}
}

func BenchmarkCalculateSchemaChecksum(b *testing.B) {
	schema := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Test API",
			"version": "1.0.0",
		},
		"paths": make(map[string]any),
	}

	// Add 100 paths
	paths := schema["paths"].(map[string]any)
	for i := range 100 {
		paths["/path"+string(rune(i))] = map[string]any{
			"get": map[string]any{
				"summary": "Test endpoint",
			},
		}
	}

	b.ResetTimer()

	for range b.N {
		CalculateSchemaChecksum(schema)
	}
}
