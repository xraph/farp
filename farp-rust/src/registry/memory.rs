//! In-memory registry implementation for testing and development.

use crate::errors::{Error, Result};
use crate::registry::{
    EventType, ManifestChangeHandler, ManifestEvent, SchemaChangeHandler, SchemaRegistry,
};
use crate::types::SchemaManifest;
use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// In-memory registry implementation
///
/// Thread-safe, useful for testing and development.
/// Not recommended for production use.
#[derive(Clone)]
pub struct MemoryRegistry {
    inner: Arc<RegistryInner>,
}

struct RegistryInner {
    manifests: RwLock<HashMap<String, SchemaManifest>>,
    schemas: RwLock<HashMap<String, serde_json::Value>>,
    watchers: RwLock<HashMap<String, Vec<tokio::sync::mpsc::UnboundedSender<ManifestEvent>>>>,
    closed: RwLock<bool>,
}

impl MemoryRegistry {
    /// Creates a new in-memory registry
    pub fn new() -> Self {
        Self {
            inner: Arc::new(RegistryInner {
                manifests: RwLock::new(HashMap::new()),
                schemas: RwLock::new(HashMap::new()),
                watchers: RwLock::new(HashMap::new()),
                closed: RwLock::new(false),
            }),
        }
    }

    /// Checks if the registry is closed
    async fn is_closed(&self) -> bool {
        *self.inner.closed.read().await
    }

    /// Notifies watchers of a manifest change
    async fn notify_watchers(&self, service_name: &str, event: ManifestEvent) {
        let watchers = self.inner.watchers.read().await;

        // Notify specific service watchers
        if let Some(service_watchers) = watchers.get(service_name) {
            for sender in service_watchers {
                let _ = sender.send(event.clone());
            }
        }

        // Notify global watchers (empty service name)
        if let Some(global_watchers) = watchers.get("") {
            for sender in global_watchers {
                let _ = sender.send(event.clone());
            }
        }
    }

    /// Clears all manifests and schemas (useful for testing)
    pub async fn clear(&self) {
        let mut manifests = self.inner.manifests.write().await;
        let mut schemas = self.inner.schemas.write().await;
        manifests.clear();
        schemas.clear();
    }
}

impl Default for MemoryRegistry {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl SchemaRegistry for MemoryRegistry {
    async fn register_manifest(&self, manifest: &SchemaManifest) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }

        // Validate manifest
        manifest.validate()?;

        let mut manifests = self.inner.manifests.write().await;
        manifests.insert(manifest.instance_id.clone(), manifest.clone());

        // Notify watchers
        let event = ManifestEvent {
            event_type: EventType::Added,
            manifest: manifest.clone(),
            timestamp: chrono::Utc::now().timestamp(),
        };
        drop(manifests); // Release lock before notifying
        self.notify_watchers(&manifest.service_name, event).await;

        Ok(())
    }

    async fn get_manifest(&self, instance_id: &str) -> Result<SchemaManifest> {
        let manifests = self.inner.manifests.read().await;
        manifests
            .get(instance_id)
            .cloned()
            .ok_or(Error::ManifestNotFound)
    }

    async fn update_manifest(&self, manifest: &SchemaManifest) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }

        // Validate manifest
        manifest.validate()?;

        let mut manifests = self.inner.manifests.write().await;
        if !manifests.contains_key(&manifest.instance_id) {
            return Err(Error::ManifestNotFound);
        }

        manifests.insert(manifest.instance_id.clone(), manifest.clone());

        // Notify watchers
        let event = ManifestEvent {
            event_type: EventType::Updated,
            manifest: manifest.clone(),
            timestamp: chrono::Utc::now().timestamp(),
        };
        drop(manifests); // Release lock before notifying
        self.notify_watchers(&manifest.service_name, event).await;

        Ok(())
    }

    async fn delete_manifest(&self, instance_id: &str) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }

        let mut manifests = self.inner.manifests.write().await;
        let manifest = manifests
            .remove(instance_id)
            .ok_or(Error::ManifestNotFound)?;

        // Notify watchers
        let event = ManifestEvent {
            event_type: EventType::Removed,
            manifest: manifest.clone(),
            timestamp: chrono::Utc::now().timestamp(),
        };
        drop(manifests); // Release lock before notifying
        self.notify_watchers(&manifest.service_name, event).await;

        Ok(())
    }

    async fn list_manifests(&self, service_name: &str) -> Result<Vec<SchemaManifest>> {
        let manifests = self.inner.manifests.read().await;
        let results: Vec<SchemaManifest> = manifests
            .values()
            .filter(|m| service_name.is_empty() || m.service_name == service_name)
            .cloned()
            .collect();
        Ok(results)
    }

    async fn publish_schema(&self, path: &str, schema: &serde_json::Value) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }

        let mut schemas = self.inner.schemas.write().await;
        schemas.insert(path.to_string(), schema.clone());
        Ok(())
    }

    async fn fetch_schema(&self, path: &str) -> Result<serde_json::Value> {
        let schemas = self.inner.schemas.read().await;
        schemas.get(path).cloned().ok_or(Error::SchemaNotFound)
    }

    async fn delete_schema(&self, path: &str) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }

        let mut schemas = self.inner.schemas.write().await;
        schemas.remove(path);
        Ok(())
    }

    async fn watch_manifests(
        &self,
        service_name: &str,
        on_change: Box<dyn ManifestChangeHandler>,
    ) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }

        let (tx, mut rx) = tokio::sync::mpsc::unbounded_channel();

        // Register watcher
        {
            let mut watchers = self.inner.watchers.write().await;
            watchers
                .entry(service_name.to_string())
                .or_insert_with(Vec::new)
                .push(tx);
        }

        // Start watching
        tokio::spawn(async move {
            while let Some(event) = rx.recv().await {
                on_change.on_change(&event);
            }
        });

        Ok(())
    }

    async fn watch_schemas(
        &self,
        _path: &str,
        _on_change: Box<dyn SchemaChangeHandler>,
    ) -> Result<()> {
        // Schema watching not implemented in memory registry
        Err(Error::Custom(
            "schema watching not supported in memory registry".to_string(),
        ))
    }

    async fn close(&self) -> Result<()> {
        let mut closed = self.inner.closed.write().await;
        if *closed {
            return Ok(());
        }

        *closed = true;

        // Clear watchers
        let mut watchers = self.inner.watchers.write().await;
        watchers.clear();

        Ok(())
    }

    async fn health(&self) -> Result<()> {
        if self.is_closed().await {
            return Err(Error::backend_unavailable("registry is closed"));
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::manifest::new_manifest;

    #[tokio::test]
    async fn test_register_and_get_manifest() {
        let registry = MemoryRegistry::new();
        let mut manifest = new_manifest("test-service", "v1.0.0", "instance-123");
        manifest.endpoints.health = "/health".to_string();
        manifest.update_checksum().unwrap();

        registry.register_manifest(&manifest).await.unwrap();

        let retrieved = registry.get_manifest("instance-123").await.unwrap();
        assert_eq!(retrieved.service_name, "test-service");
        assert_eq!(retrieved.instance_id, "instance-123");
    }

    #[tokio::test]
    async fn test_update_manifest() {
        let registry = MemoryRegistry::new();
        let mut manifest = new_manifest("test-service", "v1.0.0", "instance-123");
        manifest.endpoints.health = "/health".to_string();
        manifest.update_checksum().unwrap();

        registry.register_manifest(&manifest).await.unwrap();

        manifest.service_version = "v2.0.0".to_string();
        manifest.update_checksum().unwrap();
        registry.update_manifest(&manifest).await.unwrap();

        let retrieved = registry.get_manifest("instance-123").await.unwrap();
        assert_eq!(retrieved.service_version, "v2.0.0");
    }

    #[tokio::test]
    async fn test_delete_manifest() {
        let registry = MemoryRegistry::new();
        let mut manifest = new_manifest("test-service", "v1.0.0", "instance-123");
        manifest.endpoints.health = "/health".to_string();
        manifest.update_checksum().unwrap();

        registry.register_manifest(&manifest).await.unwrap();
        registry.delete_manifest("instance-123").await.unwrap();

        let result = registry.get_manifest("instance-123").await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_list_manifests() {
        let registry = MemoryRegistry::new();

        let mut manifest1 = new_manifest("service-a", "v1.0.0", "instance-1");
        manifest1.endpoints.health = "/health".to_string();
        manifest1.update_checksum().unwrap();

        let mut manifest2 = new_manifest("service-a", "v1.0.0", "instance-2");
        manifest2.endpoints.health = "/health".to_string();
        manifest2.update_checksum().unwrap();

        let mut manifest3 = new_manifest("service-b", "v1.0.0", "instance-3");
        manifest3.endpoints.health = "/health".to_string();
        manifest3.update_checksum().unwrap();

        registry.register_manifest(&manifest1).await.unwrap();
        registry.register_manifest(&manifest2).await.unwrap();
        registry.register_manifest(&manifest3).await.unwrap();

        let service_a_manifests = registry.list_manifests("service-a").await.unwrap();
        assert_eq!(service_a_manifests.len(), 2);

        let all_manifests = registry.list_manifests("").await.unwrap();
        assert_eq!(all_manifests.len(), 3);
    }

    #[tokio::test]
    async fn test_publish_and_fetch_schema() {
        let registry = MemoryRegistry::new();
        let schema = serde_json::json!({"openapi": "3.1.0", "info": {"title": "Test API"}});

        registry
            .publish_schema("/schemas/test/openapi", &schema)
            .await
            .unwrap();

        let fetched = registry
            .fetch_schema("/schemas/test/openapi")
            .await
            .unwrap();
        assert_eq!(fetched, schema);
    }

    #[tokio::test]
    async fn test_delete_schema() {
        let registry = MemoryRegistry::new();
        let schema = serde_json::json!({"test": "data"});

        registry
            .publish_schema("/schemas/test", &schema)
            .await
            .unwrap();
        registry.delete_schema("/schemas/test").await.unwrap();

        let result = registry.fetch_schema("/schemas/test").await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_close_registry() {
        let registry = MemoryRegistry::new();
        registry.close().await.unwrap();

        let result = registry.health().await;
        assert!(result.is_err());
    }
}
