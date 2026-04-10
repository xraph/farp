package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/xraph/farp"
)

// FARPHandler serves FARP protocol HTTP endpoints for a service.
// Mount this on your HTTP router to expose manifest, health, and schema endpoints.
//
// Endpoints served (relative to mount point):
//
//	GET /_farp/manifest         — returns SchemaManifest JSON
//	GET /_farp/health           — returns health status (200 or 503)
//	GET /_farp/schemas/{type}   — returns schema by type (openapi, asyncapi, graphql, etc.)
type FARPHandler struct {
	mu       sync.RWMutex
	manifest *farp.SchemaManifest
	schemas  map[farp.SchemaType]any
	status   farp.InstanceStatus
}

// NewFARPHandler creates a new FARP HTTP handler.
func NewFARPHandler(manifest *farp.SchemaManifest, schemas map[farp.SchemaType]any) *FARPHandler {
	if schemas == nil {
		schemas = make(map[farp.SchemaType]any)
	}

	return &FARPHandler{
		manifest: manifest,
		schemas:  schemas,
		status:   farp.InstanceStatusHealthy,
	}
}

// ServeHTTP implements http.Handler.
func (h *FARPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/_farp")
	if path == r.URL.Path {
		// Try without leading slash variation
		path = r.URL.Path
	}

	switch {
	case path == "/manifest" || path == "/_farp/manifest":
		h.serveManifest(w, r)
	case path == "/health" || path == "/_farp/health":
		h.serveHealth(w, r)
	case strings.HasPrefix(path, "/schemas/") || strings.HasPrefix(path, "/_farp/schemas/"):
		schemaType := strings.TrimPrefix(path, "/schemas/")
		schemaType = strings.TrimPrefix(schemaType, "/_farp/schemas/")
		h.serveSchema(w, r, schemaType)
	default:
		http.NotFound(w, r)
	}
}

func (h *FARPHandler) serveManifest(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	manifest := h.manifest
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		http.Error(w, "failed to encode manifest", http.StatusInternalServerError)
	}
}

func (h *FARPHandler) serveHealth(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	status := h.status
	h.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	switch status {
	case farp.InstanceStatusHealthy:
		w.WriteHeader(http.StatusOK)
	case farp.InstanceStatusDegraded:
		w.WriteHeader(http.StatusOK) // Degraded is still serving
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": string(status),
	})
}

func (h *FARPHandler) serveSchema(w http.ResponseWriter, _ *http.Request, schemaType string) {
	h.mu.RLock()
	schema, ok := h.schemas[farp.SchemaType(schemaType)]
	h.mu.RUnlock()

	if !ok {
		http.Error(w, "schema not found: "+schemaType, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(schema); err != nil {
		http.Error(w, "failed to encode schema", http.StatusInternalServerError)
	}
}

// SetHealth sets the health status returned by /_farp/health.
func (h *FARPHandler) SetHealth(status farp.InstanceStatus) {
	h.mu.Lock()
	h.status = status
	h.mu.Unlock()
}

// UpdateManifest updates the manifest served by /_farp/manifest.
func (h *FARPHandler) UpdateManifest(manifest *farp.SchemaManifest) {
	h.mu.Lock()
	h.manifest = manifest
	h.mu.Unlock()
}

// UpdateSchema updates a schema served by /_farp/schemas/{type}.
func (h *FARPHandler) UpdateSchema(schemaType farp.SchemaType, schema any) {
	h.mu.Lock()
	h.schemas[schemaType] = schema
	h.mu.Unlock()
}

// HTTPManifestFetcher fetches manifests from service instances via HTTP.
// It GETs /_farp/manifest from the service address.
type HTTPManifestFetcher struct {
	client *http.Client
}

// NewHTTPManifestFetcher creates a new HTTP-based manifest fetcher.
func NewHTTPManifestFetcher(client *http.Client) *HTTPManifestFetcher {
	if client == nil {
		client = &http.Client{}
	}

	return &HTTPManifestFetcher{client: client}
}

// FetchManifest fetches the SchemaManifest from a service instance via HTTP.
func (f *HTTPManifestFetcher) FetchManifest(_ context.Context, instance ServiceInstance) (*farp.SchemaManifest, error) {
	url := "http://" + instance.Address
	if instance.Port > 0 {
		url = fmt.Sprintf("http://%s:%d", instance.Address, instance.Port)
	}

	url += "/_farp/manifest"

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", farp.ErrManifestFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d from %s", farp.ErrManifestFetchFailed, resp.StatusCode, url)
	}

	var manifest farp.SchemaManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON: %w", farp.ErrManifestFetchFailed, err)
	}

	return &manifest, nil
}
