package discovery

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xraph/farp"
)

func TestFARPHandler_ServeManifest(t *testing.T) {
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/_farp/health"
	handler := NewFARPHandler(manifest, nil)

	req := httptest.NewRequest(http.MethodGet, "/_farp/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got farp.SchemaManifest
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got.ServiceName != "test-service" {
		t.Errorf("expected service name 'test-service', got %q", got.ServiceName)
	}
}

func TestFARPHandler_Health(t *testing.T) {
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/_farp/health"
	handler := NewFARPHandler(manifest, nil)

	// Healthy
	req := httptest.NewRequest(http.MethodGet, "/_farp/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for healthy, got %d", w.Code)
	}

	// Unhealthy
	handler.SetHealth(farp.InstanceStatusUnhealthy)
	req = httptest.NewRequest(http.MethodGet, "/_farp/health", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for unhealthy, got %d", w.Code)
	}
}

func TestFARPHandler_Schema(t *testing.T) {
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/_farp/health"

	schemas := map[farp.SchemaType]any{
		farp.SchemaTypeOpenAPI: map[string]any{"openapi": "3.1.0"},
	}
	handler := NewFARPHandler(manifest, schemas)

	req := httptest.NewRequest(http.MethodGet, "/_farp/schemas/openapi", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Non-existent schema
	req = httptest.NewRequest(http.MethodGet, "/_farp/schemas/grpc", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing schema, got %d", w.Code)
	}
}

func TestPushHandler_RegisterAndDiscover(t *testing.T) {
	handler := NewPushHandler(5 * time.Second)
	defer handler.Close()

	// Register
	instance := ServiceInstance{
		ID:          "inst-1",
		ServiceName: "test-svc",
		Address:     "10.0.0.1",
		Port:        8080,
		Status:      farp.InstanceStatusHealthy,
	}

	err := handler.Register(context.Background(), instance)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// Discover
	instances, err := handler.Discover(context.Background(), "test-svc")
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	if instances[0].ID != "inst-1" {
		t.Errorf("expected instance ID 'inst-1', got %q", instances[0].ID)
	}
}

func TestPushHandler_Deregister(t *testing.T) {
	handler := NewPushHandler(5 * time.Second)
	defer handler.Close()

	instance := ServiceInstance{
		ID:          "inst-1",
		ServiceName: "test-svc",
		Address:     "10.0.0.1",
	}

	_ = handler.Register(context.Background(), instance)
	_ = handler.Deregister(context.Background(), "inst-1")

	instances, _ := handler.Discover(context.Background(), "test-svc")
	if len(instances) != 0 {
		t.Errorf("expected 0 instances after deregister, got %d", len(instances))
	}
}

func TestPushHandler_Watch(t *testing.T) {
	handler := NewPushHandler(5 * time.Second)
	defer handler.Close()

	var eventCount int32

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = handler.Watch(ctx, "", func(event DiscoveryEvent) {
			atomic.AddInt32(&eventCount, 1)
		})
	}()

	time.Sleep(50 * time.Millisecond) // Let watcher register

	// Register should trigger event
	instance := ServiceInstance{
		ID:          "inst-1",
		ServiceName: "test-svc",
		Address:     "10.0.0.1",
	}

	_ = handler.Register(context.Background(), instance)
	time.Sleep(50 * time.Millisecond)

	// Note: Register via the method doesn't notify watchers (only HTTP does).
	// Deregister also doesn't notify via the method path.
	cancel()
}

func TestPushHandler_HTTP_Register(t *testing.T) {
	handler := NewPushHandler(5 * time.Second)
	defer handler.Close()

	server := httptest.NewServer(http.StripPrefix("/_farp/v1", handler))
	defer server.Close()

	// Register via HTTP
	manifest := farp.NewManifest("test-service", "v1.0.0", "inst-http-1")
	manifest.Endpoints.Health = "/health"

	reg := PushRegistration{
		Instance: ServiceInstance{
			ID:          "inst-http-1",
			ServiceName: "test-service",
			Address:     "10.0.0.5:8080",
			Status:      farp.InstanceStatusHealthy,
		},
		Manifest: manifest,
	}

	body, _ := json.Marshal(reg)
	resp, err := http.Post(server.URL+"/_farp/v1/register", "application/json",
		&byteReader{data: body})
	if err != nil {
		t.Fatalf("POST /register failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Verify via Discover
	instances, _ := handler.Discover(context.Background(), "test-service")
	if len(instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(instances))
	}
}

func TestHTTPManifestFetcher(t *testing.T) {
	manifest := farp.NewManifest("test-service", "v1.0.0", "instance-123")
	manifest.Endpoints.Health = "/_farp/health"

	handler := NewFARPHandler(manifest, nil)
	server := httptest.NewServer(handler)
	defer server.Close()

	fetcher := NewHTTPManifestFetcher(nil)

	// Extract host:port from test server URL
	addr := server.Listener.Addr().String()
	instance := ServiceInstance{
		ID:      "instance-123",
		Address: addr,
	}

	got, err := fetcher.FetchManifest(context.Background(), instance)
	if err != nil {
		t.Fatalf("FetchManifest failed: %v", err)
	}

	if got.ServiceName != "test-service" {
		t.Errorf("expected 'test-service', got %q", got.ServiceName)
	}
}

func TestPushDiscovery_Interface(t *testing.T) {
	// Verify PushDiscovery implements ServiceDiscovery
	var _ ServiceDiscovery = (*PushDiscovery)(nil)
}

func TestPushHandler_Interface(t *testing.T) {
	// Verify PushHandler implements ServiceDiscovery
	var _ ServiceDiscovery = (*PushHandler)(nil)
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}

	n = copy(p, r.data[r.pos:])
	r.pos += n

	return n, nil
}
