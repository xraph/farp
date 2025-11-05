//! Integration tests for OpenAPI merger

use farp::manifest::new_manifest;
use farp::merger::{Merger, MergerConfig, ServiceSchema};
use farp::types::{
    CompositionConfig, ConflictStrategy, LocationType, OpenAPIMetadata, ProtocolMetadata,
    SchemaDescriptor, SchemaType,
};

#[test]
fn test_basic_merge() {
    let merger = Merger::default();

    // Create first service manifest
    let mut manifest1 = new_manifest("user-service", "v1.0.0", "instance-1");
    manifest1.endpoints.health = "/health".to_string();

    // Add OpenAPI schema descriptor
    manifest1.add_schema(SchemaDescriptor {
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
        size: 1024,
        compatibility: None,
        metadata: None,
    });

    let schema1 = serde_json::json!({
        "openapi": "3.1.0",
        "info": {
            "title": "User Service",
            "version": "1.0.0"
        },
        "paths": {
            "/users": {
                "get": {
                    "operationId": "listUsers",
                    "tags": ["users"],
                    "summary": "List users"
                }
            }
        },
        "components": {
            "schemas": {
                "User": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string"},
                        "name": {"type": "string"}
                    }
                }
            }
        }
    });

    // Create second service manifest
    let mut manifest2 = new_manifest("product-service", "v1.0.0", "instance-2");
    manifest2.endpoints.health = "/health".to_string();

    // Add OpenAPI schema descriptor
    manifest2.add_schema(SchemaDescriptor {
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
        size: 1024,
        compatibility: None,
        metadata: None,
    });

    let schema2 = serde_json::json!({
        "openapi": "3.1.0",
        "info": {
            "title": "Product Service",
            "version": "1.0.0"
        },
        "paths": {
            "/products": {
                "get": {
                    "operationId": "listProducts",
                    "tags": ["products"],
                    "summary": "List products"
                }
            }
        },
        "components": {
            "schemas": {
                "Product": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "string"},
                        "name": {"type": "string"}
                    }
                }
            }
        }
    });

    let schemas = vec![
        ServiceSchema {
            manifest: manifest1,
            schema: schema1,
            parsed: None,
        },
        ServiceSchema {
            manifest: manifest2,
            schema: schema2,
            parsed: None,
        },
    ];

    let result = merger.merge(schemas).unwrap();

    // Verify merged spec
    assert_eq!(result.included_services.len(), 2);
    assert!(result
        .included_services
        .contains(&"user-service".to_string()));
    assert!(result
        .included_services
        .contains(&"product-service".to_string()));

    // Check paths were merged
    assert!(result.spec.paths.contains_key("/instance-1/users"));
    assert!(result.spec.paths.contains_key("/instance-2/products"));

    // Check components were prefixed and merged
    if let Some(components) = &result.spec.components {
        assert!(components.schemas.contains_key("user-service_User"));
        assert!(components.schemas.contains_key("product-service_Product"));
    }
}

#[test]
fn test_conflict_resolution_prefix() {
    let config = MergerConfig {
        default_conflict_strategy: ConflictStrategy::Prefix,
        ..Default::default()
    };
    let merger = Merger::new(config);

    // Both services have the same path
    let mut manifest1 = new_manifest("service-a", "v1.0.0", "instance-1");
    manifest1.endpoints.health = "/health".to_string();
    manifest1.routing.strategy = farp::types::MountStrategy::Root;

    // Add OpenAPI schema descriptor
    manifest1.add_schema(SchemaDescriptor {
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
        size: 1024,
        compatibility: None,
        metadata: None,
    });

    let schema1 = serde_json::json!({
        "openapi": "3.1.0",
        "info": {"title": "Service A", "version": "1.0.0"},
        "paths": {
            "/data": {"get": {"operationId": "getData"}}
        }
    });

    let mut manifest2 = new_manifest("service-b", "v1.0.0", "instance-2");
    manifest2.endpoints.health = "/health".to_string();
    manifest2.routing.strategy = farp::types::MountStrategy::Root;

    // Add OpenAPI schema descriptor
    manifest2.add_schema(SchemaDescriptor {
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
        size: 1024,
        compatibility: None,
        metadata: None,
    });

    let schema2 = serde_json::json!({
        "openapi": "3.1.0",
        "info": {"title": "Service B", "version": "1.0.0"},
        "paths": {
            "/data": {"post": {"operationId": "createData"}}
        }
    });

    let schemas = vec![
        ServiceSchema {
            manifest: manifest1,
            schema: schema1,
            parsed: None,
        },
        ServiceSchema {
            manifest: manifest2,
            schema: schema2,
            parsed: None,
        },
    ];

    let result = merger.merge(schemas).unwrap();

    // Should have conflict
    assert!(!result.conflicts.is_empty());

    // Second service's path should be prefixed
    assert!(result.spec.paths.contains_key("/data"));
    assert!(result.spec.paths.contains_key("/service-b/data"));
}

#[test]
fn test_composition_config() {
    let merger = Merger::default();

    let mut manifest = new_manifest("test-service", "v1.0.0", "instance-1");
    manifest.endpoints.health = "/health".to_string();

    // Add composition config
    let descriptor = SchemaDescriptor {
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
        size: 1024,
        compatibility: None,
        metadata: Some(ProtocolMetadata {
            openapi: Some(OpenAPIMetadata {
                extensions: None,
                server_variables: None,
                default_security: Vec::new(),
                composition: Some(CompositionConfig {
                    include_in_merged: true,
                    component_prefix: Some("custom".to_string()),
                    tag_prefix: Some("custom".to_string()),
                    operation_id_prefix: Some("custom".to_string()),
                    conflict_strategy: ConflictStrategy::Skip,
                    preserve_extensions: true,
                    custom_servers: Vec::new(),
                }),
            }),
            graphql: None,
            grpc: None,
            asyncapi: None,
            orpc: None,
        }),
    };

    manifest.add_schema(descriptor);

    let schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0.0"},
        "paths": {
            "/test": {"get": {"operationId": "getTest", "tags": ["test"]}}
        },
        "components": {
            "schemas": {"TestModel": {"type": "object"}}
        }
    });

    let schemas = vec![ServiceSchema {
        manifest,
        schema,
        parsed: None,
    }];

    let result = merger.merge(schemas).unwrap();

    // Check custom prefixes were applied
    if let Some(components) = &result.spec.components {
        assert!(components.schemas.contains_key("custom_TestModel"));
    }

    // Check operation ID was prefixed
    if let Some(path_item) = result.spec.paths.get("/instance-1/test") {
        if let Some(operation) = &path_item.get {
            assert_eq!(operation.operation_id, Some("custom_getTest".to_string()));
            assert_eq!(operation.tags, vec!["custom_test"]);
        }
    }
}

#[test]
fn test_exclude_from_merge() {
    let merger = Merger::default();

    let mut manifest = new_manifest("excluded-service", "v1.0.0", "instance-1");
    manifest.endpoints.health = "/health".to_string();

    // Add composition config that excludes from merge
    let descriptor = SchemaDescriptor {
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
        size: 1024,
        compatibility: None,
        metadata: Some(ProtocolMetadata {
            openapi: Some(OpenAPIMetadata {
                extensions: None,
                server_variables: None,
                default_security: Vec::new(),
                composition: Some(CompositionConfig {
                    include_in_merged: false,
                    component_prefix: None,
                    tag_prefix: None,
                    operation_id_prefix: None,
                    conflict_strategy: ConflictStrategy::Skip,
                    preserve_extensions: false,
                    custom_servers: Vec::new(),
                }),
            }),
            graphql: None,
            grpc: None,
            asyncapi: None,
            orpc: None,
        }),
    };

    manifest.add_schema(descriptor);

    let schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {"title": "Test", "version": "1.0.0"},
        "paths": {}
    });

    let schemas = vec![ServiceSchema {
        manifest,
        schema,
        parsed: None,
    }];

    let result = merger.merge(schemas).unwrap();

    // Should be excluded
    assert_eq!(result.included_services.len(), 0);
    assert_eq!(result.excluded_services.len(), 1);
    assert!(result
        .excluded_services
        .contains(&"excluded-service".to_string()));
}
