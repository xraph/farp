//! Gateway client for watching service changes and converting schemas to routes.

use crate::errors::{Error, Result};
use crate::registry::{EventType, ManifestEvent, SchemaRegistry};
use crate::types::{LocationType, SchemaDescriptor, SchemaManifest, SchemaType};
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// Gateway client for API gateway integration
///
/// Watches for service schema changes and provides conversion utilities
/// to gateway-specific route configurations.
pub struct Client {
    registry: Arc<dyn SchemaRegistry>,
    manifest_cache: Arc<RwLock<HashMap<String, SchemaManifest>>>,
    schema_cache: Arc<RwLock<HashMap<String, serde_json::Value>>>,
}

impl Client {
    /// Creates a new gateway client
    pub fn new(registry: Arc<dyn SchemaRegistry>) -> Self {
        Self {
            registry,
            manifest_cache: Arc::new(RwLock::new(HashMap::new())),
            schema_cache: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    /// Watches for service registrations and schema updates
    ///
    /// `on_change` is called whenever services are added, updated, or removed
    pub async fn watch_services<F>(&self, service_name: &str, on_change: Arc<F>) -> Result<()>
    where
        F: Fn(Vec<ServiceRoute>) + Send + Sync + 'static,
    {
        // Initial load
        let manifests = self.registry.list_manifests(service_name).await?;

        // Convert initial manifests to routes
        let routes = self.convert_to_routes(&manifests).await;
        on_change(routes);

        // Create handler for watch events
        let manifest_cache = self.manifest_cache.clone();
        let registry = self.registry.clone();
        let schema_cache = self.schema_cache.clone();
        let service_name = service_name.to_string();
        let on_change_ref = on_change.clone();

        let handler = Box::new(move |event: &ManifestEvent| {
            let manifest_cache = manifest_cache.clone();
            let schema_cache = schema_cache.clone();
            let registry = registry.clone();
            let event = event.clone();
            let on_change = on_change_ref.clone();

            tokio::spawn(async move {
                // Update manifest cache
                let mut cache = manifest_cache.write().await;
                match event.event_type {
                    EventType::Added | EventType::Updated => {
                        cache.insert(event.manifest.instance_id.clone(), event.manifest.clone());
                    }
                    EventType::Removed => {
                        cache.remove(&event.manifest.instance_id);
                    }
                }

                // Get all cached manifests
                let manifests: Vec<SchemaManifest> = cache.values().cloned().collect();
                drop(cache);

                // Convert to routes
                let client = Client {
                    registry: registry.clone(),
                    manifest_cache: manifest_cache.clone(),
                    schema_cache: schema_cache.clone(),
                };

                let routes = client.convert_to_routes(&manifests).await;
                on_change(routes);
            });
        });

        // Watch for changes
        self.registry.watch_manifests(&service_name, handler).await
    }

    /// Converts service manifests to gateway routes
    ///
    /// This is a reference implementation - actual gateways should customize this
    pub async fn convert_to_routes(&self, manifests: &[SchemaManifest]) -> Vec<ServiceRoute> {
        let mut routes = Vec::new();

        for manifest in manifests {
            for schema_desc in &manifest.schemas {
                // Fetch schema
                let schema = match self.fetch_schema(schema_desc).await {
                    Ok(s) => s,
                    Err(_) => continue,
                };

                // Convert schema to routes based on type
                match schema_desc.schema_type {
                    SchemaType::OpenAPI => {
                        routes.extend(self.convert_openapi_to_routes(manifest, &schema));
                    }
                    SchemaType::AsyncAPI => {
                        routes.extend(self.convert_asyncapi_to_routes(manifest, &schema));
                    }
                    SchemaType::GraphQL => {
                        routes.extend(self.convert_graphql_to_routes(manifest, &schema));
                    }
                    _ => {}
                }
            }
        }

        routes
    }

    /// Fetches a schema based on its descriptor
    async fn fetch_schema(&self, descriptor: &SchemaDescriptor) -> Result<serde_json::Value> {
        // Check cache first
        {
            let cache = self.schema_cache.read().await;
            if let Some(schema) = cache.get(&descriptor.hash) {
                return Ok(schema.clone());
            }
        }

        // Fetch based on location type
        let schema = match descriptor.location.location_type {
            LocationType::Inline => descriptor
                .inline_schema
                .clone()
                .ok_or_else(|| Error::invalid_location("inline schema is missing"))?,
            LocationType::Registry => {
                let path = descriptor
                    .location
                    .registry_path
                    .as_ref()
                    .ok_or_else(|| Error::invalid_location("registry path is missing"))?;
                self.registry.fetch_schema(path).await?
            }
            LocationType::HTTP => {
                // HTTP fetch not implemented in this reference implementation
                return Err(Error::schema_fetch_failed("HTTP fetch not implemented"));
            }
        };

        // Cache the schema
        {
            let mut cache = self.schema_cache.write().await;
            cache.insert(descriptor.hash.clone(), schema.clone());
        }

        Ok(schema)
    }

    /// Converts an OpenAPI schema to gateway routes
    fn convert_openapi_to_routes(
        &self,
        manifest: &SchemaManifest,
        schema: &serde_json::Value,
    ) -> Vec<ServiceRoute> {
        let mut routes = Vec::new();

        if let Some(paths) = schema.get("paths").and_then(|p| p.as_object()) {
            let base_url = format!("http://{}:8080", manifest.service_name);

            for (path, path_item) in paths {
                if let Some(path_obj) = path_item.as_object() {
                    let methods: Vec<String> = path_obj
                        .keys()
                        .filter(|k| {
                            matches!(
                                k.as_str(),
                                "get" | "post" | "put" | "delete" | "patch" | "options" | "head"
                            )
                        })
                        .map(|k| k.to_uppercase())
                        .collect();

                    if !methods.is_empty() {
                        routes.push(ServiceRoute {
                            path: path.clone(),
                            methods,
                            target_url: format!("{base_url}{path}"),
                            health_url: format!("{}{}", base_url, manifest.endpoints.health),
                            service_name: manifest.service_name.clone(),
                            service_version: manifest.service_version.clone(),
                            middleware: Vec::new(),
                            metadata: [("schema_type".to_string(), "openapi".into())]
                                .iter()
                                .cloned()
                                .collect(),
                        });
                    }
                }
            }
        }

        routes
    }

    /// Converts an AsyncAPI schema to gateway routes (WebSocket, SSE)
    fn convert_asyncapi_to_routes(
        &self,
        manifest: &SchemaManifest,
        schema: &serde_json::Value,
    ) -> Vec<ServiceRoute> {
        let mut routes = Vec::new();

        if let Some(channels) = schema.get("channels").and_then(|c| c.as_object()) {
            let base_url = format!("http://{}:8080", manifest.service_name);

            for channel_path in channels.keys() {
                routes.push(ServiceRoute {
                    path: channel_path.clone(),
                    methods: vec!["WEBSOCKET".to_string()],
                    target_url: format!("{base_url}{channel_path}"),
                    health_url: format!("{}{}", base_url, manifest.endpoints.health),
                    service_name: manifest.service_name.clone(),
                    service_version: manifest.service_version.clone(),
                    middleware: Vec::new(),
                    metadata: [
                        ("schema_type".to_string(), "asyncapi".into()),
                        ("protocol".to_string(), "websocket".into()),
                    ]
                    .iter()
                    .cloned()
                    .collect(),
                });
            }
        }

        routes
    }

    /// Converts a GraphQL schema to a gateway route
    fn convert_graphql_to_routes(
        &self,
        manifest: &SchemaManifest,
        _schema: &serde_json::Value,
    ) -> Vec<ServiceRoute> {
        let base_url = format!("http://{}:8080", manifest.service_name);
        let graphql_path = manifest
            .endpoints
            .graphql
            .clone()
            .unwrap_or_else(|| "/graphql".to_string());

        vec![ServiceRoute {
            path: graphql_path.clone(),
            methods: vec!["POST".to_string(), "GET".to_string()],
            target_url: format!("{base_url}{graphql_path}"),
            health_url: format!("{}{}", base_url, manifest.endpoints.health),
            service_name: manifest.service_name.clone(),
            service_version: manifest.service_version.clone(),
            middleware: Vec::new(),
            metadata: [("schema_type".to_string(), "graphql".into())]
                .iter()
                .cloned()
                .collect(),
        }]
    }

    /// Clears the schema cache
    pub async fn clear_cache(&self) {
        let mut cache = self.schema_cache.write().await;
        cache.clear();
    }

    /// Retrieves a cached manifest by instance ID
    pub async fn get_manifest(&self, instance_id: &str) -> Option<SchemaManifest> {
        let cache = self.manifest_cache.read().await;
        cache.get(instance_id).cloned()
    }
}

/// Service route configuration for the gateway
#[derive(Debug, Clone)]
pub struct ServiceRoute {
    /// Path pattern for the route
    pub path: String,
    /// HTTP methods for this route
    pub methods: Vec<String>,
    /// Target backend URL
    pub target_url: String,
    /// Health check URL
    pub health_url: String,
    /// Backend service name
    pub service_name: String,
    /// Backend service version
    pub service_version: String,
    /// Middleware names to apply
    pub middleware: Vec<String>,
    /// Additional route metadata
    pub metadata: HashMap<String, serde_json::Value>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::manifest::new_manifest;
    use crate::registry::memory::MemoryRegistry;

    #[tokio::test]
    async fn test_gateway_client() {
        let registry = Arc::new(MemoryRegistry::new());
        let client = Client::new(registry.clone());

        let mut manifest = new_manifest("test-service", "v1.0.0", "instance-123");
        manifest.endpoints.health = "/health".to_string();
        manifest.update_checksum().unwrap();

        registry.register_manifest(&manifest).await.unwrap();

        let manifests = vec![manifest];
        let routes = client.convert_to_routes(&manifests).await;

        assert!(routes.is_empty()); // No schemas registered yet
    }

    #[tokio::test]
    async fn test_convert_openapi_to_routes() {
        let registry = Arc::new(MemoryRegistry::new());
        let client = Client::new(registry);

        let mut manifest = new_manifest("user-service", "v1.0.0", "instance-123");
        manifest.endpoints.health = "/health".to_string();

        let schema = serde_json::json!({
            "openapi": "3.1.0",
            "paths": {
                "/users": {
                    "get": {},
                    "post": {}
                }
            }
        });

        let routes = client.convert_openapi_to_routes(&manifest, &schema);
        assert_eq!(routes.len(), 1);
        assert_eq!(routes[0].path, "/users");
        assert_eq!(routes[0].methods, vec!["GET", "POST"]);
    }
}
