package merger

import (
	"fmt"

	"github.com/xraph/farp"
)

// Merger handles OpenAPI schema composition
type Merger struct {
	config MergerConfig
}

// MergerConfig configures the merger behavior
type MergerConfig struct {
	// Default conflict strategy if not specified in metadata
	DefaultConflictStrategy farp.ConflictStrategy

	// Title for the merged OpenAPI spec
	MergedTitle string

	// Description for the merged OpenAPI spec
	MergedDescription string

	// Version for the merged OpenAPI spec
	MergedVersion string

	// Whether to include service tags in operations
	IncludeServiceTags bool

	// Whether to sort merged content alphabetically
	SortOutput bool

	// Custom server URLs for the merged spec
	Servers []Server
}

// DefaultMergerConfig returns default merger configuration
func DefaultMergerConfig() MergerConfig {
	return MergerConfig{
		DefaultConflictStrategy: farp.ConflictStrategyPrefix,
		MergedTitle:             "Federated API",
		MergedDescription:       "Merged API specification from multiple services",
		MergedVersion:           "1.0.0",
		IncludeServiceTags:      true,
		SortOutput:              true,
		Servers:                 []Server{},
	}
}

// NewMerger creates a new OpenAPI merger
func NewMerger(config MergerConfig) *Merger {
	return &Merger{
		config: config,
	}
}

// MergeResult contains the merged OpenAPI spec and metadata
type MergeResult struct {
	// The merged OpenAPI specification
	Spec *OpenAPISpec

	// Services that were included in the merge
	IncludedServices []string

	// Services that were excluded (not marked for inclusion)
	ExcludedServices []string

	// Conflicts that were encountered during merge
	Conflicts []Conflict

	// Warnings (non-fatal issues)
	Warnings []string
}

// Conflict represents a conflict encountered during merging
type Conflict struct {
	// Type of conflict (path, component, tag, etc.)
	Type ConflictType

	// Path or name that conflicted
	Item string

	// Services involved in the conflict
	Services []string

	// How the conflict was resolved
	Resolution string

	// Conflict strategy that was applied
	Strategy farp.ConflictStrategy
}

// ConflictType represents the type of conflict
type ConflictType string

const (
	// ConflictTypePath indicates path conflict
	ConflictTypePath ConflictType = "path"

	// ConflictTypeComponent indicates component name conflict
	ConflictTypeComponent ConflictType = "component"

	// ConflictTypeTag indicates tag conflict
	ConflictTypeTag ConflictType = "tag"

	// ConflictTypeOperationID indicates operation ID conflict
	ConflictTypeOperationID ConflictType = "operationId"

	// ConflictTypeSecurityScheme indicates security scheme conflict
	ConflictTypeSecurityScheme ConflictType = "securityScheme"
)

// Merge merges multiple OpenAPI schemas from service manifests
func (m *Merger) Merge(schemas []ServiceSchema) (*MergeResult, error) {
	result := &MergeResult{
		Spec: &OpenAPISpec{
			OpenAPI: "3.1.0", // Use latest version
			Info: Info{
				Title:       m.config.MergedTitle,
				Description: m.config.MergedDescription,
				Version:     m.config.MergedVersion,
			},
			Servers: m.config.Servers,
			Paths:   make(map[string]PathItem),
			Components: &Components{
				Schemas:         make(map[string]map[string]interface{}),
				Responses:       make(map[string]Response),
				Parameters:      make(map[string]Parameter),
				RequestBodies:   make(map[string]RequestBody),
				SecuritySchemes: make(map[string]SecurityScheme),
			},
			Tags:       []Tag{},
			Extensions: make(map[string]interface{}),
		},
		IncludedServices: []string{},
		ExcludedServices: []string{},
		Conflicts:        []Conflict{},
		Warnings:         []string{},
	}

	// Track what we've seen for conflict detection
	seenPaths := make(map[string]string)           // path -> service
	seenComponents := make(map[string]string)      // component -> service
	seenOperationIDs := make(map[string]string)    // operationID -> service
	seenTags := make(map[string]Tag)               // tag name -> tag
	seenSecuritySchemes := make(map[string]string) // security scheme -> service

	// Process each schema
	for _, schema := range schemas {
		serviceName := schema.Manifest.ServiceName

		// Check if this schema should be included
		if !shouldIncludeInMerge(schema) {
			result.ExcludedServices = append(result.ExcludedServices, serviceName)
			continue
		}

		result.IncludedServices = append(result.IncludedServices, serviceName)

		// Parse the schema if not already parsed
		if schema.Parsed == nil {
			parsed, err := ParseOpenAPISchema(schema.Schema)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Failed to parse schema for %s: %v", serviceName, err))
				continue
			}
			schema.Parsed = parsed
		}

		// Get composition config
		compConfig := getCompositionConfig(schema.Manifest)
		strategy := m.getConflictStrategy(compConfig)

		// Determine prefixes
		componentPrefix := getComponentPrefix(schema.Manifest, compConfig)
		tagPrefix := getTagPrefix(schema.Manifest, compConfig)
		operationIDPrefix := getOperationIDPrefix(schema.Manifest, compConfig)

		// Merge paths
		paths := ApplyRouting(schema.Parsed.Paths, schema.Manifest)
		for path, pathItem := range paths {
			// Check for path conflicts
			if existingService, exists := seenPaths[path]; exists {
				conflict := Conflict{
					Type:     ConflictTypePath,
					Item:     path,
					Services: []string{existingService, serviceName},
					Strategy: strategy,
				}

				switch strategy {
				case farp.ConflictStrategyError:
					return nil, fmt.Errorf("path conflict: %s exists in both %s and %s",
						path, existingService, serviceName)

				case farp.ConflictStrategySkip:
					conflict.Resolution = fmt.Sprintf("Skipped path from %s", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)
					continue

				case farp.ConflictStrategyOverwrite:
					conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)
					// Continue to overwrite

				case farp.ConflictStrategyPrefix:
					// Add service prefix to path
					newPath := fmt.Sprintf("/%s%s", serviceName, path)
					conflict.Resolution = fmt.Sprintf("Prefixed to %s", newPath)
					result.Conflicts = append(result.Conflicts, conflict)
					path = newPath

				case farp.ConflictStrategyMerge:
					// Attempt to merge operations
					pathItem = mergePathItems(result.Spec.Paths[path], pathItem)
					conflict.Resolution = "Merged operations"
					result.Conflicts = append(result.Conflicts, conflict)
				}
			}

			// Apply prefixes to operation IDs and tags
			pathItem = applyOperationPrefixes(pathItem, operationIDPrefix, tagPrefix, serviceName,
				seenOperationIDs, result)

			result.Spec.Paths[path] = pathItem
			seenPaths[path] = serviceName
		}

		// Merge components
		if schema.Parsed.Components != nil {
			prefixedComponents := PrefixComponentNames(schema.Parsed.Components, componentPrefix)

			for name, schemaObj := range prefixedComponents.Schemas {
				if existingService, exists := seenComponents[name]; exists {
					conflict := Conflict{
						Type:     ConflictTypeComponent,
						Item:     name,
						Services: []string{existingService, serviceName},
						Strategy: strategy,
					}

					if strategy == farp.ConflictStrategySkip {
						conflict.Resolution = fmt.Sprintf("Skipped component from %s", serviceName)
						result.Conflicts = append(result.Conflicts, conflict)
						continue
					}

					conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)
				}

				result.Spec.Components.Schemas[name] = schemaObj
				seenComponents[name] = serviceName
			}

			// Merge other component types
			for name, response := range prefixedComponents.Responses {
				result.Spec.Components.Responses[name] = response
			}
			for name, param := range prefixedComponents.Parameters {
				result.Spec.Components.Parameters[name] = param
			}
			for name, body := range prefixedComponents.RequestBodies {
				result.Spec.Components.RequestBodies[name] = body
			}

			// Merge security schemes (with conflict detection)
			for name, scheme := range prefixedComponents.SecuritySchemes {
				if existingService, exists := seenSecuritySchemes[name]; exists {
					conflict := Conflict{
						Type:     ConflictTypeSecurityScheme,
						Item:     name,
						Services: []string{existingService, serviceName},
						Strategy: strategy,
					}

					switch strategy {
					case farp.ConflictStrategyError:
						return nil, fmt.Errorf("security scheme conflict: %s exists in both %s and %s",
							name, existingService, serviceName)

					case farp.ConflictStrategySkip:
						conflict.Resolution = fmt.Sprintf("Skipped security scheme from %s", serviceName)
						result.Conflicts = append(result.Conflicts, conflict)
						continue

					case farp.ConflictStrategyOverwrite:
						conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
						result.Conflicts = append(result.Conflicts, conflict)

					case farp.ConflictStrategyPrefix:
						// Prefix the security scheme name
						prefixedName := componentPrefix + "_" + name
						conflict.Resolution = fmt.Sprintf("Prefixed to %s", prefixedName)
						result.Conflicts = append(result.Conflicts, conflict)
						result.Spec.Components.SecuritySchemes[prefixedName] = scheme
						seenSecuritySchemes[prefixedName] = serviceName
						continue

					case farp.ConflictStrategyMerge:
						// For security schemes, merge is same as overwrite with warning
						conflict.Resolution = fmt.Sprintf("Merged (overwritten) with %s version", serviceName)
						result.Conflicts = append(result.Conflicts, conflict)
					}
				}

				result.Spec.Components.SecuritySchemes[name] = scheme
				seenSecuritySchemes[name] = serviceName
			}
		}

		// Merge tags
		for _, tag := range schema.Parsed.Tags {
			if tagPrefix != "" && m.config.IncludeServiceTags {
				tag.Name = tagPrefix + "_" + tag.Name
			}

			if existing, exists := seenTags[tag.Name]; exists {
				// Merge descriptions
				if tag.Description != "" && existing.Description == "" {
					existing.Description = tag.Description
					seenTags[tag.Name] = existing
				}
			} else {
				seenTags[tag.Name] = tag
				result.Spec.Tags = append(result.Spec.Tags, tag)
			}
		}
	}

	// Sort output if requested
	if m.config.SortOutput {
		result.Spec.Tags = SortTags(result.Spec.Tags)
	}

	return result, nil
}

// Helper functions

func shouldIncludeInMerge(schema ServiceSchema) bool {
	// Check OpenAPI metadata for composition config
	for _, schemaDesc := range schema.Manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeOpenAPI &&
			schemaDesc.Metadata != nil &&
			schemaDesc.Metadata.OpenAPI != nil &&
			schemaDesc.Metadata.OpenAPI.Composition != nil {
			return schemaDesc.Metadata.OpenAPI.Composition.IncludeInMerged
		}
	}

	// Default: include if OpenAPI schema is present
	for _, schemaDesc := range schema.Manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeOpenAPI {
			return true
		}
	}

	return false
}

func getCompositionConfig(manifest *farp.SchemaManifest) *farp.CompositionConfig {
	for _, schemaDesc := range manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeOpenAPI &&
			schemaDesc.Metadata != nil &&
			schemaDesc.Metadata.OpenAPI != nil {
			return schemaDesc.Metadata.OpenAPI.Composition
		}
	}
	return nil
}

func (m *Merger) getConflictStrategy(config *farp.CompositionConfig) farp.ConflictStrategy {
	if config != nil && config.ConflictStrategy != "" {
		return config.ConflictStrategy
	}
	return m.config.DefaultConflictStrategy
}

func getComponentPrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}
	return manifest.ServiceName
}

func getTagPrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.TagPrefix != "" {
		return config.TagPrefix
	}
	return manifest.ServiceName
}

func getOperationIDPrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.OperationIDPrefix != "" {
		return config.OperationIDPrefix
	}
	return manifest.ServiceName
}

func applyOperationPrefixes(item PathItem, opIDPrefix, tagPrefix, serviceName string,
	seenOperationIDs map[string]string, result *MergeResult,
) PathItem {
	applyToOp := func(op *Operation) {
		if op == nil {
			return
		}

		// Prefix operation ID
		if op.OperationID != "" {
			originalID := op.OperationID
			if opIDPrefix != "" {
				op.OperationID = opIDPrefix + "_" + op.OperationID
			}

			// Check for conflicts
			if existingService, exists := seenOperationIDs[op.OperationID]; exists {
				result.Conflicts = append(result.Conflicts, Conflict{
					Type:       ConflictTypeOperationID,
					Item:       originalID,
					Services:   []string{existingService, serviceName},
					Resolution: fmt.Sprintf("Prefixed to %s", op.OperationID),
				})
			}
			seenOperationIDs[op.OperationID] = serviceName
		}

		// Prefix tags
		if tagPrefix != "" {
			op.Tags = PrefixTags(op.Tags, tagPrefix)
		}
	}

	applyToOp(item.Get)
	applyToOp(item.Post)
	applyToOp(item.Put)
	applyToOp(item.Delete)
	applyToOp(item.Patch)
	applyToOp(item.Options)
	applyToOp(item.Head)
	applyToOp(item.Trace)

	return item
}

func mergePathItems(existing, new PathItem) PathItem {
	// Merge operations - prefer non-nil operations
	if new.Get != nil {
		existing.Get = new.Get
	}
	if new.Post != nil {
		existing.Post = new.Post
	}
	if new.Put != nil {
		existing.Put = new.Put
	}
	if new.Delete != nil {
		existing.Delete = new.Delete
	}
	if new.Patch != nil {
		existing.Patch = new.Patch
	}
	if new.Options != nil {
		existing.Options = new.Options
	}
	if new.Head != nil {
		existing.Head = new.Head
	}
	if new.Trace != nil {
		existing.Trace = new.Trace
	}

	return existing
}
