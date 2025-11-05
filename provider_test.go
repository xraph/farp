package farp

import (
	"context"
	"testing"
)

// Mock application for testing.
type mockApplication struct {
	name    string
	version string
	routes  any
}

func (m *mockApplication) Name() string    { return m.name }
func (m *mockApplication) Version() string { return m.version }
func (m *mockApplication) Routes() any     { return m.routes }

// Mock schema provider for testing.
type mockSchemaProvider struct {
	BaseSchemaProvider

	generateFunc func(ctx context.Context, app Application) (any, error)
}

func (m *mockSchemaProvider) Generate(ctx context.Context, app Application) (any, error) {
	if m.generateFunc != nil {
		return m.generateFunc(ctx, app)
	}

	return map[string]any{"test": "schema"}, nil
}

func TestBaseSchemaProvider_Type(t *testing.T) {
	provider := &BaseSchemaProvider{
		schemaType: SchemaTypeOpenAPI,
	}

	if got := provider.Type(); got != SchemaTypeOpenAPI {
		t.Errorf("Type() = %v, want %v", got, SchemaTypeOpenAPI)
	}
}

func TestBaseSchemaProvider_SpecVersion(t *testing.T) {
	provider := &BaseSchemaProvider{
		specVersion: "3.1.0",
	}

	if got := provider.SpecVersion(); got != "3.1.0" {
		t.Errorf("SpecVersion() = %v, want %v", got, "3.1.0")
	}
}

func TestBaseSchemaProvider_ContentType(t *testing.T) {
	provider := &BaseSchemaProvider{
		contentType: "application/json",
	}

	if got := provider.ContentType(); got != "application/json" {
		t.Errorf("ContentType() = %v, want %v", got, "application/json")
	}
}

func TestBaseSchemaProvider_Endpoint(t *testing.T) {
	provider := &BaseSchemaProvider{
		endpoint: "/openapi.json",
	}

	if got := provider.Endpoint(); got != "/openapi.json" {
		t.Errorf("Endpoint() = %v, want %v", got, "/openapi.json")
	}
}

func TestBaseSchemaProvider_Hash(t *testing.T) {
	provider := &BaseSchemaProvider{}

	schema := map[string]any{
		"test": "data",
	}

	hash, err := provider.Hash(schema)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("Hash() length = %d, want 64", len(hash))
	}

	// Same schema should produce same hash
	hash2, err := provider.Hash(schema)
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if hash != hash2 {
		t.Error("Same schema produced different hashes")
	}
}

func TestBaseSchemaProvider_Serialize(t *testing.T) {
	provider := &BaseSchemaProvider{}

	schema := map[string]any{
		"test": "data",
	}

	data, err := provider.Serialize(schema)
	if err != nil {
		t.Fatalf("Serialize() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Serialize() returned empty data")
	}

	// Should be valid JSON
	if data[0] != '{' {
		t.Error("Serialize() did not return JSON")
	}
}

func TestBaseSchemaProvider_Validate(t *testing.T) {
	// Test with no validation function
	provider := &BaseSchemaProvider{}
	schema := map[string]any{"test": "data"}

	err := provider.Validate(schema)
	if err != nil {
		t.Errorf("Validate() with no validateFunc should return nil, got %v", err)
	}

	// Test with validation function
	validateCalled := false
	provider.validateFunc = func(any) error {
		validateCalled = true

		return nil
	}

	err = provider.Validate(schema)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	if !validateCalled {
		t.Error("validateFunc was not called")
	}
}

func TestProviderRegistry_Register(t *testing.T) {
	registry := NewProviderRegistry()

	provider := &mockSchemaProvider{
		BaseSchemaProvider: BaseSchemaProvider{
			schemaType: SchemaTypeOpenAPI,
		},
	}

	registry.Register(provider)

	if !registry.Has(SchemaTypeOpenAPI) {
		t.Error("Registry should have OpenAPI provider after registration")
	}
}

func TestProviderRegistry_Get(t *testing.T) {
	registry := NewProviderRegistry()

	provider := &mockSchemaProvider{
		BaseSchemaProvider: BaseSchemaProvider{
			schemaType: SchemaTypeOpenAPI,
		},
	}

	registry.Register(provider)

	// Test found
	got, ok := registry.Get(SchemaTypeOpenAPI)
	if !ok {
		t.Error("Get() should find registered provider")
	}

	if got.Type() != SchemaTypeOpenAPI {
		t.Errorf("Get() returned wrong provider type: %v", got.Type())
	}

	// Test not found
	_, ok = registry.Get(SchemaTypeAsyncAPI)
	if ok {
		t.Error("Get() should not find unregistered provider")
	}
}

func TestProviderRegistry_Has(t *testing.T) {
	registry := NewProviderRegistry()

	provider := &mockSchemaProvider{
		BaseSchemaProvider: BaseSchemaProvider{
			schemaType: SchemaTypeOpenAPI,
		},
	}

	if registry.Has(SchemaTypeOpenAPI) {
		t.Error("Has() should return false before registration")
	}

	registry.Register(provider)

	if !registry.Has(SchemaTypeOpenAPI) {
		t.Error("Has() should return true after registration")
	}

	if registry.Has(SchemaTypeAsyncAPI) {
		t.Error("Has() should return false for unregistered provider")
	}
}

func TestProviderRegistry_List(t *testing.T) {
	registry := NewProviderRegistry()

	// Empty registry
	list := registry.List()
	if len(list) != 0 {
		t.Errorf("List() should return empty slice for empty registry, got %d items", len(list))
	}

	// Add providers
	provider1 := &mockSchemaProvider{
		BaseSchemaProvider: BaseSchemaProvider{schemaType: SchemaTypeOpenAPI},
	}
	provider2 := &mockSchemaProvider{
		BaseSchemaProvider: BaseSchemaProvider{schemaType: SchemaTypeAsyncAPI},
	}

	registry.Register(provider1)
	registry.Register(provider2)

	list = registry.List()
	if len(list) != 2 {
		t.Errorf("List() should return 2 items, got %d", len(list))
	}

	// Check that both types are in the list
	hasOpenAPI := false
	hasAsyncAPI := false

	for _, schemaType := range list {
		if schemaType == SchemaTypeOpenAPI {
			hasOpenAPI = true
		}

		if schemaType == SchemaTypeAsyncAPI {
			hasAsyncAPI = true
		}
	}

	if !hasOpenAPI {
		t.Error("List() should include OpenAPI")
	}

	if !hasAsyncAPI {
		t.Error("List() should include AsyncAPI")
	}
}

func TestGlobalProviderRegistry(t *testing.T) {
	// Note: This test modifies global state, so we should be careful
	// Save initial state
	initialProviders := ListProviders()

	provider := &mockSchemaProvider{
		BaseSchemaProvider: BaseSchemaProvider{
			schemaType: SchemaTypeCustom, // Use custom to avoid conflicts
		},
	}

	// Test RegisterProvider
	RegisterProvider(provider)

	// Test HasProvider
	if !HasProvider(SchemaTypeCustom) {
		t.Error("HasProvider() should return true after RegisterProvider()")
	}

	// Test GetProvider
	got, ok := GetProvider(SchemaTypeCustom)
	if !ok {
		t.Error("GetProvider() should find registered provider")
	}

	if got.Type() != SchemaTypeCustom {
		t.Errorf("GetProvider() returned wrong type: %v", got.Type())
	}

	// Test ListProviders
	providers := ListProviders()
	if len(providers) <= len(initialProviders) {
		t.Error("ListProviders() should include newly registered provider")
	}
}
