//! FARP - Forge API Gateway Registration Protocol
//!
//! FARP is a protocol specification for enabling service instances to automatically
//! register their API schemas, health information, and capabilities with API gateways
//! and service meshes.
//!
//! # Overview
//!
//! FARP provides:
//! - Schema-aware service discovery (OpenAPI, AsyncAPI, gRPC, GraphQL)
//! - Dynamic gateway configuration based on registered schemas
//! - Multi-protocol support with extensibility
//! - Health and telemetry integration
//! - Backend-agnostic storage (Consul, etcd, Kubernetes, Redis, Memory)
//! - Push and pull models for schema distribution
//! - Zero-downtime schema updates with versioning
//!
//! # Basic Usage
//!
//! Creating a schema manifest:
//!
//! ```
//! use farp::{types::*, manifest::new_manifest};
//!
//! let mut manifest = new_manifest("user-service", "v1.2.3", "instance-abc123");
//! manifest.capabilities.push("rest".to_string());
//! manifest.endpoints.health = "/health".to_string();
//! ```
//!
//! # Feature Flags
//!
//! - `default`: Core types + memory registry
//! - `providers-openapi`: OpenAPI provider
//! - `providers-asyncapi`: AsyncAPI provider
//! - `providers-grpc`: gRPC provider
//! - `providers-graphql`: GraphQL provider
//! - `providers-orpc`: oRPC provider
//! - `providers-avro`: Avro provider
//! - `providers-thrift`: Thrift provider
//! - `providers-all`: All providers
//! - `gateway`: Gateway client implementation
//! - `full`: Everything enabled

pub mod errors;
pub mod manifest;
pub mod provider;
pub mod storage;
pub mod types;
pub mod version;

// Registry module
pub mod registry {
    use crate::errors::Result;
    use crate::types::SchemaManifest;
    use async_trait::async_trait;
    use serde::{Deserialize, Serialize};
    use std::collections::HashMap;

    /// Schema registry trait for managing manifests and schemas
    #[async_trait]
    pub trait SchemaRegistry: Send + Sync {
        async fn register_manifest(&self, manifest: &SchemaManifest) -> Result<()>;
        async fn get_manifest(&self, instance_id: &str) -> Result<SchemaManifest>;
        async fn update_manifest(&self, manifest: &SchemaManifest) -> Result<()>;
        async fn delete_manifest(&self, instance_id: &str) -> Result<()>;
        async fn list_manifests(&self, service_name: &str) -> Result<Vec<SchemaManifest>>;
        async fn publish_schema(&self, path: &str, schema: &serde_json::Value) -> Result<()>;
        async fn fetch_schema(&self, path: &str) -> Result<serde_json::Value>;
        async fn delete_schema(&self, path: &str) -> Result<()>;
        async fn watch_manifests(
            &self,
            service_name: &str,
            on_change: Box<dyn ManifestChangeHandler>,
        ) -> Result<()>;
        async fn watch_schemas(
            &self,
            path: &str,
            on_change: Box<dyn SchemaChangeHandler>,
        ) -> Result<()>;
        async fn close(&self) -> Result<()>;
        async fn health(&self) -> Result<()>;
    }

    pub trait ManifestChangeHandler: Send + Sync {
        fn on_change(&self, event: &ManifestEvent);
    }

    impl<F> ManifestChangeHandler for F
    where
        F: Fn(&ManifestEvent) + Send + Sync,
    {
        fn on_change(&self, event: &ManifestEvent) {
            self(event)
        }
    }

    pub trait SchemaChangeHandler: Send + Sync {
        fn on_change(&self, event: &SchemaEvent);
    }

    impl<F> SchemaChangeHandler for F
    where
        F: Fn(&SchemaEvent) + Send + Sync,
    {
        fn on_change(&self, event: &SchemaEvent) {
            self(event)
        }
    }

    #[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
    pub struct ManifestEvent {
        pub event_type: EventType,
        pub manifest: SchemaManifest,
        pub timestamp: i64,
    }

    #[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
    pub struct SchemaEvent {
        pub event_type: EventType,
        pub path: String,
        pub schema: Option<serde_json::Value>,
        pub timestamp: i64,
    }

    #[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
    #[serde(rename_all = "lowercase")]
    pub enum EventType {
        #[serde(rename = "added")]
        Added,
        #[serde(rename = "updated")]
        Updated,
        #[serde(rename = "removed")]
        Removed,
    }

    impl std::fmt::Display for EventType {
        fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
            let s = match self {
                EventType::Added => "added",
                EventType::Updated => "updated",
                EventType::Removed => "removed",
            };
            write!(f, "{s}")
        }
    }

    #[derive(Debug, Clone, Serialize, Deserialize)]
    pub struct RegistryConfig {
        pub backend: String,
        pub namespace: String,
        pub backend_config: HashMap<String, serde_json::Value>,
        pub max_schema_size: i64,
        pub compression_threshold: i64,
        pub ttl: i64,
    }

    impl Default for RegistryConfig {
        fn default() -> Self {
            Self {
                backend: "memory".to_string(),
                namespace: "farp".to_string(),
                backend_config: HashMap::new(),
                max_schema_size: 1024 * 1024,
                compression_threshold: 100 * 1024,
                ttl: 0,
            }
        }
    }

    pub trait SchemaCache: Send + Sync {
        fn get(&self, hash: &str) -> Option<serde_json::Value>;
        fn set(&self, hash: &str, schema: serde_json::Value) -> Result<()>;
        fn delete(&self, hash: &str) -> Result<()>;
        fn clear(&self) -> Result<()>;
        fn size(&self) -> usize;
    }

    #[derive(Debug, Clone)]
    pub struct FetchOptions {
        pub use_cache: bool,
        pub validate_checksum: bool,
        pub expected_hash: Option<String>,
        pub timeout: u64,
    }

    impl Default for FetchOptions {
        fn default() -> Self {
            Self {
                use_cache: true,
                validate_checksum: true,
                expected_hash: None,
                timeout: 30,
            }
        }
    }

    #[derive(Debug, Clone)]
    pub struct PublishOptions {
        pub compress: bool,
        pub ttl: i64,
        pub overwrite_existing: bool,
    }

    impl Default for PublishOptions {
        fn default() -> Self {
            Self {
                compress: false,
                ttl: 0,
                overwrite_existing: true,
            }
        }
    }

    #[cfg(feature = "memory-registry")]
    pub mod memory;
}

// Providers
pub mod providers;

// Gateway client
#[cfg(feature = "gateway")]
pub mod gateway;

// Merger for OpenAPI composition
pub mod merger;

// Re-exports for convenience
pub use errors::{Error, Result};
pub use version::{get_version, is_compatible, PROTOCOL_VERSION};

/// Prelude module for convenient imports
pub mod prelude {
    pub use crate::errors::{Error, Result};
    pub use crate::manifest::*;
    pub use crate::provider::*;
    pub use crate::registry::SchemaRegistry;
    pub use crate::storage::*;
    pub use crate::types::*;
    pub use crate::version::*;
}
