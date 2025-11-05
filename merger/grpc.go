package merger

import (
	"errors"
	"fmt"
	"sort"

	"github.com/xraph/farp"
)

// GRPCSpec represents a simplified gRPC service definition (protobuf-based).
type GRPCSpec struct {
	Syntax          string                        `json:"syntax"` // proto3
	Package         string                        `json:"package"`
	Services        map[string]GRPCService        `json:"services"`
	Messages        map[string]GRPCMessage        `json:"messages"`
	Enums           map[string]GRPCEnum           `json:"enums,omitempty"`
	SecuritySchemes map[string]GRPCSecurityScheme `json:"securitySchemes,omitempty"`
	Imports         []string                      `json:"imports,omitempty"`
}

// GRPCSecurityScheme represents gRPC authentication configuration.
type GRPCSecurityScheme struct {
	Type        string `json:"type"` // tls, oauth2, apiKey, custom
	Description string `json:"description,omitempty"`
	// TLS settings
	TLS *GRPCTLSConfig `json:"tls,omitempty"`
	// OAuth2 settings
	TokenURL string            `json:"tokenUrl,omitempty"`
	Scopes   map[string]string `json:"scopes,omitempty"`
	// API Key settings
	KeyName string `json:"keyName,omitempty"`
	// Custom metadata
	Metadata map[string]any `json:"metadata,omitempty"`
}

// GRPCTLSConfig represents TLS configuration for gRPC.
type GRPCTLSConfig struct {
	ServerName         string `json:"serverName,omitempty"`
	RequireClientCert  bool   `json:"requireClientCert"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty"` // Dev only
}

// GRPCService represents a gRPC service definition.
type GRPCService struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Methods     map[string]GRPCMethod `json:"methods"`
	Options     map[string]any        `json:"options,omitempty"`
}

// GRPCMethod represents a gRPC method (RPC).
type GRPCMethod struct {
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	InputType       string         `json:"input_type"`
	OutputType      string         `json:"output_type"`
	ClientStreaming bool           `json:"client_streaming"`
	ServerStreaming bool           `json:"server_streaming"`
	Options         map[string]any `json:"options,omitempty"`
}

// GRPCMessage represents a protobuf message.
type GRPCMessage struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Fields      map[string]GRPCField `json:"fields"`
	Options     map[string]any       `json:"options,omitempty"`
}

// GRPCField represents a message field.
type GRPCField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Number   int    `json:"number"`
	Repeated bool   `json:"repeated"`
	Optional bool   `json:"optional"`
}

// GRPCEnum represents a protobuf enum.
type GRPCEnum struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Values      map[string]int `json:"values"`
}

// GRPCServiceSchema wraps a gRPC schema with its service context.
type GRPCServiceSchema struct {
	Manifest *farp.SchemaManifest
	Schema   any
	Parsed   *GRPCSpec
}

// GRPCMerger handles gRPC schema composition.
type GRPCMerger struct {
	config MergerConfig
}

// NewGRPCMerger creates a new gRPC merger.
func NewGRPCMerger(config MergerConfig) *GRPCMerger {
	return &GRPCMerger{
		config: config,
	}
}

// GRPCMergeResult contains the merged gRPC spec and metadata.
type GRPCMergeResult struct {
	Spec             *GRPCSpec
	IncludedServices []string
	ExcludedServices []string
	Conflicts        []Conflict
	Warnings         []string
}

// MergeGRPC merges multiple gRPC schemas from service manifests.
func (m *GRPCMerger) MergeGRPC(schemas []GRPCServiceSchema) (*GRPCMergeResult, error) {
	result := &GRPCMergeResult{
		Spec: &GRPCSpec{
			Syntax:   "proto3",
			Package:  m.config.MergedTitle, // Use title as package name
			Services: make(map[string]GRPCService),
			Messages: make(map[string]GRPCMessage),
			Enums:    make(map[string]GRPCEnum),
			Imports:  []string{},
		},
		IncludedServices: []string{},
		ExcludedServices: []string{},
		Conflicts:        []Conflict{},
		Warnings:         []string{},
	}

	// Track what we've seen for conflict detection
	seenServices := make(map[string]string)        // service -> source service
	seenMessages := make(map[string]string)        // message -> source service
	seenEnums := make(map[string]string)           // enum -> source service
	seenSecuritySchemes := make(map[string]string) // security scheme -> source service

	// Process each schema
	for _, schema := range schemas {
		serviceName := schema.Manifest.ServiceName

		// Check if this schema should be included
		if !shouldIncludeGRPCInMerge(schema) {
			result.ExcludedServices = append(result.ExcludedServices, serviceName)

			continue
		}

		result.IncludedServices = append(result.IncludedServices, serviceName)

		// Parse the schema if not already parsed
		if schema.Parsed == nil {
			parsed, err := ParseGRPCSchema(schema.Schema)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Failed to parse gRPC schema for %s: %v", serviceName, err))

				continue
			}

			schema.Parsed = parsed
		}

		// Get composition config
		compConfig := getGRPCCompositionConfig(schema.Manifest)
		strategy := m.getConflictStrategy(compConfig)

		// Determine prefixes
		servicePrefix := getGRPCServicePrefix(schema.Manifest, compConfig)
		messagePrefix := getGRPCMessagePrefix(schema.Manifest, compConfig)

		// Merge services
		for svcName, service := range schema.Parsed.Services {
			prefixedName := svcName
			if servicePrefix != "" {
				prefixedName = servicePrefix + "_" + svcName
			}

			// Check for service conflicts
			if existingService, exists := seenServices[prefixedName]; exists {
				conflict := Conflict{
					Type:     "grpc_service",
					Item:     svcName,
					Services: []string{existingService, serviceName},
					Strategy: strategy,
				}

				switch strategy {
				case farp.ConflictStrategyError:
					return nil, fmt.Errorf("gRPC service conflict: %s exists in both %s and %s",
						svcName, existingService, serviceName)

				case farp.ConflictStrategySkip:
					conflict.Resolution = "Skipped service from " + serviceName
					result.Conflicts = append(result.Conflicts, conflict)

					continue

				case farp.ConflictStrategyOverwrite:
					conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)

				case farp.ConflictStrategyPrefix:
					prefixedName = serviceName + "_" + svcName
					conflict.Resolution = "Prefixed to " + prefixedName
					result.Conflicts = append(result.Conflicts, conflict)
				}
			}

			result.Spec.Services[prefixedName] = service
			seenServices[prefixedName] = serviceName
		}

		// Merge messages
		for msgName, message := range schema.Parsed.Messages {
			prefixedName := msgName
			if messagePrefix != "" {
				prefixedName = messagePrefix + "_" + msgName
			}

			if existingService, exists := seenMessages[prefixedName]; exists {
				if strategy == farp.ConflictStrategySkip {
					result.Conflicts = append(result.Conflicts, Conflict{
						Type:       "grpc_message",
						Item:       msgName,
						Services:   []string{existingService, serviceName},
						Resolution: "Skipped message from " + serviceName,
						Strategy:   strategy,
					})

					continue
				}
			}

			result.Spec.Messages[prefixedName] = message
			seenMessages[prefixedName] = serviceName
		}

		// Merge enums
		for enumName, enum := range schema.Parsed.Enums {
			prefixedName := messagePrefix + "_" + enumName
			if existingService, exists := seenEnums[prefixedName]; exists {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Enum %s from %s overwrites %s", enumName, serviceName, existingService))
			}

			result.Spec.Enums[prefixedName] = enum
			seenEnums[prefixedName] = serviceName
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
					return nil, fmt.Errorf("gRPC security scheme conflict: %s exists in both %s and %s",
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

// ParseGRPCSchema parses a raw gRPC schema into structured format.
func ParseGRPCSchema(raw any) (*GRPCSpec, error) {
	schemaMap, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("schema must be a map")
	}

	spec := &GRPCSpec{
		Syntax:          "proto3",
		Services:        make(map[string]GRPCService),
		Messages:        make(map[string]GRPCMessage),
		Enums:           make(map[string]GRPCEnum),
		SecuritySchemes: make(map[string]GRPCSecurityScheme),
		Imports:         []string{},
	}

	// Parse package
	if pkg, ok := schemaMap["package"].(string); ok {
		spec.Package = pkg
	}

	// Parse services
	if services, ok := schemaMap["services"].(map[string]any); ok {
		spec.Services = parseGRPCServices(services)
	}

	// Parse messages
	if messages, ok := schemaMap["messages"].(map[string]any); ok {
		spec.Messages = parseGRPCMessages(messages)
	}

	// Parse enums
	if enums, ok := schemaMap["enums"].(map[string]any); ok {
		spec.Enums = parseGRPCEnums(enums)
	}

	// Parse security schemes
	if securitySchemes, ok := schemaMap["securitySchemes"].(map[string]any); ok {
		spec.SecuritySchemes = parseGRPCSecuritySchemes(securitySchemes)
	}

	return spec, nil
}

func parseGRPCServices(services map[string]any) map[string]GRPCService {
	result := make(map[string]GRPCService)

	for name, svc := range services {
		if svcMap, ok := svc.(map[string]any); ok {
			service := GRPCService{
				Name:    name,
				Methods: make(map[string]GRPCMethod),
			}
			if desc, ok := svcMap["description"].(string); ok {
				service.Description = desc
			}

			if methods, ok := svcMap["methods"].(map[string]any); ok {
				service.Methods = parseGRPCMethods(methods)
			}

			result[name] = service
		}
	}

	return result
}

func parseGRPCMethods(methods map[string]any) map[string]GRPCMethod {
	result := make(map[string]GRPCMethod)

	for name, m := range methods {
		if methodMap, ok := m.(map[string]any); ok {
			method := GRPCMethod{Name: name}
			if input, ok := methodMap["input_type"].(string); ok {
				method.InputType = input
			}

			if output, ok := methodMap["output_type"].(string); ok {
				method.OutputType = output
			}

			if stream, ok := methodMap["client_streaming"].(bool); ok {
				method.ClientStreaming = stream
			}

			if stream, ok := methodMap["server_streaming"].(bool); ok {
				method.ServerStreaming = stream
			}

			result[name] = method
		}
	}

	return result
}

func parseGRPCMessages(messages map[string]any) map[string]GRPCMessage {
	result := make(map[string]GRPCMessage)

	for name, msg := range messages {
		if msgMap, ok := msg.(map[string]any); ok {
			message := GRPCMessage{
				Name:   name,
				Fields: make(map[string]GRPCField),
			}
			if desc, ok := msgMap["description"].(string); ok {
				message.Description = desc
			}
			// Parse fields (simplified)
			if fields, ok := msgMap["fields"].(map[string]any); ok {
				for fieldName, field := range fields {
					if fieldMap, ok := field.(map[string]any); ok {
						f := GRPCField{Name: fieldName}
						if t, ok := fieldMap["type"].(string); ok {
							f.Type = t
						}

						if n, ok := fieldMap["number"].(float64); ok {
							f.Number = int(n)
						}

						message.Fields[fieldName] = f
					}
				}
			}

			result[name] = message
		}
	}

	return result
}

func parseGRPCEnums(enums map[string]any) map[string]GRPCEnum {
	result := make(map[string]GRPCEnum)

	for name, e := range enums {
		if enumMap, ok := e.(map[string]any); ok {
			enum := GRPCEnum{
				Name:   name,
				Values: make(map[string]int),
			}

			if values, ok := enumMap["values"].(map[string]any); ok {
				for valName, val := range values {
					if v, ok := val.(float64); ok {
						enum.Values[valName] = int(v)
					}
				}
			}

			result[name] = enum
		}
	}

	return result
}

func parseGRPCSecuritySchemes(schemes map[string]any) map[string]GRPCSecurityScheme {
	result := make(map[string]GRPCSecurityScheme)

	for name, s := range schemes {
		if schemeMap, ok := s.(map[string]any); ok {
			scheme := GRPCSecurityScheme{}
			if t, ok := schemeMap["type"].(string); ok {
				scheme.Type = t
			}

			if desc, ok := schemeMap["description"].(string); ok {
				scheme.Description = desc
			}

			if tokenURL, ok := schemeMap["tokenUrl"].(string); ok {
				scheme.TokenURL = tokenURL
			}

			if keyName, ok := schemeMap["keyName"].(string); ok {
				scheme.KeyName = keyName
			}
			// Parse TLS config if present
			if tls, ok := schemeMap["tls"].(map[string]any); ok {
				tlsConfig := &GRPCTLSConfig{}
				if serverName, ok := tls["serverName"].(string); ok {
					tlsConfig.ServerName = serverName
				}

				if requireClient, ok := tls["requireClientCert"].(bool); ok {
					tlsConfig.RequireClientCert = requireClient
				}

				scheme.TLS = tlsConfig
			}

			result[name] = scheme
		}
	}

	return result
}

// Helper functions for gRPC composition

func shouldIncludeGRPCInMerge(schema GRPCServiceSchema) bool {
	for _, schemaDesc := range schema.Manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeGRPC {
			return true
		}
	}

	return false
}

func getGRPCCompositionConfig(manifest *farp.SchemaManifest) *farp.CompositionConfig {
	// gRPC doesn't have composition config yet in spec
	return nil
}

func (m *GRPCMerger) getConflictStrategy(config *farp.CompositionConfig) farp.ConflictStrategy {
	if config != nil && config.ConflictStrategy != "" {
		return config.ConflictStrategy
	}

	return m.config.DefaultConflictStrategy
}

func getGRPCServicePrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}

	return manifest.ServiceName
}

func getGRPCMessagePrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}

	return manifest.ServiceName
}

// SortServices sorts service names alphabetically.
func SortGRPCServices(services map[string]GRPCService) []string {
	keys := make([]string, 0, len(services))
	for k := range services {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
