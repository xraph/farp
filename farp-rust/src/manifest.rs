//! Manifest operations including creation, validation, checksums, and diffing.

use crate::errors::{Error, Result};
use crate::types::*;
use crate::version::{is_compatible, PROTOCOL_VERSION};
use sha2::{Digest, Sha256};
use std::collections::{HashMap, HashSet};

/// Creates a new schema manifest with default values
///
/// # Arguments
///
/// * `service_name` - The logical service name
/// * `service_version` - The service version (semver recommended)
/// * `instance_id` - Unique instance identifier
///
/// # Examples
///
/// ```
/// use farp::manifest::new_manifest;
///
/// let manifest = new_manifest("user-service", "v1.2.3", "instance-123");
/// assert_eq!(manifest.service_name, "user-service");
/// ```
pub fn new_manifest(
    service_name: impl Into<String>,
    service_version: impl Into<String>,
    instance_id: impl Into<String>,
) -> SchemaManifest {
    SchemaManifest {
        version: PROTOCOL_VERSION.to_string(),
        service_name: service_name.into(),
        service_version: service_version.into(),
        instance_id: instance_id.into(),
        instance: None,
        schemas: Vec::new(),
        capabilities: Vec::new(),
        endpoints: SchemaEndpoints::default(),
        routing: RoutingConfig::default(),
        auth: None,
        webhook: None,
        hints: None,
        updated_at: chrono::Utc::now().timestamp(),
        checksum: String::new(),
    }
}

impl SchemaManifest {
    /// Adds a schema descriptor to the manifest
    pub fn add_schema(&mut self, descriptor: SchemaDescriptor) {
        self.schemas.push(descriptor);
    }

    /// Adds a capability to the manifest
    pub fn add_capability(&mut self, capability: impl Into<String>) {
        let cap = capability.into();
        if !self.capabilities.contains(&cap) {
            self.capabilities.push(cap);
        }
    }

    /// Updates the checksum based on all schema hashes
    pub fn update_checksum(&mut self) -> Result<()> {
        let checksum = calculate_manifest_checksum(self)?;
        self.checksum = checksum;
        self.updated_at = chrono::Utc::now().timestamp();
        Ok(())
    }

    /// Validates the manifest for correctness
    pub fn validate(&self) -> Result<()> {
        // Check protocol version compatibility
        if !is_compatible(&self.version) {
            return Err(Error::incompatible_version(
                self.version.clone(),
                PROTOCOL_VERSION.to_string(),
            ));
        }

        // Check required fields
        if self.service_name.is_empty() {
            return Err(Error::validation(
                "service_name",
                "service name is required",
            ));
        }

        if self.instance_id.is_empty() {
            return Err(Error::validation("instance_id", "instance ID is required"));
        }

        // Validate health endpoint
        if self.endpoints.health.is_empty() {
            return Err(Error::validation(
                "endpoints.health",
                "health endpoint is required",
            ));
        }

        // Validate each schema descriptor
        for (i, schema) in self.schemas.iter().enumerate() {
            validate_schema_descriptor(schema).map_err(|e| {
                Error::invalid_manifest(format!("invalid schema at index {i}: {e}"))
            })?;
        }

        // Verify checksum if present
        if !self.checksum.is_empty() {
            let expected = calculate_manifest_checksum(self)?;
            if self.checksum != expected {
                return Err(Error::checksum_mismatch(expected, self.checksum.clone()));
            }
        }

        Ok(())
    }

    /// Retrieves a schema descriptor by type
    pub fn get_schema(&self, schema_type: SchemaType) -> Option<&SchemaDescriptor> {
        self.schemas.iter().find(|s| s.schema_type == schema_type)
    }

    /// Checks if the manifest includes a specific capability
    pub fn has_capability(&self, capability: &str) -> bool {
        self.capabilities.iter().any(|c| c == capability)
    }

    /// Serializes the manifest to JSON
    pub fn to_json(&self) -> Result<Vec<u8>> {
        serde_json::to_vec(self).map_err(Error::from)
    }

    /// Serializes the manifest to pretty-printed JSON
    pub fn to_pretty_json(&self) -> Result<Vec<u8>> {
        serde_json::to_vec_pretty(self).map_err(Error::from)
    }

    /// Deserializes a manifest from JSON
    pub fn from_json(data: &[u8]) -> Result<Self> {
        serde_json::from_slice(data).map_err(|e| Error::invalid_manifest(e.to_string()))
    }
}

/// Validates a schema descriptor
pub fn validate_schema_descriptor(sd: &SchemaDescriptor) -> Result<()> {
    // Check schema type
    if !sd.schema_type.is_valid() {
        return Err(Error::UnsupportedType(sd.schema_type));
    }

    // Check spec version
    if sd.spec_version.is_empty() {
        return Err(Error::validation(
            "spec_version",
            "spec version is required",
        ));
    }

    // Validate location
    validate_schema_location(&sd.location)?;

    // For inline schemas, inline_schema must be present
    if sd.location.location_type == LocationType::Inline && sd.inline_schema.is_none() {
        return Err(Error::validation(
            "inline_schema",
            "inline schema is required for inline location type",
        ));
    }

    // Check hash
    if sd.hash.is_empty() {
        return Err(Error::validation("hash", "schema hash is required"));
    }

    // Validate hash format (should be 64 hex characters for SHA256)
    if sd.hash.len() != 64 {
        return Err(Error::validation(
            "hash",
            "invalid hash format (expected 64 hex characters)",
        ));
    }

    // Check content type
    if sd.content_type.is_empty() {
        return Err(Error::validation(
            "content_type",
            "content type is required",
        ));
    }

    Ok(())
}

/// Validates a schema location
fn validate_schema_location(sl: &SchemaLocation) -> Result<()> {
    if !sl.location_type.is_valid() {
        return Err(Error::invalid_location(format!(
            "invalid location type: {}",
            sl.location_type
        )));
    }

    match sl.location_type {
        LocationType::HTTP => {
            if sl.url.is_none() || sl.url.as_ref().unwrap().is_empty() {
                return Err(Error::invalid_location("URL required for HTTP location"));
            }
        }
        LocationType::Registry => {
            if sl.registry_path.is_none() || sl.registry_path.as_ref().unwrap().is_empty() {
                return Err(Error::invalid_location(
                    "registry path required for registry location",
                ));
            }
        }
        LocationType::Inline => {
            // No additional validation needed for inline
        }
    }

    Ok(())
}

/// Calculates the SHA256 checksum of a manifest by combining all schema hashes
pub fn calculate_manifest_checksum(manifest: &SchemaManifest) -> Result<String> {
    if manifest.schemas.is_empty() {
        return Ok(String::new());
    }

    // Sort schemas by type for deterministic hashing
    let mut sorted_schemas = manifest.schemas.clone();
    sorted_schemas.sort_by(|a, b| a.schema_type.as_str().cmp(b.schema_type.as_str()));

    // Concatenate all schema hashes
    let combined: String = sorted_schemas.iter().map(|s| s.hash.as_str()).collect();

    // Calculate SHA256 of combined hashes
    let mut hasher = Sha256::new();
    hasher.update(combined.as_bytes());
    let result = hasher.finalize();
    Ok(hex::encode(result))
}

/// Calculates the SHA256 checksum of a schema
pub fn calculate_schema_checksum(schema: &serde_json::Value) -> Result<String> {
    // Serialize to canonical JSON (map keys are sorted by serde_json)
    let data = serde_json::to_vec(schema)?;

    // Calculate SHA256
    let mut hasher = Sha256::new();
    hasher.update(&data);
    let result = hasher.finalize();
    Ok(hex::encode(result))
}

/// Represents the difference between two manifests
#[derive(Debug, Clone, PartialEq)]
pub struct ManifestDiff {
    /// Schemas present in new but not in old
    pub schemas_added: Vec<SchemaDescriptor>,
    /// Schemas present in old but not in new
    pub schemas_removed: Vec<SchemaDescriptor>,
    /// Schemas present in both but with different hashes
    pub schemas_changed: Vec<SchemaChangeDiff>,
    /// New capabilities
    pub capabilities_added: Vec<String>,
    /// Removed capabilities
    pub capabilities_removed: Vec<String>,
    /// Whether endpoints changed
    pub endpoints_changed: bool,
}

/// Represents a changed schema
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SchemaChangeDiff {
    pub schema_type: SchemaType,
    pub old_hash: String,
    pub new_hash: String,
}

impl ManifestDiff {
    /// Returns true if there are any changes
    pub fn has_changes(&self) -> bool {
        !self.schemas_added.is_empty()
            || !self.schemas_removed.is_empty()
            || !self.schemas_changed.is_empty()
            || !self.capabilities_added.is_empty()
            || !self.capabilities_removed.is_empty()
            || self.endpoints_changed
    }
}

/// Compares two manifests and returns the differences
pub fn diff_manifests(old: &SchemaManifest, new: &SchemaManifest) -> ManifestDiff {
    let mut diff = ManifestDiff {
        schemas_added: Vec::new(),
        schemas_removed: Vec::new(),
        schemas_changed: Vec::new(),
        capabilities_added: Vec::new(),
        capabilities_removed: Vec::new(),
        endpoints_changed: false,
    };

    // Build maps for easier comparison
    let old_schemas: HashMap<SchemaType, &SchemaDescriptor> =
        old.schemas.iter().map(|s| (s.schema_type, s)).collect();
    let new_schemas: HashMap<SchemaType, &SchemaDescriptor> =
        new.schemas.iter().map(|s| (s.schema_type, s)).collect();

    // Find added and changed schemas
    for (schema_type, new_schema) in &new_schemas {
        if let Some(old_schema) = old_schemas.get(schema_type) {
            // Schema exists in both, check if changed
            if old_schema.hash != new_schema.hash {
                diff.schemas_changed.push(SchemaChangeDiff {
                    schema_type: *schema_type,
                    old_hash: old_schema.hash.clone(),
                    new_hash: new_schema.hash.clone(),
                });
            }
        } else {
            // Schema is new
            diff.schemas_added.push((*new_schema).clone());
        }
    }

    // Find removed schemas
    for (schema_type, old_schema) in &old_schemas {
        if !new_schemas.contains_key(schema_type) {
            diff.schemas_removed.push((*old_schema).clone());
        }
    }

    // Compare capabilities
    let old_caps: HashSet<&String> = old.capabilities.iter().collect();
    let new_caps: HashSet<&String> = new.capabilities.iter().collect();

    for cap in &new_caps {
        if !old_caps.contains(cap) {
            diff.capabilities_added.push((*cap).clone());
        }
    }

    for cap in &old_caps {
        if !new_caps.contains(cap) {
            diff.capabilities_removed.push((*cap).clone());
        }
    }

    // Compare endpoints (simple comparison)
    if old.endpoints != new.endpoints {
        diff.endpoints_changed = true;
    }

    diff
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_new_manifest() {
        let manifest = new_manifest("test-service", "v1.0.0", "instance-123");
        assert_eq!(manifest.service_name, "test-service");
        assert_eq!(manifest.service_version, "v1.0.0");
        assert_eq!(manifest.instance_id, "instance-123");
        assert_eq!(manifest.version, PROTOCOL_VERSION);
    }

    #[test]
    fn test_add_schema() {
        let mut manifest = new_manifest("test", "v1", "id1");
        let schema = SchemaDescriptor {
            schema_type: SchemaType::OpenAPI,
            spec_version: "3.1.0".to_string(),
            location: SchemaLocation {
                location_type: LocationType::HTTP,
                url: Some("http://example.com/openapi.json".to_string()),
                registry_path: None,
                headers: None,
            },
            content_type: "application/json".to_string(),
            inline_schema: None,
            hash: "a".repeat(64),
            size: 1024,
            compatibility: None,
            metadata: None,
        };

        manifest.add_schema(schema);
        assert_eq!(manifest.schemas.len(), 1);
    }

    #[test]
    fn test_add_capability() {
        let mut manifest = new_manifest("test", "v1", "id1");
        manifest.add_capability("rest");
        manifest.add_capability("grpc");
        manifest.add_capability("rest"); // Duplicate should be ignored

        assert_eq!(manifest.capabilities.len(), 2);
        assert!(manifest.has_capability("rest"));
        assert!(manifest.has_capability("grpc"));
        assert!(!manifest.has_capability("websocket"));
    }

    #[test]
    fn test_validate_schema_descriptor() {
        let valid = SchemaDescriptor {
            schema_type: SchemaType::OpenAPI,
            spec_version: "3.1.0".to_string(),
            location: SchemaLocation {
                location_type: LocationType::HTTP,
                url: Some("http://example.com/openapi.json".to_string()),
                registry_path: None,
                headers: None,
            },
            content_type: "application/json".to_string(),
            inline_schema: None,
            hash: "a".repeat(64),
            size: 1024,
            compatibility: None,
            metadata: None,
        };

        assert!(validate_schema_descriptor(&valid).is_ok());
    }

    #[test]
    fn test_validate_schema_descriptor_invalid_hash() {
        let invalid = SchemaDescriptor {
            schema_type: SchemaType::OpenAPI,
            spec_version: "3.1.0".to_string(),
            location: SchemaLocation {
                location_type: LocationType::HTTP,
                url: Some("http://example.com/openapi.json".to_string()),
                registry_path: None,
                headers: None,
            },
            content_type: "application/json".to_string(),
            inline_schema: None,
            hash: "invalid".to_string(), // Too short
            size: 1024,
            compatibility: None,
            metadata: None,
        };

        assert!(validate_schema_descriptor(&invalid).is_err());
    }

    #[test]
    fn test_calculate_manifest_checksum() {
        let mut manifest = new_manifest("test", "v1", "id1");
        manifest.add_schema(SchemaDescriptor {
            schema_type: SchemaType::OpenAPI,
            spec_version: "3.1.0".to_string(),
            location: SchemaLocation {
                location_type: LocationType::HTTP,
                url: Some("http://example.com".to_string()),
                registry_path: None,
                headers: None,
            },
            content_type: "application/json".to_string(),
            inline_schema: None,
            hash: "a".repeat(64),
            size: 1024,
            compatibility: None,
            metadata: None,
        });

        let checksum = calculate_manifest_checksum(&manifest).unwrap();
        assert_eq!(checksum.len(), 64); // SHA256 produces 64 hex characters
    }

    #[test]
    fn test_diff_manifests() {
        let mut old = new_manifest("test", "v1", "id1");
        old.add_capability("rest");

        let mut new = new_manifest("test", "v1", "id1");
        new.add_capability("grpc");

        let diff = diff_manifests(&old, &new);

        assert_eq!(diff.capabilities_added.len(), 1);
        assert_eq!(diff.capabilities_removed.len(), 1);
        assert!(diff.has_changes());
    }

    #[test]
    fn test_manifest_serialization() {
        let manifest = new_manifest("test-service", "v1.0.0", "instance-123");

        let json = manifest.to_json().unwrap();
        let deserialized = SchemaManifest::from_json(&json).unwrap();

        assert_eq!(deserialized.service_name, "test-service");
        assert_eq!(deserialized.instance_id, "instance-123");
    }
}
