//! Example demonstrating OpenAPI schema merging with FARP

use farp::manifest::new_manifest;
use farp::merger::{Merger, MergerConfig, Server, ServiceSchema};
use farp::types::{ConflictStrategy, LocationType, SchemaDescriptor, SchemaType};

fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("🔀 FARP OpenAPI Merger Example");
    println!("===============================\n");

    // Configure merger
    let config = MergerConfig {
        default_conflict_strategy: ConflictStrategy::Prefix,
        merged_title: "Unified E-Commerce API".to_string(),
        merged_description: "Combined API from User, Product, and Order services".to_string(),
        merged_version: "1.0.0".to_string(),
        include_service_tags: true,
        sort_output: true,
        servers: vec![Server {
            url: "https://api.example.com".to_string(),
            description: Some("Production API Gateway".to_string()),
            variables: None,
        }],
    };

    let merger = Merger::new(config);

    // 1. Create User Service schema
    println!("1. Creating User Service schema...");
    let mut user_manifest = new_manifest("user-service", "v1.0.0", "user-instance-1");
    user_manifest.endpoints.health = "/health".to_string();

    // Add OpenAPI schema descriptor
    user_manifest.add_schema(SchemaDescriptor {
        schema_type: SchemaType::OpenAPI,
        spec_version: "3.1.0".to_string(),
        location: farp::types::SchemaLocation {
            location_type: LocationType::Inline,
            url: None,
            registry_path: None,
            headers: None,
        },
        content_type: "application/json".to_string(),
        inline_schema: None,
        hash: "a".repeat(64),
        size: 2048,
        compatibility: None,
        metadata: None,
    });

    let user_schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {
            "title": "User Service API",
            "version": "1.0.0",
            "description": "Manages user accounts and profiles"
        },
        "paths": {
            "/users": {
                "get": {
                    "operationId": "listUsers",
                    "summary": "List all users",
                    "tags": ["users"],
                    "responses": {
                        "200": {"description": "Success"}
                    }
                },
                "post": {
                    "operationId": "createUser",
                    "summary": "Create a new user",
                    "tags": ["users"],
                    "responses": {
                        "201": {"description": "Created"}
                    }
                }
            },
            "/users/{id}": {
                "get": {
                    "operationId": "getUser",
                    "summary": "Get user by ID",
                    "tags": ["users"],
                    "responses": {
                        "200": {"description": "Success"}
                    }
                }
            }
        },
        "components": {
            "schemas": {
                "User": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string"},
                        "email": {"type": "string"},
                        "name": {"type": "string"}
                    }
                }
            }
        },
        "tags": [
            {"name": "users", "description": "User management operations"}
        ]
    });

    // 2. Create Product Service schema
    println!("2. Creating Product Service schema...");
    let mut product_manifest = new_manifest("product-service", "v1.2.0", "product-instance-1");
    product_manifest.endpoints.health = "/health".to_string();

    // Add OpenAPI schema descriptor
    product_manifest.add_schema(SchemaDescriptor {
        schema_type: SchemaType::OpenAPI,
        spec_version: "3.1.0".to_string(),
        location: farp::types::SchemaLocation {
            location_type: LocationType::Inline,
            url: None,
            registry_path: None,
            headers: None,
        },
        content_type: "application/json".to_string(),
        inline_schema: None,
        hash: "b".repeat(64),
        size: 2048,
        compatibility: None,
        metadata: None,
    });

    let product_schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {
            "title": "Product Service API",
            "version": "1.2.0",
            "description": "Manages product catalog"
        },
        "paths": {
            "/products": {
                "get": {
                    "operationId": "listProducts",
                    "summary": "List all products",
                    "tags": ["products"],
                    "responses": {
                        "200": {"description": "Success"}
                    }
                }
            },
            "/products/{id}": {
                "get": {
                    "operationId": "getProduct",
                    "summary": "Get product by ID",
                    "tags": ["products"],
                    "responses": {
                        "200": {"description": "Success"}
                    }
                }
            }
        },
        "components": {
            "schemas": {
                "Product": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string"},
                        "name": {"type": "string"},
                        "price": {"type": "number"}
                    }
                }
            }
        },
        "tags": [
            {"name": "products", "description": "Product catalog operations"}
        ]
    });

    // 3. Create Order Service schema
    println!("3. Creating Order Service schema...");
    let mut order_manifest = new_manifest("order-service", "v2.0.0", "order-instance-1");
    order_manifest.endpoints.health = "/health".to_string();

    // Add OpenAPI schema descriptor
    order_manifest.add_schema(SchemaDescriptor {
        schema_type: SchemaType::OpenAPI,
        spec_version: "3.1.0".to_string(),
        location: farp::types::SchemaLocation {
            location_type: LocationType::Inline,
            url: None,
            registry_path: None,
            headers: None,
        },
        content_type: "application/json".to_string(),
        inline_schema: None,
        hash: "c".repeat(64),
        size: 2048,
        compatibility: None,
        metadata: None,
    });

    let order_schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {
            "title": "Order Service API",
            "version": "2.0.0",
            "description": "Manages customer orders"
        },
        "paths": {
            "/orders": {
                "get": {
                    "operationId": "listOrders",
                    "summary": "List all orders",
                    "tags": ["orders"],
                    "responses": {
                        "200": {"description": "Success"}
                    }
                },
                "post": {
                    "operationId": "createOrder",
                    "summary": "Create a new order",
                    "tags": ["orders"],
                    "responses": {
                        "201": {"description": "Created"}
                    }
                }
            }
        },
        "components": {
            "schemas": {
                "Order": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string"},
                        "userId": {"type": "string"},
                        "total": {"type": "number"}
                    }
                }
            }
        },
        "tags": [
            {"name": "orders", "description": "Order management operations"}
        ]
    });

    // 4. Merge all schemas
    println!("\n4. Merging OpenAPI specifications...");
    let schemas = vec![
        ServiceSchema {
            manifest: user_manifest,
            schema: user_schema,
            parsed: None,
        },
        ServiceSchema {
            manifest: product_manifest,
            schema: product_schema,
            parsed: None,
        },
        ServiceSchema {
            manifest: order_manifest,
            schema: order_schema,
            parsed: None,
        },
    ];

    let result = merger.merge(schemas)?;

    // 5. Display results
    println!(
        "\n✅ Successfully merged {} services!",
        result.included_services.len()
    );
    println!("   Services: {:?}", result.included_services);

    println!("\n📋 Merged Specification Details:");
    println!("   Title: {}", result.spec.info.title);
    println!("   Version: {}", result.spec.info.version);
    println!("   OpenAPI: {}", result.spec.openapi);

    println!("\n🛣️  Merged Paths ({} total):", result.spec.paths.len());
    let mut paths: Vec<&String> = result.spec.paths.keys().collect();
    paths.sort();
    for path in paths {
        println!("   {path}");
    }

    if let Some(components) = &result.spec.components {
        println!("\n📦 Merged Components:");
        println!("   Schemas: {}", components.schemas.len());
        for schema_name in components.schemas.keys() {
            println!("      - {schema_name}");
        }
    }

    println!("\n🏷️  Merged Tags ({} total):", result.spec.tags.len());
    for tag in &result.spec.tags {
        println!(
            "   - {}: {}",
            tag.name,
            tag.description.as_deref().unwrap_or("")
        );
    }

    if !result.conflicts.is_empty() {
        println!("\n⚠️  Conflicts Resolved ({}):", result.conflicts.len());
        for conflict in &result.conflicts {
            println!(
                "   - {:?} '{}': {}",
                conflict.conflict_type, conflict.item, conflict.resolution
            );
        }
    }

    if !result.warnings.is_empty() {
        println!("\n⚠️  Warnings:");
        for warning in &result.warnings {
            println!("   - {warning}");
        }
    }

    // 6. Serialize to JSON
    println!("\n6. Serializing merged OpenAPI spec to JSON...");
    let json = serde_json::to_string_pretty(&result.spec)?;
    println!("   JSON size: {} bytes", json.len());

    // Optionally save to file
    // std::fs::write("merged-openapi.json", &json)?;
    // println!("   Saved to: merged-openapi.json");

    println!("\n✅ Merge complete!");

    Ok(())
}
