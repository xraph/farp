package asyncapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/xraph/farp"
)

// Provider generates AsyncAPI 2.x or 3.x schemas from applications.
type Provider struct {
	specVersion string
	endpoint    string
}

// NewProvider creates a new AsyncAPI schema provider
// specVersion should be "2.6.0" or "3.0.0" (recommended).
func NewProvider(specVersion string, endpoint string) *Provider {
	if specVersion == "" {
		specVersion = "3.0.0"
	}

	if endpoint == "" {
		endpoint = "/asyncapi.json"
	}

	return &Provider{
		specVersion: specVersion,
		endpoint:    endpoint,
	}
}

// Type returns the schema type.
func (p *Provider) Type() farp.SchemaType {
	return farp.SchemaTypeAsyncAPI
}

// SpecVersion returns the AsyncAPI specification version.
func (p *Provider) SpecVersion() string {
	return p.specVersion
}

// ContentType returns the content type.
func (p *Provider) ContentType() string {
	return "application/json"
}

// Endpoint returns the HTTP endpoint where the schema is served.
func (p *Provider) Endpoint() string {
	return p.endpoint
}

// AsyncAPISchemaProvider is an optional interface that applications can implement
// to provide a base AsyncAPI schema that will be merged with the generated schema.
type AsyncAPISchemaProvider interface {
	// AsyncAPISchema returns a base AsyncAPI schema (map[string]any) if the application provides one.
	// The returned schema will be merged with the generated schema.
	AsyncAPISchema() map[string]any
}

// Generate generates an AsyncAPI schema from the application
// app should provide Routes() method that returns route information.
// If the app implements AsyncAPISchemaProvider, the provided schema will be merged with the generated one.
func (p *Provider) Generate(ctx context.Context, app farp.Application) (any, error) {
	routes := app.Routes()
	if routes == nil {
		return nil, errors.New("application does not provide routes")
	}

	// Check if app provides a base schema
	var baseSchema map[string]any
	if schemaProvider, ok := app.(AsyncAPISchemaProvider); ok {
		baseSchema = schemaProvider.AsyncAPISchema()
	}

	// Build minimal AsyncAPI spec
	spec := map[string]any{
		"asyncapi": p.specVersion,
		"info": map[string]any{
			"title":   app.Name(),
			"version": app.Version(),
		},
		"channels":   map[string]any{},
		"operations": map[string]any{},
	}
	// Merge with base schema if provided
	if baseSchema != nil {
		spec = p.mergeSchemas(baseSchema, spec)
	}

	return spec, nil
}

// mergeSchemas merges a base schema with a generated schema.
// The base schema takes precedence for top-level fields, but generated fields
// are merged into the base where appropriate (e.g., channels and operations are combined).
func (p *Provider) mergeSchemas(base, generated map[string]any) map[string]any {
	// Start with a copy of the base schema
	result := maps.Clone(base)

	// Ensure asyncapi version is set (prefer generated version if base doesn't have it)
	if result["asyncapi"] == nil || result["asyncapi"] == "" {
		if asyncapiVersion, ok := generated["asyncapi"].(string); ok && asyncapiVersion != "" {
			result["asyncapi"] = asyncapiVersion
		}
	}

	// Merge info - prefer base but fill in missing fields from generated
	if baseInfo, ok := base["info"].(map[string]any); ok {
		resultInfo := maps.Clone(baseInfo)
		if generatedInfo, ok := generated["info"].(map[string]any); ok {
			// Merge info fields, base takes precedence
			for k, v := range generatedInfo {
				if _, exists := resultInfo[k]; !exists {
					resultInfo[k] = v
				}
			}
		}
		result["info"] = resultInfo
	} else if generatedInfo, ok := generated["info"].(map[string]any); ok {
		result["info"] = generatedInfo
	}

	// Merge channels - combine both sets (base channels first, then generated)
	resultChannels := make(map[string]any)
	if baseChannels, ok := base["channels"].(map[string]any); ok {
		maps.Copy(resultChannels, baseChannels)
	}
	if generatedChannels, ok := generated["channels"].(map[string]any); ok {
		maps.Copy(resultChannels, generatedChannels)
	}
	result["channels"] = resultChannels

	// Merge operations - combine both sets (base operations first, then generated)
	resultOperations := make(map[string]any)
	if baseOperations, ok := base["operations"].(map[string]any); ok {
		maps.Copy(resultOperations, baseOperations)
	}
	if generatedOperations, ok := generated["operations"].(map[string]any); ok {
		maps.Copy(resultOperations, generatedOperations)
	}
	result["operations"] = resultOperations

	// Copy other fields from generated schema if not present in base
	for k, v := range generated {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}

	// Preserve all base schema fields (result already has them from Clone, but ensure completeness)
	for k, v := range base {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}

	return result
}

// Validate validates an AsyncAPI schema.
func (p *Provider) Validate(schema any) error {
	// Basic validation - check for required fields
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: schema must be a map", farp.ErrInvalidSchema)
	}

	// Check asyncapi version
	if _, ok := schemaMap["asyncapi"]; !ok {
		return fmt.Errorf("%w: missing 'asyncapi' field", farp.ErrInvalidSchema)
	}

	// Check info
	if _, ok := schemaMap["info"]; !ok {
		return fmt.Errorf("%w: missing 'info' field", farp.ErrInvalidSchema)
	}

	// For AsyncAPI 3.x, check channels and operations
	if p.specVersion >= "3.0.0" {
		if _, ok := schemaMap["channels"]; !ok {
			return fmt.Errorf("%w: missing 'channels' field (AsyncAPI 3.x)", farp.ErrInvalidSchema)
		}

		if _, ok := schemaMap["operations"]; !ok {
			return fmt.Errorf("%w: missing 'operations' field (AsyncAPI 3.x)", farp.ErrInvalidSchema)
		}
	}

	return nil
}

// Hash calculates SHA256 hash of the schema.
func (p *Provider) Hash(schema any) (string, error) {
	return farp.CalculateSchemaChecksum(schema)
}

// Serialize converts schema to JSON bytes.
func (p *Provider) Serialize(schema any) ([]byte, error) {
	return json.Marshal(schema)
}

// GenerateDescriptor generates a complete SchemaDescriptor for this schema.
func (p *Provider) GenerateDescriptor(ctx context.Context, app farp.Application, locationType farp.LocationType, locationConfig map[string]string) (*farp.SchemaDescriptor, error) {
	// Generate schema
	schema, err := p.Generate(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("failed to generate schema: %w", err)
	}

	// Calculate hash
	hash, err := p.Hash(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Calculate size
	data, err := p.Serialize(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize schema: %w", err)
	}

	// Build location
	location := farp.SchemaLocation{
		Type: locationType,
	}

	switch locationType {
	case farp.LocationTypeHTTP:
		url := locationConfig["url"]
		if url == "" {
			return nil, errors.New("url required for HTTP location")
		}

		location.URL = url

		if headers := locationConfig["headers"]; headers != "" {
			// Parse headers from JSON string
			var headersMap map[string]string

			err := json.Unmarshal([]byte(headers), &headersMap)
			if err == nil {
				location.Headers = headersMap
			}
		}

	case farp.LocationTypeRegistry:
		registryPath := locationConfig["registry_path"]
		if registryPath == "" {
			return nil, errors.New("registry_path required for registry location")
		}

		location.RegistryPath = registryPath

	case farp.LocationTypeInline:
		// Schema will be embedded
	}

	descriptor := &farp.SchemaDescriptor{
		Type:        p.Type(),
		SpecVersion: p.SpecVersion(),
		Location:    location,
		ContentType: p.ContentType(),
		Hash:        hash,
		Size:        int64(len(data)),
	}

	// Add inline schema if location type is inline
	if locationType == farp.LocationTypeInline {
		descriptor.InlineSchema = schema
	}

	return descriptor, nil
}
