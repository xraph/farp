//! Apache Thrift schema provider implementation

use crate::errors::{Error, Result};
use crate::provider::{Application, SchemaProvider};
use crate::types::SchemaType;
use async_trait::async_trait;

/// Apache Thrift schema provider
///
/// Generates Thrift IDL definitions
pub struct ThriftProvider {
    spec_version: String,
}

impl ThriftProvider {
    /// Creates a new Thrift provider
    pub fn new(spec_version: impl Into<String>) -> Self {
        Self {
            spec_version: spec_version.into(),
        }
    }
}

impl Default for ThriftProvider {
    fn default() -> Self {
        Self::new("0.19.0")
    }
}

#[async_trait]
impl SchemaProvider for ThriftProvider {
    fn schema_type(&self) -> SchemaType {
        SchemaType::Thrift
    }

    async fn generate(&self, app: &dyn Application) -> Result<serde_json::Value> {
        let schema = serde_json::json!({
            "namespace": format!("com.{}", app.name()),
            "services": [],
            "structs": []
        });

        Ok(schema)
    }

    fn validate(&self, schema: &serde_json::Value) -> Result<()> {
        if !schema.is_object() {
            return Err(Error::validation_failed("schema must be an object"));
        }

        Ok(())
    }

    fn spec_version(&self) -> String {
        self.spec_version.clone()
    }

    fn content_type(&self) -> String {
        "application/x-thrift".to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    struct TestApp;

    impl Application for TestApp {
        fn name(&self) -> &str {
            "TestService"
        }

        fn version(&self) -> &str {
            "1.0.0"
        }

        fn routes(&self) -> Box<dyn std::any::Any + Send + Sync> {
            Box::new(())
        }
    }

    #[tokio::test]
    async fn test_thrift_provider() {
        let provider = ThriftProvider::default();
        let app = TestApp;

        let schema = provider.generate(&app).await.unwrap();
        provider.validate(&schema).unwrap();
    }
}
