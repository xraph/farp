package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xraph/farp"
)

// PushHandler is the gateway-side HTTP handler for push-based service registration.
// Services POST their manifests directly to this handler.
//
// It also implements ServiceDiscovery so GatewayNode can use it like any other backend.
//
// Endpoints:
//
//	POST   /register              — service pushes instance + manifest
//	PUT    /heartbeat/{id}        — service sends heartbeat
//	DELETE /deregister/{id}       — service deregisters
//	GET    /services              — list all registered services
//	GET    /services/{name}       — get instances of a service
type PushHandler struct {
	mu               sync.RWMutex
	instances        map[string]*pushEntry // key: instanceID
	watchers         []DiscoveryEventHandler
	heartbeatTimeout time.Duration
	cancel           context.CancelFunc
}

type pushEntry struct {
	Instance    ServiceInstance
	Manifest    *farp.SchemaManifest
	LastSeen    time.Time
}

// NewPushHandler creates a new push registration handler.
// heartbeatTimeout is how long before an instance with no heartbeat is evicted.
func NewPushHandler(heartbeatTimeout time.Duration) *PushHandler {
	if heartbeatTimeout == 0 {
		heartbeatTimeout = 30 * time.Second
	}

	h := &PushHandler{
		instances:        make(map[string]*pushEntry),
		heartbeatTimeout: heartbeatTimeout,
	}

	// Start reaper goroutine
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	go h.reaper(ctx)

	return h
}

// ServeHTTP routes push API requests.
func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/register"):
		h.handleRegister(w, r)
	case r.Method == http.MethodPut && strings.Contains(path, "/heartbeat/"):
		id := path[strings.LastIndex(path, "/")+1:]
		h.handleHeartbeat(w, r, id)
	case r.Method == http.MethodDelete && strings.Contains(path, "/deregister/"):
		id := path[strings.LastIndex(path, "/")+1:]
		h.handleDeregister(w, id)
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/services"):
		h.handleListServices(w)
	case r.Method == http.MethodGet && strings.Contains(path, "/services/"):
		name := path[strings.LastIndex(path, "/")+1:]
		h.handleGetService(w, name)
	default:
		http.NotFound(w, r)
	}
}

func (h *PushHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var reg PushRegistration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if reg.Instance.ID == "" {
		http.Error(w, "instance ID is required", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	existing := h.instances[reg.Instance.ID]
	eventType := farp.EventTypeAdded

	if existing != nil {
		eventType = farp.EventTypeUpdated
	}

	reg.Instance.LastHealthCheck = time.Now()
	if reg.Instance.RegisteredAt.IsZero() {
		reg.Instance.RegisteredAt = time.Now()
	}

	h.instances[reg.Instance.ID] = &pushEntry{
		Instance: reg.Instance,
		Manifest: reg.Manifest,
		LastSeen: time.Now(),
	}

	watchers := make([]DiscoveryEventHandler, len(h.watchers))
	copy(watchers, h.watchers)
	h.mu.Unlock()

	// Notify watchers
	event := DiscoveryEvent{
		Type:      eventType,
		Instance:  reg.Instance,
		Manifest:  reg.Manifest,
		Timestamp: time.Now(),
	}

	for _, w := range watchers {
		w(event)
	}

	// Build ack with checksum (§17.4.1 reconciliation)
	routesChecksum := ""
	schemasApplied := 0

	if entry, ok := h.instances[reg.Instance.ID]; ok && entry.Manifest != nil {
		routesChecksum = entry.Manifest.RoutesChecksum
		schemasApplied = len(entry.Manifest.Schemas)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "registered",
		"routes_checksum": routesChecksum,
		"schemas_applied": schemasApplied,
	})
}

func (h *PushHandler) handleHeartbeat(w http.ResponseWriter, r *http.Request, instanceID string) {
	var hb PushHeartbeat
	if err := json.NewDecoder(r.Body).Decode(&hb); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	entry, ok := h.instances[instanceID]
	if ok {
		entry.LastSeen = time.Now()
		entry.Instance.Status = hb.Status
		entry.Instance.LastHealthCheck = time.Now()
	}
	h.mu.Unlock()

	if !ok {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	// Respond with gateway state for reconciliation (§17.4.1)
	routesChecksum := ""
	schemasApplied := 0

	h.mu.RLock()
	if entry, ok := h.instances[instanceID]; ok && entry.Manifest != nil {
		routesChecksum = entry.Manifest.RoutesChecksum
		schemasApplied = len(entry.Manifest.Schemas)
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"routes_checksum": routesChecksum,
		"schemas_applied": schemasApplied,
	})
}

func (h *PushHandler) handleDeregister(w http.ResponseWriter, instanceID string) {
	h.mu.Lock()
	entry, ok := h.instances[instanceID]
	if ok {
		delete(h.instances, instanceID)
	}

	watchers := make([]DiscoveryEventHandler, len(h.watchers))
	copy(watchers, h.watchers)
	h.mu.Unlock()

	if !ok {
		http.Error(w, "instance not found", http.StatusNotFound)
		return
	}

	// Notify watchers
	event := DiscoveryEvent{
		Type:      farp.EventTypeRemoved,
		Instance:  entry.Instance,
		Timestamp: time.Now(),
	}

	for _, w := range watchers {
		w(event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *PushHandler) handleListServices(w http.ResponseWriter) {
	h.mu.RLock()
	var instances []ServiceInstance
	for _, entry := range h.instances {
		instances = append(instances, entry.Instance)
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(instances)
}

func (h *PushHandler) handleGetService(w http.ResponseWriter, serviceName string) {
	h.mu.RLock()
	var instances []ServiceInstance

	for _, entry := range h.instances {
		if entry.Instance.ServiceName == serviceName {
			instances = append(instances, entry.Instance)
		}
	}
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(instances)
}

// reaper evicts instances that have missed heartbeats.
func (h *PushHandler) reaper(ctx context.Context) {
	ticker := time.NewTicker(h.heartbeatTimeout / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.evictStale()
		}
	}
}

func (h *PushHandler) evictStale() {
	now := time.Now()

	h.mu.Lock()
	var evicted []pushEntry

	for id, entry := range h.instances {
		if now.Sub(entry.LastSeen) > h.heartbeatTimeout {
			evicted = append(evicted, *entry)
			delete(h.instances, id)
		}
	}

	watchers := make([]DiscoveryEventHandler, len(h.watchers))
	copy(watchers, h.watchers)
	h.mu.Unlock()

	// Notify watchers about evicted instances
	for _, entry := range evicted {
		event := DiscoveryEvent{
			Type:      farp.EventTypeRemoved,
			Instance:  entry.Instance,
			Timestamp: now,
		}

		for _, w := range watchers {
			w(event)
		}
	}
}

// =============================================================================
// ServiceDiscovery interface implementation (so GatewayNode can use it)
// =============================================================================

// Discover returns all known instances, optionally filtered by service name.
func (h *PushHandler) Discover(_ context.Context, serviceName string) ([]ServiceInstance, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var instances []ServiceInstance

	for _, entry := range h.instances {
		if serviceName == "" || entry.Instance.ServiceName == serviceName {
			instances = append(instances, entry.Instance)
		}
	}

	return instances, nil
}

// Watch registers a watcher for discovery events.
func (h *PushHandler) Watch(ctx context.Context, _ string, handler DiscoveryEventHandler) error {
	h.mu.Lock()
	h.watchers = append(h.watchers, handler)
	h.mu.Unlock()

	<-ctx.Done()

	return nil
}

// Register adds an instance (same as receiving a push via HTTP).
func (h *PushHandler) Register(_ context.Context, instance ServiceInstance) error {
	h.mu.Lock()
	h.instances[instance.ID] = &pushEntry{
		Instance: instance,
		LastSeen: time.Now(),
	}
	h.mu.Unlock()

	return nil
}

// Deregister removes an instance.
func (h *PushHandler) Deregister(_ context.Context, instanceID string) error {
	h.mu.Lock()
	delete(h.instances, instanceID)
	h.mu.Unlock()

	return nil
}

// ReportHealth updates the health status of an instance.
func (h *PushHandler) ReportHealth(_ context.Context, instanceID string, status farp.InstanceStatus) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	entry, ok := h.instances[instanceID]
	if !ok {
		return farp.ErrInstanceNotFound
	}

	entry.Instance.Status = status
	entry.LastSeen = time.Now()

	return nil
}

// Close stops the reaper goroutine.
func (h *PushHandler) Close() error {
	if h.cancel != nil {
		h.cancel()
	}

	return nil
}

// Health always returns nil (push handler is always "healthy").
func (h *PushHandler) Health(_ context.Context) error {
	return nil
}
