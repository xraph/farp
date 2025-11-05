package farp

import "fmt"

// SchemaType represents supported schema/protocol types
type SchemaType string

const (
	// SchemaTypeOpenAPI represents OpenAPI/Swagger specifications
	SchemaTypeOpenAPI SchemaType = "openapi"

	// SchemaTypeAsyncAPI represents AsyncAPI specifications
	SchemaTypeAsyncAPI SchemaType = "asyncapi"

	// SchemaTypeGRPC represents gRPC protocol buffer definitions
	SchemaTypeGRPC SchemaType = "grpc"

	// SchemaTypeGraphQL represents GraphQL Schema Definition Language
	SchemaTypeGraphQL SchemaType = "graphql"

	// SchemaTypeORPC represents oRPC (OpenAPI-based RPC) specifications
	SchemaTypeORPC SchemaType = "orpc"

	// SchemaTypeThrift represents Apache Thrift IDL (future support)
	SchemaTypeThrift SchemaType = "thrift"

	// SchemaTypeAvro represents Apache Avro schemas (future support)
	SchemaTypeAvro SchemaType = "avro"

	// SchemaTypeCustom represents custom/proprietary schema types
	SchemaTypeCustom SchemaType = "custom"
)

// IsValid checks if the schema type is valid
func (st SchemaType) IsValid() bool {
	switch st {
	case SchemaTypeOpenAPI, SchemaTypeAsyncAPI, SchemaTypeGRPC,
		SchemaTypeGraphQL, SchemaTypeORPC, SchemaTypeThrift, SchemaTypeAvro, SchemaTypeCustom:
		return true
	default:
		return false
	}
}

// String returns the string representation of the schema type
func (st SchemaType) String() string {
	return string(st)
}

// LocationType represents how schemas can be retrieved
type LocationType string

const (
	// LocationTypeHTTP means fetch schema via HTTP GET
	LocationTypeHTTP LocationType = "http"

	// LocationTypeRegistry means fetch schema from backend KV store
	LocationTypeRegistry LocationType = "registry"

	// LocationTypeInline means schema is embedded in the manifest
	LocationTypeInline LocationType = "inline"
)

// IsValid checks if the location type is valid
func (lt LocationType) IsValid() bool {
	switch lt {
	case LocationTypeHTTP, LocationTypeRegistry, LocationTypeInline:
		return true
	default:
		return false
	}
}

// String returns the string representation of the location type
func (lt LocationType) String() string {
	return string(lt)
}

// SchemaManifest describes all API contracts for a service instance
type SchemaManifest struct {
	// Version of the FARP protocol (semver)
	Version string `json:"version"`

	// Service identity
	ServiceName    string `json:"service_name"`
	ServiceVersion string `json:"service_version"`
	InstanceID     string `json:"instance_id"`

	// Instance metadata
	Instance *InstanceMetadata `json:"instance,omitempty"`

	// Schemas exposed by this instance
	Schemas []SchemaDescriptor `json:"schemas"`

	// Capabilities/protocols supported (e.g., ["rest", "grpc", "websocket"])
	Capabilities []string `json:"capabilities"`

	// Endpoints for introspection and health
	Endpoints SchemaEndpoints `json:"endpoints"`

	// Routing configuration for gateway federation
	Routing RoutingConfig `json:"routing"`

	// Authentication configuration
	Auth AuthConfig `json:"auth,omitempty"`

	// Webhook for bidirectional communication
	Webhook WebhookConfig `json:"webhook,omitempty"`

	// Service operational hints (non-binding)
	Hints *ServiceHints `json:"hints,omitempty"`

	// Change tracking
	UpdatedAt int64  `json:"updated_at"` // Unix timestamp
	Checksum  string `json:"checksum"`   // SHA256 of all schemas combined
}

// SchemaDescriptor describes a single API schema/contract
type SchemaDescriptor struct {
	// Type of schema (openapi, asyncapi, grpc, graphql, etc.)
	Type SchemaType `json:"type"`

	// Specification version (e.g., "3.1.0" for OpenAPI, "3.0.0" for AsyncAPI)
	SpecVersion string `json:"spec_version"`

	// How to retrieve the schema
	Location SchemaLocation `json:"location"`

	// Content type (e.g., "application/json", "application/x-protobuf")
	ContentType string `json:"content_type"`

	// Optional: Inline schema for small schemas (< 100KB recommended)
	InlineSchema interface{} `json:"inline_schema,omitempty"`

	// Integrity validation
	Hash string `json:"hash"` // SHA256 of schema content
	Size int64  `json:"size"` // Size in bytes

	// Schema compatibility metadata
	Compatibility *SchemaCompatibility `json:"compatibility,omitempty"`

	// Protocol-specific metadata
	Metadata *ProtocolMetadata `json:"metadata,omitempty"`
}

// SchemaLocation describes where and how to fetch a schema
type SchemaLocation struct {
	// Location type (http, registry, inline)
	Type LocationType `json:"type"`

	// HTTP URL (if Type == HTTP)
	// Example: "http://user-service:8080/openapi.json"
	URL string `json:"url,omitempty"`

	// Registry path in backend KV store (if Type == Registry)
	// Example: "/schemas/user-service/v1/openapi"
	RegistryPath string `json:"registry_path,omitempty"`

	// HTTP headers for authentication (if Type == HTTP)
	// Example: {"Authorization": "Bearer token"}
	Headers map[string]string `json:"headers,omitempty"`
}

// Validate checks if the schema location is valid
func (sl *SchemaLocation) Validate() error {
	if !sl.Type.IsValid() {
		return fmt.Errorf("%w: invalid location type: %s", ErrInvalidLocation, sl.Type)
	}

	switch sl.Type {
	case LocationTypeHTTP:
		if sl.URL == "" {
			return fmt.Errorf("%w: URL required for HTTP location", ErrInvalidLocation)
		}
	case LocationTypeRegistry:
		if sl.RegistryPath == "" {
			return fmt.Errorf("%w: registry path required for registry location", ErrInvalidLocation)
		}
	case LocationTypeInline:
		// No additional validation needed for inline
	}

	return nil
}

// SchemaEndpoints provides URLs for service introspection
type SchemaEndpoints struct {
	// Health check endpoint (required)
	// Example: "/health" or "/healthz"
	Health string `json:"health"`

	// Prometheus metrics endpoint (optional)
	// Example: "/metrics"
	Metrics string `json:"metrics,omitempty"`

	// OpenAPI spec endpoint (optional)
	// Example: "/openapi.json"
	OpenAPI string `json:"openapi,omitempty"`

	// AsyncAPI spec endpoint (optional)
	// Example: "/asyncapi.json"
	AsyncAPI string `json:"asyncapi,omitempty"`

	// Whether gRPC server reflection is enabled
	GRPCReflection bool `json:"grpc_reflection,omitempty"`

	// GraphQL introspection endpoint (optional)
	// Example: "/graphql"
	GraphQL string `json:"graphql,omitempty"`
}

// Capability represents a protocol capability
type Capability string

const (
	// CapabilityREST indicates REST API support
	CapabilityREST Capability = "rest"

	// CapabilityGRPC indicates gRPC support
	CapabilityGRPC Capability = "grpc"

	// CapabilityWebSocket indicates WebSocket support
	CapabilityWebSocket Capability = "websocket"

	// CapabilitySSE indicates Server-Sent Events support
	CapabilitySSE Capability = "sse"

	// CapabilityGraphQL indicates GraphQL support
	CapabilityGraphQL Capability = "graphql"

	// CapabilityMQTT indicates MQTT support
	CapabilityMQTT Capability = "mqtt"

	// CapabilityAMQP indicates AMQP support
	CapabilityAMQP Capability = "amqp"
)

// String returns the string representation of the capability
func (c Capability) String() string {
	return string(c)
}

// InstanceMetadata provides information about a service instance
type InstanceMetadata struct {
	// Instance address (host:port)
	Address string `json:"address"`

	// Instance region/zone/datacenter
	Region string `json:"region,omitempty"`
	Zone   string `json:"zone,omitempty"`

	// Instance labels for selection
	Labels map[string]string `json:"labels,omitempty"`

	// Instance weight for load balancing (0-100, default: 100)
	Weight int `json:"weight,omitempty"`

	// Instance status
	Status InstanceStatus `json:"status"`

	// Instance role in deployment
	Role InstanceRole `json:"role,omitempty"`

	// Canary/blue-green deployment metadata
	Deployment *DeploymentMetadata `json:"deployment,omitempty"`

	// Instance start time
	StartedAt int64 `json:"started_at"` // Unix timestamp

	// Expected schema checksum (for validation)
	ExpectedSchemaChecksum string `json:"expected_schema_checksum,omitempty"`
}

// InstanceStatus represents the status of a service instance
type InstanceStatus string

const (
	// InstanceStatusStarting indicates the instance is starting
	InstanceStatusStarting InstanceStatus = "starting"

	// InstanceStatusHealthy indicates the instance is healthy
	InstanceStatusHealthy InstanceStatus = "healthy"

	// InstanceStatusDegraded indicates the instance is degraded
	InstanceStatusDegraded InstanceStatus = "degraded"

	// InstanceStatusUnhealthy indicates the instance is unhealthy
	InstanceStatusUnhealthy InstanceStatus = "unhealthy"

	// InstanceStatusDraining indicates the instance is draining connections
	InstanceStatusDraining InstanceStatus = "draining"

	// InstanceStatusStopping indicates the instance is stopping
	InstanceStatusStopping InstanceStatus = "stopping"
)

// String returns the string representation of the instance status
func (is InstanceStatus) String() string {
	return string(is)
}

// InstanceRole represents the role of an instance in a deployment
type InstanceRole string

const (
	// InstanceRolePrimary indicates the instance is primary/production
	InstanceRolePrimary InstanceRole = "primary"

	// InstanceRoleCanary indicates the instance is a canary deployment
	InstanceRoleCanary InstanceRole = "canary"

	// InstanceRoleBlue indicates the instance is part of blue deployment
	InstanceRoleBlue InstanceRole = "blue"

	// InstanceRoleGreen indicates the instance is part of green deployment
	InstanceRoleGreen InstanceRole = "green"

	// InstanceRoleShadow indicates the instance is a shadow deployment
	InstanceRoleShadow InstanceRole = "shadow"
)

// String returns the string representation of the instance role
func (ir InstanceRole) String() string {
	return string(ir)
}

// DeploymentMetadata provides information about a deployment
type DeploymentMetadata struct {
	// Deployment ID
	DeploymentID string `json:"deployment_id"`

	// Deployment strategy
	Strategy DeploymentStrategy `json:"strategy"`

	// Traffic percentage (0-100)
	TrafficPercent int `json:"traffic_percent,omitempty"`

	// Deployment stage
	Stage string `json:"stage,omitempty"` // "canary", "rollout", "stable"

	// Deployment time
	DeployedAt int64 `json:"deployed_at"`
}

// DeploymentStrategy represents the deployment strategy
type DeploymentStrategy string

const (
	// DeploymentStrategyRolling indicates a rolling update deployment
	DeploymentStrategyRolling DeploymentStrategy = "rolling"

	// DeploymentStrategyCanary indicates a canary deployment
	DeploymentStrategyCanary DeploymentStrategy = "canary"

	// DeploymentStrategyBlueGreen indicates a blue-green deployment
	DeploymentStrategyBlueGreen DeploymentStrategy = "blue_green"

	// DeploymentStrategyShadow indicates a shadow deployment
	DeploymentStrategyShadow DeploymentStrategy = "shadow"

	// DeploymentStrategyRecreate indicates a recreate deployment
	DeploymentStrategyRecreate DeploymentStrategy = "recreate"
)

// String returns the string representation of the deployment strategy
func (ds DeploymentStrategy) String() string {
	return string(ds)
}

// RoutingConfig provides gateway route mounting configuration
type RoutingConfig struct {
	// Mounting strategy
	Strategy MountStrategy `json:"strategy"`

	// Base path for mounting (used with path-based strategies)
	BasePath string `json:"base_path,omitempty"`

	// Subdomain for mounting (used with subdomain strategy)
	Subdomain string `json:"subdomain,omitempty"`

	// Path rewriting rules
	Rewrite []PathRewrite `json:"rewrite,omitempty"`

	// Strip prefix before forwarding to service
	StripPrefix bool `json:"strip_prefix,omitempty"`

	// Priority for conflict resolution (higher = higher priority)
	Priority int `json:"priority,omitempty"`

	// Tags for route grouping and filtering
	Tags []string `json:"tags,omitempty"`
}

// MountStrategy defines how routes are mounted in the gateway
type MountStrategy string

const (
	// MountStrategyRoot merges service routes to gateway root (no prefix)
	MountStrategyRoot MountStrategy = "root"

	// MountStrategyInstance mounts under /instance-id/* (default)
	MountStrategyInstance MountStrategy = "instance"

	// MountStrategyService mounts under /service-name/*
	MountStrategyService MountStrategy = "service"

	// MountStrategyVersioned mounts under /service-name/version/*
	MountStrategyVersioned MountStrategy = "versioned"

	// MountStrategyCustom mounts under custom base path
	MountStrategyCustom MountStrategy = "custom"

	// MountStrategySubdomain mounts on subdomain: service.gateway.com
	MountStrategySubdomain MountStrategy = "subdomain"
)

// String returns the string representation of the mount strategy
func (ms MountStrategy) String() string {
	return string(ms)
}

// IsValid checks if the mount strategy is valid
func (ms MountStrategy) IsValid() bool {
	switch ms {
	case MountStrategyRoot, MountStrategyInstance, MountStrategyService,
		MountStrategyVersioned, MountStrategyCustom, MountStrategySubdomain:
		return true
	default:
		return false
	}
}

// PathRewrite defines a path rewriting rule
type PathRewrite struct {
	// Pattern to match (regex)
	Pattern string `json:"pattern"`

	// Replacement string
	Replacement string `json:"replacement"`
}

// AuthConfig provides authentication and authorization configuration
type AuthConfig struct {
	// Authentication schemes supported by service
	Schemes []AuthScheme `json:"schemes"`

	// Required permissions/scopes
	RequiredScopes []string `json:"required_scopes,omitempty"`

	// Access control rules
	AccessControl []AccessRule `json:"access_control,omitempty"`

	// Token validation endpoint
	TokenValidationURL string `json:"token_validation_url,omitempty"`

	// Public (unauthenticated) routes
	PublicRoutes []string `json:"public_routes,omitempty"`
}

// AuthScheme describes an authentication scheme
type AuthScheme struct {
	// Scheme type
	Type AuthType `json:"type"`

	// Scheme configuration (varies by type)
	Config map[string]interface{} `json:"config,omitempty"`
}

// AuthType represents an authentication type
type AuthType string

const (
	// AuthTypeBearer indicates Bearer token authentication (JWT, opaque)
	AuthTypeBearer AuthType = "bearer"

	// AuthTypeAPIKey indicates API key authentication
	AuthTypeAPIKey AuthType = "apikey"

	// AuthTypeBasic indicates Basic authentication
	AuthTypeBasic AuthType = "basic"

	// AuthTypeMTLS indicates Mutual TLS authentication
	AuthTypeMTLS AuthType = "mtls"

	// AuthTypeOAuth2 indicates OAuth 2.0 authentication
	AuthTypeOAuth2 AuthType = "oauth2"

	// AuthTypeOIDC indicates OpenID Connect authentication
	AuthTypeOIDC AuthType = "oidc"

	// AuthTypeCustom indicates custom authentication scheme
	AuthTypeCustom AuthType = "custom"
)

// String returns the string representation of the auth type
func (at AuthType) String() string {
	return string(at)
}

// AccessRule defines an access control rule
type AccessRule struct {
	// Path pattern (glob or regex)
	Path string `json:"path"`

	// HTTP methods
	Methods []string `json:"methods"`

	// Required roles
	Roles []string `json:"roles,omitempty"`

	// Required permissions
	Permissions []string `json:"permissions,omitempty"`

	// Allow anonymous access
	AllowAnonymous bool `json:"allow_anonymous,omitempty"`
}

// WebhookConfig provides bidirectional communication configuration
type WebhookConfig struct {
	// Webhook endpoint on service for gateway notifications
	ServiceWebhook string `json:"service_webhook,omitempty"`

	// Webhook endpoint on gateway for service notifications
	GatewayWebhook string `json:"gateway_webhook,omitempty"`

	// Webhook secret for HMAC signature verification
	Secret string `json:"secret,omitempty"`

	// Event types service wants to receive from gateway
	SubscribeEvents []WebhookEventType `json:"subscribe_events,omitempty"`

	// Event types service will send to gateway
	PublishEvents []WebhookEventType `json:"publish_events,omitempty"`

	// Retry configuration
	Retry RetryConfig `json:"retry,omitempty"`

	// HTTP-based communication routes (alternative to push webhooks)
	HTTPRoutes *HTTPCommunicationRoutes `json:"http_routes,omitempty"`
}

// HTTPCommunicationRoutes defines HTTP communication routes
type HTTPCommunicationRoutes struct {
	// Service exposes these routes for gateway to call
	ServiceRoutes []CommunicationRoute `json:"service_routes,omitempty"`

	// Gateway exposes these routes for service to call
	GatewayRoutes []CommunicationRoute `json:"gateway_routes,omitempty"`

	// Polling configuration if HTTP polling is used
	Polling *PollingConfig `json:"polling,omitempty"`
}

// CommunicationRoute defines a communication route
type CommunicationRoute struct {
	// Route identifier
	ID string `json:"id"`

	// Route path
	Path string `json:"path"`

	// HTTP method
	Method string `json:"method"`

	// Route purpose/type
	Type CommunicationRouteType `json:"type"`

	// Description
	Description string `json:"description,omitempty"`

	// Request schema (JSON Schema, OpenAPI schema, etc.)
	RequestSchema interface{} `json:"request_schema,omitempty"`

	// Response schema
	ResponseSchema interface{} `json:"response_schema,omitempty"`

	// Authentication required
	AuthRequired bool `json:"auth_required"`

	// Idempotent operation
	Idempotent bool `json:"idempotent"`

	// Expected timeout
	Timeout string `json:"timeout,omitempty"`
}

// CommunicationRouteType defines the type of communication route
type CommunicationRouteType string

const (
	// RouteTypeControl indicates control plane operations
	RouteTypeControl CommunicationRouteType = "control"

	// RouteTypeAdmin indicates admin operations
	RouteTypeAdmin CommunicationRouteType = "admin"

	// RouteTypeManagement indicates management operations
	RouteTypeManagement CommunicationRouteType = "management"

	// RouteTypeLifecycleStart indicates lifecycle start hook
	RouteTypeLifecycleStart CommunicationRouteType = "lifecycle.start"

	// RouteTypeLifecycleStop indicates lifecycle stop hook
	RouteTypeLifecycleStop CommunicationRouteType = "lifecycle.stop"

	// RouteTypeLifecycleReload indicates lifecycle reload hook
	RouteTypeLifecycleReload CommunicationRouteType = "lifecycle.reload"

	// RouteTypeConfigUpdate indicates config update
	RouteTypeConfigUpdate CommunicationRouteType = "config.update"

	// RouteTypeConfigQuery indicates config query
	RouteTypeConfigQuery CommunicationRouteType = "config.query"

	// RouteTypeEventPoll indicates event polling
	RouteTypeEventPoll CommunicationRouteType = "event.poll"

	// RouteTypeEventAck indicates event acknowledgment
	RouteTypeEventAck CommunicationRouteType = "event.ack"

	// RouteTypeHealthCheck indicates health check
	RouteTypeHealthCheck CommunicationRouteType = "health.check"

	// RouteTypeStatusQuery indicates status query
	RouteTypeStatusQuery CommunicationRouteType = "status.query"

	// RouteTypeSchemaQuery indicates schema query
	RouteTypeSchemaQuery CommunicationRouteType = "schema.query"

	// RouteTypeSchemaValidate indicates schema validation
	RouteTypeSchemaValidate CommunicationRouteType = "schema.validate"

	// RouteTypeMetricsQuery indicates metrics query
	RouteTypeMetricsQuery CommunicationRouteType = "metrics.query"

	// RouteTypeTracingExport indicates tracing export
	RouteTypeTracingExport CommunicationRouteType = "tracing.export"

	// RouteTypeCustom indicates custom route type
	RouteTypeCustom CommunicationRouteType = "custom"
)

// String returns the string representation of the communication route type
func (crt CommunicationRouteType) String() string {
	return string(crt)
}

// PollingConfig defines polling configuration
type PollingConfig struct {
	// Polling interval
	Interval string `json:"interval"` // Duration string

	// Polling timeout
	Timeout string `json:"timeout,omitempty"`

	// Long polling support
	LongPolling bool `json:"long_polling,omitempty"`

	// Long polling timeout
	LongPollingTimeout string `json:"long_polling_timeout,omitempty"`
}

// WebhookEventType defines types of webhook events
type WebhookEventType string

const (
	// EventSchemaUpdated indicates schema was updated
	EventSchemaUpdated WebhookEventType = "schema.updated"

	// EventHealthChanged indicates health status changed
	EventHealthChanged WebhookEventType = "health.changed"

	// EventInstanceScaling indicates instance scaling event
	EventInstanceScaling WebhookEventType = "instance.scaling"

	// EventMaintenanceMode indicates maintenance mode event
	EventMaintenanceMode WebhookEventType = "maintenance.mode"

	// EventRateLimitChanged indicates rate limit changed
	EventRateLimitChanged WebhookEventType = "ratelimit.changed"

	// EventCircuitBreakerOpen indicates circuit breaker opened
	EventCircuitBreakerOpen WebhookEventType = "circuit.breaker.open"

	// EventCircuitBreakerClosed indicates circuit breaker closed
	EventCircuitBreakerClosed WebhookEventType = "circuit.breaker.closed"

	// EventConfigUpdated indicates config was updated
	EventConfigUpdated WebhookEventType = "config.updated"

	// EventTrafficShift indicates traffic shift event
	EventTrafficShift WebhookEventType = "traffic.shift"
)

// String returns the string representation of the webhook event type
func (wet WebhookEventType) String() string {
	return string(wet)
}

// RetryConfig defines retry configuration
type RetryConfig struct {
	// Maximum retry attempts
	MaxAttempts int `json:"max_attempts"`

	// Initial retry delay
	InitialDelay string `json:"initial_delay"` // Duration string

	// Maximum retry delay
	MaxDelay string `json:"max_delay"`

	// Backoff multiplier
	Multiplier float64 `json:"multiplier"`
}

// SchemaCompatibility provides schema compatibility metadata
type SchemaCompatibility struct {
	// Compatibility mode
	Mode CompatibilityMode `json:"mode"`

	// Previous schema versions (for lineage tracking)
	PreviousVersions []string `json:"previous_versions,omitempty"`

	// Breaking changes from previous version
	BreakingChanges []BreakingChange `json:"breaking_changes,omitempty"`

	// Deprecation notices
	Deprecations []Deprecation `json:"deprecations,omitempty"`
}

// CompatibilityMode defines schema compatibility guarantees
type CompatibilityMode string

const (
	// CompatibilityBackward indicates new schema can read data written by old schema
	CompatibilityBackward CompatibilityMode = "backward"

	// CompatibilityForward indicates old schema can read data written by new schema
	CompatibilityForward CompatibilityMode = "forward"

	// CompatibilityFull indicates both backward and forward compatible
	CompatibilityFull CompatibilityMode = "full"

	// CompatibilityNone indicates breaking changes, no compatibility guaranteed
	CompatibilityNone CompatibilityMode = "none"

	// CompatibilityBackwardTransitive indicates transitive backward compatibility across N versions
	CompatibilityBackwardTransitive CompatibilityMode = "backward_transitive"

	// CompatibilityForwardTransitive indicates transitive forward compatibility across N versions
	CompatibilityForwardTransitive CompatibilityMode = "forward_transitive"
)

// String returns the string representation of the compatibility mode
func (cm CompatibilityMode) String() string {
	return string(cm)
}

// BreakingChange describes a breaking change in a schema
type BreakingChange struct {
	// Type of breaking change
	Type ChangeType `json:"type"`

	// Path in schema (e.g., "/paths/users/get", "User.email")
	Path string `json:"path"`

	// Human-readable description
	Description string `json:"description"`

	// Severity level
	Severity ChangeSeverity `json:"severity"`

	// Migration instructions
	Migration string `json:"migration,omitempty"`
}

// ChangeType defines types of schema changes
type ChangeType string

const (
	// ChangeTypeFieldRemoved indicates a field was removed
	ChangeTypeFieldRemoved ChangeType = "field_removed"

	// ChangeTypeFieldTypeChanged indicates a field type was changed
	ChangeTypeFieldTypeChanged ChangeType = "field_type_changed"

	// ChangeTypeFieldRequired indicates a field became required
	ChangeTypeFieldRequired ChangeType = "field_required"

	// ChangeTypeEndpointRemoved indicates an endpoint was removed
	ChangeTypeEndpointRemoved ChangeType = "endpoint_removed"

	// ChangeTypeEndpointChanged indicates an endpoint was changed
	ChangeTypeEndpointChanged ChangeType = "endpoint_changed"

	// ChangeTypeEnumValueRemoved indicates an enum value was removed
	ChangeTypeEnumValueRemoved ChangeType = "enum_value_removed"

	// ChangeTypeMethodRemoved indicates a method was removed
	ChangeTypeMethodRemoved ChangeType = "method_removed"
)

// String returns the string representation of the change type
func (ct ChangeType) String() string {
	return string(ct)
}

// ChangeSeverity defines the severity of a schema change
type ChangeSeverity string

const (
	// SeverityCritical indicates immediate breakage
	SeverityCritical ChangeSeverity = "critical"

	// SeverityHigh indicates likely breakage
	SeverityHigh ChangeSeverity = "high"

	// SeverityMedium indicates possible breakage
	SeverityMedium ChangeSeverity = "medium"

	// SeverityLow indicates minimal risk
	SeverityLow ChangeSeverity = "low"
)

// String returns the string representation of the change severity
func (cs ChangeSeverity) String() string {
	return string(cs)
}

// Deprecation describes a deprecated schema element
type Deprecation struct {
	// Path in schema
	Path string `json:"path"`

	// Deprecation date (ISO 8601)
	DeprecatedAt string `json:"deprecated_at"`

	// Planned removal date (ISO 8601)
	RemovalDate string `json:"removal_date,omitempty"`

	// Replacement recommendation
	Replacement string `json:"replacement,omitempty"`

	// Migration guide
	Migration string `json:"migration,omitempty"`

	// Reason for deprecation
	Reason string `json:"reason,omitempty"`
}

// ServiceHints provides operational hints for the gateway
type ServiceHints struct {
	// Recommended timeout for operations
	RecommendedTimeout string `json:"recommended_timeout,omitempty"`

	// Expected latency profile
	ExpectedLatency *LatencyProfile `json:"expected_latency,omitempty"`

	// Scaling characteristics
	Scaling *ScalingProfile `json:"scaling,omitempty"`

	// Dependencies on other services
	Dependencies []ServiceDependency `json:"dependencies,omitempty"`
}

// LatencyProfile describes expected latency characteristics
type LatencyProfile struct {
	// Median latency
	P50 string `json:"p50,omitempty"` // e.g., "10ms"

	// 95th percentile
	P95 string `json:"p95,omitempty"` // e.g., "50ms"

	// 99th percentile
	P99 string `json:"p99,omitempty"` // e.g., "100ms"

	// 99.9th percentile
	P999 string `json:"p999,omitempty"` // e.g., "500ms"
}

// ScalingProfile describes scaling characteristics
type ScalingProfile struct {
	// Auto-scaling enabled
	AutoScale bool `json:"auto_scale"`

	// Minimum instances
	MinInstances int `json:"min_instances,omitempty"`

	// Maximum instances
	MaxInstances int `json:"max_instances,omitempty"`

	// Target CPU utilization
	TargetCPU float64 `json:"target_cpu,omitempty"` // 0.0-1.0

	// Target memory utilization
	TargetMemory float64 `json:"target_memory,omitempty"` // 0.0-1.0
}

// ServiceDependency describes a service dependency
type ServiceDependency struct {
	// Service name
	ServiceName string `json:"service_name"`

	// Schema type
	SchemaType SchemaType `json:"schema_type"`

	// Version requirement (semver range)
	VersionRange string `json:"version_range,omitempty"`

	// Is dependency critical for operation?
	Critical bool `json:"critical"`

	// Types/operations used from dependency
	UsedOperations []string `json:"used_operations,omitempty"`
}

// RouteMetadata provides per-route metadata
type RouteMetadata struct {
	// Operation/route identifier
	OperationID string `json:"operation_id"`

	// Path pattern
	Path string `json:"path"`

	// HTTP method (for REST/OpenAPI)
	Method string `json:"method,omitempty"`

	// Is operation idempotent?
	Idempotent bool `json:"idempotent"`

	// Recommended timeout for this operation
	TimeoutHint string `json:"timeout_hint,omitempty"`

	// Operation cost/complexity (1-10 scale)
	Cost int `json:"cost,omitempty"`

	// Is result cacheable?
	Cacheable bool `json:"cacheable,omitempty"`

	// Cache TTL hint
	CacheTTL string `json:"cache_ttl,omitempty"`

	// Data sensitivity level
	Sensitivity DataSensitivity `json:"sensitivity,omitempty"`

	// Expected response size
	ResponseSize SizeHint `json:"response_size,omitempty"`

	// Rate limit hint (requests per second)
	RateLimitHint int `json:"rate_limit_hint,omitempty"`
}

// DataSensitivity defines data sensitivity levels
type DataSensitivity string

const (
	// SensitivityPublic indicates public data
	SensitivityPublic DataSensitivity = "public"

	// SensitivityInternal indicates internal data
	SensitivityInternal DataSensitivity = "internal"

	// SensitivityConfidential indicates confidential data
	SensitivityConfidential DataSensitivity = "confidential"

	// SensitivityPII indicates personally identifiable information
	SensitivityPII DataSensitivity = "pii"

	// SensitivityPHI indicates protected health information
	SensitivityPHI DataSensitivity = "phi"

	// SensitivityPCI indicates payment card industry data
	SensitivityPCI DataSensitivity = "pci"
)

// String returns the string representation of the data sensitivity
func (ds DataSensitivity) String() string {
	return string(ds)
}

// SizeHint provides hints about expected data size
type SizeHint string

const (
	// SizeSmall indicates < 1KB
	SizeSmall SizeHint = "small"

	// SizeMedium indicates 1KB - 100KB
	SizeMedium SizeHint = "medium"

	// SizeLarge indicates 100KB - 1MB
	SizeLarge SizeHint = "large"

	// SizeXLarge indicates > 1MB
	SizeXLarge SizeHint = "xlarge"
)

// String returns the string representation of the size hint
func (sh SizeHint) String() string {
	return string(sh)
}

// ProtocolMetadata provides protocol-specific metadata
type ProtocolMetadata struct {
	// GraphQL-specific metadata
	GraphQL *GraphQLMetadata `json:"graphql,omitempty"`

	// gRPC-specific metadata
	GRPC *GRPCMetadata `json:"grpc,omitempty"`

	// OpenAPI-specific metadata
	OpenAPI *OpenAPIMetadata `json:"openapi,omitempty"`

	// AsyncAPI-specific metadata
	AsyncAPI *AsyncAPIMetadata `json:"asyncapi,omitempty"`

	// oRPC-specific metadata
	ORPC *ORPCMetadata `json:"orpc,omitempty"`
}

// GraphQLMetadata provides GraphQL-specific metadata
type GraphQLMetadata struct {
	// Federation configuration
	Federation *GraphQLFederation `json:"federation,omitempty"`

	// Subscription support
	SubscriptionsEnabled bool `json:"subscriptions_enabled,omitempty"`

	// Subscription protocol
	SubscriptionProtocol string `json:"subscription_protocol,omitempty"` // "graphql-ws", "graphql-transport-ws"

	// Query complexity limits
	ComplexityLimit int `json:"complexity_limit,omitempty"`

	// Query depth limits
	DepthLimit int `json:"depth_limit,omitempty"`
}

// GraphQLFederation provides GraphQL Federation metadata
type GraphQLFederation struct {
	// Federation version
	Version string `json:"version"` // "v1", "v2"

	// Subgraph name
	SubgraphName string `json:"subgraph_name"`

	// Entity types owned by this service
	Entities []FederatedEntity `json:"entities,omitempty"`

	// External types from other services
	Extends []string `json:"extends,omitempty"`

	// Provides relationships
	Provides []ProvidesRelation `json:"provides,omitempty"`

	// Requires relationships
	Requires []RequiresRelation `json:"requires,omitempty"`
}

// FederatedEntity describes a federated GraphQL entity
type FederatedEntity struct {
	// Type name
	TypeName string `json:"type_name"`

	// Key fields for entity resolution
	KeyFields []string `json:"key_fields"`

	// Fields owned by this service
	Fields []string `json:"fields"`

	// Resolvable via this subgraph
	Resolvable bool `json:"resolvable"`
}

// ProvidesRelation describes a GraphQL @provides relationship
type ProvidesRelation struct {
	// Field path
	Field string `json:"field"`

	// Provided fields
	Fields []string `json:"fields"`
}

// RequiresRelation describes a GraphQL @requires relationship
type RequiresRelation struct {
	// Field path
	Field string `json:"field"`

	// Required fields
	Fields []string `json:"fields"`
}

// GRPCMetadata provides gRPC-specific metadata
type GRPCMetadata struct {
	// Reflection enabled
	ReflectionEnabled bool `json:"reflection_enabled"`

	// Package names
	Packages []string `json:"packages"`

	// Service names
	Services []string `json:"services"`

	// gRPC-Web support
	GRPCWebEnabled bool `json:"grpc_web_enabled,omitempty"`

	// gRPC-Web protocol
	GRPCWebProtocol string `json:"grpc_web_protocol,omitempty"` // "grpc-web", "grpc-web-text"

	// Server streaming support
	ServerStreamingEnabled bool `json:"server_streaming_enabled,omitempty"`

	// Client streaming support
	ClientStreamingEnabled bool `json:"client_streaming_enabled,omitempty"`

	// Bidirectional streaming support
	BidirectionalStreamingEnabled bool `json:"bidirectional_streaming_enabled,omitempty"`
}

// OpenAPIMetadata provides OpenAPI-specific metadata
type OpenAPIMetadata struct {
	// x-extension fields to preserve
	Extensions map[string]interface{} `json:"extensions,omitempty"`

	// Server variables for URL templating
	ServerVariables map[string]ServerVariable `json:"server_variables,omitempty"`

	// Default security schemes
	DefaultSecurity []string `json:"default_security,omitempty"`

	// Composition settings for schema merging
	Composition *CompositionConfig `json:"composition,omitempty"`
}

// CompositionConfig defines how this schema should be composed with others
type CompositionConfig struct {
	// Include this schema in merged/federated API documentation
	IncludeInMerged bool `json:"include_in_merged"`

	// Prefix for component schemas to avoid naming conflicts
	// If empty, defaults to service name
	ComponentPrefix string `json:"component_prefix,omitempty"`

	// Tag prefix for operation tags
	TagPrefix string `json:"tag_prefix,omitempty"`

	// Operation ID prefix to avoid conflicts
	OperationIDPrefix string `json:"operation_id_prefix,omitempty"`

	// Conflict resolution strategy when paths/components collide
	ConflictStrategy ConflictStrategy `json:"conflict_strategy"`

	// Whether to preserve x-extensions from this schema
	PreserveExtensions bool `json:"preserve_extensions"`

	// Custom servers to use in merged spec (overrides service defaults)
	CustomServers []OpenAPIServer `json:"custom_servers,omitempty"`
}

// ConflictStrategy defines how to handle conflicts during composition
type ConflictStrategy string

const (
	// ConflictStrategyPrefix adds service prefix to conflicting items
	ConflictStrategyPrefix ConflictStrategy = "prefix"

	// ConflictStrategyError fails composition on conflicts
	ConflictStrategyError ConflictStrategy = "error"

	// ConflictStrategySkip skips conflicting items from this service
	ConflictStrategySkip ConflictStrategy = "skip"

	// ConflictStrategyOverwrite overwrites existing with this service's version
	ConflictStrategyOverwrite ConflictStrategy = "overwrite"

	// ConflictStrategyMerge attempts to merge conflicting schemas
	ConflictStrategyMerge ConflictStrategy = "merge"
)

// String returns the string representation of the conflict strategy
func (cs ConflictStrategy) String() string {
	return string(cs)
}

// OpenAPIServer represents an OpenAPI server definition
type OpenAPIServer struct {
	// Server URL
	URL string `json:"url"`

	// Server description
	Description string `json:"description,omitempty"`

	// Server variables
	Variables map[string]ServerVariable `json:"variables,omitempty"`
}

// ServerVariable describes an OpenAPI server variable
type ServerVariable struct {
	// Default value
	Default string `json:"default"`

	// Enum values
	Enum []string `json:"enum,omitempty"`

	// Description
	Description string `json:"description,omitempty"`
}

// AsyncAPIMetadata provides AsyncAPI-specific metadata
type AsyncAPIMetadata struct {
	// Message broker type
	Protocol string `json:"protocol"` // "kafka", "amqp", "mqtt", "ws"

	// Channel bindings
	ChannelBindings map[string]interface{} `json:"channel_bindings,omitempty"`

	// Message bindings
	MessageBindings map[string]interface{} `json:"message_bindings,omitempty"`
}

// ORPCMetadata provides oRPC-specific metadata
type ORPCMetadata struct {
	// Batch operations supported
	BatchEnabled bool `json:"batch_enabled,omitempty"`

	// Streaming procedures
	StreamingProcedures []string `json:"streaming_procedures,omitempty"`
}
