// =============================================================================
// Schema Type
// =============================================================================

export const SchemaType = {
  OpenAPI: 'openapi',
  AsyncAPI: 'asyncapi',
  GRPC: 'grpc',
  GraphQL: 'graphql',
  ORPC: 'orpc',
  Thrift: 'thrift',
  Avro: 'avro',
  Custom: 'custom',
} as const;

export type SchemaType = (typeof SchemaType)[keyof typeof SchemaType];

const VALID_SCHEMA_TYPES: ReadonlySet<string> = new Set(Object.values(SchemaType));

export function isValidSchemaType(value: string): value is SchemaType {
  return VALID_SCHEMA_TYPES.has(value);
}

// =============================================================================
// Location Type
// =============================================================================

export const LocationType = {
  HTTP: 'http',
  Registry: 'registry',
  Inline: 'inline',
} as const;

export type LocationType = (typeof LocationType)[keyof typeof LocationType];

const VALID_LOCATION_TYPES: ReadonlySet<string> = new Set(Object.values(LocationType));

export function isValidLocationType(value: string): value is LocationType {
  return VALID_LOCATION_TYPES.has(value);
}

// =============================================================================
// Instance Status
// =============================================================================

export const InstanceStatus = {
  Starting: 'starting',
  Healthy: 'healthy',
  Degraded: 'degraded',
  Unhealthy: 'unhealthy',
  Draining: 'draining',
  Stopping: 'stopping',
} as const;

export type InstanceStatus = (typeof InstanceStatus)[keyof typeof InstanceStatus];

// =============================================================================
// Instance Role
// =============================================================================

export const InstanceRole = {
  Primary: 'primary',
  Canary: 'canary',
  Blue: 'blue',
  Green: 'green',
  Shadow: 'shadow',
} as const;

export type InstanceRole = (typeof InstanceRole)[keyof typeof InstanceRole];

// =============================================================================
// Deployment Strategy
// =============================================================================

export const DeploymentStrategy = {
  Rolling: 'rolling',
  Canary: 'canary',
  BlueGreen: 'blue_green',
  Shadow: 'shadow',
  Recreate: 'recreate',
} as const;

export type DeploymentStrategy = (typeof DeploymentStrategy)[keyof typeof DeploymentStrategy];

// =============================================================================
// Mount Strategy
// =============================================================================

export const MountStrategy = {
  Root: 'root',
  Instance: 'instance',
  Service: 'service',
  Versioned: 'versioned',
  Custom: 'custom',
  Subdomain: 'subdomain',
} as const;

export type MountStrategy = (typeof MountStrategy)[keyof typeof MountStrategy];

const VALID_MOUNT_STRATEGIES: ReadonlySet<string> = new Set(Object.values(MountStrategy));

export function isValidMountStrategy(value: string): value is MountStrategy {
  return VALID_MOUNT_STRATEGIES.has(value);
}

// =============================================================================
// Path Rule Action
// =============================================================================

export const PathRuleAction = {
  Include: 'include',
  Exclude: 'exclude',
} as const;

export type PathRuleAction = (typeof PathRuleAction)[keyof typeof PathRuleAction];

// =============================================================================
// Auth Type
// =============================================================================

export const AuthType = {
  Bearer: 'bearer',
  APIKey: 'apikey',
  Basic: 'basic',
  MTLS: 'mtls',
  OAuth2: 'oauth2',
  OIDC: 'oidc',
  Custom: 'custom',
} as const;

export type AuthType = (typeof AuthType)[keyof typeof AuthType];

// =============================================================================
// Capability
// =============================================================================

export const Capability = {
  REST: 'rest',
  GRPC: 'grpc',
  WebSocket: 'websocket',
  SSE: 'sse',
  GraphQL: 'graphql',
  MQTT: 'mqtt',
  AMQP: 'amqp',
} as const;

export type Capability = (typeof Capability)[keyof typeof Capability];

// =============================================================================
// Communication Route Type
// =============================================================================

export const CommunicationRouteType = {
  Control: 'control',
  Admin: 'admin',
  Management: 'management',
  LifecycleStart: 'lifecycle.start',
  LifecycleStop: 'lifecycle.stop',
  LifecycleReload: 'lifecycle.reload',
  ConfigUpdate: 'config.update',
  ConfigQuery: 'config.query',
  EventPoll: 'event.poll',
  EventAck: 'event.ack',
  HealthCheck: 'health.check',
  StatusQuery: 'status.query',
  SchemaQuery: 'schema.query',
  SchemaValidate: 'schema.validate',
  MetricsQuery: 'metrics.query',
  TracingExport: 'tracing.export',
  Custom: 'custom',
} as const;

export type CommunicationRouteType = (typeof CommunicationRouteType)[keyof typeof CommunicationRouteType];

// =============================================================================
// Webhook Event Type
// =============================================================================

export const WebhookEventType = {
  SchemaUpdated: 'schema.updated',
  HealthChanged: 'health.changed',
  InstanceScaling: 'instance.scaling',
  MaintenanceMode: 'maintenance.mode',
  RateLimitChanged: 'ratelimit.changed',
  CircuitBreakerOpen: 'circuit.breaker.open',
  CircuitBreakerClosed: 'circuit.breaker.closed',
  ConfigUpdated: 'config.updated',
  TrafficShift: 'traffic.shift',
  RoutesChanging: 'routes.changing',
  RoutesChanged: 'routes.changed',
  RoutesDraining: 'routes.draining',
  GatewayShutdown: 'gateway.shutdown',
} as const;

export type WebhookEventType = (typeof WebhookEventType)[keyof typeof WebhookEventType];

// =============================================================================
// Compatibility Mode
// =============================================================================

export const CompatibilityMode = {
  Backward: 'backward',
  Forward: 'forward',
  Full: 'full',
  None: 'none',
  BackwardTransitive: 'backward_transitive',
  ForwardTransitive: 'forward_transitive',
} as const;

export type CompatibilityMode = (typeof CompatibilityMode)[keyof typeof CompatibilityMode];

// =============================================================================
// Change Type
// =============================================================================

export const ChangeType = {
  FieldRemoved: 'field_removed',
  FieldTypeChanged: 'field_type_changed',
  FieldRequired: 'field_required',
  EndpointRemoved: 'endpoint_removed',
  EndpointChanged: 'endpoint_changed',
  EnumValueRemoved: 'enum_value_removed',
  MethodRemoved: 'method_removed',
} as const;

export type ChangeType = (typeof ChangeType)[keyof typeof ChangeType];

// =============================================================================
// Change Severity
// =============================================================================

export const ChangeSeverity = {
  Critical: 'critical',
  High: 'high',
  Medium: 'medium',
  Low: 'low',
} as const;

export type ChangeSeverity = (typeof ChangeSeverity)[keyof typeof ChangeSeverity];

// =============================================================================
// Data Sensitivity
// =============================================================================

export const DataSensitivity = {
  Public: 'public',
  Internal: 'internal',
  Confidential: 'confidential',
  PII: 'pii',
  PHI: 'phi',
  PCI: 'pci',
} as const;

export type DataSensitivity = (typeof DataSensitivity)[keyof typeof DataSensitivity];

// =============================================================================
// Size Hint
// =============================================================================

export const SizeHint = {
  Small: 'small',
  Medium: 'medium',
  Large: 'large',
  XLarge: 'xlarge',
} as const;

export type SizeHint = (typeof SizeHint)[keyof typeof SizeHint];

// =============================================================================
// Conflict Strategy
// =============================================================================

export const ConflictStrategy = {
  Prefix: 'prefix',
  Error: 'error',
  Skip: 'skip',
  Overwrite: 'overwrite',
  Merge: 'merge',
} as const;

export type ConflictStrategy = (typeof ConflictStrategy)[keyof typeof ConflictStrategy];

// =============================================================================
// Rate Limit Strategy
// =============================================================================

export const RateLimitStrategy = {
  FixedWindow: 'fixed_window',
  SlidingWindow: 'sliding_window',
  TokenBucket: 'token_bucket',
  LeakyBucket: 'leaky_bucket',
} as const;

export type RateLimitStrategy = (typeof RateLimitStrategy)[keyof typeof RateLimitStrategy];

// =============================================================================
// Rate Limit Key
// =============================================================================

export const RateLimitKey = {
  IP: 'ip',
  User: 'user',
  APIKey: 'api_key',
  Global: 'global',
} as const;

export type RateLimitKey = (typeof RateLimitKey)[keyof typeof RateLimitKey];

// =============================================================================
// Load Balancing Strategy
// =============================================================================

export const LoadBalancingStrategy = {
  RoundRobin: 'round_robin',
  LeastConnections: 'least_connections',
  WeightedRoundRobin: 'weighted_round_robin',
  IPHash: 'ip_hash',
  Random: 'random',
  ConsistentHash: 'consistent_hash',
} as const;

export type LoadBalancingStrategy = (typeof LoadBalancingStrategy)[keyof typeof LoadBalancingStrategy];

// =============================================================================
// Versioning Strategy
// =============================================================================

export const VersioningStrategy = {
  URLPath: 'url_path',
  Header: 'header',
  QueryParam: 'query_param',
} as const;

export type VersioningStrategy = (typeof VersioningStrategy)[keyof typeof VersioningStrategy];

// =============================================================================
// Event Type
// =============================================================================

export const EventType = {
  Added: 'added',
  Updated: 'updated',
  Removed: 'removed',
} as const;

export type EventType = (typeof EventType)[keyof typeof EventType];

// =============================================================================
// Interfaces / Structs
// =============================================================================

/** SchemaManifest describes all API contracts for a service instance. */
export interface SchemaManifest {
  version: string;
  service_name: string;
  service_version: string;
  instance_id: string;
  instance?: InstanceMetadata;
  schemas: SchemaDescriptor[];
  capabilities: string[];
  endpoints: SchemaEndpoints;
  routing: RoutingConfig;
  auth?: AuthConfig;
  webhook?: WebhookConfig;
  hints?: ServiceHints;
  route_table?: RouteDescriptor[];
  updated_at: number;
  checksum: string;
  routes_checksum?: string;
}

/** SchemaDescriptor describes a single API schema/contract. */
export interface SchemaDescriptor {
  type: SchemaType;
  spec_version: string;
  location: SchemaLocation;
  content_type: string;
  inline_schema?: unknown;
  hash: string;
  size: number;
  compatibility?: SchemaCompatibility;
  metadata?: ProtocolMetadata;
}

/** SchemaLocation describes where and how to fetch a schema. */
export interface SchemaLocation {
  type: LocationType;
  url?: string;
  registry_path?: string;
  headers?: Record<string, string>;
}

/** SchemaEndpoints provides URLs for service introspection. */
export interface SchemaEndpoints {
  health: string;
  metrics?: string;
  openapi?: string;
  asyncapi?: string;
  grpc_reflection?: boolean;
  graphql?: string;
  documentation?: string;
  changelog?: string;
}

/** InstanceMetadata provides information about a service instance. */
export interface InstanceMetadata {
  address: string;
  region?: string;
  zone?: string;
  labels?: Record<string, string>;
  weight?: number;
  status: InstanceStatus;
  role?: InstanceRole;
  deployment?: DeploymentMetadata;
  started_at: number;
  expected_schema_checksum?: string;
}

/** DeploymentMetadata provides information about a deployment. */
export interface DeploymentMetadata {
  deployment_id: string;
  strategy: DeploymentStrategy;
  traffic_percent?: number;
  stage?: string;
  deployed_at: number;
}

/** RoutingConfig provides gateway route mounting configuration. */
export interface RoutingConfig {
  strategy: MountStrategy;
  base_path?: string;
  subdomain?: string;
  rewrite?: PathRewrite[];
  strip_prefix?: boolean;
  priority?: number;
  tags?: string[];
  middleware?: MiddlewareDeclaration[];
  versioning?: APIVersioningConfig;
  path_rules?: PathRule[];
}

/** PathRule defines an include or exclude rule for API paths. */
export interface PathRule {
  pattern: string;
  action: PathRuleAction;
}

/** PathRewrite defines a path rewriting rule. */
export interface PathRewrite {
  pattern: string;
  replacement: string;
}

/** AuthConfig provides authentication and authorization configuration. */
export interface AuthConfig {
  schemes: AuthScheme[];
  required_scopes?: string[];
  access_control?: AccessRule[];
  token_validation_url?: string;
  public_routes?: string[];
}

/** AuthScheme describes an authentication scheme. */
export interface AuthScheme {
  type: AuthType;
  config?: Record<string, unknown>;
}

/** AccessRule defines an access control rule. */
export interface AccessRule {
  path: string;
  methods: string[];
  roles?: string[];
  permissions?: string[];
  allow_anonymous?: boolean;
}

/** WebhookConfig provides bidirectional communication configuration. */
export interface WebhookConfig {
  service_webhook?: string;
  gateway_webhook?: string;
  secret?: string;
  subscribe_events?: WebhookEventType[];
  publish_events?: WebhookEventType[];
  retry?: RetryConfig;
  http_routes?: HTTPCommunicationRoutes;
}

/** HTTPCommunicationRoutes defines HTTP communication routes. */
export interface HTTPCommunicationRoutes {
  service_routes?: CommunicationRoute[];
  gateway_routes?: CommunicationRoute[];
  polling?: PollingConfig;
}

/** CommunicationRoute defines a communication route. */
export interface CommunicationRoute {
  id: string;
  path: string;
  method: string;
  type: CommunicationRouteType;
  description?: string;
  request_schema?: unknown;
  response_schema?: unknown;
  auth_required: boolean;
  idempotent: boolean;
  timeout?: string;
}

/** PollingConfig defines polling configuration. */
export interface PollingConfig {
  interval: string;
  timeout?: string;
  long_polling?: boolean;
  long_polling_timeout?: string;
}

/** WebhookEvent is the payload sent to service webhook endpoints. */
export interface WebhookEvent {
  type: WebhookEventType;
  timestamp: number;
  source?: string;
  data?: Record<string, unknown>;
}

/** RetryConfig defines retry configuration. */
export interface RetryConfig {
  max_attempts: number;
  initial_delay: string;
  max_delay: string;
  multiplier: number;
  retryable_status_codes?: number[];
  retryable_methods?: string[];
  retry_on_connection_error?: boolean;
  per_attempt_timeout?: string;
}

/** SchemaCompatibility provides schema compatibility metadata. */
export interface SchemaCompatibility {
  mode: CompatibilityMode;
  previous_versions?: string[];
  breaking_changes?: BreakingChange[];
  deprecations?: Deprecation[];
}

/** BreakingChange describes a breaking change in a schema. */
export interface BreakingChange {
  type: ChangeType;
  path: string;
  description: string;
  severity: ChangeSeverity;
  migration?: string;
}

/** Deprecation describes a deprecated schema element. */
export interface Deprecation {
  path: string;
  deprecated_at: string;
  removal_date?: string;
  replacement?: string;
  migration?: string;
  reason?: string;
}

/** ServiceHints provides operational hints for the gateway. */
export interface ServiceHints {
  recommended_timeout?: string;
  expected_latency?: LatencyProfile;
  scaling?: ScalingProfile;
  dependencies?: ServiceDependency[];
  rate_limit?: RateLimitConfig;
  circuit_breaker?: CircuitBreakerConfig;
  cors?: CORSConfig;
  retry_policy?: RetryConfig;
  observability?: ObservabilityConfig;
  graceful_shutdown?: GracefulShutdownConfig;
  cache?: CacheConfig;
  load_balancing?: LoadBalancingConfig;
  transformations?: TransformationConfig;
}

/** LatencyProfile describes expected latency characteristics. */
export interface LatencyProfile {
  p50?: string;
  p95?: string;
  p99?: string;
  p999?: string;
}

/** ScalingProfile describes scaling characteristics. */
export interface ScalingProfile {
  auto_scale: boolean;
  min_instances?: number;
  max_instances?: number;
  target_cpu?: number;
  target_memory?: number;
}

/** ServiceDependency describes a service dependency. */
export interface ServiceDependency {
  service_name: string;
  schema_type: SchemaType;
  version_range?: string;
  critical: boolean;
  used_operations?: string[];
}

/** RouteMetadata provides per-route metadata. */
export interface RouteMetadata {
  operation_id: string;
  path: string;
  method?: string;
  idempotent: boolean;
  timeout_hint?: string;
  cost?: number;
  cacheable?: boolean;
  cache_ttl?: string;
  sensitivity?: DataSensitivity;
  response_size?: SizeHint;
  rate_limit_hint?: number;
}

/** ProtocolMetadata provides protocol-specific metadata. */
export interface ProtocolMetadata {
  graphql?: GraphQLMetadata;
  grpc?: GRPCMetadata;
  openapi?: OpenAPIMetadata;
  asyncapi?: AsyncAPIMetadata;
  orpc?: ORPCMetadata;
}

/** GraphQLMetadata provides GraphQL-specific metadata. */
export interface GraphQLMetadata {
  federation?: GraphQLFederation;
  subscriptions_enabled?: boolean;
  subscription_protocol?: string;
  complexity_limit?: number;
  depth_limit?: number;
}

/** GraphQLFederation provides GraphQL Federation metadata. */
export interface GraphQLFederation {
  version: string;
  subgraph_name: string;
  entities?: FederatedEntity[];
  extends?: string[];
  provides?: ProvidesRelation[];
  requires?: RequiresRelation[];
}

/** FederatedEntity describes a federated GraphQL entity. */
export interface FederatedEntity {
  type_name: string;
  key_fields: string[];
  fields: string[];
  resolvable: boolean;
}

/** ProvidesRelation describes a GraphQL @provides relationship. */
export interface ProvidesRelation {
  field: string;
  fields: string[];
}

/** RequiresRelation describes a GraphQL @requires relationship. */
export interface RequiresRelation {
  field: string;
  fields: string[];
}

/** GRPCMetadata provides gRPC-specific metadata. */
export interface GRPCMetadata {
  reflection_enabled: boolean;
  packages: string[];
  services: string[];
  grpc_web_enabled?: boolean;
  grpc_web_protocol?: string;
  server_streaming_enabled?: boolean;
  client_streaming_enabled?: boolean;
  bidirectional_streaming_enabled?: boolean;
}

/** OpenAPIMetadata provides OpenAPI-specific metadata. */
export interface OpenAPIMetadata {
  extensions?: Record<string, unknown>;
  server_variables?: Record<string, ServerVariable>;
  default_security?: string[];
  composition?: CompositionConfig;
}

/** CompositionConfig defines how this schema should be composed with others. */
export interface CompositionConfig {
  include_in_merged: boolean;
  component_prefix?: string;
  tag_prefix?: string;
  operation_id_prefix?: string;
  conflict_strategy: ConflictStrategy;
  preserve_extensions: boolean;
  custom_servers?: OpenAPIServer[];
}

/** OpenAPIServer represents an OpenAPI server definition. */
export interface OpenAPIServer {
  url: string;
  description?: string;
  variables?: Record<string, ServerVariable>;
}

/** ServerVariable describes an OpenAPI server variable. */
export interface ServerVariable {
  default: string;
  enum?: string[];
  description?: string;
}

/** AsyncAPIMetadata provides AsyncAPI-specific metadata. */
export interface AsyncAPIMetadata {
  protocol: string;
  channel_bindings?: Record<string, unknown>;
  message_bindings?: Record<string, unknown>;
}

/** ORPCMetadata provides oRPC-specific metadata. */
export interface ORPCMetadata {
  batch_enabled?: boolean;
  streaming_procedures?: string[];
}

// =============================================================================
// Route Table Types
// =============================================================================

/** RouteDescriptor describes a single route that the service exposes. */
export interface RouteDescriptor {
  path: string;
  methods?: string[];
  protocol: string;
  operation_id?: string;
  timeout?: string;
  rate_limit?: RateLimitConfig;
  cors?: CORSConfig;
  middleware?: MiddlewareDeclaration[];
  cache?: CacheConfig;
  metadata?: Record<string, unknown>;
  public?: boolean;
  deprecated?: boolean;
}

/** RouteUpdateHandler provides atomic route update callbacks. */
export interface RouteUpdateHandler {
  prepareRoutes(routes: RouteDescriptor[]): void;
  commitRoutes(): void;
  rollbackRoutes(): void;
}

// =============================================================================
// Rate Limiting Configuration
// =============================================================================

/** RateLimitConfig provides rate limiting configuration. */
export interface RateLimitConfig {
  requests_per_second?: number;
  burst_size?: number;
  strategy?: RateLimitStrategy;
  key?: RateLimitKey;
  status_code?: number;
  response_headers?: Record<string, string>;
}

// =============================================================================
// Circuit Breaker Configuration
// =============================================================================

/** CircuitBreakerConfig provides circuit breaker configuration hints. */
export interface CircuitBreakerConfig {
  enabled: boolean;
  error_threshold_percent?: number;
  minimum_requests?: number;
  open_duration?: string;
  half_open_requests?: number;
  window_size?: string;
  error_status_codes?: number[];
}

// =============================================================================
// CORS Configuration
// =============================================================================

/** CORSConfig provides Cross-Origin Resource Sharing configuration. */
export interface CORSConfig {
  allowed_origins?: string[];
  allowed_methods?: string[];
  allowed_headers?: string[];
  exposed_headers?: string[];
  allow_credentials?: boolean;
  max_age?: number;
}

// =============================================================================
// Observability Configuration
// =============================================================================

/** ObservabilityConfig provides tracing and metrics configuration hints. */
export interface ObservabilityConfig {
  tracing?: TracingConfig;
  metric_labels?: Record<string, string>;
  log_level?: string;
  sampling_rate?: number;
}

/** TracingConfig provides distributed tracing configuration. */
export interface TracingConfig {
  propagation_format?: string;
  service_name?: string;
  trace_id_header?: string;
  baggage_headers?: string[];
}

// =============================================================================
// Graceful Shutdown Configuration
// =============================================================================

/** GracefulShutdownConfig provides graceful shutdown configuration hints. */
export interface GracefulShutdownConfig {
  drain_timeout?: string;
  shutdown_delay?: string;
  health_failure_threshold?: number;
}

// =============================================================================
// Middleware Declarations
// =============================================================================

/** MiddlewareDeclaration declares a middleware the gateway should apply to routes. */
export interface MiddlewareDeclaration {
  name: string;
  order?: number;
  config?: Record<string, unknown>;
  required?: boolean;
}

// =============================================================================
// Response Caching Configuration
// =============================================================================

/** CacheConfig provides response caching configuration hints. */
export interface CacheConfig {
  enabled: boolean;
  ttl?: string;
  vary_headers?: string[];
  key_template?: string;
  stale_while_revalidate?: string;
  cacheable_statuses?: number[];
}

// =============================================================================
// Load Balancing Configuration
// =============================================================================

/** LoadBalancingConfig provides load balancing configuration hints. */
export interface LoadBalancingConfig {
  strategy: LoadBalancingStrategy;
  sticky_session?: StickySessionConfig;
  health_check?: LBHealthCheckConfig;
}

/** StickySessionConfig provides sticky session configuration. */
export interface StickySessionConfig {
  enabled: boolean;
  cookie_name?: string;
  ttl?: string;
}

/** LBHealthCheckConfig provides load balancer health check configuration. */
export interface LBHealthCheckConfig {
  interval?: string;
  timeout?: string;
  unhealthy_threshold?: number;
  healthy_threshold?: number;
}

// =============================================================================
// API Versioning Configuration
// =============================================================================

/** APIVersioningConfig provides API versioning strategy configuration. */
export interface APIVersioningConfig {
  strategy: VersioningStrategy;
  current_version?: string;
  supported_versions?: string[];
  default_version?: string;
  header_name?: string;
  query_param?: string;
  deprecation_policy?: VersionDeprecationPolicy;
}

/** VersionDeprecationPolicy provides version deprecation and sunset configuration. */
export interface VersionDeprecationPolicy {
  sunset_date?: string;
  deprecation_date?: string;
}

// =============================================================================
// Request/Response Transformation Hints
// =============================================================================

/** TransformationConfig provides request/response transformation hints. */
export interface TransformationConfig {
  request_headers?: Record<string, string>;
  response_headers?: Record<string, string>;
  remove_request_headers?: string[];
  remove_response_headers?: string[];
}

// =============================================================================
// Registry Types
// =============================================================================

/** ManifestEvent represents a manifest change event. */
export interface ManifestEvent {
  type: EventType;
  manifest: SchemaManifest;
  timestamp: number;
}

/** SchemaEvent represents a schema change event. */
export interface SchemaEvent {
  type: EventType;
  path: string;
  schema?: unknown;
  timestamp: number;
}

/** ManifestChangeHandler is called when a manifest changes. */
export type ManifestChangeHandler = (event: ManifestEvent) => void;

/** SchemaChangeHandler is called when a schema changes. */
export type SchemaChangeHandler = (event: SchemaEvent) => void;

/** RegistryConfig holds configuration for a schema registry. */
export interface RegistryConfig {
  backend: string;
  namespace: string;
  backend_config: Record<string, unknown>;
  max_schema_size: number;
  compression_threshold: number;
  ttl: number;
}

/** FetchOptions provides options for fetching schemas. */
export interface FetchOptions {
  use_cache: boolean;
  validate_checksum: boolean;
  expected_hash: string;
  timeout: number;
}

/** PublishOptions provides options for publishing schemas. */
export interface PublishOptions {
  compress: boolean;
  ttl: number;
  overwrite_existing: boolean;
}

/** VersionInfo provides version information about the protocol. */
export interface VersionInfo {
  version: string;
  major: number;
  minor: number;
  patch: number;
}

/** ManifestDiff represents the difference between two manifests. */
export interface ManifestDiff {
  schemas_added: SchemaDescriptor[];
  schemas_removed: SchemaDescriptor[];
  schemas_changed: SchemaChangeDiff[];
  capabilities_added: string[];
  capabilities_removed: string[];
  endpoints_changed: boolean;
  routing_changed: boolean;
  routes_checksum_changed: boolean;
}

/** SchemaChangeDiff represents a changed schema. */
export interface SchemaChangeDiff {
  type: SchemaType;
  old_hash: string;
  new_hash: string;
}
