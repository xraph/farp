package merger

import (
	"fmt"

	"github.com/xraph/farp"
)

// MultiProtocolMerger handles composition of different schema types
type MultiProtocolMerger struct {
	config         MergerConfig
	openAPIMerger  *Merger
	asyncAPIMerger *AsyncAPIMerger
	grpcMerger     *GRPCMerger
	orpcMerger     *ORPCMerger
}

// NewMultiProtocolMerger creates a new multi-protocol merger
func NewMultiProtocolMerger(config MergerConfig) *MultiProtocolMerger {
	return &MultiProtocolMerger{
		config:         config,
		openAPIMerger:  NewMerger(config),
		asyncAPIMerger: NewAsyncAPIMerger(config),
		grpcMerger:     NewGRPCMerger(config),
		orpcMerger:     NewORPCMerger(config),
	}
}

// MultiProtocolResult contains merged specs for all protocol types
type MultiProtocolResult struct {
	OpenAPI          *MergeResult
	AsyncAPI         *AsyncAPIMergeResult
	GRPC             *GRPCMergeResult
	ORPC             *ORPCMergeResult
	IncludedServices map[farp.SchemaType][]string
	Warnings         []string
}

// MergeAll merges schemas across all protocols
func (m *MultiProtocolMerger) MergeAll(manifests []*farp.SchemaManifest, schemaFetcher func(string) (interface{}, error)) (*MultiProtocolResult, error) {
	result := &MultiProtocolResult{
		IncludedServices: make(map[farp.SchemaType][]string),
		Warnings:         []string{},
	}

	// Organize schemas by type
	openAPISchemas := []ServiceSchema{}
	asyncAPISchemas := []AsyncAPIServiceSchema{}
	grpcSchemas := []GRPCServiceSchema{}
	orpcSchemas := []ORPCServiceSchema{}

	for _, manifest := range manifests {
		for _, schemaDesc := range manifest.Schemas {
			// Fetch the schema
			schema, err := schemaFetcher(schemaDesc.Hash)
			if err != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Failed to fetch schema %s for %s: %v",
						schemaDesc.Hash, manifest.ServiceName, err))
				continue
			}

			// Route to appropriate merger based on type
			switch schemaDesc.Type {
			case farp.SchemaTypeOpenAPI:
				openAPISchemas = append(openAPISchemas, ServiceSchema{
					Manifest: manifest,
					Schema:   schema,
				})

			case farp.SchemaTypeAsyncAPI:
				asyncAPISchemas = append(asyncAPISchemas, AsyncAPIServiceSchema{
					Manifest: manifest,
					Schema:   schema,
				})

			case farp.SchemaTypeGRPC:
				grpcSchemas = append(grpcSchemas, GRPCServiceSchema{
					Manifest: manifest,
					Schema:   schema,
				})

			case farp.SchemaTypeORPC:
				orpcSchemas = append(orpcSchemas, ORPCServiceSchema{
					Manifest: manifest,
					Schema:   schema,
				})

			default:
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Unsupported schema type %s for %s",
						schemaDesc.Type, manifest.ServiceName))
			}
		}
	}

	// Merge OpenAPI schemas
	if len(openAPISchemas) > 0 {
		openAPIResult, err := m.openAPIMerger.Merge(openAPISchemas)
		if err != nil {
			return nil, fmt.Errorf("failed to merge OpenAPI schemas: %w", err)
		}
		result.OpenAPI = openAPIResult
		result.IncludedServices[farp.SchemaTypeOpenAPI] = openAPIResult.IncludedServices
	}

	// Merge AsyncAPI schemas
	if len(asyncAPISchemas) > 0 {
		asyncAPIResult, err := m.asyncAPIMerger.MergeAsyncAPI(asyncAPISchemas)
		if err != nil {
			return nil, fmt.Errorf("failed to merge AsyncAPI schemas: %w", err)
		}
		result.AsyncAPI = asyncAPIResult
		result.IncludedServices[farp.SchemaTypeAsyncAPI] = asyncAPIResult.IncludedServices
	}

	// Merge gRPC schemas
	if len(grpcSchemas) > 0 {
		grpcResult, err := m.grpcMerger.MergeGRPC(grpcSchemas)
		if err != nil {
			return nil, fmt.Errorf("failed to merge gRPC schemas: %w", err)
		}
		result.GRPC = grpcResult
		result.IncludedServices[farp.SchemaTypeGRPC] = grpcResult.IncludedServices
	}

	// Merge oRPC schemas
	if len(orpcSchemas) > 0 {
		orpcResult, err := m.orpcMerger.MergeORPC(orpcSchemas)
		if err != nil {
			return nil, fmt.Errorf("failed to merge oRPC schemas: %w", err)
		}
		result.ORPC = orpcResult
		result.IncludedServices[farp.SchemaTypeORPC] = orpcResult.IncludedServices
	}

	return result, nil
}

// GetSummary returns a summary of what was merged
func (r *MultiProtocolResult) GetSummary() string {
	summary := "Multi-Protocol Merge Summary:\n"

	if r.OpenAPI != nil {
		summary += fmt.Sprintf("  OpenAPI: %d services, %d paths\n",
			len(r.OpenAPI.IncludedServices), len(r.OpenAPI.Spec.Paths))
	}

	if r.AsyncAPI != nil {
		summary += fmt.Sprintf("  AsyncAPI: %d services, %d channels\n",
			len(r.AsyncAPI.IncludedServices), len(r.AsyncAPI.Spec.Channels))
	}

	if r.GRPC != nil {
		summary += fmt.Sprintf("  gRPC: %d services, %d service definitions\n",
			len(r.GRPC.IncludedServices), len(r.GRPC.Spec.Services))
	}

	if r.ORPC != nil {
		summary += fmt.Sprintf("  oRPC: %d services, %d procedures\n",
			len(r.ORPC.IncludedServices), len(r.ORPC.Spec.Procedures))
	}

	if len(r.Warnings) > 0 {
		summary += fmt.Sprintf("\nWarnings: %d\n", len(r.Warnings))
	}

	return summary
}

// GetTotalConflicts returns the total number of conflicts across all protocols
func (r *MultiProtocolResult) GetTotalConflicts() int {
	total := 0
	if r.OpenAPI != nil {
		total += len(r.OpenAPI.Conflicts)
	}
	if r.AsyncAPI != nil {
		total += len(r.AsyncAPI.Conflicts)
	}
	if r.GRPC != nil {
		total += len(r.GRPC.Conflicts)
	}
	if r.ORPC != nil {
		total += len(r.ORPC.Conflicts)
	}
	return total
}

// HasProtocol checks if a specific protocol was merged
func (r *MultiProtocolResult) HasProtocol(schemaType farp.SchemaType) bool {
	switch schemaType {
	case farp.SchemaTypeOpenAPI:
		return r.OpenAPI != nil && len(r.OpenAPI.IncludedServices) > 0
	case farp.SchemaTypeAsyncAPI:
		return r.AsyncAPI != nil && len(r.AsyncAPI.IncludedServices) > 0
	case farp.SchemaTypeGRPC:
		return r.GRPC != nil && len(r.GRPC.IncludedServices) > 0
	case farp.SchemaTypeORPC:
		return r.ORPC != nil && len(r.ORPC.IncludedServices) > 0
	default:
		return false
	}
}
