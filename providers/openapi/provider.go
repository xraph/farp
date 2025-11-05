package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/xraph/farp"
)

// Provider generates OpenAPI 3.x schemas from applications.
type Provider struct {
	specVersion string
	endpoint    string
}

// NewProvider creates a new OpenAPI schema provider
// specVersion should be "3.0.0", "3.0.1", or "3.1.0" (recommended).
func NewProvider(specVersion string, endpoint string) *Provider {
	if specVersion == "" {
		specVersion = "3.1.0"
	}

	if endpoint == "" {
		endpoint = "/openapi.json"
	}

	return &Provider{
		specVersion: specVersion,
		endpoint:    endpoint,
	}
}

// Type returns the schema type.
func (p *Provider) Type() farp.SchemaType {
	return farp.SchemaTypeOpenAPI
}

// SpecVersion returns the OpenAPI specification version.
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

// OpenAPISchemaProvider is an optional interface that applications can implement
// to provide a base OpenAPI schema that will be merged with the generated schema.
type OpenAPISchemaProvider interface {
	// OpenAPISchema returns a base OpenAPI schema (map[string]any) if the application provides one.
	// The returned schema will be merged with the generated schema.
	OpenAPISchema() map[string]any
}

// Generate generates an OpenAPI schema from the application
// app should provide Routes() method that returns route information.
// If the app implements OpenAPISchemaProvider, the provided schema will be merged with the generated one.
func (p *Provider) Generate(ctx context.Context, app farp.Application) (any, error) {
	routes := app.Routes()
	if routes == nil {
		return nil, errors.New("application does not provide routes")
	}

	// Check if app provides a base schema
	var baseSchema map[string]any
	if schemaProvider, ok := app.(OpenAPISchemaProvider); ok {
		baseSchema = schemaProvider.OpenAPISchema()
	}

	// Build minimal OpenAPI spec
	spec := map[string]any{
		"openapi": p.specVersion,
		"info": map[string]any{
			"title":   app.Name(),
			"version": app.Version(),
		},
		"paths": map[string]any{},
	}

	// Merge with base schema if provided
	if baseSchema != nil {
		spec = p.mergeSchemas(baseSchema, spec)
	}

	return spec, nil
}

// mergeSchemas merges a base schema with a generated schema.
// The base schema takes precedence for top-level fields, but generated fields
// are merged into the base where appropriate (e.g., paths and components are combined).
func (p *Provider) mergeSchemas(base, generated map[string]any) map[string]any {
	// Start with a copy of the base schema
	result := maps.Clone(base)

	// Ensure openapi version is set (prefer generated version if base doesn't have it)
	if result["openapi"] == nil || result["openapi"] == "" {
		if openapiVersion, ok := generated["openapi"].(string); ok && openapiVersion != "" {
			result["openapi"] = openapiVersion
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

	// Merge paths - combine both sets (base paths first, then generated)
	resultPaths := make(map[string]any)
	if basePaths, ok := base["paths"].(map[string]any); ok {
		maps.Copy(resultPaths, basePaths)
	}
	if generatedPaths, ok := generated["paths"].(map[string]any); ok {
		maps.Copy(resultPaths, generatedPaths)
	}
	result["paths"] = resultPaths

	// Merge components - combine both sets
	if baseComponents, ok := base["components"].(map[string]any); ok {
		resultComponents := maps.Clone(baseComponents)
		if generatedComponents, ok := generated["components"].(map[string]any); ok {
			// Merge each component type
			for componentType, componentValue := range generatedComponents {
				if componentMap, ok := componentValue.(map[string]any); ok {
					if existing, ok := resultComponents[componentType].(map[string]any); ok {
						// Merge component maps
						maps.Copy(existing, componentMap)
						resultComponents[componentType] = existing
					} else {
						resultComponents[componentType] = componentMap
					}
				} else {
					resultComponents[componentType] = componentValue
				}
			}
		}
		result["components"] = resultComponents
	} else if generatedComponents, ok := generated["components"].(map[string]any); ok {
		result["components"] = generatedComponents
	}

	// Merge servers - combine both sets
	if baseServers, ok := base["servers"].([]any); ok {
		resultServers := make([]any, len(baseServers))
		copy(resultServers, baseServers)
		if generatedServers, ok := generated["servers"].([]any); ok {
			resultServers = append(resultServers, generatedServers...)
		}
		result["servers"] = resultServers
	} else if generatedServers, ok := generated["servers"].([]any); ok {
		result["servers"] = generatedServers
	}

	// Copy other fields from generated schema if not present in base
	for k, v := range generated {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}

	// Preserve all base schema fields
	for k, v := range base {
		if _, exists := result[k]; !exists {
			result[k] = v
		}
	}

	return result
}

// Validate validates an OpenAPI schema.
func (p *Provider) Validate(schema any) error {
	// Basic validation - check for required fields
	schemaMap, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: schema must be a map", farp.ErrInvalidSchema)
	}

	// Check openapi version
	if _, ok := schemaMap["openapi"]; !ok {
		return fmt.Errorf("%w: missing 'openapi' field", farp.ErrInvalidSchema)
	}

	// Check info
	if _, ok := schemaMap["info"]; !ok {
		return fmt.Errorf("%w: missing 'info' field", farp.ErrInvalidSchema)
	}

	// Check paths
	if _, ok := schemaMap["paths"]; !ok {
		return fmt.Errorf("%w: missing 'paths' field", farp.ErrInvalidSchema)
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
