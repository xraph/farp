package merger

import (
	"errors"
	"fmt"
	"sort"

	"github.com/xraph/farp"
)

// ORPCSpec represents a simplified oRPC specification.
type ORPCSpec struct {
	ORPC            string                        `json:"orpc"`
	Info            Info                          `json:"info"`
	Servers         []Server                      `json:"servers,omitempty"`
	Procedures      map[string]ORPCProcedure      `json:"procedures"`
	Schemas         map[string]any                `json:"schemas,omitempty"`
	SecuritySchemes map[string]ORPCSecurityScheme `json:"securitySchemes,omitempty"`
	Security        []map[string][]string         `json:"security,omitempty"`
	Extensions      map[string]any                `json:"-"`
}

// ORPCSecurityScheme represents an oRPC security scheme.
type ORPCSecurityScheme struct {
	Type             string         `json:"type"` // apiKey, http, oauth2, openIdConnect
	Description      string         `json:"description,omitempty"`
	Name             string         `json:"name,omitempty"`             // For apiKey
	In               string         `json:"in,omitempty"`               // For apiKey: header, query, cookie
	Scheme           string         `json:"scheme,omitempty"`           // For http
	BearerFormat     string         `json:"bearerFormat,omitempty"`     // For http bearer
	OpenIdConnectURL string         `json:"openIdConnectUrl,omitempty"` // For openIdConnect
	Flows            map[string]any `json:"flows,omitempty"`            // For oauth2
}

// ORPCProcedure represents an oRPC procedure definition.
type ORPCProcedure struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Input       *ORPCSchema    `json:"input,omitempty"`
	Output      *ORPCSchema    `json:"output,omitempty"`
	Errors      []ORPCError    `json:"errors,omitempty"`
	Streaming   bool           `json:"streaming,omitempty"`
	Batch       bool           `json:"batch,omitempty"`
	Options     map[string]any `json:"options,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// ORPCSchema represents a schema reference or inline schema.
type ORPCSchema struct {
	Ref    string         `json:"$ref,omitempty"`
	Type   string         `json:"type,omitempty"`
	Schema map[string]any `json:"schema,omitempty"`
}

// ORPCError represents an error definition.
type ORPCError struct {
	Code        int            `json:"code"`
	Message     string         `json:"message"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// ORPCServiceSchema wraps an oRPC schema with its service context.
type ORPCServiceSchema struct {
	Manifest *farp.SchemaManifest
	Schema   any
	Parsed   *ORPCSpec
}

// ORPCMerger handles oRPC schema composition.
type ORPCMerger struct {
	config MergerConfig
}

// NewORPCMerger creates a new oRPC merger.
func NewORPCMerger(config MergerConfig) *ORPCMerger {
	return &ORPCMerger{
		config: config,
	}
}

// ORPCMergeResult contains the merged oRPC spec and metadata.
type ORPCMergeResult struct {
	Spec             *ORPCSpec
	IncludedServices []string
	ExcludedServices []string
	Conflicts        []Conflict
	Warnings         []string
}

// MergeORPC merges multiple oRPC schemas from service manifests.
func (m *ORPCMerger) MergeORPC(schemas []ORPCServiceSchema) (*ORPCMergeResult, error) {
	result := &ORPCMergeResult{
		Spec: &ORPCSpec{
			ORPC: "1.0.0",
			Info: Info{
				Title:       m.config.MergedTitle,
				Description: m.config.MergedDescription,
				Version:     m.config.MergedVersion,
			},
			Servers:    m.config.Servers,
			Procedures: make(map[string]ORPCProcedure),
			Schemas:    make(map[string]any),
			Extensions: make(map[string]any),
		},
		IncludedServices: []string{},
		ExcludedServices: []string{},
		Conflicts:        []Conflict{},
		Warnings:         []string{},
	}

	// Track what we've seen for conflict detection
	seenProcedures := make(map[string]string)      // procedure -> service
	seenSchemas := make(map[string]string)         // schema -> service
	seenSecuritySchemes := make(map[string]string) // security scheme -> service

	// Process each schema
	for _, schema := range schemas {
		serviceName := schema.Manifest.ServiceName

		// Check if this schema should be included
		if !shouldIncludeORPCInMerge(schema) {
			result.ExcludedServices = append(result.ExcludedServices, serviceName)

			continue
		}

		result.IncludedServices = append(result.IncludedServices, serviceName)

		// Parse the schema if not already parsed
		if schema.Parsed == nil {
			parsed, err := ParseORPCSchema(schema.Schema)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Failed to parse oRPC schema for %s: %v", serviceName, err))

				continue
			}

			schema.Parsed = parsed
		}

		// Get composition config
		compConfig := getORPCCompositionConfig(schema.Manifest)
		strategy := m.getConflictStrategy(compConfig)

		// Determine prefixes
		procedurePrefix := getORPCProcedurePrefix(schema.Manifest, compConfig)
		schemaPrefix := getORPCSchemaPrefix(schema.Manifest, compConfig)

		// Merge procedures
		for procName, procedure := range schema.Parsed.Procedures {
			prefixedName := procName
			if procedurePrefix != "" {
				prefixedName = procedurePrefix + "." + procName
			}

			// Check for procedure conflicts
			if existingService, exists := seenProcedures[prefixedName]; exists {
				conflict := Conflict{
					Type:     "orpc_procedure",
					Item:     procName,
					Services: []string{existingService, serviceName},
					Strategy: strategy,
				}

				switch strategy {
				case farp.ConflictStrategyError:
					return nil, fmt.Errorf("oRPC procedure conflict: %s exists in both %s and %s",
						procName, existingService, serviceName)

				case farp.ConflictStrategySkip:
					conflict.Resolution = "Skipped procedure from " + serviceName
					result.Conflicts = append(result.Conflicts, conflict)

					continue

				case farp.ConflictStrategyOverwrite:
					conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)

				case farp.ConflictStrategyPrefix:
					prefixedName = serviceName + "." + procName
					conflict.Resolution = "Prefixed to " + prefixedName
					result.Conflicts = append(result.Conflicts, conflict)
				}
			}

			result.Spec.Procedures[prefixedName] = procedure
			seenProcedures[prefixedName] = serviceName
		}

		// Merge schemas
		for schemaName, schemaObj := range schema.Parsed.Schemas {
			prefixedName := schemaName
			if schemaPrefix != "" {
				prefixedName = schemaPrefix + "_" + schemaName
			}

			if existingService, exists := seenSchemas[prefixedName]; exists {
				if strategy == farp.ConflictStrategySkip {
					result.Conflicts = append(result.Conflicts, Conflict{
						Type:       ConflictTypeComponent,
						Item:       schemaName,
						Services:   []string{existingService, serviceName},
						Resolution: "Skipped schema from " + serviceName,
						Strategy:   strategy,
					})

					continue
				}
			}

			result.Spec.Schemas[prefixedName] = schemaObj
			seenSchemas[prefixedName] = serviceName
		}

		// Merge security schemes
		for name, secScheme := range schema.Parsed.SecuritySchemes {
			if existingService, exists := seenSecuritySchemes[name]; exists {
				conflict := Conflict{
					Type:     ConflictTypeSecurityScheme,
					Item:     name,
					Services: []string{existingService, serviceName},
					Strategy: strategy,
				}

				switch strategy {
				case farp.ConflictStrategyError:
					return nil, fmt.Errorf("oRPC security scheme conflict: %s exists in both %s and %s",
						name, existingService, serviceName)

				case farp.ConflictStrategySkip:
					conflict.Resolution = "Skipped security scheme from " + serviceName
					result.Conflicts = append(result.Conflicts, conflict)

					continue

				case farp.ConflictStrategyOverwrite:
					conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)

				case farp.ConflictStrategyPrefix:
					prefixedName := serviceName + "_" + name
					conflict.Resolution = "Prefixed to " + prefixedName
					result.Conflicts = append(result.Conflicts, conflict)
					result.Spec.SecuritySchemes[prefixedName] = secScheme
					seenSecuritySchemes[prefixedName] = serviceName

					continue

				case farp.ConflictStrategyMerge:
					conflict.Resolution = fmt.Sprintf("Merged (overwritten) with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)
				}
			}

			result.Spec.SecuritySchemes[name] = secScheme
			seenSecuritySchemes[name] = serviceName
		}
	}

	return result, nil
}

// ParseORPCSchema parses a raw oRPC schema into structured format.
func ParseORPCSchema(raw any) (*ORPCSpec, error) {
	schemaMap, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("schema must be a map")
	}

	spec := &ORPCSpec{
		ORPC:            "1.0.0",
		Procedures:      make(map[string]ORPCProcedure),
		Schemas:         make(map[string]any),
		SecuritySchemes: make(map[string]ORPCSecurityScheme),
		Extensions:      make(map[string]any),
	}

	// Parse oRPC version
	if v, ok := schemaMap["orpc"].(string); ok {
		spec.ORPC = v
	} else {
		spec.ORPC = "1.0.0" // Default
	}

	// Parse info
	if info, ok := schemaMap["info"].(map[string]any); ok {
		spec.Info = parseInfo(info)
	}

	// Parse servers
	if servers, ok := schemaMap["servers"].([]any); ok {
		spec.Servers = parseServers(servers)
	}

	// Parse procedures
	if procedures, ok := schemaMap["procedures"].(map[string]any); ok {
		spec.Procedures = parseORPCProcedures(procedures)
	}

	// Parse schemas
	if schemas, ok := schemaMap["schemas"].(map[string]any); ok {
		spec.Schemas = schemas
	}

	// Parse security schemes
	if securitySchemes, ok := schemaMap["securitySchemes"].(map[string]any); ok {
		spec.SecuritySchemes = parseORPCSecuritySchemes(securitySchemes)
	}

	// Parse global security
	if security, ok := schemaMap["security"].([]any); ok {
		spec.Security = parseSecurityRequirements(security)
	}

	return spec, nil
}

func parseORPCProcedures(procedures map[string]any) map[string]ORPCProcedure {
	result := make(map[string]ORPCProcedure)

	for name, proc := range procedures {
		if procMap, ok := proc.(map[string]any); ok {
			procedure := ORPCProcedure{
				Name:       name,
				Extensions: make(map[string]any),
			}
			if desc, ok := procMap["description"].(string); ok {
				procedure.Description = desc
			}

			if streaming, ok := procMap["streaming"].(bool); ok {
				procedure.Streaming = streaming
			}

			if batch, ok := procMap["batch"].(bool); ok {
				procedure.Batch = batch
			}
			// Parse input/output schemas (simplified)
			result[name] = procedure
		}
	}

	return result
}

func parseORPCSecuritySchemes(schemes map[string]any) map[string]ORPCSecurityScheme {
	result := make(map[string]ORPCSecurityScheme)

	for name, s := range schemes {
		if schemeMap, ok := s.(map[string]any); ok {
			scheme := ORPCSecurityScheme{}
			if t, ok := schemeMap["type"].(string); ok {
				scheme.Type = t
			}

			if desc, ok := schemeMap["description"].(string); ok {
				scheme.Description = desc
			}

			if n, ok := schemeMap["name"].(string); ok {
				scheme.Name = n
			}

			if in, ok := schemeMap["in"].(string); ok {
				scheme.In = in
			}

			if scheme_, ok := schemeMap["scheme"].(string); ok {
				scheme.Scheme = scheme_
			}

			if bf, ok := schemeMap["bearerFormat"].(string); ok {
				scheme.BearerFormat = bf
			}

			if oidc, ok := schemeMap["openIdConnectUrl"].(string); ok {
				scheme.OpenIdConnectURL = oidc
			}

			if flows, ok := schemeMap["flows"].(map[string]any); ok {
				scheme.Flows = flows
			}

			result[name] = scheme
		}
	}

	return result
}

func parseSecurityRequirements(security []any) []map[string][]string {
	result := make([]map[string][]string, 0, len(security))

	for _, req := range security {
		if reqMap, ok := req.(map[string]any); ok {
			requirement := make(map[string][]string)

			for schemeName, scopes := range reqMap {
				if scopeList, ok := scopes.([]any); ok {
					scopeStrings := make([]string, 0, len(scopeList))

					for _, scope := range scopeList {
						if s, ok := scope.(string); ok {
							scopeStrings = append(scopeStrings, s)
						}
					}

					requirement[schemeName] = scopeStrings
				}
			}

			result = append(result, requirement)
		}
	}

	return result
}

// Helper functions for oRPC composition

func shouldIncludeORPCInMerge(schema ORPCServiceSchema) bool {
	for _, schemaDesc := range schema.Manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeORPC {
			return true
		}
	}

	return false
}

func getORPCCompositionConfig(manifest *farp.SchemaManifest) *farp.CompositionConfig {
	// oRPC doesn't have composition config yet in spec
	return nil
}

func (m *ORPCMerger) getConflictStrategy(config *farp.CompositionConfig) farp.ConflictStrategy {
	if config != nil && config.ConflictStrategy != "" {
		return config.ConflictStrategy
	}

	return m.config.DefaultConflictStrategy
}

func getORPCProcedurePrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}

	return manifest.ServiceName
}

func getORPCSchemaPrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}

	return manifest.ServiceName
}

// SortProcedures sorts procedure names alphabetically.
func SortProcedures(procedures map[string]ORPCProcedure) []string {
	keys := make([]string, 0, len(procedures))
	for k := range procedures {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
