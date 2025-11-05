//! Schema provider traits and registry.
//!
//! Schema providers generate schemas from application code for various protocols
//! (OpenAPI, AsyncAPI, gRPC, GraphQL, etc.).

use crate::errors::{Error, Result};
use crate::manifest::calculate_schema_checksum;
use crate::types::SchemaType;
use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::{Arc, RwLock};

/// Schema provider trait for generating schemas from applications
#[async_trait]
pub trait SchemaProvider: Send + Sync {
    /// Returns the schema type this provider generates
    fn schema_type(&self) -> SchemaType;

    /// Generates a schema from the application
    ///
    /// Returns the schema as a JSON value
    async fn generate(&self, app: &dyn Application) -> Result<serde_json::Value>;

    /// Validates a generated schema for correctness
    fn validate(&self, schema: &serde_json::Value) -> Result<()>;

    /// Calculates the SHA256 hash of a schema
    fn hash(&self, schema: &serde_json::Value) -> Result<String> {
        calculate_schema_checksum(schema)
    }

    /// Serializes schema to bytes for storage/transmission
    fn serialize(&self, schema: &serde_json::Value) -> Result<Vec<u8>> {
        serde_json::to_vec(schema).map_err(Error::from)
    }

    /// Returns the HTTP endpoint where the schema is served (if any)
    fn endpoint(&self) -> Option<String> {
        None
    }

    /// Returns the specification version (e.g., "3.1.0" for OpenAPI)
    fn spec_version(&self) -> String;

    /// Returns the content type for the schema
    fn content_type(&self) -> String {
        "application/json".to_string()
    }
}

/// Application trait for abstracting application interfaces
///
/// This allows providers to generate schemas without depending on specific
/// framework implementations.
pub trait Application: Send + Sync {
    /// Returns the application/service name
    fn name(&self) -> &str;

    /// Returns the application version
    fn version(&self) -> &str;

    /// Returns route information for schema generation
    ///
    /// The actual type depends on the framework and schema provider
    fn routes(&self) -> Box<dyn std::any::Any + Send + Sync>;
}

/// Base schema provider with common functionality
pub struct BaseSchemaProvider {
    schema_type: SchemaType,
    spec_version: String,
    content_type: String,
    endpoint: Option<String>,
}

impl BaseSchemaProvider {
    /// Creates a new base schema provider
    pub fn new(
        schema_type: SchemaType,
        spec_version: impl Into<String>,
        content_type: impl Into<String>,
        endpoint: Option<String>,
    ) -> Self {
        Self {
            schema_type,
            spec_version: spec_version.into(),
            content_type: content_type.into(),
            endpoint,
        }
    }

    /// Gets the schema type
    pub fn get_schema_type(&self) -> SchemaType {
        self.schema_type
    }

    /// Gets the spec version
    pub fn get_spec_version(&self) -> &str {
        &self.spec_version
    }

    /// Gets the content type
    pub fn get_content_type(&self) -> &str {
        &self.content_type
    }

    /// Gets the endpoint
    pub fn get_endpoint(&self) -> Option<&str> {
        self.endpoint.as_deref()
    }
}

/// Thread-safe registry for schema providers
#[derive(Clone)]
pub struct ProviderRegistry {
    providers: Arc<RwLock<HashMap<SchemaType, Arc<dyn SchemaProvider>>>>,
}

impl ProviderRegistry {
    /// Creates a new provider registry
    pub fn new() -> Self {
        Self {
            providers: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    /// Registers a schema provider
    pub fn register(&self, provider: Arc<dyn SchemaProvider>) {
        let schema_type = provider.schema_type();
        let mut providers = self.providers.write().unwrap();
        providers.insert(schema_type, provider);
    }

    /// Gets a provider by schema type
    pub fn get(&self, schema_type: SchemaType) -> Option<Arc<dyn SchemaProvider>> {
        let providers = self.providers.read().unwrap();
        providers.get(&schema_type).cloned()
    }

    /// Checks if a provider exists for a schema type
    pub fn has(&self, schema_type: SchemaType) -> bool {
        let providers = self.providers.read().unwrap();
        providers.contains_key(&schema_type)
    }

    /// Lists all registered schema types
    pub fn list(&self) -> Vec<SchemaType> {
        let providers = self.providers.read().unwrap();
        providers.keys().copied().collect()
    }

    /// Unregisters a provider
    pub fn unregister(&self, schema_type: SchemaType) -> bool {
        let mut providers = self.providers.write().unwrap();
        providers.remove(&schema_type).is_some()
    }

    /// Clears all registered providers
    pub fn clear(&self) {
        let mut providers = self.providers.write().unwrap();
        providers.clear();
    }
}

impl Default for ProviderRegistry {
    fn default() -> Self {
        Self::new()
    }
}

// Global provider registry
static GLOBAL_REGISTRY: once_cell::sync::Lazy<ProviderRegistry> =
    once_cell::sync::Lazy::new(ProviderRegistry::new);

/// Registers a schema provider globally
pub fn register_provider(provider: Arc<dyn SchemaProvider>) {
    GLOBAL_REGISTRY.register(provider);
}

/// Gets a provider from the global registry
pub fn get_provider(schema_type: SchemaType) -> Option<Arc<dyn SchemaProvider>> {
    GLOBAL_REGISTRY.get(schema_type)
}

/// Checks if a provider exists in the global registry
pub fn has_provider(schema_type: SchemaType) -> bool {
    GLOBAL_REGISTRY.has(schema_type)
}

/// Lists all providers in the global registry
pub fn list_providers() -> Vec<SchemaType> {
    GLOBAL_REGISTRY.list()
}

/// Unregisters a provider from the global registry
pub fn unregister_provider(schema_type: SchemaType) -> bool {
    GLOBAL_REGISTRY.unregister(schema_type)
}

/// Clears all providers from the global registry
pub fn clear_providers() {
    GLOBAL_REGISTRY.clear();
}

#[cfg(test)]
mod tests {
    use super::*;

    struct TestProvider {
        base: BaseSchemaProvider,
    }

    #[async_trait]
    impl SchemaProvider for TestProvider {
        fn schema_type(&self) -> SchemaType {
            self.base.get_schema_type()
        }

        async fn generate(&self, _app: &dyn Application) -> Result<serde_json::Value> {
            Ok(serde_json::json!({"test": "schema"}))
        }

        fn validate(&self, _schema: &serde_json::Value) -> Result<()> {
            Ok(())
        }

        fn spec_version(&self) -> String {
            self.base.get_spec_version().to_string()
        }
    }

    #[test]
    fn test_provider_registry() {
        let registry = ProviderRegistry::new();

        let provider = Arc::new(TestProvider {
            base: BaseSchemaProvider::new(SchemaType::OpenAPI, "3.1.0", "application/json", None),
        });

        registry.register(provider.clone());

        assert!(registry.has(SchemaType::OpenAPI));
        assert!(!registry.has(SchemaType::AsyncAPI));

        let retrieved = registry.get(SchemaType::OpenAPI);
        assert!(retrieved.is_some());

        let types = registry.list();
        assert_eq!(types.len(), 1);

        registry.unregister(SchemaType::OpenAPI);
        assert!(!registry.has(SchemaType::OpenAPI));
    }

    #[test]
    fn test_base_provider() {
        let base = BaseSchemaProvider::new(
            SchemaType::OpenAPI,
            "3.1.0",
            "application/json",
            Some("/openapi.json".to_string()),
        );

        assert_eq!(base.get_schema_type(), SchemaType::OpenAPI);
        assert_eq!(base.get_spec_version(), "3.1.0");
        assert_eq!(base.get_content_type(), "application/json");
        assert_eq!(base.get_endpoint(), Some("/openapi.json"));
    }
}
