//! Error types for FARP operations.

use thiserror::Error;

use crate::types::SchemaType;

/// Result type alias for FARP operations
pub type Result<T> = std::result::Result<T, Error>;

/// Main error type for FARP operations
#[derive(Debug, Error)]
pub enum Error {
    /// Schema manifest not found
    #[error("schema manifest not found")]
    ManifestNotFound,

    /// Schema not found
    #[error("schema not found")]
    SchemaNotFound,

    /// Invalid manifest format
    #[error("invalid manifest format: {0}")]
    InvalidManifest(String),

    /// Invalid schema format
    #[error("invalid schema format: {0}")]
    InvalidSchema(String),

    /// Schema exceeds size limits
    #[error("schema exceeds size limit: {size} bytes (max {max_size})")]
    SchemaToLarge { size: usize, max_size: usize },

    /// Schema checksum mismatch
    #[error("schema checksum mismatch: expected {expected}, got {actual}")]
    ChecksumMismatch { expected: String, actual: String },

    /// Unsupported schema type
    #[error("unsupported schema type: {0}")]
    UnsupportedType(SchemaType),

    /// Backend unavailable
    #[error("backend unavailable: {0}")]
    BackendUnavailable(String),

    /// Incompatible protocol version
    #[error("incompatible protocol version: manifest version {manifest_version}, protocol version {protocol_version}")]
    IncompatibleVersion {
        manifest_version: String,
        protocol_version: String,
    },

    /// Invalid schema location
    #[error("invalid schema location: {0}")]
    InvalidLocation(String),

    /// Schema provider not found
    #[error("schema provider not found for type: {0}")]
    ProviderNotFound(SchemaType),

    /// Schema registry not configured
    #[error("schema registry not configured")]
    RegistryNotConfigured,

    /// Failed to fetch schema
    #[error("failed to fetch schema: {0}")]
    SchemaFetchFailed(String),

    /// Schema validation failed
    #[error("schema validation failed: {0}")]
    ValidationFailed(String),

    /// Manifest-specific error
    #[error("manifest error for service={service_name} instance={instance_id}: {source}")]
    Manifest {
        service_name: String,
        instance_id: String,
        #[source]
        source: Box<Error>,
    },

    /// Schema-specific error
    #[error("schema error type={schema_type} path={path}: {source}")]
    Schema {
        schema_type: SchemaType,
        path: String,
        #[source]
        source: Box<Error>,
    },

    /// Validation error
    #[error("validation error: field={field} message={message}")]
    Validation { field: String, message: String },

    /// Serialization error
    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    /// I/O error
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),

    /// Channel send error
    #[error("channel send error")]
    ChannelSend,

    /// Channel receive error
    #[error("channel receive error")]
    ChannelReceive,

    /// Custom error for extensibility
    #[error("custom error: {0}")]
    Custom(String),
}

impl Error {
    /// Creates a new manifest error
    pub fn manifest(service_name: String, instance_id: String, source: Error) -> Self {
        Error::Manifest {
            service_name,
            instance_id,
            source: Box::new(source),
        }
    }

    /// Creates a new schema error
    pub fn schema(schema_type: SchemaType, path: String, source: Error) -> Self {
        Error::Schema {
            schema_type,
            path,
            source: Box::new(source),
        }
    }

    /// Creates a new validation error
    pub fn validation(field: impl Into<String>, message: impl Into<String>) -> Self {
        Error::Validation {
            field: field.into(),
            message: message.into(),
        }
    }

    /// Creates a new invalid manifest error
    pub fn invalid_manifest(message: impl Into<String>) -> Self {
        Error::InvalidManifest(message.into())
    }

    /// Creates a new invalid schema error
    pub fn invalid_schema(message: impl Into<String>) -> Self {
        Error::InvalidSchema(message.into())
    }

    /// Creates a new schema too large error
    pub fn schema_too_large(size: usize, max_size: usize) -> Self {
        Error::SchemaToLarge { size, max_size }
    }

    /// Creates a new checksum mismatch error
    pub fn checksum_mismatch(expected: String, actual: String) -> Self {
        Error::ChecksumMismatch { expected, actual }
    }

    /// Creates a new incompatible version error
    pub fn incompatible_version(manifest_version: String, protocol_version: String) -> Self {
        Error::IncompatibleVersion {
            manifest_version,
            protocol_version,
        }
    }

    /// Creates a new invalid location error
    pub fn invalid_location(message: impl Into<String>) -> Self {
        Error::InvalidLocation(message.into())
    }

    /// Creates a new backend unavailable error
    pub fn backend_unavailable(message: impl Into<String>) -> Self {
        Error::BackendUnavailable(message.into())
    }

    /// Creates a new schema fetch failed error
    pub fn schema_fetch_failed(message: impl Into<String>) -> Self {
        Error::SchemaFetchFailed(message.into())
    }

    /// Creates a new validation failed error
    pub fn validation_failed(message: impl Into<String>) -> Self {
        Error::ValidationFailed(message.into())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_error_display() {
        let err = Error::ManifestNotFound;
        assert_eq!(err.to_string(), "schema manifest not found");

        let err = Error::SchemaNotFound;
        assert_eq!(err.to_string(), "schema not found");

        let err = Error::invalid_manifest("test error");
        assert_eq!(err.to_string(), "invalid manifest format: test error");
    }

    #[test]
    fn test_validation_error() {
        let err = Error::validation("field_name", "field is required");
        assert!(err.to_string().contains("field_name"));
        assert!(err.to_string().contains("field is required"));
    }

    #[test]
    fn test_checksum_mismatch() {
        let err = Error::checksum_mismatch("abc123".to_string(), "def456".to_string());
        assert!(err.to_string().contains("abc123"));
        assert!(err.to_string().contains("def456"));
    }
}
