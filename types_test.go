package farp

import "testing"

func TestSchemaType_IsValid(t *testing.T) {
	tests := []struct {
		name       string
		schemaType SchemaType
		want       bool
	}{
		{"OpenAPI", SchemaTypeOpenAPI, true},
		{"AsyncAPI", SchemaTypeAsyncAPI, true},
		{"gRPC", SchemaTypeGRPC, true},
		{"GraphQL", SchemaTypeGraphQL, true},
		{"oRPC", SchemaTypeORPC, true},
		{"Thrift", SchemaTypeThrift, true},
		{"Avro", SchemaTypeAvro, true},
		{"Custom", SchemaTypeCustom, true},
		{"Invalid", SchemaType("invalid"), false},
		{"Empty", SchemaType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schemaType.IsValid(); got != tt.want {
				t.Errorf("SchemaType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSchemaType_String(t *testing.T) {
	tests := []struct {
		name       string
		schemaType SchemaType
		want       string
	}{
		{"OpenAPI", SchemaTypeOpenAPI, "openapi"},
		{"AsyncAPI", SchemaTypeAsyncAPI, "asyncapi"},
		{"gRPC", SchemaTypeGRPC, "grpc"},
		{"GraphQL", SchemaTypeGraphQL, "graphql"},
		{"oRPC", SchemaTypeORPC, "orpc"},
		{"Thrift", SchemaTypeThrift, "thrift"},
		{"Avro", SchemaTypeAvro, "avro"},
		{"Custom", SchemaTypeCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schemaType.String(); got != tt.want {
				t.Errorf("SchemaType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocationType_IsValid(t *testing.T) {
	tests := []struct {
		name         string
		locationType LocationType
		want         bool
	}{
		{"HTTP", LocationTypeHTTP, true},
		{"Registry", LocationTypeRegistry, true},
		{"Inline", LocationTypeInline, true},
		{"Invalid", LocationType("invalid"), false},
		{"Empty", LocationType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.locationType.IsValid(); got != tt.want {
				t.Errorf("LocationType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocationType_String(t *testing.T) {
	tests := []struct {
		name         string
		locationType LocationType
		want         string
	}{
		{"HTTP", LocationTypeHTTP, "http"},
		{"Registry", LocationTypeRegistry, "registry"},
		{"Inline", LocationTypeInline, "inline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.locationType.String(); got != tt.want {
				t.Errorf("LocationType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapability_String(t *testing.T) {
	tests := []struct {
		name       string
		capability Capability
		want       string
	}{
		{"REST", CapabilityREST, "rest"},
		{"gRPC", CapabilityGRPC, "grpc"},
		{"WebSocket", CapabilityWebSocket, "websocket"},
		{"SSE", CapabilitySSE, "sse"},
		{"GraphQL", CapabilityGraphQL, "graphql"},
		{"MQTT", CapabilityMQTT, "mqtt"},
		{"AMQP", CapabilityAMQP, "amqp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.capability.String(); got != tt.want {
				t.Errorf("Capability.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      string
	}{
		{"Added", EventTypeAdded, "added"},
		{"Updated", EventTypeUpdated, "updated"},
		{"Removed", EventTypeRemoved, "removed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.want {
				t.Errorf("EventType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultRegistryConfig(t *testing.T) {
	config := DefaultRegistryConfig()

	if config.Backend != "memory" {
		t.Errorf("expected backend 'memory', got '%s'", config.Backend)
	}

	if config.Namespace != "farp" {
		t.Errorf("expected namespace 'farp', got '%s'", config.Namespace)
	}

	if config.MaxSchemaSize != 1024*1024 {
		t.Errorf("expected max schema size 1MB, got %d", config.MaxSchemaSize)
	}

	if config.CompressionThreshold != 100*1024 {
		t.Errorf("expected compression threshold 100KB, got %d", config.CompressionThreshold)
	}

	if config.TTL != 0 {
		t.Errorf("expected TTL 0, got %d", config.TTL)
	}

	if config.BackendConfig == nil {
		t.Error("expected BackendConfig to be initialized")
	}
}

func TestInstanceStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status InstanceStatus
		want   string
	}{
		{"Starting", InstanceStatusStarting, "starting"},
		{"Healthy", InstanceStatusHealthy, "healthy"},
		{"Degraded", InstanceStatusDegraded, "degraded"},
		{"Unhealthy", InstanceStatusUnhealthy, "unhealthy"},
		{"Draining", InstanceStatusDraining, "draining"},
		{"Stopping", InstanceStatusStopping, "stopping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("InstanceStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInstanceRole_String(t *testing.T) {
	tests := []struct {
		name string
		role InstanceRole
		want string
	}{
		{"Primary", InstanceRolePrimary, "primary"},
		{"Canary", InstanceRoleCanary, "canary"},
		{"Blue", InstanceRoleBlue, "blue"},
		{"Green", InstanceRoleGreen, "green"},
		{"Shadow", InstanceRoleShadow, "shadow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("InstanceRole.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeploymentStrategy_String(t *testing.T) {
	tests := []struct {
		name     string
		strategy DeploymentStrategy
		want     string
	}{
		{"Rolling", DeploymentStrategyRolling, "rolling"},
		{"Canary", DeploymentStrategyCanary, "canary"},
		{"BlueGreen", DeploymentStrategyBlueGreen, "blue_green"},
		{"Shadow", DeploymentStrategyShadow, "shadow"},
		{"Recreate", DeploymentStrategyRecreate, "recreate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.String(); got != tt.want {
				t.Errorf("DeploymentStrategy.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMountStrategy_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		strategy MountStrategy
		want     bool
	}{
		{"Root", MountStrategyRoot, true},
		{"Instance", MountStrategyInstance, true},
		{"Service", MountStrategyService, true},
		{"Versioned", MountStrategyVersioned, true},
		{"Custom", MountStrategyCustom, true},
		{"Subdomain", MountStrategySubdomain, true},
		{"Invalid", MountStrategy("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.IsValid(); got != tt.want {
				t.Errorf("MountStrategy.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthType_String(t *testing.T) {
	tests := []struct {
		name     string
		authType AuthType
		want     string
	}{
		{"Bearer", AuthTypeBearer, "bearer"},
		{"APIKey", AuthTypeAPIKey, "apikey"},
		{"Basic", AuthTypeBasic, "basic"},
		{"MTLS", AuthTypeMTLS, "mtls"},
		{"OAuth2", AuthTypeOAuth2, "oauth2"},
		{"OIDC", AuthTypeOIDC, "oidc"},
		{"Custom", AuthTypeCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.authType.String(); got != tt.want {
				t.Errorf("AuthType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompatibilityMode_String(t *testing.T) {
	tests := []struct {
		name string
		mode CompatibilityMode
		want string
	}{
		{"Backward", CompatibilityBackward, "backward"},
		{"Forward", CompatibilityForward, "forward"},
		{"Full", CompatibilityFull, "full"},
		{"None", CompatibilityNone, "none"},
		{"BackwardTransitive", CompatibilityBackwardTransitive, "backward_transitive"},
		{"ForwardTransitive", CompatibilityForwardTransitive, "forward_transitive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("CompatibilityMode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		name       string
		changeType ChangeType
		want       string
	}{
		{"FieldRemoved", ChangeTypeFieldRemoved, "field_removed"},
		{"FieldTypeChanged", ChangeTypeFieldTypeChanged, "field_type_changed"},
		{"FieldRequired", ChangeTypeFieldRequired, "field_required"},
		{"EndpointRemoved", ChangeTypeEndpointRemoved, "endpoint_removed"},
		{"EndpointChanged", ChangeTypeEndpointChanged, "endpoint_changed"},
		{"EnumValueRemoved", ChangeTypeEnumValueRemoved, "enum_value_removed"},
		{"MethodRemoved", ChangeTypeMethodRemoved, "method_removed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.changeType.String(); got != tt.want {
				t.Errorf("ChangeType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChangeSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity ChangeSeverity
		want     string
	}{
		{"Critical", SeverityCritical, "critical"},
		{"High", SeverityHigh, "high"},
		{"Medium", SeverityMedium, "medium"},
		{"Low", SeverityLow, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("ChangeSeverity.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDataSensitivity_String(t *testing.T) {
	tests := []struct {
		name        string
		sensitivity DataSensitivity
		want        string
	}{
		{"Public", SensitivityPublic, "public"},
		{"Internal", SensitivityInternal, "internal"},
		{"Confidential", SensitivityConfidential, "confidential"},
		{"PII", SensitivityPII, "pii"},
		{"PHI", SensitivityPHI, "phi"},
		{"PCI", SensitivityPCI, "pci"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sensitivity.String(); got != tt.want {
				t.Errorf("DataSensitivity.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSizeHint_String(t *testing.T) {
	tests := []struct {
		name string
		hint SizeHint
		want string
	}{
		{"Small", SizeSmall, "small"},
		{"Medium", SizeMedium, "medium"},
		{"Large", SizeLarge, "large"},
		{"XLarge", SizeXLarge, "xlarge"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hint.String(); got != tt.want {
				t.Errorf("SizeHint.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookEventType_String(t *testing.T) {
	tests := []struct {
		name      string
		eventType WebhookEventType
		want      string
	}{
		{"SchemaUpdated", EventSchemaUpdated, "schema.updated"},
		{"HealthChanged", EventHealthChanged, "health.changed"},
		{"InstanceScaling", EventInstanceScaling, "instance.scaling"},
		{"MaintenanceMode", EventMaintenanceMode, "maintenance.mode"},
		{"RateLimitChanged", EventRateLimitChanged, "ratelimit.changed"},
		{"CircuitBreakerOpen", EventCircuitBreakerOpen, "circuit.breaker.open"},
		{"CircuitBreakerClosed", EventCircuitBreakerClosed, "circuit.breaker.closed"},
		{"ConfigUpdated", EventConfigUpdated, "config.updated"},
		{"TrafficShift", EventTrafficShift, "traffic.shift"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.want {
				t.Errorf("WebhookEventType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommunicationRouteType_String(t *testing.T) {
	tests := []struct {
		name      string
		routeType CommunicationRouteType
		want      string
	}{
		{"Control", RouteTypeControl, "control"},
		{"Admin", RouteTypeAdmin, "admin"},
		{"Management", RouteTypeManagement, "management"},
		{"LifecycleStart", RouteTypeLifecycleStart, "lifecycle.start"},
		{"LifecycleStop", RouteTypeLifecycleStop, "lifecycle.stop"},
		{"LifecycleReload", RouteTypeLifecycleReload, "lifecycle.reload"},
		{"ConfigUpdate", RouteTypeConfigUpdate, "config.update"},
		{"ConfigQuery", RouteTypeConfigQuery, "config.query"},
		{"EventPoll", RouteTypeEventPoll, "event.poll"},
		{"EventAck", RouteTypeEventAck, "event.ack"},
		{"HealthCheck", RouteTypeHealthCheck, "health.check"},
		{"StatusQuery", RouteTypeStatusQuery, "status.query"},
		{"SchemaQuery", RouteTypeSchemaQuery, "schema.query"},
		{"SchemaValidate", RouteTypeSchemaValidate, "schema.validate"},
		{"MetricsQuery", RouteTypeMetricsQuery, "metrics.query"},
		{"TracingExport", RouteTypeTracingExport, "tracing.export"},
		{"Custom", RouteTypeCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.routeType.String(); got != tt.want {
				t.Errorf("CommunicationRouteType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
