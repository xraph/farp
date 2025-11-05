package merger

import (
	"errors"
	"fmt"
	"sort"

	"github.com/xraph/farp"
)

// AsyncAPISpec represents a simplified AsyncAPI 2.x/3.x specification.
type AsyncAPISpec struct {
	AsyncAPI   string                 `json:"asyncapi"`
	Info       Info                   `json:"info"`
	Servers    map[string]AsyncServer `json:"servers,omitempty"`
	Channels   map[string]Channel     `json:"channels"`
	Components *AsyncComponents       `json:"components,omitempty"`
	Security   []map[string][]string  `json:"security,omitempty"`
	Extensions map[string]any         `json:"-"`
}

// AsyncServer represents an AsyncAPI server (broker connection).
type AsyncServer struct {
	URL         string         `json:"url"`
	Protocol    string         `json:"protocol"` // kafka, amqp, mqtt, ws, etc.
	Description string         `json:"description,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	Bindings    map[string]any `json:"bindings,omitempty"`
}

// Channel represents an AsyncAPI channel.
type Channel struct {
	Description string         `json:"description,omitempty"`
	Subscribe   *Operation     `json:"subscribe,omitempty"`
	Publish     *Operation     `json:"publish,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Bindings    map[string]any `json:"bindings,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// AsyncComponents represents AsyncAPI components.
type AsyncComponents struct {
	Messages        map[string]map[string]any      `json:"messages,omitempty"`
	Schemas         map[string]map[string]any      `json:"schemas,omitempty"`
	SecuritySchemes map[string]AsyncSecurityScheme `json:"securitySchemes,omitempty"`
}

// AsyncSecurityScheme represents an AsyncAPI security scheme.
type AsyncSecurityScheme struct {
	Type             string         `json:"type"` // userPassword, apiKey, X509, symmetricEncryption, asymmetricEncryption, httpApiKey, http, oauth2, openIdConnect
	Description      string         `json:"description,omitempty"`
	Name             string         `json:"name,omitempty"`             // For apiKey, httpApiKey
	In               string         `json:"in,omitempty"`               // For apiKey, httpApiKey: user, password, query, header, cookie
	Scheme           string         `json:"scheme,omitempty"`           // For http
	BearerFormat     string         `json:"bearerFormat,omitempty"`     // For http bearer
	OpenIdConnectURL string         `json:"openIdConnectUrl,omitempty"` // For openIdConnect
	Flows            map[string]any `json:"flows,omitempty"`            // For oauth2
}

// AsyncAPIServiceSchema wraps an AsyncAPI schema with its service context.
type AsyncAPIServiceSchema struct {
	Manifest *farp.SchemaManifest
	Schema   any
	Parsed   *AsyncAPISpec
}

// AsyncAPIMerger handles AsyncAPI schema composition.
type AsyncAPIMerger struct {
	config MergerConfig
}

// NewAsyncAPIMerger creates a new AsyncAPI merger.
func NewAsyncAPIMerger(config MergerConfig) *AsyncAPIMerger {
	return &AsyncAPIMerger{
		config: config,
	}
}

// AsyncAPIMergeResult contains the merged AsyncAPI spec and metadata.
type AsyncAPIMergeResult struct {
	Spec             *AsyncAPISpec
	IncludedServices []string
	ExcludedServices []string
	Conflicts        []Conflict
	Warnings         []string
}

// MergeAsyncAPI merges multiple AsyncAPI schemas from service manifests.
func (m *AsyncAPIMerger) MergeAsyncAPI(schemas []AsyncAPIServiceSchema) (*AsyncAPIMergeResult, error) {
	result := &AsyncAPIMergeResult{
		Spec: &AsyncAPISpec{
			AsyncAPI: "2.6.0", // Use AsyncAPI 2.6
			Info: Info{
				Title:       m.config.MergedTitle,
				Description: m.config.MergedDescription,
				Version:     m.config.MergedVersion,
			},
			Servers:  make(map[string]AsyncServer),
			Channels: make(map[string]Channel),
			Components: &AsyncComponents{
				Messages: make(map[string]map[string]any),
				Schemas:  make(map[string]map[string]any),
			},
			Extensions: make(map[string]any),
		},
		IncludedServices: []string{},
		ExcludedServices: []string{},
		Conflicts:        []Conflict{},
		Warnings:         []string{},
	}

	// Track what we've seen for conflict detection
	seenChannels := make(map[string]string)        // channel -> service
	seenMessages := make(map[string]string)        // message -> service
	seenServers := make(map[string]string)         // server -> service
	seenSecuritySchemes := make(map[string]string) // security scheme -> service

	// Process each schema
	for _, schema := range schemas {
		serviceName := schema.Manifest.ServiceName

		// Check if this schema should be included
		if !shouldIncludeAsyncAPIInMerge(schema) {
			result.ExcludedServices = append(result.ExcludedServices, serviceName)

			continue
		}

		result.IncludedServices = append(result.IncludedServices, serviceName)

		// Parse the schema if not already parsed
		if schema.Parsed == nil {
			parsed, err := ParseAsyncAPISchema(schema.Schema)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Failed to parse AsyncAPI schema for %s: %v", serviceName, err))

				continue
			}

			schema.Parsed = parsed
		}

		// Get composition config
		compConfig := getAsyncAPICompositionConfig(schema.Manifest)
		strategy := m.getConflictStrategy(compConfig)

		// Determine prefixes
		channelPrefix := getAsyncAPIChannelPrefix(schema.Manifest, compConfig)
		messagePrefix := getAsyncAPIMessagePrefix(schema.Manifest, compConfig)

		// Merge channels
		for channelName, channel := range schema.Parsed.Channels {
			prefixedName := channelName
			if channelPrefix != "" {
				prefixedName = channelPrefix + "." + channelName
			}

			// Check for channel conflicts
			if existingService, exists := seenChannels[prefixedName]; exists {
				conflict := Conflict{
					Type:     "channel",
					Item:     channelName,
					Services: []string{existingService, serviceName},
					Strategy: strategy,
				}

				switch strategy {
				case farp.ConflictStrategyError:
					return nil, fmt.Errorf("channel conflict: %s exists in both %s and %s",
						channelName, existingService, serviceName)

				case farp.ConflictStrategySkip:
					conflict.Resolution = "Skipped channel from " + serviceName
					result.Conflicts = append(result.Conflicts, conflict)

					continue

				case farp.ConflictStrategyOverwrite:
					conflict.Resolution = fmt.Sprintf("Overwritten with %s version", serviceName)
					result.Conflicts = append(result.Conflicts, conflict)

				case farp.ConflictStrategyPrefix:
					prefixedName = serviceName + "." + channelName
					conflict.Resolution = "Prefixed to " + prefixedName
					result.Conflicts = append(result.Conflicts, conflict)

				case farp.ConflictStrategyMerge:
					// Merge operations
					channel = mergeChannels(result.Spec.Channels[prefixedName], channel)
					conflict.Resolution = "Merged operations"
					result.Conflicts = append(result.Conflicts, conflict)
				}
			}

			result.Spec.Channels[prefixedName] = channel
			seenChannels[prefixedName] = serviceName
		}

		// Merge components
		if schema.Parsed.Components != nil {
			// Merge messages
			for name, message := range schema.Parsed.Components.Messages {
				prefixedName := name
				if messagePrefix != "" {
					prefixedName = messagePrefix + "_" + name
				}

				if existingService, exists := seenMessages[prefixedName]; exists {
					if strategy == farp.ConflictStrategySkip {
						result.Conflicts = append(result.Conflicts, Conflict{
							Type:       ConflictTypeComponent,
							Item:       name,
							Services:   []string{existingService, serviceName},
							Resolution: "Skipped message from " + serviceName,
							Strategy:   strategy,
						})

						continue
					}
				}

				result.Spec.Components.Messages[prefixedName] = message
				seenMessages[prefixedName] = serviceName
			}

			// Merge schemas
			for name, schemaObj := range schema.Parsed.Components.Schemas {
				prefixedName := messagePrefix + "_" + name
				result.Spec.Components.Schemas[prefixedName] = schemaObj
			}

			// Merge security schemes
			for name, secScheme := range schema.Parsed.Components.SecuritySchemes {
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
						result.Spec.Components.SecuritySchemes[prefixedName] = secScheme
						seenSecuritySchemes[prefixedName] = serviceName

						continue

					case farp.ConflictStrategyMerge:
						conflict.Resolution = fmt.Sprintf("Merged (overwritten) with %s version", serviceName)
						result.Conflicts = append(result.Conflicts, conflict)
					}
				}

				result.Spec.Components.SecuritySchemes[name] = secScheme
				seenSecuritySchemes[name] = serviceName
			}
		}

		// Merge servers
		for serverName, server := range schema.Parsed.Servers {
			prefixedName := serviceName + "_" + serverName
			if existingService, exists := seenServers[prefixedName]; exists {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Server %s from %s overwrites %s", serverName, serviceName, existingService))
			}

			result.Spec.Servers[prefixedName] = server
			seenServers[prefixedName] = serviceName
		}
	}

	return result, nil
}

// ParseAsyncAPISchema parses a raw AsyncAPI schema into structured format.
func ParseAsyncAPISchema(raw any) (*AsyncAPISpec, error) {
	schemaMap, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("schema must be a map")
	}

	spec := &AsyncAPISpec{
		Servers:    make(map[string]AsyncServer),
		Channels:   make(map[string]Channel),
		Extensions: make(map[string]any),
	}

	// Parse AsyncAPI version
	if v, ok := schemaMap["asyncapi"].(string); ok {
		spec.AsyncAPI = v
	} else {
		return nil, errors.New("missing asyncapi version")
	}

	// Parse info
	if info, ok := schemaMap["info"].(map[string]any); ok {
		spec.Info = parseInfo(info)
	}

	// Parse servers
	if servers, ok := schemaMap["servers"].(map[string]any); ok {
		spec.Servers = parseAsyncServers(servers)
	}

	// Parse channels
	if channels, ok := schemaMap["channels"].(map[string]any); ok {
		spec.Channels = parseChannels(channels)
	}

	// Parse components
	if components, ok := schemaMap["components"].(map[string]any); ok {
		spec.Components = parseAsyncComponents(components)
	}

	return spec, nil
}

func parseAsyncServers(servers map[string]any) map[string]AsyncServer {
	result := make(map[string]AsyncServer)

	for name, s := range servers {
		if serverMap, ok := s.(map[string]any); ok {
			server := AsyncServer{}
			if url, ok := serverMap["url"].(string); ok {
				server.URL = url
			}

			if protocol, ok := serverMap["protocol"].(string); ok {
				server.Protocol = protocol
			}

			if desc, ok := serverMap["description"].(string); ok {
				server.Description = desc
			}

			result[name] = server
		}
	}

	return result
}

func parseChannels(channels map[string]any) map[string]Channel {
	result := make(map[string]Channel)

	for name, ch := range channels {
		if channelMap, ok := ch.(map[string]any); ok {
			channel := Channel{Extensions: make(map[string]any)}
			if desc, ok := channelMap["description"].(string); ok {
				channel.Description = desc
			}
			// Parse subscribe/publish operations (simplified)
			if sub, ok := channelMap["subscribe"].(map[string]any); ok {
				channel.Subscribe = parseOperation(sub)
			}

			if pub, ok := channelMap["publish"].(map[string]any); ok {
				channel.Publish = parseOperation(pub)
			}

			result[name] = channel
		}
	}

	return result
}

func parseAsyncComponents(components map[string]any) *AsyncComponents {
	result := &AsyncComponents{
		Messages:        make(map[string]map[string]any),
		Schemas:         make(map[string]map[string]any),
		SecuritySchemes: make(map[string]AsyncSecurityScheme),
	}

	if messages, ok := components["messages"].(map[string]any); ok {
		for name, msg := range messages {
			if msgMap, ok := msg.(map[string]any); ok {
				result.Messages[name] = msgMap
			}
		}
	}

	if schemas, ok := components["schemas"].(map[string]any); ok {
		for name, schema := range schemas {
			if schemaMap, ok := schema.(map[string]any); ok {
				result.Schemas[name] = schemaMap
			}
		}
	}

	// Parse security schemes
	if securitySchemes, ok := components["securitySchemes"].(map[string]any); ok {
		for name, scheme := range securitySchemes {
			if schemeMap, ok := scheme.(map[string]any); ok {
				sec := AsyncSecurityScheme{}
				if t, ok := schemeMap["type"].(string); ok {
					sec.Type = t
				}

				if desc, ok := schemeMap["description"].(string); ok {
					sec.Description = desc
				}

				if n, ok := schemeMap["name"].(string); ok {
					sec.Name = n
				}

				if in, ok := schemeMap["in"].(string); ok {
					sec.In = in
				}

				if s, ok := schemeMap["scheme"].(string); ok {
					sec.Scheme = s
				}

				if bf, ok := schemeMap["bearerFormat"].(string); ok {
					sec.BearerFormat = bf
				}

				if oidc, ok := schemeMap["openIdConnectUrl"].(string); ok {
					sec.OpenIdConnectURL = oidc
				}

				if flows, ok := schemeMap["flows"].(map[string]any); ok {
					sec.Flows = flows
				}

				result.SecuritySchemes[name] = sec
			}
		}
	}

	return result
}

func mergeChannels(existing, newChannel Channel) Channel {
	// Merge subscribe/publish operations
	if newChannel.Subscribe != nil {
		existing.Subscribe = newChannel.Subscribe
	}

	if newChannel.Publish != nil {
		existing.Publish = newChannel.Publish
	}

	return existing
}

// Helper functions for AsyncAPI composition

func shouldIncludeAsyncAPIInMerge(schema AsyncAPIServiceSchema) bool {
	for _, schemaDesc := range schema.Manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeAsyncAPI &&
			schemaDesc.Metadata != nil &&
			schemaDesc.Metadata.AsyncAPI != nil {
			// Check for composition config when available
			return true
		}
	}

	return false
}

func getAsyncAPICompositionConfig(manifest *farp.SchemaManifest) *farp.CompositionConfig {
	for _, schemaDesc := range manifest.Schemas {
		if schemaDesc.Type == farp.SchemaTypeAsyncAPI &&
			schemaDesc.Metadata != nil &&
			schemaDesc.Metadata.AsyncAPI != nil {
			// For now, return nil - AsyncAPI doesn't have composition config yet
			// Could be added to spec later
			return nil
		}
	}

	return nil
}

func (m *AsyncAPIMerger) getConflictStrategy(config *farp.CompositionConfig) farp.ConflictStrategy {
	if config != nil && config.ConflictStrategy != "" {
		return config.ConflictStrategy
	}

	return m.config.DefaultConflictStrategy
}

func getAsyncAPIChannelPrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}

	return manifest.ServiceName
}

func getAsyncAPIMessagePrefix(manifest *farp.SchemaManifest, config *farp.CompositionConfig) string {
	if config != nil && config.ComponentPrefix != "" {
		return config.ComponentPrefix
	}

	return manifest.ServiceName
}

// SortChannels sorts channels alphabetically.
func SortChannels(channels map[string]Channel) []string {
	keys := make([]string, 0, len(channels))
	for k := range channels {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
