package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/xraph/farp"
)

// PushDiscovery implements ServiceDiscovery by pushing manifests directly
// to a gateway via HTTP. No external service registry needed.
//
// This is the service-side component of push-based discovery.
// The gateway-side component is PushHandler.
type PushDiscovery struct {
	gatewayURL string // base URL, e.g., "http://gateway:9090/_farp/v1"
	client     *http.Client
}

// NewPushDiscovery creates a new push-based discovery client.
// gatewayURL is the base URL of the gateway's FARP push endpoint.
func NewPushDiscovery(gatewayURL string, client *http.Client) *PushDiscovery {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	return &PushDiscovery{
		gatewayURL: strings.TrimSuffix(gatewayURL, "/"),
		client:     client,
	}
}

// PushRegistration is the body sent when a service pushes its manifest to the gateway.
type PushRegistration struct {
	Instance ServiceInstance      `json:"instance"`
	Manifest *farp.SchemaManifest `json:"manifest"`
}

// PushHeartbeat is the body sent for heartbeat updates.
// RoutesChecksum is optional (§17.4.1 reconciliation).
type PushHeartbeat struct {
	Status         farp.InstanceStatus `json:"status"`
	RoutesChecksum string              `json:"routes_checksum,omitempty"`
}

// PushRegistrationResponse is the gateway's response to a registration.
type PushRegistrationResponse struct {
	Status         string `json:"status"`
	RoutesChecksum string `json:"routes_checksum"`
	SchemasApplied int    `json:"schemas_applied"`
}

// PushHeartbeatResponse is the gateway's response to a heartbeat.
type PushHeartbeatResponse struct {
	Status         string `json:"status"`
	RoutesChecksum string `json:"routes_checksum"`
	SchemasApplied int    `json:"schemas_applied"`
}

// Register pushes the service instance to the gateway.
func (p *PushDiscovery) Register(ctx context.Context, instance ServiceInstance) error {
	_, err := p.doRegister(ctx, PushRegistration{Instance: instance})
	return err
}

// RegisterWithManifest pushes both the instance and manifest to the gateway.
// Use this when the gateway needs the manifest directly (e.g. after a
// heartbeat reconciliation mismatch — see §17.4.1).
func (p *PushDiscovery) RegisterWithManifest(ctx context.Context, instance ServiceInstance, manifest *farp.SchemaManifest) (*PushRegistrationResponse, error) {
	return p.doRegister(ctx, PushRegistration{Instance: instance, Manifest: manifest})
}

func (p *PushDiscovery) doRegister(ctx context.Context, payload PushRegistration) (*PushRegistrationResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.gatewayURL+"/register", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrRegistrationFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("%w: HTTP %d", farp.ErrRegistrationFailed, resp.StatusCode)
	}

	var ack PushRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		// Gateway may not support ack yet — treat as success with empty state
		return &PushRegistrationResponse{Status: "registered"}, nil
	}

	return &ack, nil
}

// Deregister removes the service from the gateway.
func (p *PushDiscovery) Deregister(ctx context.Context, instanceID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.gatewayURL+"/deregister/"+instanceID, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDeregistrationFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("%w: HTTP %d", farp.ErrDeregistrationFailed, resp.StatusCode)
	}

	return nil
}

// ReportHealth sends a heartbeat to the gateway (without checksum).
// Satisfies the ServiceDiscovery interface.
func (p *PushDiscovery) ReportHealth(ctx context.Context, instanceID string, status farp.InstanceStatus) error {
	_, err := p.ReportHealthWithChecksum(ctx, instanceID, status, "")
	return err
}

// ReportHealthWithChecksum sends a heartbeat with the service's expected
// routes_checksum for reconciliation (FARP spec §17.4.1).
// The response includes the gateway's applied checksum and schema count.
func (p *PushDiscovery) ReportHealthWithChecksum(ctx context.Context, instanceID string, status farp.InstanceStatus, routesChecksum string) (*PushHeartbeatResponse, error) {
	payload := PushHeartbeat{
		Status:         status,
		RoutesChecksum: routesChecksum,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, p.gatewayURL+"/heartbeat/"+instanceID, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrHealthCheckFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", farp.ErrHealthCheckFailed, resp.StatusCode)
	}

	var ack PushHeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ack); err != nil {
		// Gateway may not support ack yet
		return &PushHeartbeatResponse{Status: "ok"}, nil
	}

	return &ack, nil
}

// Discover queries the gateway for instances of a service.
func (p *PushDiscovery) Discover(ctx context.Context, serviceName string) ([]ServiceInstance, error) {
	url := p.gatewayURL + "/services"
	if serviceName != "" {
		url += "/" + serviceName
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}
	defer resp.Body.Close()

	var instances []ServiceInstance
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return nil, err
	}

	return instances, nil
}

// Watch is not natively supported in push mode from the service side.
// Services push to the gateway; they don't watch it.
// This is a no-op that blocks until context is cancelled.
func (p *PushDiscovery) Watch(ctx context.Context, _ string, _ DiscoveryEventHandler) error {
	<-ctx.Done()
	return nil
}

// Close is a no-op for push discovery.
func (p *PushDiscovery) Close() error {
	return nil
}

// Health checks if the gateway is reachable.
func (p *PushDiscovery) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.gatewayURL+"/services", nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", farp.ErrDiscoveryUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: HTTP %d", farp.ErrDiscoveryUnavailable, resp.StatusCode)
	}

	return nil
}
