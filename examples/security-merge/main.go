// Package main demonstrates security/auth handling in OpenAPI composition
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/xraph/farp"
	"github.com/xraph/farp/merger"
)

func main() {
	fmt.Println("FARP Security/Auth Merging Example")
	fmt.Println("===================================")

	// Create merger
	config := merger.DefaultMergerConfig()
	config.MergedTitle = "Secure Multi-Service API"
	m := merger.NewMerger(config)

	// Service 1: Uses API Key auth
	fmt.Println("\n1. User Service (API Key auth)")
	userManifest := farp.NewManifest("user-service", "v1.0.0", "user-1")
	userManifest.Routing.Strategy = farp.MountStrategyService
	userManifest.AddSchema(createOpenAPIDescriptor())

	userSchema := createOpenAPIWithAuth("User API", []SecurityDef{
		{Name: "apiKey", Type: "apiKey", In: "header", KeyName: "X-API-Key"},
	})

	// Service 2: Uses Bearer token
	fmt.Println("2. Order Service (Bearer token auth)")
	orderManifest := farp.NewManifest("order-service", "v1.0.0", "order-1")
	orderManifest.Routing.Strategy = farp.MountStrategyService
	orderManifest.AddSchema(createOpenAPIDescriptor())

	orderSchema := createOpenAPIWithAuth("Order API", []SecurityDef{
		{Name: "bearerAuth", Type: "http", Scheme: "bearer"},
	})

	// Service 3: Uses OAuth2 (conflict with same "oauth" name from service 4)
	fmt.Println("3. Payment Service (OAuth2)")
	paymentManifest := farp.NewManifest("payment-service", "v1.0.0", "payment-1")
	paymentManifest.Routing.Strategy = farp.MountStrategyService
	paymentManifest.AddSchema(createOpenAPIDescriptor())

	paymentSchema := createOpenAPIWithAuth("Payment API", []SecurityDef{
		{Name: "oauth", Type: "oauth2", Description: "Payment OAuth"},
	})

	// Service 4: Also uses OAuth2 with SAME name (will conflict!)
	fmt.Println("4. Billing Service (OAuth2 - CONFLICTING NAME)")
	billingManifest := farp.NewManifest("billing-service", "v1.0.0", "billing-1")
	billingManifest.Routing.Strategy = farp.MountStrategyService
	billingManifest.AddSchema(createOpenAPIDescriptor())
	// Use ConflictStrategyPrefix for this service
	billingManifest.Schemas[0].Metadata = &farp.ProtocolMetadata{
		OpenAPI: &farp.OpenAPIMetadata{
			Composition: &farp.CompositionConfig{
				IncludeInMerged:  true,
				ConflictStrategy: farp.ConflictStrategyPrefix, // Will prefix security schemes
			},
		},
	}

	billingSchema := createOpenAPIWithAuth("Billing API", []SecurityDef{
		{Name: "oauth", Type: "oauth2", Description: "Billing OAuth"}, // SAME NAME!
	})

	// Merge all services
	fmt.Println("\n5. Merging all services...")
	schemas := []merger.ServiceSchema{
		{Manifest: userManifest, Schema: userSchema},
		{Manifest: orderManifest, Schema: orderSchema},
		{Manifest: paymentManifest, Schema: paymentSchema},
		{Manifest: billingManifest, Schema: billingSchema},
	}

	result, err := m.Merge(schemas)
	if err != nil {
		log.Fatalf("Failed to merge: %v", err)
	}

	// Display results
	fmt.Printf("\n✓ Successfully merged %d services\n\n", len(result.IncludedServices))

	// Show security schemes
	fmt.Println("Merged Security Schemes:")
	fmt.Println("───────────────────────────────────────────────────────────")
	for name, scheme := range result.Spec.Components.SecuritySchemes {
		fmt.Printf("  • %-25s (type: %s", name, scheme.Type)
		if scheme.In != "" {
			fmt.Printf(", in: %s", scheme.In)
		}
		if scheme.Scheme != "" {
			fmt.Printf(", scheme: %s", scheme.Scheme)
		}
		if scheme.Description != "" {
			fmt.Printf(", desc: %s", scheme.Description)
		}
		fmt.Println(")")
	}

	// Show conflicts
	if len(result.Conflicts) > 0 {
		fmt.Println("\n📋 Security Conflicts Resolved:")
		fmt.Println("───────────────────────────────────────────────────────────")
		securityConflicts := 0
		for _, conflict := range result.Conflicts {
			if conflict.Type == merger.ConflictTypeSecurityScheme {
				securityConflicts++
				fmt.Printf("  %d. Scheme: %s\n", securityConflicts, conflict.Item)
				fmt.Printf("     Services: %v\n", conflict.Services)
				fmt.Printf("     Strategy: %s\n", conflict.Strategy)
				fmt.Printf("     Resolution: %s\n", conflict.Resolution)
			}
		}
		if securityConflicts == 0 {
			fmt.Println("  (No security-specific conflicts)")
		}
	}

	// Show summary
	fmt.Println("\n📊 Security Summary:")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Printf("  Total security schemes: %d\n", len(result.Spec.Components.SecuritySchemes))

	byType := make(map[string]int)
	for _, scheme := range result.Spec.Components.SecuritySchemes {
		byType[scheme.Type]++
	}
	fmt.Println("  By type:")
	for schemeType, count := range byType {
		fmt.Printf("    - %s: %d\n", schemeType, count)
	}

	// Export sample
	sampleSchemes := make(map[string]interface{})
	for name, scheme := range result.Spec.Components.SecuritySchemes {
		sampleSchemes[name] = map[string]interface{}{
			"type":        scheme.Type,
			"description": scheme.Description,
		}
	}

	jsonBytes, _ := json.MarshalIndent(map[string]interface{}{
		"components": map[string]interface{}{
			"securitySchemes": sampleSchemes,
		},
	}, "", "  ")

	fmt.Println("\n📄 OpenAPI Security Schemes (JSON):")
	fmt.Println("───────────────────────────────────────────────────────────")
	fmt.Println(string(jsonBytes))

	fmt.Println("\n✓ Security merging complete!")
	fmt.Println("\nKey Takeaways:")
	fmt.Println("  1. Security schemes are automatically merged")
	fmt.Println("  2. Conflicts are detected and resolved based on strategy")
	fmt.Println("  3. Prefix strategy prevents naming conflicts")
	fmt.Println("  4. All auth types supported (apiKey, http, oauth2, oidc)")
}

type SecurityDef struct {
	Name        string
	Type        string
	In          string // For apiKey
	KeyName     string // For apiKey
	Scheme      string // For http
	Description string
}

func createOpenAPIDescriptor() farp.SchemaDescriptor {
	return farp.SchemaDescriptor{
		Type:        farp.SchemaTypeOpenAPI,
		SpecVersion: "3.1.0",
		Location: farp.SchemaLocation{
			Type: farp.LocationTypeInline,
		},
		ContentType: "application/json",
		Hash:        "hash-random",
		Size:        1024,
		Metadata: &farp.ProtocolMetadata{
			OpenAPI: &farp.OpenAPIMetadata{
				Composition: &farp.CompositionConfig{
					IncludeInMerged:  true,
					ConflictStrategy: farp.ConflictStrategyOverwrite, // Default: overwrite conflicts
				},
			},
		},
	}
}

func createOpenAPIWithAuth(title string, securityDefs []SecurityDef) map[string]interface{} {
	securitySchemes := make(map[string]interface{})
	globalSecurity := []interface{}{}

	for _, def := range securityDefs {
		scheme := map[string]interface{}{
			"type": def.Type,
		}

		if def.Description != "" {
			scheme["description"] = def.Description
		}

		switch def.Type {
		case "apiKey":
			scheme["name"] = def.KeyName
			scheme["in"] = def.In

		case "http":
			scheme["scheme"] = def.Scheme
			if def.Scheme == "bearer" {
				scheme["bearerFormat"] = "JWT"
			}

		case "oauth2":
			scheme["flows"] = map[string]interface{}{
				"authorizationCode": map[string]interface{}{
					"authorizationUrl": "https://example.com/oauth/authorize",
					"tokenUrl":         "https://example.com/oauth/token",
					"scopes": map[string]interface{}{
						"read":  "Read access",
						"write": "Write access",
					},
				},
			}
		}

		securitySchemes[def.Name] = scheme

		// Add to global security requirement
		globalSecurity = append(globalSecurity, map[string]interface{}{
			def.Name: []interface{}{},
		})
	}

	return map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":   title,
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/data": map[string]interface{}{
				"get": map[string]interface{}{
					"operationId": "getData",
					"summary":     "Get data",
					"security":    globalSecurity, // Operation-level security
				},
			},
		},
		"components": map[string]interface{}{
			"securitySchemes": securitySchemes,
		},
		"security": globalSecurity, // Global security requirement
	}
}

