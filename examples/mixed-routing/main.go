// Package main demonstrates mixed routing strategies in OpenAPI composition
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/xraph/farp"
	"github.com/xraph/farp/merger"
)

func main() {
	fmt.Println("FARP Mixed Routing Strategies Example")
	fmt.Println("======================================")

	// Create merger
	m := merger.NewMerger(merger.DefaultMergerConfig())

	// Service 1: Root mounting - public API at /
	fmt.Println("\n1. Public API Service (Root mounting)")
	publicManifest := farp.NewManifest("public-api", "v1.0.0", "public-1")
	publicManifest.Routing.Strategy = farp.MountStrategyRoot // ← At root /
	publicManifest.AddSchema(createOpenAPIDescriptor())

	publicSchema := createSimpleOpenAPI("Public API", "/health", "/version")

	// Service 2: Service-based mounting - internal APIs under /admin
	fmt.Println("2. Admin Service (Service mounting)")
	adminManifest := farp.NewManifest("admin-service", "v2.0.0", "admin-1")
	adminManifest.Routing.Strategy = farp.MountStrategyService // ← Under /admin-service/
	adminManifest.AddSchema(createOpenAPIDescriptor())

	adminSchema := createSimpleOpenAPI("Admin API", "/users", "/config")

	// Service 3: Custom path mounting - legacy API at specific path
	fmt.Println("3. Legacy Service (Custom path mounting)")
	legacyManifest := farp.NewManifest("legacy-api", "v0.9.0", "legacy-1")
	legacyManifest.Routing.Strategy = farp.MountStrategyCustom // ← Custom path
	legacyManifest.Routing.BasePath = "/api/legacy"
	legacyManifest.AddSchema(createOpenAPIDescriptor())

	legacySchema := createSimpleOpenAPI("Legacy API", "/orders", "/products")

	// Service 4: Versioned mounting - API with version in path
	fmt.Println("4. Versioned Service (Versioned mounting)")
	versionedManifest := farp.NewManifest("user-api", "v3.1.0", "user-1")
	versionedManifest.Routing.Strategy = farp.MountStrategyVersioned // ← /user-api/v3.1.0/
	versionedManifest.AddSchema(createOpenAPIDescriptor())

	versionedSchema := createSimpleOpenAPI("User API", "/users", "/profile")

	// Service 5: Instance mounting - multi-instance service
	fmt.Println("5. Multi-Instance Service (Instance mounting)")
	instanceManifest := farp.NewManifest("cache-service", "v1.0.0", "cache-east-1")
	instanceManifest.Routing.Strategy = farp.MountStrategyInstance // ← /cache-east-1/
	instanceManifest.AddSchema(createOpenAPIDescriptor())

	instanceSchema := createSimpleOpenAPI("Cache API", "/get", "/set")

	// Merge all services
	fmt.Println("\n6. Merging all services...")
	schemas := []merger.ServiceSchema{
		{Manifest: publicManifest, Schema: publicSchema},
		{Manifest: adminManifest, Schema: adminSchema},
		{Manifest: legacyManifest, Schema: legacySchema},
		{Manifest: versionedManifest, Schema: versionedSchema},
		{Manifest: instanceManifest, Schema: instanceSchema},
	}

	result, err := m.Merge(schemas)
	if err != nil {
		log.Fatalf("Failed to merge: %v", err)
	}

	// Display results
	fmt.Printf("\n✓ Successfully merged %d services\n\n", len(result.IncludedServices))

	fmt.Println("Merged API Endpoints (showing routing strategies):")
	fmt.Println("───────────────────────────────────────────────────────────")

	fmt.Println("\n📍 Root Mounted (/):")
	for path := range result.Spec.Paths {
		if !hasPrefix(path) {
			fmt.Printf("  %s\n", path)
		}
	}

	fmt.Println("\n📍 Service Mounted (/service-name/):")
	for path := range result.Spec.Paths {
		if isServicePath(path, "admin-service") {
			fmt.Printf("  %s\n", path)
		}
	}

	fmt.Println("\n📍 Custom Path Mounted (/api/legacy/):")
	for path := range result.Spec.Paths {
		if isCustomPath(path, "/api/legacy") {
			fmt.Printf("  %s\n", path)
		}
	}

	fmt.Println("\n📍 Versioned Mounted (/service/version/):")
	for path := range result.Spec.Paths {
		if isVersionedPath(path, "user-api") {
			fmt.Printf("  %s\n", path)
		}
	}

	fmt.Println("\n📍 Instance Mounted (/instance-id/):")
	for path := range result.Spec.Paths {
		if isInstancePath(path, "cache-east-1") {
			fmt.Printf("  %s\n", path)
		}
	}

	fmt.Println("\n───────────────────────────────────────────────────────────")
	fmt.Printf("\nTotal Endpoints: %d\n", len(result.Spec.Paths))

	// Show JSON snippet
	jsonBytes, _ := json.MarshalIndent(map[string]interface{}{
		"info":  result.Spec.Info,
		"paths": getSamplePaths(result.Spec.Paths, 3),
	}, "", "  ")

	fmt.Println("\nMerged Specification Sample:")
	fmt.Println(string(jsonBytes))

	fmt.Println("\n✓ All routing strategies successfully applied!")
}

func createOpenAPIDescriptor() farp.SchemaDescriptor {
	return farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "hash-" + fmt.Sprintf("%d", len("random")),
		Size:        1024,
		Metadata: &farp.ProtocolMetadata{
			OpenAPI: &farp.OpenAPIMetadata{
				Composition: &farp.CompositionConfig{
					IncludeInMerged:  true,
					ConflictStrategy: farp.ConflictStrategyPrefix,
				},
			},
		},
	}
}

func createSimpleOpenAPI(title, path1, path2 string) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":   title,
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			path1: map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "get" + path1,
					"summary":     "Get " + path1,
				},
			},
			path2: map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "get" + path2,
					"summary":     "Get " + path2,
				},
			},
		},
	}
}

// Helper functions
func hasPrefix(path string) bool {
	return len(path) > 1 && path[0] == '/' && path[1] != '/' &&
		(containsPath(path, "/admin-service") ||
			containsPath(path, "/api/legacy") ||
			containsPath(path, "/user-api") ||
			containsPath(path, "/cache-east-1"))
}

func containsPath(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

func isServicePath(path, service string) bool {
	return containsPath(path, "/"+service)
}

func isCustomPath(path, prefix string) bool {
	return containsPath(path, prefix)
}

func isVersionedPath(path, service string) bool {
	return containsPath(path, "/"+service+"/v")
}

func isInstancePath(path, instance string) bool {
	return containsPath(path, "/"+instance)
}

func getSamplePaths(paths map[string]merger.PathItem, limit int) map[string]interface{} {
	result := make(map[string]interface{})
	count := 0
	for path := range paths {
		if count >= limit {
			break
		}
		result[path] = "..."
		count++
	}
	return result
}
