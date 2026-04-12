package merger

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/xraph/farp"
)

// OpenAPISpec represents a simplified OpenAPI 3.x specification.
type OpenAPISpec struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Security   []map[string][]string `json:"security,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
	Extensions map[string]any        `json:"-"` // x-* extensions
}

// Info represents OpenAPI info object.
type Info struct {
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	Version        string         `json:"version"`
	TermsOfService string         `json:"termsOfService,omitempty"`
	Contact        *Contact       `json:"contact,omitempty"`
	License        *License       `json:"license,omitempty"`
	Extensions     map[string]any `json:"-"`
}

// Contact represents contact information.
type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License represents license information.
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server represents an OpenAPI server.
type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable represents a server variable.
type ServerVariable struct {
	Default     string   `json:"default"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description,omitempty"`
}

// PathItem represents an OpenAPI path item.
type PathItem struct {
	Summary     string         `json:"summary,omitempty"`
	Description string         `json:"description,omitempty"`
	Get         *Operation     `json:"get,omitempty"`
	Put         *Operation     `json:"put,omitempty"`
	Post        *Operation     `json:"post,omitempty"`
	Delete      *Operation     `json:"delete,omitempty"`
	Options     *Operation     `json:"options,omitempty"`
	Head        *Operation     `json:"head,omitempty"`
	Patch       *Operation     `json:"patch,omitempty"`
	Trace       *Operation     `json:"trace,omitempty"`
	Parameters  []Parameter    `json:"parameters,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// Operation represents an OpenAPI operation.
type Operation struct {
	OperationID string                `json:"operationId,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses,omitempty"`
	Security    []map[string][]string `json:"security,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
	Extensions  map[string]any        `json:"-"`
}

// Parameter represents an OpenAPI parameter.
type Parameter struct {
	Name        string         `json:"name"`
	In          string         `json:"in"` // query, header, path, cookie
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
	Example     any            `json:"example,omitempty"`
}

// RequestBody represents an OpenAPI request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content"`
	Required    bool                 `json:"required,omitempty"`
	Extensions  map[string]any       `json:"-"`
}

// Response represents an OpenAPI response.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Headers     map[string]Header    `json:"headers,omitempty"`
	Extensions  map[string]any       `json:"-"`
}

// MediaType represents a media type object.
type MediaType struct {
	Schema   map[string]any     `json:"schema,omitempty"`
	Example  any                `json:"example,omitempty"`
	Examples map[string]Example `json:"examples,omitempty"`
}

// Example represents an example object.
type Example struct {
	Summary       string `json:"summary,omitempty"`
	Description   string `json:"description,omitempty"`
	Value         any    `json:"value,omitempty"`
	ExternalValue string `json:"externalValue,omitempty"`
}

// Header represents a header object.
type Header struct {
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// Components represents OpenAPI components.
type Components struct {
	Schemas         map[string]map[string]any `json:"schemas,omitempty"`
	Responses       map[string]Response       `json:"responses,omitempty"`
	Parameters      map[string]Parameter      `json:"parameters,omitempty"`
	RequestBodies   map[string]RequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]Header         `json:"headers,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// SecurityScheme represents a security scheme.
type SecurityScheme struct {
	Type             string `json:"type"` // apiKey, http, oauth2, openIdConnect
	Description      string `json:"description,omitempty"`
	Name             string `json:"name,omitempty"`             // For apiKey
	In               string `json:"in,omitempty"`               // For apiKey: query, header, cookie
	Scheme           string `json:"scheme,omitempty"`           // For http: bearer, basic
	BearerFormat     string `json:"bearerFormat,omitempty"`     // For http bearer
	OpenIdConnectURL string `json:"openIdConnectUrl,omitempty"` // For openIdConnect
}

// Tag represents an OpenAPI tag.
type Tag struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Extensions  map[string]any `json:"-"`
}

// ServiceSchema wraps a schema with its service context.
type ServiceSchema struct {
	Manifest *farp.SchemaManifest
	Schema   any // Raw OpenAPI schema (map[string]interface{})
	Parsed   *OpenAPISpec
}

// ParseOpenAPISchema parses a raw OpenAPI schema into structured format.
func ParseOpenAPISchema(raw any) (*OpenAPISpec, error) {
	schemaMap, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("schema must be a map")
	}

	spec := &OpenAPISpec{
		Paths:      make(map[string]PathItem),
		Extensions: make(map[string]any),
	}

	// Parse OpenAPI version
	if v, ok := schemaMap["openapi"].(string); ok {
		spec.OpenAPI = v
	} else {
		return nil, errors.New("missing openapi version")
	}

	// Parse info
	if info, ok := schemaMap["info"].(map[string]any); ok {
		spec.Info = parseInfo(info)
	}

	// Parse servers
	if servers, ok := schemaMap["servers"].([]any); ok {
		spec.Servers = parseServers(servers)
	}

	// Parse paths
	if paths, ok := schemaMap["paths"].(map[string]any); ok {
		spec.Paths = parsePaths(paths)
	}

	// Parse components
	if components, ok := schemaMap["components"].(map[string]any); ok {
		spec.Components = parseComponents(components)
	}

	// Parse tags
	if tags, ok := schemaMap["tags"].([]any); ok {
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

// Helper parsing functions.
func parseInfo(info map[string]any) Info {
	result := Info{
		Extensions: make(map[string]any),
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

func parseServers(servers []any) []Server {
	result := make([]Server, 0, len(servers))

	for _, s := range servers {
		if serverMap, ok := s.(map[string]any); ok {
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

func parsePaths(paths map[string]any) map[string]PathItem {
	result := make(map[string]PathItem)

	for path, item := range paths {
		if pathMap, ok := item.(map[string]any); ok {
			result[path] = parsePathItem(pathMap)
		}
	}

	return result
}

func parsePathItem(item map[string]any) PathItem {
	pathItem := PathItem{
		Extensions: make(map[string]any),
	}

	if v, ok := item["summary"].(string); ok {
		pathItem.Summary = v
	}

	if v, ok := item["description"].(string); ok {
		pathItem.Description = v
	}

	// Parse operations
	if op, ok := item["get"].(map[string]any); ok {
		pathItem.Get = parseOperation(op)
	}

	if op, ok := item["post"].(map[string]any); ok {
		pathItem.Post = parseOperation(op)
	}

	if op, ok := item["put"].(map[string]any); ok {
		pathItem.Put = parseOperation(op)
	}

	if op, ok := item["delete"].(map[string]any); ok {
		pathItem.Delete = parseOperation(op)
	}

	if op, ok := item["patch"].(map[string]any); ok {
		pathItem.Patch = parseOperation(op)
	}

	if op, ok := item["options"].(map[string]any); ok {
		pathItem.Options = parseOperation(op)
	}

	if op, ok := item["head"].(map[string]any); ok {
		pathItem.Head = parseOperation(op)
	}

	if op, ok := item["trace"].(map[string]any); ok {
		pathItem.Trace = parseOperation(op)
	}

	// Parse path-level parameters.
	if params, ok := item["parameters"].([]any); ok {
		pathItem.Parameters = parseParameters(params)
	}

	// Parse extensions.
	for key, value := range item {
		if strings.HasPrefix(key, "x-") {
			pathItem.Extensions[key] = value
		}
	}

	return pathItem
}

func parseOperation(op map[string]any) *Operation {
	operation := &Operation{
		Extensions: make(map[string]any),
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
	if tags, ok := op["tags"].([]any); ok {
		operation.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				operation.Tags = append(operation.Tags, tagStr)
			}
		}
	}

	// Parse parameters.
	if params, ok := op["parameters"].([]any); ok {
		operation.Parameters = parseParameters(params)
	}

	// Parse requestBody.
	if body, ok := op["requestBody"].(map[string]any); ok {
		operation.RequestBody = parseRequestBody(body)
	}

	// Parse responses.
	if responses, ok := op["responses"].(map[string]any); ok {
		operation.Responses = parseResponses(responses)
	}

	// Parse security.
	if security, ok := op["security"].([]any); ok {
		operation.Security = parseSecurity(security)
	}

	// Parse deprecated.
	if deprecated, ok := op["deprecated"].(bool); ok {
		operation.Deprecated = deprecated
	}

	// Parse extensions.
	for key, value := range op {
		if strings.HasPrefix(key, "x-") {
			operation.Extensions[key] = value
		}
	}

	return operation
}

func parseComponents(components map[string]any) *Components {
	result := &Components{
		Schemas:         make(map[string]map[string]any),
		Responses:       make(map[string]Response),
		Parameters:      make(map[string]Parameter),
		RequestBodies:   make(map[string]RequestBody),
		Headers:         make(map[string]Header),
		SecuritySchemes: make(map[string]SecurityScheme),
	}

	// Parse schemas
	if schemas, ok := components["schemas"].(map[string]any); ok {
		for name, schema := range schemas {
			if schemaMap, ok := schema.(map[string]any); ok {
				result.Schemas[name] = schemaMap
			}
		}
	}

	// Parse responses
	if responses, ok := components["responses"].(map[string]any); ok {
		for name, resp := range responses {
			if respMap, ok := resp.(map[string]any); ok {
				r := Response{}
				if desc, ok := respMap["description"].(string); ok {
					r.Description = desc
				}

				if content, ok := respMap["content"].(map[string]any); ok {
					r.Content = parseMediaTypes(content)
				}

				if headers, ok := respMap["headers"].(map[string]any); ok {
					r.Headers = parseHeaders(headers)
				}

				result.Responses[name] = r
			}
		}
	}

	// Parse parameters
	if parameters, ok := components["parameters"].(map[string]any); ok {
		for name, param := range parameters {
			if paramMap, ok := param.(map[string]any); ok {
				p := Parameter{}
				if v, ok := paramMap["name"].(string); ok {
					p.Name = v
				}

				if v, ok := paramMap["in"].(string); ok {
					p.In = v
				}

				if v, ok := paramMap["description"].(string); ok {
					p.Description = v
				}

				if v, ok := paramMap["required"].(bool); ok {
					p.Required = v
				}

				if v, ok := paramMap["schema"].(map[string]any); ok {
					p.Schema = v
				}

				if v, ok := paramMap["example"]; ok {
					p.Example = v
				}

				result.Parameters[name] = p
			}
		}
	}

	// Parse requestBodies
	if requestBodies, ok := components["requestBodies"].(map[string]any); ok {
		for name, body := range requestBodies {
			if bodyMap, ok := body.(map[string]any); ok {
				rb := RequestBody{}
				if desc, ok := bodyMap["description"].(string); ok {
					rb.Description = desc
				}

				if req, ok := bodyMap["required"].(bool); ok {
					rb.Required = req
				}

				if content, ok := bodyMap["content"].(map[string]any); ok {
					rb.Content = parseMediaTypes(content)
				}

				result.RequestBodies[name] = rb
			}
		}
	}

	// Parse headers
	if headers, ok := components["headers"].(map[string]any); ok {
		result.Headers = parseHeaders(headers)
	}

	// Parse security schemes
	if securitySchemes, ok := components["securitySchemes"].(map[string]any); ok {
		for name, scheme := range securitySchemes {
			if schemeMap, ok := scheme.(map[string]any); ok {
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

func parseTags(tags []any) []Tag {
	result := make([]Tag, 0, len(tags))

	for _, t := range tags {
		if tagMap, ok := t.(map[string]any); ok {
			tag := Tag{Extensions: make(map[string]any)}
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

// parseParameters parses an array of OpenAPI parameter objects.
func parseParameters(params []any) []Parameter {
	result := make([]Parameter, 0, len(params))
	for _, p := range params {
		paramMap, ok := p.(map[string]any)
		if !ok {
			continue
		}

		param := Parameter{}
		if v, ok := paramMap["name"].(string); ok {
			param.Name = v
		}

		if v, ok := paramMap["in"].(string); ok {
			param.In = v
		}

		if v, ok := paramMap["description"].(string); ok {
			param.Description = v
		}

		if v, ok := paramMap["required"].(bool); ok {
			param.Required = v
		}

		if v, ok := paramMap["schema"].(map[string]any); ok {
			param.Schema = v
		}

		if v, ok := paramMap["example"]; ok {
			param.Example = v
		}

		result = append(result, param)
	}

	return result
}

// parseRequestBody parses an OpenAPI requestBody object.
func parseRequestBody(body map[string]any) *RequestBody {
	rb := &RequestBody{
		Extensions: make(map[string]any),
	}
	if desc, ok := body["description"].(string); ok {
		rb.Description = desc
	}

	if req, ok := body["required"].(bool); ok {
		rb.Required = req
	}

	if content, ok := body["content"].(map[string]any); ok {
		rb.Content = parseMediaTypes(content)
	}

	for key, value := range body {
		if strings.HasPrefix(key, "x-") {
			rb.Extensions[key] = value
		}
	}

	return rb
}

// parseMediaTypes parses a map of media type objects.
func parseMediaTypes(content map[string]any) map[string]MediaType {
	result := make(map[string]MediaType, len(content))
	for mediaType, mt := range content {
		mtMap, ok := mt.(map[string]any)
		if !ok {
			continue
		}

		m := MediaType{}
		if schema, ok := mtMap["schema"].(map[string]any); ok {
			m.Schema = schema
		}

		if example, ok := mtMap["example"]; ok {
			m.Example = example
		}

		if examples, ok := mtMap["examples"].(map[string]any); ok {
			m.Examples = make(map[string]Example, len(examples))
			for name, ex := range examples {
				if exMap, ok := ex.(map[string]any); ok {
					e := Example{}
					if v, ok := exMap["summary"].(string); ok {
						e.Summary = v
					}

					if v, ok := exMap["description"].(string); ok {
						e.Description = v
					}

					if v, ok := exMap["value"]; ok {
						e.Value = v
					}

					if v, ok := exMap["externalValue"].(string); ok {
						e.ExternalValue = v
					}

					m.Examples[name] = e
				}
			}
		}

		result[mediaType] = m
	}

	return result
}

// parseResponses parses a map of OpenAPI response objects.
func parseResponses(responses map[string]any) map[string]Response {
	result := make(map[string]Response, len(responses))
	for status, resp := range responses {
		respMap, ok := resp.(map[string]any)
		if !ok {
			continue
		}

		r := Response{}
		if desc, ok := respMap["description"].(string); ok {
			r.Description = desc
		}

		if content, ok := respMap["content"].(map[string]any); ok {
			r.Content = parseMediaTypes(content)
		}

		if headers, ok := respMap["headers"].(map[string]any); ok {
			r.Headers = parseHeaders(headers)
		}

		result[status] = r
	}

	return result
}

// parseHeaders parses a map of OpenAPI header objects.
func parseHeaders(headers map[string]any) map[string]Header {
	result := make(map[string]Header, len(headers))
	for name, h := range headers {
		hMap, ok := h.(map[string]any)
		if !ok {
			continue
		}

		header := Header{}
		if desc, ok := hMap["description"].(string); ok {
			header.Description = desc
		}

		if schema, ok := hMap["schema"].(map[string]any); ok {
			header.Schema = schema
		}

		result[name] = header
	}

	return result
}

// parseSecurity parses an array of OpenAPI security requirement objects.
func parseSecurity(security []any) []map[string][]string {
	result := make([]map[string][]string, 0, len(security))
	for _, s := range security {
		sMap, ok := s.(map[string]any)
		if !ok {
			continue
		}

		req := make(map[string][]string, len(sMap))
		for name, scopes := range sMap {
			scopeArr, ok := scopes.([]any)
			if !ok {
				req[name] = []string{}

				continue
			}

			scopeStrs := make([]string, 0, len(scopeArr))
			for _, scope := range scopeArr {
				if str, ok := scope.(string); ok {
					scopeStrs = append(scopeStrs, str)
				}
			}

			req[name] = scopeStrs
		}

		result = append(result, req)
	}

	return result
}

// ApplyRouting applies routing configuration to paths.
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
		return fmt.Sprintf("/%s%s", strings.ToLower(manifest.InstanceID), path)

	case farp.MountStrategyService:
		return fmt.Sprintf("/%s%s", strings.ToLower(manifest.ServiceName), path)

	case farp.MountStrategyVersioned:
		return fmt.Sprintf("/%s/%s%s", strings.ToLower(manifest.ServiceName), strings.ToLower(manifest.ServiceVersion), path)

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

// PrefixComponentNames adds prefix to component schema names.
func PrefixComponentNames(components *Components, prefix string) *Components {
	if components == nil || prefix == "" {
		return components
	}

	result := &Components{
		Schemas:         make(map[string]map[string]any),
		Responses:       make(map[string]Response),
		Parameters:      make(map[string]Parameter),
		RequestBodies:   make(map[string]RequestBody),
		Headers:         make(map[string]Header),
		SecuritySchemes: make(map[string]SecurityScheme),
	}

	// Prefix schema names and rewrite $ref strings within schemas
	for name, schema := range components.Schemas {
		prefixedName := prefix + "_" + name

		rewritten := RewriteRefs(schema, prefix)
		if rewrittenMap, ok := rewritten.(map[string]any); ok {
			result.Schemas[prefixedName] = rewrittenMap
		} else {
			result.Schemas[prefixedName] = schema
		}
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

	for name, header := range components.Headers {
		result.Headers[prefix+"_"+name] = header
	}

	// Security schemes typically don't need prefixing (shared across services)
	maps.Copy(result.SecuritySchemes, components.SecuritySchemes)

	return result
}

// RewriteRefs recursively walks a schema value and rewrites $ref strings
// that point to local components. After component names are prefixed,
// references like "#/components/schemas/Foo" must become "#/components/schemas/prefix_Foo".
func RewriteRefs(value any, prefix string) any {
	if prefix == "" {
		return value
	}

	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			if key == "$ref" {
				if refStr, ok := val.(string); ok {
					result[key] = rewriteRefString(refStr, prefix)
				} else {
					result[key] = val
				}
			} else {
				result[key] = RewriteRefs(val, prefix)
			}
		}

		return result

	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = RewriteRefs(item, prefix)
		}

		return result

	default:
		return value
	}
}

// rewriteRefString rewrites a single $ref string if it points to a local component.
// "#/components/schemas/Foo" → "#/components/schemas/prefix_Foo".
func rewriteRefString(ref, prefix string) string {
	componentPrefixes := []string{
		"#/components/schemas/",
		"#/components/responses/",
		"#/components/parameters/",
		"#/components/requestBodies/",
		"#/components/headers/",
	}

	for _, cp := range componentPrefixes {
		if name, ok := strings.CutPrefix(ref, cp); ok {
			return cp + prefix + "_" + name
		}
	}

	return ref
}

// PrefixTags adds prefix to operation tags.
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

// SortTags sorts tags alphabetically.
func SortTags(tags []Tag) []Tag {
	sorted := make([]Tag, len(tags))
	copy(sorted, tags)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	return sorted
}
