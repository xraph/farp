// Package main demonstrates OpenAPI schema composition with FARP
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/xraph/farp"
	"github.com/xraph/farp/merger"
)

func main() {
	fmt.Println("FARP OpenAPI Schema Composition Example")
	fmt.Println("========================================")

	// Create merger with custom config
	mergerConfig := merger.MergerConfig{
		DefaultConflictStrategy: farp.ConflictStrategyPrefix,
		MergedTitle:             "Unified E-Commerce API",
		MergedDescription:       "Composed API from User, Product, and Order services",
		MergedVersion:           "1.0.0",
		IncludeServiceTags:      true,
		SortOutput:              true,
		Servers: []merger.Server{
			{
				URL:         "https://api.example.com",
				Description: "Production API Gateway",
			},
		},
	}

	m := merger.NewMerger(mergerConfig)

	// Create three service schemas
	fmt.Println("1. Creating User Service schema...")
	userServiceSchema := merger.ServiceSchema{
		Manifest: createUserServiceManifest(),
		Schema:   createUserServiceOpenAPI(),
	}

	fmt.Println("2. Creating Product Service schema...")
	productServiceSchema := merger.ServiceSchema{
		Manifest: createProductServiceManifest(),
		Schema:   createProductServiceOpenAPI(),
	}

	fmt.Println("3. Creating Order Service schema...")
	orderServiceSchema := merger.ServiceSchema{
		Manifest: createOrderServiceManifest(),
		Schema:   createOrderServiceOpenAPI(),
	}

	// Merge all schemas
	fmt.Println("\n4. Merging OpenAPI specifications...")
	schemas := []merger.ServiceSchema{
		userServiceSchema,
		productServiceSchema,
		orderServiceSchema,
	}

	result, err := m.Merge(schemas)
	if err != nil {
		log.Fatalf("Failed to merge schemas: %v", err)
	}

	// Display results
	fmt.Printf("\n✓ Successfully merged %d services\n", len(result.IncludedServices))
	fmt.Printf("  Services: %v\n\n", result.IncludedServices)

	fmt.Printf("Merged Specification Details:\n")
	fmt.Printf("  Title: %s\n", result.Spec.Info.Title)
	fmt.Printf("  Version: %s\n", result.Spec.Info.Version)
	fmt.Printf("  Total Paths: %d\n", len(result.Spec.Paths))
	fmt.Printf("  Total Components: %d\n\n", len(result.Spec.Components.Schemas))

	// Display paths
	fmt.Println("API Endpoints:")
	for path, pathItem := range result.Spec.Paths {
		methods := []string{}
		if pathItem.Get != nil {
			methods = append(methods, "GET")
		}
		if pathItem.Post != nil {
			methods = append(methods, "POST")
		}
		if pathItem.Put != nil {
			methods = append(methods, "PUT")
		}
		if pathItem.Delete != nil {
			methods = append(methods, "DELETE")
		}
		fmt.Printf("  - %-40s [%v]\n", path, methods)
	}

	// Display component schemas
	fmt.Println("\nComponent Schemas:")
	for name := range result.Spec.Components.Schemas {
		fmt.Printf("  - %s\n", name)
	}

	// Display conflicts
	if len(result.Conflicts) > 0 {
		fmt.Printf("\nConflicts Resolved: %d\n", len(result.Conflicts))
		for i, conflict := range result.Conflicts {
			fmt.Printf("  %d. Type: %s, Item: %s\n", i+1, conflict.Type, conflict.Item)
			fmt.Printf("     Services: %v\n", conflict.Services)
			fmt.Printf("     Resolution: %s\n", conflict.Resolution)
		}
	} else {
		fmt.Println("\n✓ No conflicts encountered")
	}

	// Display warnings
	if len(result.Warnings) > 0 {
		fmt.Printf("\nWarnings: %d\n", len(result.Warnings))
		for i, warning := range result.Warnings {
			fmt.Printf("  %d. %s\n", i+1, warning)
		}
	}

	// Export as JSON (pretty print)
	fmt.Println("\n5. Exporting Merged Specification...")
	jsonBytes, err := json.MarshalIndent(result.Spec, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nMerged OpenAPI Specification (truncated):")
	fmt.Println(string(jsonBytes[:min(500, len(jsonBytes))]) + "...")

	fmt.Println("\n✓ Composition complete!")
	fmt.Println("\nThe merged specification includes all paths, components, and tags")
	fmt.Println("from the three services with automatic conflict resolution.")
}

func createUserServiceManifest() *farp.SchemaManifest {
	manifest := farp.NewManifest("user-service", "v1.0.0", "user-instance-1")
	manifest.Routing.Strategy = farp.MountStrategyService // Routes under /user-service
	manifest.Endpoints.Health = "/health"

	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "user-service-hash-abc123",
		Size:        2048,
		Metadata: &farp.ProtocolMetadata{
			OpenAPI: &farp.OpenAPIMetadata{
				Composition: &farp.CompositionConfig{
					IncludeInMerged:   true,
					ComponentPrefix:   "User",
					TagPrefix:         "Users",
					OperationIDPrefix: "UserAPI",
					ConflictStrategy:  farp.ConflictStrategyPrefix,
				},
			},
		},
	})

	return manifest
}

func createProductServiceManifest() *farp.SchemaManifest {
	manifest := farp.NewManifest("product-service", "v2.1.0", "product-instance-1")
	manifest.Routing.Strategy = farp.MountStrategyService // Routes under /product-service
	manifest.Endpoints.Health = "/health"

	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "product-service-hash-def456",
		Size:        3072,
		Metadata: &farp.ProtocolMetadata{
			OpenAPI: &farp.OpenAPIMetadata{
				Composition: &farp.CompositionConfig{
					IncludeInMerged:   true,
					ComponentPrefix:   "Product",
					TagPrefix:         "Products",
					OperationIDPrefix: "ProductAPI",
					ConflictStrategy:  farp.ConflictStrategyPrefix,
				},
			},
		},
	})

	return manifest
}

func createOrderServiceManifest() *farp.SchemaManifest {
	manifest := farp.NewManifest("order-service", "v1.2.0", "order-instance-1")
	manifest.Routing.Strategy = farp.MountStrategyService // Routes under /order-service
	manifest.Endpoints.Health = "/health"

	manifest.AddSchema(farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "order-service-hash-ghi789",
		Size:        2560,
		Metadata: &farp.ProtocolMetadata{
			OpenAPI: &farp.OpenAPIMetadata{
				Composition: &farp.CompositionConfig{
					IncludeInMerged:   true,
					ComponentPrefix:   "Order",
					TagPrefix:         "Orders",
					OperationIDPrefix: "OrderAPI",
					ConflictStrategy:  farp.ConflictStrategyPrefix,
				},
			},
		},
	})

	return manifest
}

func createUserServiceOpenAPI() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       "User Service API",
			"description": "Manages user accounts and profiles",
			"version":     "1.0.0",
		},
		"paths": map[string]interface{}{
			"/users": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "listUsers",
					"summary":     "List all users",
					"tags":        []interface{}{"users"},
				},
				"post": map[string]interface{}{
					"operationId": "createUser",
					"summary":     "Create a new user",
					"tags":        []interface{}{"users"},
				},
			},
			"/users/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getUser",
					"summary":     "Get user by ID",
					"tags":        []interface{}{"users"},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"User": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":    map[string]interface{}{"type": "string"},
						"email": map[string]interface{}{"type": "string"},
						"name":  map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	}
}

func createProductServiceOpenAPI() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       "Product Service API",
			"description": "Manages product catalog",
			"version":     "2.1.0",
		},
		"paths": map[string]interface{}{
			"/products": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "listProducts",
					"summary":     "List all products",
					"tags":        []interface{}{"products"},
				},
			},
			"/products/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getProduct",
					"summary":     "Get product by ID",
					"tags":        []interface{}{"products"},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"Product": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":    map[string]interface{}{"type": "string"},
						"name":  map[string]interface{}{"type": "string"},
						"price": map[string]interface{}{"type": "number"},
					},
				},
			},
		},
	}
}

func createOrderServiceOpenAPI() map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       "Order Service API",
			"description": "Manages customer orders",
			"version":     "1.2.0",
		},
		"paths": map[string]interface{}{
			"/orders": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "listOrders",
					"summary":     "List all orders",
					"tags":        []interface{}{"orders"},
				},
				"post": map[string]interface{}{
					"operationId": "createOrder",
					"summary":     "Create a new order",
					"tags":        []interface{}{"orders"},
				},
			},
			"/orders/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getOrder",
					"summary":     "Get order by ID",
					"tags":        []interface{}{"orders"},
				},
			},
		},
		"components": map[string]interface{}{
			"schemas": map[string]interface{}{
				"Order": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":         map[string]interface{}{"type": "string"},
						"userId":     map[string]interface{}{"type": "string"},
						"productIds": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						"total":      map[string]interface{}{"type": "number"},
					},
				},
			},
		},
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
