//! Integration tests covering end-to-end workflows

use farp::prelude::*;
use farp::registry::memory::MemoryRegistry;
use std::sync::Arc;

#[tokio::test]
async fn test_full_workflow() {
    // Create manifest
    let mut manifest = new_manifest("test-service", "v1.0.0", "instance-1");
    manifest.add_capability("rest");
    manifest.endpoints.health = "/health".to_string();

    // Add schema
    let schema = serde_json::json!({"openapi": "3.1.0", "info": {}, "paths": {}});
    let hash = calculate_schema_checksum(&schema).unwrap();

    manifest.add_schema(SchemaDescriptor {
        schema_type: SchemaType::OpenAPI,
        spec_version: "3.1.0".to_string(),
        location: SchemaLocation {
            location_type: LocationType::Inline,
            url: None,
            registry_path: None,
            headers: None,
        },
        content_type: "application/json".to_string(),
        inline_schema: Some(schema),
        hash,
        size: 100,
        compatibility: None,
        metadata: None,
    });

    manifest.update_checksum().unwrap();
    manifest.validate().unwrap();

    // Register
    let registry = Arc::new(MemoryRegistry::new());
    registry.register_manifest(&manifest).await.unwrap();

    // Fetch
    let fetched = registry.get_manifest("instance-1").await.unwrap();
    assert_eq!(fetched.service_name, "test-service");

    // Update
    let mut updated = manifest.clone();
    updated.service_version = "v2.0.0".to_string();
    updated.update_checksum().unwrap();
    registry.update_manifest(&updated).await.unwrap();

    // Verify update
    let fetched = registry.get_manifest("instance-1").await.unwrap();
    assert_eq!(fetched.service_version, "v2.0.0");

    // Delete
    registry.delete_manifest("instance-1").await.unwrap();
    assert!(registry.get_manifest("instance-1").await.is_err());
}

#[tokio::test]
async fn test_multiple_services() {
    let registry = Arc::new(MemoryRegistry::new());

    // Register multiple services
    for i in 1..=3 {
        let mut manifest = new_manifest("service-a", "v1.0.0", format!("instance-{i}"));
        manifest.endpoints.health = "/health".to_string();
        manifest.update_checksum().unwrap();
        registry.register_manifest(&manifest).await.unwrap();
    }

    // List manifests
    let manifests = registry.list_manifests("service-a").await.unwrap();
    assert_eq!(manifests.len(), 3);

    // List all
    let all = registry.list_manifests("").await.unwrap();
    assert_eq!(all.len(), 3);
}

#[tokio::test]
async fn test_schema_publishing() {
    let registry = Arc::new(MemoryRegistry::new());

    let schema = serde_json::json!({"test": "data"});
    registry
        .publish_schema("/schemas/test", &schema)
        .await
        .unwrap();

    let fetched = registry.fetch_schema("/schemas/test").await.unwrap();
    assert_eq!(fetched, schema);

    registry.delete_schema("/schemas/test").await.unwrap();
    assert!(registry.fetch_schema("/schemas/test").await.is_err());
}

#[tokio::test]
async fn test_manifest_validation() {
    let mut manifest = new_manifest("test", "v1", "id1");
    manifest.endpoints.health = "/health".to_string();

    // Missing checksum - should still validate
    assert!(manifest.validate().is_ok());

    // Add checksum
    manifest.update_checksum().unwrap();
    assert!(manifest.validate().is_ok());

    // Corrupt checksum
    manifest.checksum = "invalid".to_string();
    assert!(manifest.validate().is_err());
}

#[tokio::test]
async fn test_manifest_diff() {
    let mut old = new_manifest("test", "v1", "id1");
    old.add_capability("rest");
    old.endpoints.health = "/health".to_string();

    let mut new = old.clone();
    new.add_capability("grpc");
    new.service_version = "v2".to_string();

    let diff = diff_manifests(&old, &new);
    assert!(diff.has_changes());
    assert_eq!(diff.capabilities_added.len(), 1);
    assert_eq!(diff.capabilities_added[0], "grpc");
}

#[cfg(feature = "gateway")]
#[tokio::test]
async fn test_gateway_client() {
    use farp::gateway::Client;

    let registry = Arc::new(MemoryRegistry::new());
    let client = Client::new(registry.clone());

    let mut manifest = new_manifest("test", "v1", "id1");
    manifest.endpoints.health = "/health".to_string();
    manifest.update_checksum().unwrap();

    registry.register_manifest(&manifest).await.unwrap();

    let manifests = vec![manifest];
    let routes = client.convert_to_routes(&manifests).await;
    assert_eq!(routes.len(), 0); // No schemas, no routes
}

#[tokio::test]
async fn test_version_compatibility() {
    use farp::version::{is_compatible, PROTOCOL_VERSION};

    assert!(is_compatible(PROTOCOL_VERSION));
    assert!(is_compatible("1.0.0"));
    assert!(is_compatible("1.0.1"));
    assert!(!is_compatible("2.0.0"));
    assert!(!is_compatible("0.9.0"));
}

#[tokio::test]
async fn test_checksum_calculation() {
    let schema = serde_json::json!({"test": "data"});
    let hash1 = calculate_schema_checksum(&schema).unwrap();
    let hash2 = calculate_schema_checksum(&schema).unwrap();

    assert_eq!(hash1, hash2); // Deterministic
    assert_eq!(hash1.len(), 64); // SHA256 produces 64 hex chars
}
