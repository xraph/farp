//! Basic example demonstrating FARP usage
//!
//! This example shows:
//! - Creating a schema manifest
//! - Adding schemas and capabilities
//! - Registering with an in-memory registry
//! - Fetching manifests
//! - Watching for changes

use farp::prelude::*;
use std::sync::Arc;

#[tokio::main]
async fn main() -> Result<()> {
    println!("🚀 FARP Basic Example");
    println!("====================\n");

    // 1. Create a schema manifest
    println!("1. Creating schema manifest...");
    let mut manifest = new_manifest("user-service", "v1.2.3", "instance-abc123");
    manifest.add_capability("rest");
    manifest.endpoints.health = "/health".to_string();
    manifest.endpoints.openapi = Some("/openapi.json".to_string());
    println!("✓ Created manifest for service: {}", manifest.service_name);

    // 2. Add a schema descriptor
    println!("\n2. Adding OpenAPI schema descriptor...");
    let openapi_schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {
            "title": "User Service API",
            "version": "1.2.3"
        },
        "paths": {
            "/users": {
                "get": {
                    "summary": "List users",
                    "responses": {
                        "200": {
                            "description": "Success"
                        }
                    }
                }
            }
        }
    });

    // Calculate schema hash
    let schema_hash = calculate_schema_checksum(&openapi_schema)?;
    let schema_json = serde_json::to_vec(&openapi_schema)?;

    let schema_descriptor = SchemaDescriptor {
        schema_type: SchemaType::OpenAPI,
        spec_version: "3.1.0".to_string(),
        location: SchemaLocation {
            location_type: LocationType::Registry,
            url: None,
            registry_path: Some("/schemas/user-service/v1/openapi".to_string()),
            headers: None,
        },
        content_type: "application/json".to_string(),
        inline_schema: None,
        hash: schema_hash,
        size: schema_json.len() as i64,
        compatibility: None,
        metadata: None,
    };

    manifest.add_schema(schema_descriptor);
    println!("✓ Added OpenAPI 3.1.0 schema descriptor");

    // 3. Update checksum
    println!("\n3. Calculating manifest checksum...");
    manifest.update_checksum()?;
    println!("✓ Manifest checksum: {}", manifest.checksum);

    // 4. Validate manifest
    println!("\n4. Validating manifest...");
    manifest.validate()?;
    println!("✓ Manifest is valid");

    // 5. Create in-memory registry
    println!("\n5. Creating in-memory registry...");
    use farp::registry::memory::MemoryRegistry;
    let registry = Arc::new(MemoryRegistry::new());
    println!("✓ Registry created");

    // 6. Publish schema to registry
    println!("\n6. Publishing OpenAPI schema to registry...");
    registry
        .publish_schema("/schemas/user-service/v1/openapi", &openapi_schema)
        .await?;
    println!("✓ Schema published to registry");

    // 7. Register manifest
    println!("\n7. Registering manifest with registry...");
    registry.register_manifest(&manifest).await?;
    println!("✓ Manifest registered");

    // 8. Fetch manifest
    println!("\n8. Fetching manifest from registry...");
    let fetched = registry.get_manifest("instance-abc123").await?;
    println!("✓ Fetched manifest for: {}", fetched.service_name);
    println!("  - Version: {}", fetched.service_version);
    println!("  - Instance ID: {}", fetched.instance_id);
    println!("  - Schemas: {}", fetched.schemas.len());
    println!("  - Capabilities: {:?}", fetched.capabilities);

    // 9. List manifests
    println!("\n9. Listing all manifests for user-service...");
    let manifests = registry.list_manifests("user-service").await?;
    println!("✓ Found {} manifest(s)", manifests.len());

    // 10. Update manifest
    println!("\n10. Updating manifest...");
    let mut updated_manifest = manifest.clone();
    updated_manifest.service_version = "v1.2.4".to_string();
    updated_manifest.update_checksum()?;
    registry.update_manifest(&updated_manifest).await?;
    println!(
        "✓ Manifest updated to version: {}",
        updated_manifest.service_version
    );

    // 11. Diff manifests
    println!("\n11. Computing diff between old and new manifests...");
    let diff = diff_manifests(&manifest, &updated_manifest);
    println!("✓ Diff computed:");
    println!("  - Has changes: {}", diff.has_changes());
    println!("  - Schemas added: {}", diff.schemas_added.len());
    println!("  - Schemas removed: {}", diff.schemas_removed.len());
    println!("  - Schemas changed: {}", diff.schemas_changed.len());

    // 12. Gateway client example
    #[cfg(feature = "gateway")]
    {
        println!("\n12. Creating gateway client...");
        use farp::gateway::Client;
        let client = Client::new(registry.clone());
        println!("✓ Gateway client created");

        let manifests = vec![updated_manifest.clone()];
        let routes = client.convert_to_routes(&manifests).await;
        println!(
            "✓ Converted {} manifest(s) to {} route(s)",
            manifests.len(),
            routes.len()
        );

        for route in &routes {
            println!(
                "  - {} {} -> {}",
                route.methods.join(","),
                route.path,
                route.target_url
            );
        }
    }

    // 13. Check health
    println!("\n13. Checking registry health...");
    registry.health().await?;
    println!("✓ Registry is healthy");

    // 14. Cleanup
    println!("\n14. Cleaning up...");
    registry.delete_manifest("instance-abc123").await?;
    registry.close().await?;
    println!("✓ Cleanup complete");

    println!("\n✅ All operations completed successfully!");
    Ok(())
}
