//! Core type definitions for FARP protocol.
//!
//! This module contains all the data structures used in the FARP protocol,
//! including schemas, manifests, routing configurations, and metadata types.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Schema type enumeration
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SchemaType {
    /// OpenAPI/Swagger specifications
    #[serde(rename = "openapi")]
    OpenAPI,
    /// AsyncAPI specifications
    #[serde(rename = "asyncapi")]
    AsyncAPI,
    /// gRPC protocol buffer definitions
    #[serde(rename = "grpc")]
    GRPC,
    /// GraphQL Schema Definition Language
    #[serde(rename = "graphql")]
    GraphQL,
    /// oRPC (OpenAPI-based RPC) specifications
    #[serde(rename = "orpc")]
    ORPC,
    /// Apache Thrift IDL
    #[serde(rename = "thrift")]
    Thrift,
    /// Apache Avro schemas
    #[serde(rename = "avro")]
    Avro,
    /// Custom/proprietary schema types
    #[serde(rename = "custom")]
    Custom,
}

impl SchemaType {
    /// Checks if the schema type is valid
    pub fn is_valid(&self) -> bool {
        matches!(
            self,
            SchemaType::OpenAPI
                | SchemaType::AsyncAPI
                | SchemaType::GRPC
                | SchemaType::GraphQL
                | SchemaType::ORPC
                | SchemaType::Thrift
                | SchemaType::Avro
                | SchemaType::Custom
        )
    }

    /// Returns the string representation
    pub fn as_str(&self) -> &'static str {
        match self {
            SchemaType::OpenAPI => "openapi",
            SchemaType::AsyncAPI => "asyncapi",
            SchemaType::GRPC => "grpc",
            SchemaType::GraphQL => "graphql",
            SchemaType::ORPC => "orpc",
            SchemaType::Thrift => "thrift",
            SchemaType::Avro => "avro",
            SchemaType::Custom => "custom",
        }
    }
}

impl std::fmt::Display for SchemaType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

/// Location type enumeration
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum LocationType {
    /// Fetch schema via HTTP GET
    #[serde(rename = "http")]
    HTTP,
    /// Fetch schema from backend KV store
    #[serde(rename = "registry")]
    Registry,
    /// Schema is embedded in the manifest
    #[serde(rename = "inline")]
    Inline,
}

impl LocationType {
    /// Checks if the location type is valid
    pub fn is_valid(&self) -> bool {
        matches!(
            self,
            LocationType::HTTP | LocationType::Registry | LocationType::Inline
        )
    }

    /// Returns the string representation
    pub fn as_str(&self) -> &'static str {
        match self {
            LocationType::HTTP => "http",
            LocationType::Registry => "registry",
            LocationType::Inline => "inline",
        }
    }
}

impl std::fmt::Display for LocationType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

/// Protocol capability enumeration
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Capability {
    /// REST API support
    #[serde(rename = "rest")]
    REST,
    /// gRPC support
    #[serde(rename = "grpc")]
    GRPC,
    /// WebSocket support
    #[serde(rename = "websocket")]
    WebSocket,
    /// Server-Sent Events support
    #[serde(rename = "sse")]
    SSE,
    /// GraphQL support
    #[serde(rename = "graphql")]
    GraphQL,
    /// MQTT support
    #[serde(rename = "mqtt")]
    MQTT,
    /// AMQP support
    #[serde(rename = "amqp")]
    AMQP,
}

impl Capability {
    /// Returns the string representation
    pub fn as_str(&self) -> &'static str {
        match self {
            Capability::REST => "rest",
            Capability::GRPC => "grpc",
            Capability::WebSocket => "websocket",
            Capability::SSE => "sse",
            Capability::GraphQL => "graphql",
            Capability::MQTT => "mqtt",
            Capability::AMQP => "amqp",
        }
    }
}

impl std::fmt::Display for Capability {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

/// Instance status enumeration
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum InstanceStatus {
    /// Instance is starting
    #[serde(rename = "starting")]
    Starting,
    /// Instance is healthy
    #[serde(rename = "healthy")]
    Healthy,
    /// Instance is degraded
    #[serde(rename = "degraded")]
    Degraded,
    /// Instance is unhealthy
    #[serde(rename = "unhealthy")]
    Unhealthy,
    /// Instance is draining connections
    #[serde(rename = "draining")]
    Draining,
    /// Instance is stopping
    #[serde(rename = "stopping")]
    Stopping,
}

impl std::fmt::Display for InstanceStatus {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            InstanceStatus::Starting => "starting",
            InstanceStatus::Healthy => "healthy",
            InstanceStatus::Degraded => "degraded",
            InstanceStatus::Unhealthy => "unhealthy",
            InstanceStatus::Draining => "draining",
            InstanceStatus::Stopping => "stopping",
        };
        write!(f, "{s}")
    }
}

/// Instance role in a deployment
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum InstanceRole {
    /// Instance is primary/production
    #[serde(rename = "primary")]
    Primary,
    /// Instance is a canary deployment
    #[serde(rename = "canary")]
    Canary,
    /// Instance is part of blue deployment
    #[serde(rename = "blue")]
    Blue,
    /// Instance is part of green deployment
    #[serde(rename = "green")]
    Green,
    /// Instance is a shadow deployment
    #[serde(rename = "shadow")]
    Shadow,
}

impl std::fmt::Display for InstanceRole {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            InstanceRole::Primary => "primary",
            InstanceRole::Canary => "canary",
            InstanceRole::Blue => "blue",
            InstanceRole::Green => "green",
            InstanceRole::Shadow => "shadow",
        };
        write!(f, "{s}")
    }
}

/// Deployment strategy
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DeploymentStrategy {
    /// Rolling update deployment
    #[serde(rename = "rolling")]
    Rolling,
    /// Canary deployment
    #[serde(rename = "canary")]
    Canary,
    /// Blue-green deployment
    #[serde(rename = "blue_green")]
    BlueGreen,
    /// Shadow deployment
    #[serde(rename = "shadow")]
    Shadow,
    /// Recreate deployment
    #[serde(rename = "recreate")]
    Recreate,
}

impl std::fmt::Display for DeploymentStrategy {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            DeploymentStrategy::Rolling => "rolling",
            DeploymentStrategy::Canary => "canary",
            DeploymentStrategy::BlueGreen => "blue_green",
            DeploymentStrategy::Shadow => "shadow",
            DeploymentStrategy::Recreate => "recreate",
        };
        write!(f, "{s}")
    }
}

/// Mount strategy for gateway routes
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
#[derive(Default)]
pub enum MountStrategy {
    /// Merge service routes to gateway root (no prefix)
    #[serde(rename = "root")]
    Root,
    /// Mount under /instance-id/* (default)
    #[serde(rename = "instance")]
    #[default]
    Instance,
    /// Mount under /service-name/*
    #[serde(rename = "service")]
    Service,
    /// Mount under /service-name/version/*
    #[serde(rename = "versioned")]
    Versioned,
    /// Mount under custom base path
    #[serde(rename = "custom")]
    Custom,
    /// Mount on subdomain: service.gateway.com
    #[serde(rename = "subdomain")]
    Subdomain,
}

impl MountStrategy {
    /// Checks if the mount strategy is valid
    pub fn is_valid(&self) -> bool {
        matches!(
            self,
            MountStrategy::Root
                | MountStrategy::Instance
                | MountStrategy::Service
                | MountStrategy::Versioned
                | MountStrategy::Custom
                | MountStrategy::Subdomain
        )
    }

    /// Returns the string representation
    pub fn as_str(&self) -> &'static str {
        match self {
            MountStrategy::Root => "root",
            MountStrategy::Instance => "instance",
            MountStrategy::Service => "service",
            MountStrategy::Versioned => "versioned",
            MountStrategy::Custom => "custom",
            MountStrategy::Subdomain => "subdomain",
        }
    }
}

impl std::fmt::Display for MountStrategy {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

/// Authentication type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AuthType {
    /// Bearer token authentication (JWT, opaque)
    #[serde(rename = "bearer")]
    Bearer,
    /// API key authentication
    #[serde(rename = "apikey")]
    APIKey,
    /// Basic authentication
    #[serde(rename = "basic")]
    Basic,
    /// Mutual TLS authentication
    #[serde(rename = "mtls")]
    MTLS,
    /// OAuth 2.0 authentication
    #[serde(rename = "oauth2")]
    OAuth2,
    /// OpenID Connect authentication
    #[serde(rename = "oidc")]
    OIDC,
    /// Custom authentication scheme
    #[serde(rename = "custom")]
    Custom,
}

impl std::fmt::Display for AuthType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            AuthType::Bearer => "bearer",
            AuthType::APIKey => "apikey",
            AuthType::Basic => "basic",
            AuthType::MTLS => "mtls",
            AuthType::OAuth2 => "oauth2",
            AuthType::OIDC => "oidc",
            AuthType::Custom => "custom",
        };
        write!(f, "{s}")
    }
}

/// Communication route type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CommunicationRouteType {
    /// Control plane operations
    #[serde(rename = "control")]
    Control,
    /// Admin operations
    #[serde(rename = "admin")]
    Admin,
    /// Management operations
    #[serde(rename = "management")]
    Management,
    /// Lifecycle start hook
    #[serde(rename = "lifecycle.start")]
    LifecycleStart,
    /// Lifecycle stop hook
    #[serde(rename = "lifecycle.stop")]
    LifecycleStop,
    /// Lifecycle reload hook
    #[serde(rename = "lifecycle.reload")]
    LifecycleReload,
    /// Config update
    #[serde(rename = "config.update")]
    ConfigUpdate,
    /// Config query
    #[serde(rename = "config.query")]
    ConfigQuery,
    /// Event poll
    #[serde(rename = "event.poll")]
    EventPoll,
    /// Event acknowledgment
    #[serde(rename = "event.ack")]
    EventAck,
    /// Health check
    #[serde(rename = "health.check")]
    HealthCheck,
    /// Status query
    #[serde(rename = "status.query")]
    StatusQuery,
    /// Schema query
    #[serde(rename = "schema.query")]
    SchemaQuery,
    /// Schema validate
    #[serde(rename = "schema.validate")]
    SchemaValidate,
    /// Metrics query
    #[serde(rename = "metrics.query")]
    MetricsQuery,
    /// Tracing export
    #[serde(rename = "tracing.export")]
    TracingExport,
    /// Custom route type
    #[serde(rename = "custom")]
    Custom,
}

impl std::fmt::Display for CommunicationRouteType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            CommunicationRouteType::Control => "control",
            CommunicationRouteType::Admin => "admin",
            CommunicationRouteType::Management => "management",
            CommunicationRouteType::LifecycleStart => "lifecycle.start",
            CommunicationRouteType::LifecycleStop => "lifecycle.stop",
            CommunicationRouteType::LifecycleReload => "lifecycle.reload",
            CommunicationRouteType::ConfigUpdate => "config.update",
            CommunicationRouteType::ConfigQuery => "config.query",
            CommunicationRouteType::EventPoll => "event.poll",
            CommunicationRouteType::EventAck => "event.ack",
            CommunicationRouteType::HealthCheck => "health.check",
            CommunicationRouteType::StatusQuery => "status.query",
            CommunicationRouteType::SchemaQuery => "schema.query",
            CommunicationRouteType::SchemaValidate => "schema.validate",
            CommunicationRouteType::MetricsQuery => "metrics.query",
            CommunicationRouteType::TracingExport => "tracing.export",
            CommunicationRouteType::Custom => "custom",
        };
        write!(f, "{s}")
    }
}

/// Webhook event type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WebhookEventType {
    /// Schema was updated
    #[serde(rename = "schema.updated")]
    SchemaUpdated,
    /// Health status changed
    #[serde(rename = "health.changed")]
    HealthChanged,
    /// Instance scaling event
    #[serde(rename = "instance.scaling")]
    InstanceScaling,
    /// Maintenance mode event
    #[serde(rename = "maintenance.mode")]
    MaintenanceMode,
    /// Rate limit changed
    #[serde(rename = "ratelimit.changed")]
    RateLimitChanged,
    /// Circuit breaker opened
    #[serde(rename = "circuit.breaker.open")]
    CircuitBreakerOpen,
    /// Circuit breaker closed
    #[serde(rename = "circuit.breaker.closed")]
    CircuitBreakerClosed,
    /// Config was updated
    #[serde(rename = "config.updated")]
    ConfigUpdated,
    /// Traffic shift event
    #[serde(rename = "traffic.shift")]
    TrafficShift,
}

impl std::fmt::Display for WebhookEventType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            WebhookEventType::SchemaUpdated => "schema.updated",
            WebhookEventType::HealthChanged => "health.changed",
            WebhookEventType::InstanceScaling => "instance.scaling",
            WebhookEventType::MaintenanceMode => "maintenance.mode",
            WebhookEventType::RateLimitChanged => "ratelimit.changed",
            WebhookEventType::CircuitBreakerOpen => "circuit.breaker.open",
            WebhookEventType::CircuitBreakerClosed => "circuit.breaker.closed",
            WebhookEventType::ConfigUpdated => "config.updated",
            WebhookEventType::TrafficShift => "traffic.shift",
        };
        write!(f, "{s}")
    }
}

/// Schema compatibility mode
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CompatibilityMode {
    /// New schema can read data written by old schema
    #[serde(rename = "backward")]
    Backward,
    /// Old schema can read data written by new schema
    #[serde(rename = "forward")]
    Forward,
    /// Both backward and forward compatible
    #[serde(rename = "full")]
    Full,
    /// Breaking changes, no compatibility guaranteed
    #[serde(rename = "none")]
    None,
    /// Transitive backward compatibility across N versions
    #[serde(rename = "backward_transitive")]
    BackwardTransitive,
    /// Transitive forward compatibility across N versions
    #[serde(rename = "forward_transitive")]
    ForwardTransitive,
}

impl std::fmt::Display for CompatibilityMode {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            CompatibilityMode::Backward => "backward",
            CompatibilityMode::Forward => "forward",
            CompatibilityMode::Full => "full",
            CompatibilityMode::None => "none",
            CompatibilityMode::BackwardTransitive => "backward_transitive",
            CompatibilityMode::ForwardTransitive => "forward_transitive",
        };
        write!(f, "{s}")
    }
}

/// Type of schema change
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ChangeType {
    /// Field was removed
    #[serde(rename = "field_removed")]
    FieldRemoved,
    /// Field type was changed
    #[serde(rename = "field_type_changed")]
    FieldTypeChanged,
    /// Field became required
    #[serde(rename = "field_required")]
    FieldRequired,
    /// Endpoint was removed
    #[serde(rename = "endpoint_removed")]
    EndpointRemoved,
    /// Endpoint was changed
    #[serde(rename = "endpoint_changed")]
    EndpointChanged,
    /// Enum value was removed
    #[serde(rename = "enum_value_removed")]
    EnumValueRemoved,
    /// Method was removed
    #[serde(rename = "method_removed")]
    MethodRemoved,
}

impl std::fmt::Display for ChangeType {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            ChangeType::FieldRemoved => "field_removed",
            ChangeType::FieldTypeChanged => "field_type_changed",
            ChangeType::FieldRequired => "field_required",
            ChangeType::EndpointRemoved => "endpoint_removed",
            ChangeType::EndpointChanged => "endpoint_changed",
            ChangeType::EnumValueRemoved => "enum_value_removed",
            ChangeType::MethodRemoved => "method_removed",
        };
        write!(f, "{s}")
    }
}

/// Severity of a schema change
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ChangeSeverity {
    /// Immediate breakage
    #[serde(rename = "critical")]
    Critical,
    /// Likely breakage
    #[serde(rename = "high")]
    High,
    /// Possible breakage
    #[serde(rename = "medium")]
    Medium,
    /// Minimal risk
    #[serde(rename = "low")]
    Low,
}

impl std::fmt::Display for ChangeSeverity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            ChangeSeverity::Critical => "critical",
            ChangeSeverity::High => "high",
            ChangeSeverity::Medium => "medium",
            ChangeSeverity::Low => "low",
        };
        write!(f, "{s}")
    }
}

/// Data sensitivity level
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DataSensitivity {
    /// Public data
    #[serde(rename = "public")]
    Public,
    /// Internal data
    #[serde(rename = "internal")]
    Internal,
    /// Confidential data
    #[serde(rename = "confidential")]
    Confidential,
    /// Personally identifiable information
    #[serde(rename = "pii")]
    PII,
    /// Protected health information
    #[serde(rename = "phi")]
    PHI,
    /// Payment card industry data
    #[serde(rename = "pci")]
    PCI,
}

impl std::fmt::Display for DataSensitivity {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            DataSensitivity::Public => "public",
            DataSensitivity::Internal => "internal",
            DataSensitivity::Confidential => "confidential",
            DataSensitivity::PII => "pii",
            DataSensitivity::PHI => "phi",
            DataSensitivity::PCI => "pci",
        };
        write!(f, "{s}")
    }
}

/// Size hint for expected data size
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SizeHint {
    /// < 1KB
    #[serde(rename = "small")]
    Small,
    /// 1KB - 100KB
    #[serde(rename = "medium")]
    Medium,
    /// 100KB - 1MB
    #[serde(rename = "large")]
    Large,
    /// > 1MB
    #[serde(rename = "xlarge")]
    XLarge,
}

impl std::fmt::Display for SizeHint {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            SizeHint::Small => "small",
            SizeHint::Medium => "medium",
            SizeHint::Large => "large",
            SizeHint::XLarge => "xlarge",
        };
        write!(f, "{s}")
    }
}

/// Schema manifest describing all API contracts for a service instance
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SchemaManifest {
    /// Version of the FARP protocol (semver)
    pub version: String,
    /// Service name
    pub service_name: String,
    /// Service version
    pub service_version: String,
    /// Instance ID
    pub instance_id: String,
    /// Instance metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub instance: Option<InstanceMetadata>,
    /// Schemas exposed by this instance
    pub schemas: Vec<SchemaDescriptor>,
    /// Capabilities/protocols supported
    pub capabilities: Vec<String>,
    /// Endpoints for introspection and health
    pub endpoints: SchemaEndpoints,
    /// Routing configuration
    #[serde(default)]
    pub routing: RoutingConfig,
    /// Authentication configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub auth: Option<AuthConfig>,
    /// Webhook configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub webhook: Option<WebhookConfig>,
    /// Service operational hints
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hints: Option<ServiceHints>,
    /// Timestamp of last update (Unix timestamp)
    pub updated_at: i64,
    /// SHA256 checksum of all schemas
    pub checksum: String,
}

/// Schema descriptor describing a single API schema/contract
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SchemaDescriptor {
    /// Type of schema
    #[serde(rename = "type")]
    pub schema_type: SchemaType,
    /// Specification version
    pub spec_version: String,
    /// How to retrieve the schema
    pub location: SchemaLocation,
    /// Content type
    pub content_type: String,
    /// Optional inline schema for small schemas
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inline_schema: Option<serde_json::Value>,
    /// SHA256 hash of schema content
    pub hash: String,
    /// Size in bytes
    pub size: i64,
    /// Schema compatibility metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub compatibility: Option<SchemaCompatibility>,
    /// Protocol-specific metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<ProtocolMetadata>,
}

/// Schema location descriptor
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SchemaLocation {
    /// Location type
    #[serde(rename = "type")]
    pub location_type: LocationType,
    /// HTTP URL (if type == HTTP)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    /// Registry path (if type == Registry)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub registry_path: Option<String>,
    /// HTTP headers for authentication
    #[serde(skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
}

/// Schema endpoints for introspection
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SchemaEndpoints {
    /// Health check endpoint (required)
    pub health: String,
    /// Prometheus metrics endpoint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metrics: Option<String>,
    /// OpenAPI spec endpoint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub openapi: Option<String>,
    /// AsyncAPI spec endpoint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub asyncapi: Option<String>,
    /// gRPC reflection enabled
    #[serde(default)]
    pub grpc_reflection: bool,
    /// GraphQL introspection endpoint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub graphql: Option<String>,
}

/// Instance metadata
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct InstanceMetadata {
    /// Instance address (host:port)
    pub address: String,
    /// Instance region
    #[serde(skip_serializing_if = "Option::is_none")]
    pub region: Option<String>,
    /// Instance zone
    #[serde(skip_serializing_if = "Option::is_none")]
    pub zone: Option<String>,
    /// Instance labels for selection
    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels: Option<HashMap<String, String>>,
    /// Instance weight for load balancing (0-100)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub weight: Option<i32>,
    /// Instance status
    pub status: InstanceStatus,
    /// Instance role in deployment
    #[serde(skip_serializing_if = "Option::is_none")]
    pub role: Option<InstanceRole>,
    /// Deployment metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub deployment: Option<DeploymentMetadata>,
    /// Instance start time (Unix timestamp)
    pub started_at: i64,
    /// Expected schema checksum
    #[serde(skip_serializing_if = "Option::is_none")]
    pub expected_schema_checksum: Option<String>,
}

/// Deployment metadata
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeploymentMetadata {
    /// Deployment ID
    pub deployment_id: String,
    /// Deployment strategy
    pub strategy: DeploymentStrategy,
    /// Traffic percentage (0-100)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub traffic_percent: Option<i32>,
    /// Deployment stage
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stage: Option<String>,
    /// Deployment time (Unix timestamp)
    pub deployed_at: i64,
}

/// Routing configuration
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct RoutingConfig {
    /// Mounting strategy
    #[serde(default = "default_mount_strategy")]
    pub strategy: MountStrategy,
    /// Base path for mounting
    #[serde(skip_serializing_if = "Option::is_none")]
    pub base_path: Option<String>,
    /// Subdomain for mounting
    #[serde(skip_serializing_if = "Option::is_none")]
    pub subdomain: Option<String>,
    /// Path rewriting rules
    #[serde(default)]
    pub rewrite: Vec<PathRewrite>,
    /// Strip prefix before forwarding
    #[serde(default)]
    pub strip_prefix: bool,
    /// Priority for conflict resolution
    #[serde(skip_serializing_if = "Option::is_none")]
    pub priority: Option<i32>,
    /// Tags for route grouping
    #[serde(default)]
    pub tags: Vec<String>,
}

fn default_mount_strategy() -> MountStrategy {
    MountStrategy::Instance
}

/// Path rewrite rule
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PathRewrite {
    /// Pattern to match (regex)
    pub pattern: String,
    /// Replacement string
    pub replacement: String,
}

/// Authentication configuration
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AuthConfig {
    /// Authentication schemes supported
    pub schemes: Vec<AuthScheme>,
    /// Required permissions/scopes
    #[serde(default)]
    pub required_scopes: Vec<String>,
    /// Access control rules
    #[serde(default)]
    pub access_control: Vec<AccessRule>,
    /// Token validation endpoint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub token_validation_url: Option<String>,
    /// Public (unauthenticated) routes
    #[serde(default)]
    pub public_routes: Vec<String>,
}

/// Authentication scheme
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AuthScheme {
    /// Scheme type
    #[serde(rename = "type")]
    pub auth_type: AuthType,
    /// Scheme configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub config: Option<HashMap<String, serde_json::Value>>,
}

/// Access control rule
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AccessRule {
    /// Path pattern (glob or regex)
    pub path: String,
    /// HTTP methods
    pub methods: Vec<String>,
    /// Required roles
    #[serde(default)]
    pub roles: Vec<String>,
    /// Required permissions
    #[serde(default)]
    pub permissions: Vec<String>,
    /// Allow anonymous access
    #[serde(default)]
    pub allow_anonymous: bool,
}

/// Webhook configuration
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct WebhookConfig {
    /// Webhook endpoint on service
    #[serde(skip_serializing_if = "Option::is_none")]
    pub service_webhook: Option<String>,
    /// Webhook endpoint on gateway
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gateway_webhook: Option<String>,
    /// Webhook secret for HMAC
    #[serde(skip_serializing_if = "Option::is_none")]
    pub secret: Option<String>,
    /// Events to subscribe to
    #[serde(default)]
    pub subscribe_events: Vec<WebhookEventType>,
    /// Events to publish
    #[serde(default)]
    pub publish_events: Vec<WebhookEventType>,
    /// Retry configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub retry: Option<RetryConfig>,
    /// HTTP communication routes
    #[serde(skip_serializing_if = "Option::is_none")]
    pub http_routes: Option<HTTPCommunicationRoutes>,
}

/// HTTP communication routes
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct HTTPCommunicationRoutes {
    /// Service routes
    #[serde(default)]
    pub service_routes: Vec<CommunicationRoute>,
    /// Gateway routes
    #[serde(default)]
    pub gateway_routes: Vec<CommunicationRoute>,
    /// Polling configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub polling: Option<PollingConfig>,
}

/// Communication route
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CommunicationRoute {
    /// Route identifier
    pub id: String,
    /// Route path
    pub path: String,
    /// HTTP method
    pub method: String,
    /// Route type
    #[serde(rename = "type")]
    pub route_type: CommunicationRouteType,
    /// Description
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    /// Request schema
    #[serde(skip_serializing_if = "Option::is_none")]
    pub request_schema: Option<serde_json::Value>,
    /// Response schema
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_schema: Option<serde_json::Value>,
    /// Authentication required
    pub auth_required: bool,
    /// Idempotent operation
    pub idempotent: bool,
    /// Expected timeout
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<String>,
}

/// Polling configuration
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct PollingConfig {
    /// Polling interval
    pub interval: String,
    /// Polling timeout
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout: Option<String>,
    /// Long polling support
    #[serde(default)]
    pub long_polling: bool,
    /// Long polling timeout
    #[serde(skip_serializing_if = "Option::is_none")]
    pub long_polling_timeout: Option<String>,
}

/// Retry configuration
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct RetryConfig {
    /// Maximum retry attempts
    pub max_attempts: i32,
    /// Initial retry delay
    pub initial_delay: String,
    /// Maximum retry delay
    pub max_delay: String,
    /// Backoff multiplier
    pub multiplier: f64,
}

/// Schema compatibility metadata
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SchemaCompatibility {
    /// Compatibility mode
    pub mode: CompatibilityMode,
    /// Previous schema versions
    #[serde(default)]
    pub previous_versions: Vec<String>,
    /// Breaking changes
    #[serde(default)]
    pub breaking_changes: Vec<BreakingChange>,
    /// Deprecation notices
    #[serde(default)]
    pub deprecations: Vec<Deprecation>,
}

/// Breaking change descriptor
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct BreakingChange {
    /// Type of breaking change
    #[serde(rename = "type")]
    pub change_type: ChangeType,
    /// Path in schema
    pub path: String,
    /// Description
    pub description: String,
    /// Severity level
    pub severity: ChangeSeverity,
    /// Migration instructions
    #[serde(skip_serializing_if = "Option::is_none")]
    pub migration: Option<String>,
}

/// Deprecation descriptor
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Deprecation {
    /// Path in schema
    pub path: String,
    /// Deprecation date (ISO 8601)
    pub deprecated_at: String,
    /// Planned removal date
    #[serde(skip_serializing_if = "Option::is_none")]
    pub removal_date: Option<String>,
    /// Replacement recommendation
    #[serde(skip_serializing_if = "Option::is_none")]
    pub replacement: Option<String>,
    /// Migration guide
    #[serde(skip_serializing_if = "Option::is_none")]
    pub migration: Option<String>,
    /// Reason for deprecation
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
}

/// Service operational hints
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ServiceHints {
    /// Recommended timeout
    #[serde(skip_serializing_if = "Option::is_none")]
    pub recommended_timeout: Option<String>,
    /// Expected latency profile
    #[serde(skip_serializing_if = "Option::is_none")]
    pub expected_latency: Option<LatencyProfile>,
    /// Scaling characteristics
    #[serde(skip_serializing_if = "Option::is_none")]
    pub scaling: Option<ScalingProfile>,
    /// Service dependencies
    #[serde(default)]
    pub dependencies: Vec<ServiceDependency>,
}

/// Latency profile
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LatencyProfile {
    /// Median latency (p50)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub p50: Option<String>,
    /// 95th percentile
    #[serde(skip_serializing_if = "Option::is_none")]
    pub p95: Option<String>,
    /// 99th percentile
    #[serde(skip_serializing_if = "Option::is_none")]
    pub p99: Option<String>,
    /// 99.9th percentile
    #[serde(skip_serializing_if = "Option::is_none")]
    pub p999: Option<String>,
}

/// Scaling profile
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ScalingProfile {
    /// Auto-scaling enabled
    pub auto_scale: bool,
    /// Minimum instances
    #[serde(skip_serializing_if = "Option::is_none")]
    pub min_instances: Option<i32>,
    /// Maximum instances
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_instances: Option<i32>,
    /// Target CPU utilization (0.0-1.0)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target_cpu: Option<f64>,
    /// Target memory utilization (0.0-1.0)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub target_memory: Option<f64>,
}

/// Service dependency
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ServiceDependency {
    /// Service name
    pub service_name: String,
    /// Schema type
    pub schema_type: SchemaType,
    /// Version requirement (semver range)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version_range: Option<String>,
    /// Is dependency critical
    pub critical: bool,
    /// Used operations
    #[serde(default)]
    pub used_operations: Vec<String>,
}

/// Route metadata
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct RouteMetadata {
    /// Operation/route identifier
    pub operation_id: String,
    /// Path pattern
    pub path: String,
    /// HTTP method
    #[serde(skip_serializing_if = "Option::is_none")]
    pub method: Option<String>,
    /// Is operation idempotent
    pub idempotent: bool,
    /// Recommended timeout
    #[serde(skip_serializing_if = "Option::is_none")]
    pub timeout_hint: Option<String>,
    /// Operation cost/complexity (1-10)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cost: Option<i32>,
    /// Is result cacheable
    #[serde(default)]
    pub cacheable: bool,
    /// Cache TTL hint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cache_ttl: Option<String>,
    /// Data sensitivity level
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sensitivity: Option<DataSensitivity>,
    /// Expected response size
    #[serde(skip_serializing_if = "Option::is_none")]
    pub response_size: Option<SizeHint>,
    /// Rate limit hint
    #[serde(skip_serializing_if = "Option::is_none")]
    pub rate_limit_hint: Option<i32>,
}

/// Protocol-specific metadata
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ProtocolMetadata {
    /// GraphQL-specific metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub graphql: Option<GraphQLMetadata>,
    /// gRPC-specific metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub grpc: Option<GRPCMetadata>,
    /// OpenAPI-specific metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub openapi: Option<OpenAPIMetadata>,
    /// AsyncAPI-specific metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub asyncapi: Option<AsyncAPIMetadata>,
    /// oRPC-specific metadata
    #[serde(skip_serializing_if = "Option::is_none")]
    pub orpc: Option<ORPCMetadata>,
}

/// GraphQL-specific metadata
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GraphQLMetadata {
    /// Federation configuration
    #[serde(skip_serializing_if = "Option::is_none")]
    pub federation: Option<GraphQLFederation>,
    /// Subscription support
    #[serde(default)]
    pub subscriptions_enabled: bool,
    /// Subscription protocol
    #[serde(skip_serializing_if = "Option::is_none")]
    pub subscription_protocol: Option<String>,
    /// Query complexity limits
    #[serde(skip_serializing_if = "Option::is_none")]
    pub complexity_limit: Option<i32>,
    /// Query depth limits
    #[serde(skip_serializing_if = "Option::is_none")]
    pub depth_limit: Option<i32>,
}

/// GraphQL Federation configuration
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GraphQLFederation {
    /// Federation version
    pub version: String,
    /// Subgraph name
    pub subgraph_name: String,
    /// Entity types owned
    #[serde(default)]
    pub entities: Vec<FederatedEntity>,
    /// External types
    #[serde(default)]
    pub extends: Vec<String>,
    /// Provides relationships
    #[serde(default)]
    pub provides: Vec<ProvidesRelation>,
    /// Requires relationships
    #[serde(default)]
    pub requires: Vec<RequiresRelation>,
}

/// Federated GraphQL entity
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct FederatedEntity {
    /// Type name
    pub type_name: String,
    /// Key fields
    pub key_fields: Vec<String>,
    /// Fields owned
    pub fields: Vec<String>,
    /// Resolvable via this subgraph
    pub resolvable: bool,
}

/// GraphQL @provides relationship
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProvidesRelation {
    /// Field path
    pub field: String,
    /// Provided fields
    pub fields: Vec<String>,
}

/// GraphQL @requires relationship
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RequiresRelation {
    /// Field path
    pub field: String,
    /// Required fields
    pub fields: Vec<String>,
}

/// gRPC-specific metadata
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GRPCMetadata {
    /// Reflection enabled
    pub reflection_enabled: bool,
    /// Package names
    pub packages: Vec<String>,
    /// Service names
    pub services: Vec<String>,
    /// gRPC-Web support
    #[serde(default)]
    pub grpc_web_enabled: bool,
    /// gRPC-Web protocol
    #[serde(skip_serializing_if = "Option::is_none")]
    pub grpc_web_protocol: Option<String>,
    /// Server streaming support
    #[serde(default)]
    pub server_streaming_enabled: bool,
    /// Client streaming support
    #[serde(default)]
    pub client_streaming_enabled: bool,
    /// Bidirectional streaming support
    #[serde(default)]
    pub bidirectional_streaming_enabled: bool,
}

/// OpenAPI-specific metadata
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct OpenAPIMetadata {
    /// x-extension fields
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extensions: Option<HashMap<String, serde_json::Value>>,
    /// Server variables
    #[serde(skip_serializing_if = "Option::is_none")]
    pub server_variables: Option<HashMap<String, ServerVariable>>,
    /// Default security schemes
    #[serde(default)]
    pub default_security: Vec<String>,
    /// Composition settings for schema merging
    #[serde(skip_serializing_if = "Option::is_none")]
    pub composition: Option<CompositionConfig>,
}

/// OpenAPI server variable
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ServerVariable {
    /// Default value
    pub default: String,
    /// Enum values
    #[serde(default)]
    pub enum_values: Vec<String>,
    /// Description
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

/// AsyncAPI-specific metadata
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AsyncAPIMetadata {
    /// Message broker protocol
    pub protocol: String,
    /// Channel bindings
    #[serde(skip_serializing_if = "Option::is_none")]
    pub channel_bindings: Option<HashMap<String, serde_json::Value>>,
    /// Message bindings
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message_bindings: Option<HashMap<String, serde_json::Value>>,
}

/// oRPC-specific metadata
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ORPCMetadata {
    /// Batch operations supported
    #[serde(default)]
    pub batch_enabled: bool,
    /// Streaming procedures
    #[serde(default)]
    pub streaming_procedures: Vec<String>,
}

/// Composition configuration for schema merging
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CompositionConfig {
    /// Include this schema in merged/federated API documentation
    pub include_in_merged: bool,
    /// Prefix for component schemas to avoid naming conflicts
    #[serde(skip_serializing_if = "Option::is_none")]
    pub component_prefix: Option<String>,
    /// Tag prefix for operation tags
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tag_prefix: Option<String>,
    /// Operation ID prefix to avoid conflicts
    #[serde(skip_serializing_if = "Option::is_none")]
    pub operation_id_prefix: Option<String>,
    /// Conflict resolution strategy when paths/components collide
    pub conflict_strategy: ConflictStrategy,
    /// Whether to preserve x-extensions from this schema
    pub preserve_extensions: bool,
    /// Custom servers to use in merged spec
    #[serde(default)]
    pub custom_servers: Vec<OpenAPIServer>,
}

/// Conflict resolution strategy for schema merging
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ConflictStrategy {
    /// Add service prefix to conflicting items
    #[serde(rename = "prefix")]
    Prefix,
    /// Fail composition on conflicts
    #[serde(rename = "error")]
    Error,
    /// Skip conflicting items from this service
    #[serde(rename = "skip")]
    Skip,
    /// Overwrite existing with this service's version
    #[serde(rename = "overwrite")]
    Overwrite,
    /// Attempt to merge conflicting schemas
    #[serde(rename = "merge")]
    Merge,
}

impl std::fmt::Display for ConflictStrategy {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let s = match self {
            ConflictStrategy::Prefix => "prefix",
            ConflictStrategy::Error => "error",
            ConflictStrategy::Skip => "skip",
            ConflictStrategy::Overwrite => "overwrite",
            ConflictStrategy::Merge => "merge",
        };
        write!(f, "{s}")
    }
}

/// OpenAPI server definition
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct OpenAPIServer {
    /// Server URL
    pub url: String,
    /// Server description
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    /// Server variables
    #[serde(skip_serializing_if = "Option::is_none")]
    pub variables: Option<HashMap<String, ServerVariable>>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_schema_type_serde() {
        let schema_type = SchemaType::OpenAPI;
        let json = serde_json::to_string(&schema_type).unwrap();
        assert_eq!(json, "\"openapi\"");

        let deserialized: SchemaType = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized, schema_type);
    }

    #[test]
    fn test_schema_type_is_valid() {
        assert!(SchemaType::OpenAPI.is_valid());
        assert!(SchemaType::AsyncAPI.is_valid());
        assert!(SchemaType::GRPC.is_valid());
    }

    #[test]
    fn test_location_type_serde() {
        let location = LocationType::HTTP;
        let json = serde_json::to_string(&location).unwrap();
        assert_eq!(json, "\"http\"");
    }

    #[test]
    fn test_mount_strategy_default() {
        let strategy = MountStrategy::default();
        assert_eq!(strategy, MountStrategy::Instance);
    }

    #[test]
    fn test_schema_manifest_serde() {
        let manifest = SchemaManifest {
            version: "1.0.0".to_string(),
            service_name: "test-service".to_string(),
            service_version: "v1.0.0".to_string(),
            instance_id: "instance-123".to_string(),
            instance: None,
            schemas: vec![],
            capabilities: vec!["rest".to_string()],
            endpoints: SchemaEndpoints {
                health: "/health".to_string(),
                ..Default::default()
            },
            routing: RoutingConfig::default(),
            auth: None,
            webhook: None,
            hints: None,
            updated_at: 1234567890,
            checksum: "abc123".to_string(),
        };

        let json = serde_json::to_string(&manifest).unwrap();
        let deserialized: SchemaManifest = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.service_name, "test-service");
    }
}
