//! Storage backend abstraction and utilities.

use crate::errors::{Error, Result};
use crate::registry::EventType;
use crate::types::SchemaManifest;
use async_trait::async_trait;
use flate2::read::GzDecoder;
use flate2::write::GzEncoder;
use flate2::Compression;
use std::io::{Read, Write};

/// Storage backend trait for low-level key-value operations
///
/// This abstracts the underlying storage mechanism (Consul KV, etcd, Redis, etc.)
#[async_trait]
pub trait StorageBackend: Send + Sync {
    /// Stores a value at the given key
    async fn put(&self, key: &str, value: &[u8]) -> Result<()>;

    /// Retrieves a value by key
    ///
    /// Returns `Error::SchemaNotFound` if key doesn't exist
    async fn get(&self, key: &str) -> Result<Vec<u8>>;

    /// Deletes a key
    async fn delete(&self, key: &str) -> Result<()>;

    /// Lists all keys with the given prefix
    async fn list(&self, prefix: &str) -> Result<Vec<String>>;

    /// Watches for changes to keys with the given prefix
    ///
    /// Returns a channel that receives change events
    async fn watch(&self, prefix: &str) -> Result<tokio::sync::mpsc::Receiver<StorageEvent>>;

    /// Closes the backend connection
    async fn close(&self) -> Result<()>;
}

/// Storage change event
#[derive(Debug, Clone)]
pub struct StorageEvent {
    /// Type of event
    pub event_type: EventType,
    /// Key that changed
    pub key: String,
    /// Value (None for delete events)
    pub value: Option<Vec<u8>>,
}

/// Storage helper for JSON serialization and compression
pub struct StorageHelper {
    compression_threshold: i64,
    max_size: i64,
}

impl StorageHelper {
    /// Creates a new storage helper
    pub fn new(compression_threshold: i64, max_size: i64) -> Self {
        Self {
            compression_threshold,
            max_size,
        }
    }

    /// Stores a JSON-serializable value
    pub async fn put_json<B: StorageBackend>(
        &self,
        backend: &B,
        key: &str,
        value: &impl serde::Serialize,
    ) -> Result<()> {
        // Serialize to JSON
        let data = serde_json::to_vec(value)?;

        // Check size limit
        if self.max_size > 0 && data.len() as i64 > self.max_size {
            return Err(Error::schema_too_large(data.len(), self.max_size as usize));
        }

        // Compress if above threshold
        let (final_data, final_key) =
            if self.compression_threshold > 0 && data.len() as i64 > self.compression_threshold {
                let compressed = compress_data(&data)?;
                (compressed, format!("{key}.gz"))
            } else {
                (data, key.to_string())
            };

        backend.put(&final_key, &final_data).await
    }

    /// Retrieves and deserializes a JSON value
    pub async fn get_json<B: StorageBackend, T: serde::de::DeserializeOwned>(
        &self,
        backend: &B,
        key: &str,
    ) -> Result<T> {
        // Try compressed version first
        let compressed_key = format!("{key}.gz");
        let data = match backend.get(&compressed_key).await {
            Ok(compressed) => {
                // Decompress
                decompress_data(&compressed)?
            }
            Err(_) => {
                // Try uncompressed version
                backend.get(key).await?
            }
        };

        // Deserialize JSON
        serde_json::from_slice(&data).map_err(|e| Error::invalid_schema(e.to_string()))
    }
}

/// Compresses data using gzip
fn compress_data(data: &[u8]) -> Result<Vec<u8>> {
    let mut encoder = GzEncoder::new(Vec::new(), Compression::default());
    encoder.write_all(data)?;
    Ok(encoder.finish()?)
}

/// Decompresses gzip data
fn decompress_data(data: &[u8]) -> Result<Vec<u8>> {
    let mut decoder = GzDecoder::new(data);
    let mut decompressed = Vec::new();
    decoder.read_to_end(&mut decompressed)?;
    Ok(decompressed)
}

/// High-level manifest storage operations
pub struct ManifestStorage<B: StorageBackend> {
    backend: B,
    helper: StorageHelper,
    namespace: String,
}

impl<B: StorageBackend> ManifestStorage<B> {
    /// Creates a new manifest storage
    pub fn new(
        backend: B,
        namespace: impl Into<String>,
        compression_threshold: i64,
        max_size: i64,
    ) -> Self {
        Self {
            backend,
            helper: StorageHelper::new(compression_threshold, max_size),
            namespace: namespace.into(),
        }
    }

    /// Generates a storage key for a manifest
    fn manifest_key(&self, service_name: &str, instance_id: &str) -> String {
        format!(
            "{}/services/{}/instances/{}/manifest",
            self.namespace, service_name, instance_id
        )
    }

    /// Generates a storage key for a schema
    fn schema_key(&self, path: &str) -> String {
        format!("{}{}", self.namespace, path)
    }

    /// Stores a manifest
    pub async fn put(&self, manifest: &SchemaManifest) -> Result<()> {
        let key = self.manifest_key(&manifest.service_name, &manifest.instance_id);
        self.helper.put_json(&self.backend, &key, manifest).await
    }

    /// Retrieves a manifest
    pub async fn get(&self, service_name: &str, instance_id: &str) -> Result<SchemaManifest> {
        let key = self.manifest_key(service_name, instance_id);
        self.helper
            .get_json(&self.backend, &key)
            .await
            .map_err(|e| match e {
                Error::SchemaNotFound => Error::ManifestNotFound,
                _ => e,
            })
    }

    /// Deletes a manifest
    pub async fn delete(&self, service_name: &str, instance_id: &str) -> Result<()> {
        let key = self.manifest_key(service_name, instance_id);
        self.backend.delete(&key).await
    }

    /// Lists all manifests for a service
    pub async fn list(&self, service_name: &str) -> Result<Vec<SchemaManifest>> {
        let prefix = format!("{}/services/{}/instances/", self.namespace, service_name);
        let keys = self.backend.list(&prefix).await?;

        let mut manifests = Vec::new();
        for key in keys {
            match self
                .helper
                .get_json::<_, SchemaManifest>(&self.backend, &key)
                .await
            {
                Ok(manifest) => manifests.push(manifest),
                Err(_) => {
                    // Skip invalid manifests
                    continue;
                }
            }
        }

        Ok(manifests)
    }

    /// Stores a schema
    pub async fn put_schema(&self, path: &str, schema: &serde_json::Value) -> Result<()> {
        let key = self.schema_key(path);
        self.helper.put_json(&self.backend, &key, schema).await
    }

    /// Retrieves a schema
    pub async fn get_schema(&self, path: &str) -> Result<serde_json::Value> {
        let key = self.schema_key(path);
        self.helper.get_json(&self.backend, &key).await
    }

    /// Deletes a schema
    pub async fn delete_schema(&self, path: &str) -> Result<()> {
        let key = self.schema_key(path);
        self.backend.delete(&key).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_compress_decompress() {
        // Use a longer, more repetitive string for better compression
        let data = b"Hello, World! This is a test string for compression. ".repeat(100);
        let data_slice = data.as_slice();

        let compressed = compress_data(data_slice).unwrap();
        assert!(compressed.len() < data_slice.len());

        let decompressed = decompress_data(&compressed).unwrap();
        assert_eq!(&decompressed[..], data_slice);
    }

    #[test]
    fn test_storage_helper() {
        let helper = StorageHelper::new(100, 1024 * 1024);
        assert_eq!(helper.compression_threshold, 100);
        assert_eq!(helper.max_size, 1024 * 1024);
    }
}
