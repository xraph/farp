//! AsyncAPI schema provider implementation

use crate::errors::{Error, Result};
use crate::provider::{Application, SchemaProvider};
use crate::types::SchemaType;
use async_trait::async_trait;

/// AsyncAPI schema provider
///
/// Generates AsyncAPI 3.0.0 specifications for async/streaming endpoints
pub struct AsyncAPIProvider {
    spec_version: String,
    endpoint: Option<String>,
}

impl AsyncAPIProvider {
    /// Creates a new AsyncAPI provider
    pub fn new(spec_version: impl Into<String>, endpoint: Option<String>) -> Self {
        Self {
            spec_version: spec_version.into(),
            endpoint,
        }
    }

    /// Creates a default AsyncAPI 3.0.0 provider
    pub fn default_v3() -> Self {
        Self::new("3.0.0", Some("/asyncapi.json".to_string()))
    }
}

impl Default for AsyncAPIProvider {
    fn default() -> Self {
        Self::default_v3()
    }
}

#[async_trait]
impl SchemaProvider for AsyncAPIProvider {
    fn schema_type(&self) -> SchemaType {
        SchemaType::AsyncAPI
    }

    async fn generate(&self, app: &dyn Application) -> Result<serde_json::Value> {
        let schema = serde_json::json!({
            "asyncapi": self.spec_version,
            "info": {
                "title": app.name(),
                "version": app.version(),
                "description": format!("Async API documentation for {}", app.name())
            },
            "channels": {}
        });

        Ok(schema)
    }

    fn validate(&self, schema: &serde_json::Value) -> Result<()> {
        if !schema.is_object() {
            return Err(Error::validation_failed("schema must be an object"));
        }

        let obj = schema.as_object().unwrap();

        if !obj.contains_key("asyncapi") {
            return Err(Error::validation_failed("missing 'asyncapi' field"));
        }

        if !obj.contains_key("info") {
            return Err(Error::validation_failed("missing 'info' field"));
        }

        Ok(())
    }

    fn spec_version(&self) -> String {
        self.spec_version.clone()
    }

    fn endpoint(&self) -> Option<String> {
        self.endpoint.clone()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    struct TestApp;

    impl Application for TestApp {
        fn name(&self) -> &str {
            "test-app"
        }

        fn version(&self) -> &str {
            "1.0.0"
        }

        fn routes(&self) -> Box<dyn std::any::Any + Send + Sync> {
            Box::new(())
        }
    }

    #[tokio::test]
    async fn test_asyncapi_provider() {
        let provider = AsyncAPIProvider::default();
        let app = TestApp;

        let schema = provider.generate(&app).await.unwrap();
        assert!(schema.is_object());

        provider.validate(&schema).unwrap();
    }
}
