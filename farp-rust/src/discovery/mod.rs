//! Pluggable service discovery for FARP.
//!
//! Provides three discovery modes:
//! - **Registry-based (pull)**: Services register in Consul/etcd/K8s/Redis/mDNS,
//!   gateways watch for changes.
//! - **Push-based (reverse)**: Services push manifests directly to the gateway.
//!   No external registry needed.
//! - **Hybrid**: Both modes combined.
//!
//! # Service Side
//!
//! ```ignore
//! use farp::discovery::ServiceNodeConfig;
//!
//! let config = ServiceNodeConfig::new("user-service", "v1.0.0", "10.0.0.5:8080");
//! ```
//!
//! # Gateway Side
//!
//! ```ignore
//! use farp::discovery::GatewayNodeConfig;
//!
//! let config = GatewayNodeConfig::new();
//! ```

pub mod push;

use crate::errors::Result;
use crate::types::{InstanceStatus, SchemaManifest};
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Represents a discovered service instance in infrastructure.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServiceInstance {
    /// Unique ID for this instance
    pub id: String,
    /// Service name
    pub service_name: String,
    /// Network address (host or host:port)
    pub address: String,
    /// Port number
    pub port: u16,
    /// Health status
    pub status: InstanceStatus,
    /// Metadata tags
    #[serde(default)]
    pub metadata: HashMap<String, String>,
    /// When this instance was registered (Unix timestamp)
    pub registered_at: i64,
    /// When health was last reported (Unix timestamp)
    pub last_health_check: i64,
}

/// Represents a change in service discovery.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DiscoveryEvent {
    /// Event type (added, updated, removed)
    pub event_type: crate::registry::EventType,
    /// The instance that changed
    pub instance: ServiceInstance,
    /// Timestamp of the event (Unix timestamp)
    pub timestamp: i64,
    /// Manifest (populated in push mode, may be None for pull mode)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub manifest: Option<SchemaManifest>,
}

/// Handler for discovery events.
pub trait DiscoveryEventHandler: Send + Sync {
    fn on_event(&self, event: DiscoveryEvent);
}

/// Blanket implementation for closures.
impl<F> DiscoveryEventHandler for F
where
    F: Fn(DiscoveryEvent) + Send + Sync,
{
    fn on_event(&self, event: DiscoveryEvent) {
        self(event)
    }
}

/// Pluggable service discovery interface.
///
/// Implementations wrap infrastructure-specific discovery mechanisms
/// (Consul, etcd, Kubernetes, Redis, mDNS, or push-based).
#[async_trait]
pub trait ServiceDiscovery: Send + Sync {
    /// Returns all known instances of a service.
    async fn discover(&self, service_name: &str) -> Result<Vec<ServiceInstance>>;

    /// Watches for changes to instances of a service.
    async fn watch(
        &self,
        service_name: &str,
        handler: Box<dyn DiscoveryEventHandler>,
    ) -> Result<()>;

    /// Registers a service instance.
    async fn register(&self, instance: &ServiceInstance) -> Result<()>;

    /// Deregisters a service instance.
    async fn deregister(&self, instance_id: &str) -> Result<()>;

    /// Reports health status for a registered instance.
    async fn report_health(&self, instance_id: &str, status: InstanceStatus) -> Result<()>;

    /// Closes the discovery backend connection.
    async fn close(&self) -> Result<()>;

    /// Checks if the discovery backend is reachable.
    async fn health(&self) -> Result<()>;
}

/// Fetches a FARP manifest from a live service instance.
#[async_trait]
pub trait ManifestFetcher: Send + Sync {
    /// Fetches the SchemaManifest from a service instance.
    async fn fetch_manifest(&self, instance: &ServiceInstance) -> Result<SchemaManifest>;
}

/// Configuration for a ServiceNode.
pub struct ServiceNodeConfig {
    pub service_name: String,
    pub service_version: String,
    pub instance_id: Option<String>,
    pub address: String,
    pub gateway_url: Option<String>,
    pub health_interval_secs: u64,
    pub ttl_secs: u64,
}

impl ServiceNodeConfig {
    pub fn new(
        service_name: impl Into<String>,
        service_version: impl Into<String>,
        address: impl Into<String>,
    ) -> Self {
        Self {
            service_name: service_name.into(),
            service_version: service_version.into(),
            instance_id: None,
            address: address.into(),
            gateway_url: None,
            health_interval_secs: 10,
            ttl_secs: 30,
        }
    }
}

/// Configuration for a GatewayNode.
pub struct GatewayNodeConfig {
    pub enable_push: bool,
    pub service_names: Vec<String>,
    pub health_poll_interval_secs: u64,
    pub heartbeat_timeout_secs: u64,
}

impl GatewayNodeConfig {
    pub fn new() -> Self {
        Self {
            enable_push: false,
            service_names: Vec::new(),
            health_poll_interval_secs: 15,
            heartbeat_timeout_secs: 30,
        }
    }
}

impl Default for GatewayNodeConfig {
    fn default() -> Self {
        Self::new()
    }
}
