//! OpenAPI schema provider implementation

use crate::errors::{Error, Result};
use crate::provider::{Application, SchemaProvider};
use crate::types::{RouteDescriptor, SchemaType};
use async_trait::async_trait;
use std::collections::HashMap;

/// OpenAPI schema provider
///
/// Generates OpenAPI 3.1.0 specifications from application routes
pub struct OpenAPIProvider {
    spec_version: String,
    endpoint: Option<String>,
}

impl OpenAPIProvider {
    /// Creates a new OpenAPI provider
    ///
    /// # Arguments
    ///
    /// * `spec_version` - OpenAPI specification version (e.g., "3.1.0")
    /// * `endpoint` - Optional HTTP endpoint where schema is served
    pub fn new(spec_version: impl Into<String>, endpoint: Option<String>) -> Self {
        Self {
            spec_version: spec_version.into(),
            endpoint,
        }
    }

    /// Creates a default OpenAPI 3.1.0 provider
    pub fn default_v3_1() -> Self {
        Self::new("3.1.0", Some("/openapi.json".to_string()))
    }

    /// Build OpenAPI paths from application routes.
    ///
    /// Attempts to downcast `app.routes()` to `Vec<RouteDescriptor>` and
    /// converts each route into an OpenAPI path item. Falls back to empty
    /// paths if the routes type is unrecognized.
    fn build_paths(&self, app: &dyn Application) -> serde_json::Value {
        let routes_any = app.routes();

        // Try to downcast to Vec<RouteDescriptor>
        if let Some(routes) = routes_any.downcast_ref::<Vec<RouteDescriptor>>() {
            let mut paths: HashMap<String, serde_json::Value> = HashMap::new();

            for rd in routes {
                let mut path_item = paths
                    .remove(&rd.path)
                    .and_then(|v| {
                        if let serde_json::Value::Object(map) = v {
                            Some(map)
                        } else {
                            None
                        }
                    })
                    .unwrap_or_default();

                let methods = if rd.methods.is_empty() {
                    vec!["get".to_string()]
                } else {
                    rd.methods.iter().map(|m| m.to_lowercase()).collect()
                };

                for method in &methods {
                    let mut operation = serde_json::json!({
                        "responses": {
                            "200": {
                                "description": "Successful response"
                            }
                        }
                    });

                    if let Some(ref op_id) = rd.operation_id {
                        operation["operationId"] =
                            serde_json::Value::String(format!("{method}_{op_id}"));
                    }

                    if let Some(ref metadata) = rd.metadata {
                        if let Some(summary) = metadata.get("summary") {
                            operation["summary"] = summary.clone();
                        }
                        if let Some(description) = metadata.get("description") {
                            operation["description"] = description.clone();
                        }
                        if let Some(tags) = metadata.get("tags") {
                            operation["tags"] = tags.clone();
                        }
                    }

                    path_item.insert(method.clone(), operation);
                }

                paths.insert(rd.path.clone(), serde_json::Value::Object(path_item));
            }

            return serde_json::to_value(paths).unwrap_or_default();
        }

        // Try to downcast to serde_json::Value (already OpenAPI paths)
        if let Some(value) = routes_any.downcast_ref::<serde_json::Value>() {
            if value.is_object() {
                return value.clone();
            }
        }

        // Fallback: empty paths
        serde_json::json!({})
    }
}

impl Default for OpenAPIProvider {
    fn default() -> Self {
        Self::default_v3_1()
    }
}

#[async_trait]
impl SchemaProvider for OpenAPIProvider {
    fn schema_type(&self) -> SchemaType {
        SchemaType::OpenAPI
    }

    async fn generate(&self, app: &dyn Application) -> Result<serde_json::Value> {
        // Build OpenAPI paths from application routes
        let paths = self.build_paths(app);

        let schema = serde_json::json!({
            "openapi": self.spec_version,
            "info": {
                "title": app.name(),
                "version": app.version(),
                "description": format!("API documentation for {}", app.name())
            },
            "servers": [{
                "url": "/"
            }],
            "paths": paths,
            "components": {
                "schemas": {}
            }
        });

        Ok(schema)
    }

    fn validate(&self, schema: &serde_json::Value) -> Result<()> {
        // Basic validation - check required fields
        if !schema.is_object() {
            return Err(Error::validation_failed("schema must be an object"));
        }

        let obj = schema.as_object().unwrap();

        if !obj.contains_key("openapi") {
            return Err(Error::validation_failed("missing 'openapi' field"));
        }

        if !obj.contains_key("info") {
            return Err(Error::validation_failed("missing 'info' field"));
        }

        if !obj.contains_key("paths") {
            return Err(Error::validation_failed("missing 'paths' field"));
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
    async fn test_openapi_provider() {
        let provider = OpenAPIProvider::default();
        let app = TestApp;

        let schema = provider.generate(&app).await.unwrap();
        assert!(schema.is_object());

        provider.validate(&schema).unwrap();
    }

    #[test]
    fn test_openapi_provider_properties() {
        let provider = OpenAPIProvider::default_v3_1();
        assert_eq!(provider.schema_type(), SchemaType::OpenAPI);
        assert_eq!(provider.spec_version(), "3.1.0");
        assert_eq!(provider.endpoint(), Some("/openapi.json".to_string()));
    }
}
