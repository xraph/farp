package merger

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xraph/farp"
)

// OpenAPISpec represents a simplified OpenAPI 3.x specification
type OpenAPISpec struct {
	OpenAPI    string                 `json:"openapi"`
	Info       Info                   `json:"info"`
	Servers    []Server               `json:"servers,omitempty"`
	Paths      map[string]PathItem    `json:"paths"`
	Components *Components            `json:"components,omitempty"`
	Security   []map[string][]string  `json:"security,omitempty"`
	Tags       []Tag                  `json:"tags,omitempty"`
	Extensions map[string]interface{} `json:"-"` // x-* extensions
}

// Info represents OpenAPI info object
type Info struct {
	Title          string                 `json:"title"`
	Description    string                 `json:"description,omitempty"`
	Version        string                 `json:"version"`
	TermsOfService string                 `json:"termsOfService,omitempty"`
	Contact        *Contact               `json:"contact,omitempty"`
	License        *License               `json:"license,omitempty"`
	Extensions     map[string]interface{} `json:"-"`
}

// Contact represents contact information
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License represents license information
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server represents an OpenAPI server
type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable represents a server variable
type ServerVariable struct {
	Default     string   `json:"default"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description,omitempty"`
}

// PathItem represents an OpenAPI path item
type PathItem struct {
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Get         *Operation             `json:"get,omitempty"`
	Put         *Operation             `json:"put,omitempty"`
	Post        *Operation             `json:"post,omitempty"`
	Delete      *Operation             `json:"delete,omitempty"`
	Options     *Operation             `json:"options,omitempty"`
	Head        *Operation             `json:"head,omitempty"`
	Patch       *Operation             `json:"patch,omitempty"`
	Trace       *Operation             `json:"trace,omitempty"`
	Parameters  []Parameter            `json:"parameters,omitempty"`
	Extensions  map[string]interface{} `json:"-"`
}

// Operation represents an OpenAPI operation
type Operation struct {
	OperationID string                 `json:"operationId,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Parameters  []Parameter            `json:"parameters,omitempty"`
	RequestBody *RequestBody           `json:"requestBody,omitempty"`
	Responses   map[string]Response    `json:"responses,omitempty"`
	Security    []map[string][]string  `json:"security,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty"`
	Extensions  map[string]interface{} `json:"-"`
}

// Parameter represents an OpenAPI parameter
type Parameter struct {
	Name        string                 `json:"name"`
	In          string                 `json:"in"` // query, header, path, cookie
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
	Example     interface{}            `json:"example,omitempty"`
}

// RequestBody represents an OpenAPI request body
type RequestBody struct {
	Description string                            `json:"description,omitempty"`
	Content     map[string]MediaType              `json:"content"`
	Required    bool                              `json:"required,omitempty"`
	Extensions  map[string]interface{}            `json:"-"`
}

// Response represents an OpenAPI response
type Response struct {
	Description string                 `json:"description"`
	Content     map[string]MediaType   `json:"content,omitempty"`
	Headers     map[string]Header      `json:"headers,omitempty"`
	Extensions  map[string]interface{} `json:"-"`
}

// MediaType represents a media type object
type MediaType struct {
	Schema   map[string]interface{} `json:"schema,omitempty"`
	Example  interface{}            `json:"example,omitempty"`
	Examples map[string]Example     `json:"examples,omitempty"`
}

// Example represents an example object
type Example struct {
	Summary       string      `json:"summary,omitempty"`
	Description   string      `json:"description,omitempty"`
	Value         interface{} `json:"value,omitempty"`
	ExternalValue string      `json:"externalValue,omitempty"`
}

// Header represents a header object
type Header struct {
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

// Components represents OpenAPI components
type Components struct {
	Schemas         map[string]map[string]interface{} `json:"schemas,omitempty"`
	Responses       map[string]Response                `json:"responses,omitempty"`
	Parameters      map[string]Parameter               `json:"parameters,omitempty"`
	RequestBodies   map[string]RequestBody             `json:"requestBodies,omitempty"`
	Headers         map[string]Header                  `json:"headers,omitempty"`
	SecuritySchemes map[string]SecurityScheme          `json:"securitySchemes,omitempty"`
}

// SecurityScheme represents a security scheme
type SecurityScheme struct {
	Type             string `json:"type"` // apiKey, http, oauth2, openIdConnect
	Description      string `json:"description,omitempty"`
	Name             string `json:"name,omitempty"`              // For apiKey
	In               string `json:"in,omitempty"`                // For apiKey: query, header, cookie
	Scheme           string `json:"scheme,omitempty"`            // For http: bearer, basic
	BearerFormat     string `json:"bearerFormat,omitempty"`      // For http bearer
	OpenIdConnectURL string `json:"openIdConnectUrl,omitempty"` // For openIdConnect
}

// Tag represents an OpenAPI tag
type Tag struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Extensions  map[string]interface{} `json:"-"`
}

// ServiceSchema wraps a schema with its service context
type ServiceSchema struct {
	Manifest *farp.SchemaManifest
	Schema   interface{} // Raw OpenAPI schema (map[string]interface{})
	Parsed   *OpenAPISpec
}

// ParseOpenAPISchema parses a raw OpenAPI schema into structured format
func ParseOpenAPISchema(raw interface{}) (*OpenAPISpec, error) {
	schemaMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("schema must be a map")
	}

	spec := &OpenAPISpec{
		Paths:      make(map[string]PathItem),
		Extensions: make(map[string]interface{}),
	}

	// Parse OpenAPI version
	if v, ok := schemaMap["openapi"].(string); ok {
		spec.OpenAPI = v
	} else {
		return nil, fmt.Errorf("missing openapi version")
	}

	// Parse info
	if info, ok := schemaMap["info"].(map[string]interface{}); ok {
		spec.Info = parseInfo(info)
	}

	// Parse servers
	if servers, ok := schemaMap["servers"].([]interface{}); ok {
		spec.Servers = parseServers(servers)
	}

	// Parse paths
	if paths, ok := schemaMap["paths"].(map[string]interface{}); ok {
		spec.Paths = parsePaths(paths)
	}

	// Parse components
	if components, ok := schemaMap["components"].(map[string]interface{}); ok {
		spec.Components = parseComponents(components)
	}

	// Parse tags
	if tags, ok := schemaMap["tags"].([]interface{}); ok {
		spec.Tags = parseTags(tags)
	}

	// Parse extensions (x-*)
	for key, value := range schemaMap {
		if strings.HasPrefix(key, "x-") {
			spec.Extensions[key] = value
		}
	}

	return spec, nil
}

// Helper parsing functions
func parseInfo(info map[string]interface{}) Info {
	result := Info{
		Extensions: make(map[string]interface{}),
	}

	if v, ok := info["title"].(string); ok {
		result.Title = v
	}
	if v, ok := info["description"].(string); ok {
		result.Description = v
	}
	if v, ok := info["version"].(string); ok {
		result.Version = v
	}

	// Parse extensions
	for key, value := range info {
		if strings.HasPrefix(key, "x-") {
			result.Extensions[key] = value
		}
	}

	return result
}

func parseServers(servers []interface{}) []Server {
	result := make([]Server, 0, len(servers))
	for _, s := range servers {
		if serverMap, ok := s.(map[string]interface{}); ok {
			server := Server{}
			if url, ok := serverMap["url"].(string); ok {
				server.URL = url
			}
			if desc, ok := serverMap["description"].(string); ok {
				server.Description = desc
			}
			result = append(result, server)
		}
	}
	return result
}

func parsePaths(paths map[string]interface{}) map[string]PathItem {
	result := make(map[string]PathItem)
	for path, item := range paths {
		if pathMap, ok := item.(map[string]interface{}); ok {
			result[path] = parsePathItem(pathMap)
		}
	}
	return result
}

func parsePathItem(item map[string]interface{}) PathItem {
	pathItem := PathItem{
		Extensions: make(map[string]interface{}),
	}

	// Parse operations
	if op, ok := item["get"].(map[string]interface{}); ok {
		pathItem.Get = parseOperation(op)
	}
	if op, ok := item["post"].(map[string]interface{}); ok {
		pathItem.Post = parseOperation(op)
	}
	if op, ok := item["put"].(map[string]interface{}); ok {
		pathItem.Put = parseOperation(op)
	}
	if op, ok := item["delete"].(map[string]interface{}); ok {
		pathItem.Delete = parseOperation(op)
	}
	if op, ok := item["patch"].(map[string]interface{}); ok {
		pathItem.Patch = parseOperation(op)
	}

	return pathItem
}

func parseOperation(op map[string]interface{}) *Operation {
	operation := &Operation{
		Extensions: make(map[string]interface{}),
	}

	if v, ok := op["operationId"].(string); ok {
		operation.OperationID = v
	}
	if v, ok := op["summary"].(string); ok {
		operation.Summary = v
	}
	if v, ok := op["description"].(string); ok {
		operation.Description = v
	}

	// Parse tags
	if tags, ok := op["tags"].([]interface{}); ok {
		operation.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				operation.Tags = append(operation.Tags, tagStr)
			}
		}
	}

	return operation
}

func parseComponents(components map[string]interface{}) *Components {
	result := &Components{
		Schemas:         make(map[string]map[string]interface{}),
		Responses:       make(map[string]Response),
		Parameters:      make(map[string]Parameter),
		RequestBodies:   make(map[string]RequestBody),
		SecuritySchemes: make(map[string]SecurityScheme),
	}

	// Parse schemas
	if schemas, ok := components["schemas"].(map[string]interface{}); ok {
		for name, schema := range schemas {
			if schemaMap, ok := schema.(map[string]interface{}); ok {
				result.Schemas[name] = schemaMap
			}
		}
	}

	// Parse security schemes
	if securitySchemes, ok := components["securitySchemes"].(map[string]interface{}); ok {
		for name, scheme := range securitySchemes {
			if schemeMap, ok := scheme.(map[string]interface{}); ok {
				sec := SecurityScheme{}
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
				result.SecuritySchemes[name] = sec
			}
		}
	}

	return result
}

func parseTags(tags []interface{}) []Tag {
	result := make([]Tag, 0, len(tags))
	for _, t := range tags {
		if tagMap, ok := t.(map[string]interface{}); ok {
			tag := Tag{Extensions: make(map[string]interface{})}
			if name, ok := tagMap["name"].(string); ok {
				tag.Name = name
			}
			if desc, ok := tagMap["description"].(string); ok {
				tag.Description = desc
			}
			result = append(result, tag)
		}
	}
	return result
}

// ApplyRouting applies routing configuration to paths
func ApplyRouting(paths map[string]PathItem, manifest *farp.SchemaManifest) map[string]PathItem {
	result := make(map[string]PathItem)

	for path, item := range paths {
		newPath := applyMountStrategy(path, manifest)
		result[newPath] = item
	}

	return result
}

func applyMountStrategy(path string, manifest *farp.SchemaManifest) string {
	routing := manifest.Routing

	switch routing.Strategy {
	case farp.MountStrategyRoot:
		return path

	case farp.MountStrategyInstance:
		return fmt.Sprintf("/%s%s", manifest.InstanceID, path)

	case farp.MountStrategyService:
		return fmt.Sprintf("/%s%s", manifest.ServiceName, path)

	case farp.MountStrategyVersioned:
		return fmt.Sprintf("/%s/%s%s", manifest.ServiceName, manifest.ServiceVersion, path)

	case farp.MountStrategyCustom:
		if routing.BasePath != "" {
			return routing.BasePath + path
		}
		return path

	case farp.MountStrategySubdomain:
		// Subdomain routing doesn't change path
		return path

	default:
		// Default to instance strategy
		return fmt.Sprintf("/%s%s", manifest.InstanceID, path)
	}
}

// PrefixComponentNames adds prefix to component schema names
func PrefixComponentNames(components *Components, prefix string) *Components {
	if components == nil || prefix == "" {
		return components
	}

	result := &Components{
		Schemas:         make(map[string]map[string]interface{}),
		Responses:       make(map[string]Response),
		Parameters:      make(map[string]Parameter),
		RequestBodies:   make(map[string]RequestBody),
		SecuritySchemes: make(map[string]SecurityScheme),
	}

	// Prefix schema names
	for name, schema := range components.Schemas {
		prefixedName := prefix + "_" + name
		result.Schemas[prefixedName] = schema
	}

	// Prefix other components
	for name, response := range components.Responses {
		result.Responses[prefix+"_"+name] = response
	}

	for name, param := range components.Parameters {
		result.Parameters[prefix+"_"+name] = param
	}

	for name, body := range components.RequestBodies {
		result.RequestBodies[prefix+"_"+name] = body
	}

	// Security schemes typically don't need prefixing (shared across services)
	for name, scheme := range components.SecuritySchemes {
		result.SecuritySchemes[name] = scheme
	}

	return result
}

// PrefixTags adds prefix to operation tags
func PrefixTags(tags []string, prefix string) []string {
	if prefix == "" {
		return tags
	}

	result := make([]string, len(tags))
	for i, tag := range tags {
		result[i] = prefix + "_" + tag
	}
	return result
}

// SortTags sorts tags alphabetically
func SortTags(tags []Tag) []Tag {
	sorted := make([]Tag, len(tags))
	copy(sorted, tags)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})
	return sorted
}

