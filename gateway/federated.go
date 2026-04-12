package gateway

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xraph/farp/merger"
)

// FederatedSchemaCache holds pre-serialized merged schemas per protocol.
// JSON serialization happens once at merge time, not on every HTTP request.
// The manifestsHash detects when cached data is stale.
type FederatedSchemaCache struct {
	mu            sync.RWMutex
	manifestsHash string                      // composite hash of all manifest checksums
	result        *merger.MultiProtocolResult // cached merge result
	openAPIJSON   []byte                      // pre-serialized OpenAPI JSON
	asyncAPIJSON  []byte                      // pre-serialized AsyncAPI JSON
	grpcJSON      []byte                      // pre-serialized gRPC JSON
	orpcJSON      []byte                      // pre-serialized oRPC JSON
	lastBuilt     time.Time
}

// FederatedSchemaHandler serves federated/merged schema endpoints.
// It implements http.Handler and should be mounted at a path like /_farp/federated/.
//
// Endpoints:
//
//	GET /openapi.json   - Merged OpenAPI 3.x specification
//	GET /asyncapi.json  - Merged AsyncAPI 2.x specification
//	GET /grpc.json      - Merged gRPC service definitions
//	GET /orpc.json      - Merged oRPC procedures
//	GET /summary        - Merge metadata (services, conflicts, warnings)
//
// Cache invalidation is lazy: manifest changes flow through WatchServices into
// the Client's manifestCache, changing the composite hash. On the next HTTP
// request, ensureFresh detects the hash mismatch and triggers a re-merge.
// There are no background goroutines or timers.
//
// Example usage:
//
//	client := gateway.NewClient(registry)
//	fedHandler := gateway.NewFederatedSchemaHandler(client)
//	http.Handle("/_farp/federated/", http.StripPrefix("/_farp/federated", fedHandler))
type FederatedSchemaHandler struct {
	client   *Client
	cache    *FederatedSchemaCache
	basePath string // path prefix to strip when routing, defaults to ""
}

// FederatedHandlerOption configures a FederatedSchemaHandler.
type FederatedHandlerOption func(*FederatedSchemaHandler)

// WithBasePath sets the base path prefix that will be stripped from incoming
// request paths before routing. For example, if mounting at "/_farp/federated/"
// without http.StripPrefix, set this to "/_farp/federated".
func WithBasePath(basePath string) FederatedHandlerOption {
	return func(h *FederatedSchemaHandler) {
		h.basePath = strings.TrimSuffix(basePath, "/")
	}
}

// NewFederatedSchemaHandler creates a handler that serves federated schema endpoints.
// The client must already be configured and watching services.
//
// Example:
//
//	handler := gateway.NewFederatedSchemaHandler(client, gateway.WithBasePath("/_farp/federated"))
//	http.Handle("/_farp/federated/", handler)
func NewFederatedSchemaHandler(client *Client, opts ...FederatedHandlerOption) *FederatedSchemaHandler {
	h := &FederatedSchemaHandler{
		client: client,
		cache:  &FederatedSchemaCache{},
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// ensureFresh checks whether the cached federated schemas are current.
// If the composite manifest hash has changed, it triggers a re-merge
// and pre-serializes all protocol results to JSON.
func (h *FederatedSchemaHandler) ensureFresh(r *http.Request) error {
	currentHash := h.client.GetManifestsHash()

	h.cache.mu.RLock()
	if h.cache.manifestsHash == currentHash && h.cache.result != nil {
		h.cache.mu.RUnlock()
		return nil
	}
	h.cache.mu.RUnlock()

	// Re-merge needed
	result, err := h.client.GenerateMergedSchemas(r.Context(), "")
	if err != nil {
		return err
	}

	// Pre-serialize each protocol to JSON
	var openAPIJSON, asyncAPIJSON, grpcJSON, orpcJSON []byte

	if result.OpenAPI != nil && result.OpenAPI.Spec != nil {
		openAPIJSON, err = json.MarshalIndent(result.OpenAPI.Spec, "", "  ")
		if err != nil {
			return err
		}
	}

	if result.AsyncAPI != nil && result.AsyncAPI.Spec != nil {
		asyncAPIJSON, err = json.MarshalIndent(result.AsyncAPI.Spec, "", "  ")
		if err != nil {
			return err
		}
	}

	if result.GRPC != nil && result.GRPC.Spec != nil {
		grpcJSON, err = json.MarshalIndent(result.GRPC.Spec, "", "  ")
		if err != nil {
			return err
		}
	}

	if result.ORPC != nil && result.ORPC.Spec != nil {
		orpcJSON, err = json.MarshalIndent(result.ORPC.Spec, "", "  ")
		if err != nil {
			return err
		}
	}

	h.cache.mu.Lock()
	h.cache.manifestsHash = currentHash
	h.cache.result = result
	h.cache.openAPIJSON = openAPIJSON
	h.cache.asyncAPIJSON = asyncAPIJSON
	h.cache.grpcJSON = grpcJSON
	h.cache.orpcJSON = orpcJSON
	h.cache.lastBuilt = time.Now()
	h.cache.mu.Unlock()

	return nil
}

// ServeHTTP implements http.Handler.
// It routes requests to the appropriate pre-serialized schema JSON.
func (h *FederatedSchemaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Determine the sub-path
	path := r.URL.Path
	if h.basePath != "" {
		path = strings.TrimPrefix(path, h.basePath)
	}

	// Normalize: ensure leading slash, strip trailing slash
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	path = strings.TrimSuffix(path, "/")

	if err := h.ensureFresh(r); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		errResp, _ := json.Marshal(map[string]string{"error": err.Error()})
		w.Write(errResp)

		return
	}

	h.cache.mu.RLock()
	defer h.cache.mu.RUnlock()

	switch path {
	case "/openapi.json":
		h.serveJSON(w, h.cache.openAPIJSON)
	case "/asyncapi.json":
		h.serveJSON(w, h.cache.asyncAPIJSON)
	case "/grpc.json":
		h.serveJSON(w, h.cache.grpcJSON)
	case "/orpc.json":
		h.serveJSON(w, h.cache.orpcJSON)
	case "/summary":
		h.serveSummary(w)
	default:
		http.NotFound(w, r)
	}
}

// serveJSON writes pre-serialized JSON with appropriate headers.
func (h *FederatedSchemaHandler) serveJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Farp-Federated", "true")

	if !h.cache.lastBuilt.IsZero() {
		w.Header().Set("X-Farp-Built-At", h.cache.lastBuilt.UTC().Format(time.RFC3339))
	}

	if len(data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"no schemas available for this protocol"}`))

		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// federatedSummary is the JSON structure for the /summary endpoint.
type federatedSummary struct {
	BuiltAt          string              `json:"built_at"`
	HasOpenAPI       bool                `json:"has_openapi"`
	HasAsyncAPI      bool                `json:"has_asyncapi"`
	HasGRPC          bool                `json:"has_grpc"`
	HasORPC          bool                `json:"has_orpc"`
	IncludedServices map[string][]string `json:"included_services,omitempty"`
	TotalConflicts   int                 `json:"total_conflicts"`
	Warnings         []string            `json:"warnings,omitempty"`
}

// serveSummary writes merge metadata as JSON.
func (h *FederatedSchemaHandler) serveSummary(w http.ResponseWriter) {
	summary := federatedSummary{
		HasOpenAPI:  len(h.cache.openAPIJSON) > 0,
		HasAsyncAPI: len(h.cache.asyncAPIJSON) > 0,
		HasGRPC:     len(h.cache.grpcJSON) > 0,
		HasORPC:     len(h.cache.orpcJSON) > 0,
	}

	if !h.cache.lastBuilt.IsZero() {
		summary.BuiltAt = h.cache.lastBuilt.UTC().Format(time.RFC3339)
	}

	if h.cache.result != nil {
		if len(h.cache.result.IncludedServices) > 0 {
			summary.IncludedServices = make(map[string][]string, len(h.cache.result.IncludedServices))
			for schemaType, services := range h.cache.result.IncludedServices {
				summary.IncludedServices[string(schemaType)] = services
			}
		}

		summary.TotalConflicts = h.cache.result.GetTotalConflicts()
		summary.Warnings = h.cache.result.Warnings
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Farp-Federated", "true")
	w.WriteHeader(http.StatusOK)

	json.NewEncoder(w).Encode(summary)
}

// Invalidate forces the next request to rebuild the federated schemas.
// Use this when you know manifests have changed outside of the normal
// WatchServices flow.
func (h *FederatedSchemaHandler) Invalidate() {
	h.cache.mu.Lock()
	h.cache.manifestsHash = ""
	h.cache.mu.Unlock()
}

// LastBuilt returns the time the federated schemas were last built.
// Returns zero time if schemas have never been built.
func (h *FederatedSchemaHandler) LastBuilt() time.Time {
	h.cache.mu.RLock()
	defer h.cache.mu.RUnlock()

	return h.cache.lastBuilt
}
