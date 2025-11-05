//! GraphQL schema provider implementation

use crate::errors::{Error, Result};
use crate::provider::{Application, SchemaProvider};
use crate::types::SchemaType;
use async_trait::async_trait;

/// GraphQL schema provider
///
/// Generates GraphQL Schema Definition Language (SDL) from application
pub struct GraphQLProvider {
    spec_version: String,
    endpoint: Option<String>,
}

impl GraphQLProvider {
    /// Creates a new GraphQL provider
    pub fn new(spec_version: impl Into<String>, endpoint: Option<String>) -> Self {
        Self {
            spec_version: spec_version.into(),
            endpoint,
        }
    }
}

impl Default for GraphQLProvider {
    fn default() -> Self {
        Self::new("2023", Some("/graphql".to_string()))
    }
}

#[async_trait]
impl SchemaProvider for GraphQLProvider {
    fn schema_type(&self) -> SchemaType {
        SchemaType::GraphQL
    }

    async fn generate(&self, app: &dyn Application) -> Result<serde_json::Value> {
        let schema = serde_json::json!({
            "version": self.spec_version,
            "schema": format!("type Query {{\n  {}: String\n}}", app.name()),
            "types": []
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
            "test_app"
        }

        fn version(&self) -> &str {
            "1.0.0"
        }

        fn routes(&self) -> Box<dyn std::any::Any + Send + Sync> {
            Box::new(())
        }
    }

    #[tokio::test]
    async fn test_graphql_provider() {
        let provider = GraphQLProvider::default();
        let app = TestApp;

        let schema = provider.generate(&app).await.unwrap();
        provider.validate(&schema).unwrap();
    }
}
