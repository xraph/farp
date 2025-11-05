//! gRPC schema provider implementation

use crate::errors::{Error, Result};
use crate::provider::{Application, SchemaProvider};
use crate::types::SchemaType;
use async_trait::async_trait;

/// gRPC schema provider
///
/// Generates Protocol Buffer definitions and gRPC service descriptors
pub struct GRPCProvider {
    spec_version: String,
}

impl GRPCProvider {
    /// Creates a new gRPC provider
    pub fn new(spec_version: impl Into<String>) -> Self {
        Self {
            spec_version: spec_version.into(),
        }
    }
}

impl Default for GRPCProvider {
    fn default() -> Self {
        Self::new("proto3")
    }
}

#[async_trait]
impl SchemaProvider for GRPCProvider {
    fn schema_type(&self) -> SchemaType {
        SchemaType::GRPC
    }

    async fn generate(&self, app: &dyn Application) -> Result<serde_json::Value> {
        let schema = serde_json::json!({
            "syntax": self.spec_version,
            "package": app.name(),
            "services": [],
            "messages": []
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
        "application/x-protobuf".to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    struct TestApp;

    impl Application for TestApp {
        fn name(&self) -> &str {
            "test_service"
        }

        fn version(&self) -> &str {
            "1.0.0"
        }

        fn routes(&self) -> Box<dyn std::any::Any + Send + Sync> {
            Box::new(())
        }
    }

    #[tokio::test]
    async fn test_grpc_provider() {
        let provider = GRPCProvider::default();
        let app = TestApp;

        let schema = provider.generate(&app).await.unwrap();
        provider.validate(&schema).unwrap();
    }
}
