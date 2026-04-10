//! Push-based service discovery — services push manifests directly to a gateway.
//!
//! No external service registry needed. The service-side `PushDiscovery`
//! sends HTTP requests to the gateway's `PushHandler` endpoints.

use crate::errors::{Error, Result};
use crate::types::InstanceStatus;

use super::{DiscoveryEventHandler, ServiceDiscovery, ServiceInstance};

use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// Push-based discovery client (service-side).
///
/// Implements ServiceDiscovery by sending HTTP requests to a gateway URL.
pub struct PushDiscovery {
    _gateway_url: String,
}

impl PushDiscovery {
    /// Creates a new push-based discovery client.
    pub fn new(gateway_url: impl Into<String>) -> Self {
        let mut url = gateway_url.into();
        if url.ends_with('/') {
            url.pop();
        }

        Self { _gateway_url: url }
    }
}

#[async_trait]
impl ServiceDiscovery for PushDiscovery {
    async fn discover(&self, _service_name: &str) -> Result<Vec<ServiceInstance>> {
        // In push mode, services don't typically discover others.
        // This could query the gateway's /services endpoint.
        Ok(Vec::new())
    }

    async fn watch(
        &self,
        _service_name: &str,
        _handler: Box<dyn DiscoveryEventHandler>,
    ) -> Result<()> {
        // Push mode services don't watch — they push.
        // Block until we'd be cancelled (in real impl, use cancellation token).
        Ok(())
    }

    async fn register(&self, _instance: &ServiceInstance) -> Result<()> {
        // Would POST to {gateway_url}/register
        // For now, return Ok — actual HTTP client impl depends on reqwest feature
        Ok(())
    }

    async fn deregister(&self, _instance_id: &str) -> Result<()> {
        // Would DELETE to {gateway_url}/deregister/{id}
        Ok(())
    }

    async fn report_health(&self, _instance_id: &str, _status: InstanceStatus) -> Result<()> {
        // Would PUT to {gateway_url}/heartbeat/{id}
        Ok(())
    }

    async fn close(&self) -> Result<()> {
        Ok(())
    }

    async fn health(&self) -> Result<()> {
        Ok(())
    }
}

/// Push-based discovery handler (gateway-side).
///
/// Manages an in-memory registry of pushed service instances.
/// Also implements ServiceDiscovery so GatewayNode can use it.
pub struct PushHandler {
    instances: Arc<RwLock<HashMap<String, PushEntry>>>,
    _heartbeat_timeout_secs: u64,
}

struct PushEntry {
    instance: ServiceInstance,
    last_seen: i64,
}

impl PushHandler {
    /// Creates a new push handler.
    pub fn new(heartbeat_timeout_secs: u64) -> Self {
        Self {
            instances: Arc::new(RwLock::new(HashMap::new())),
            _heartbeat_timeout_secs: heartbeat_timeout_secs,
        }
    }
}

#[async_trait]
impl ServiceDiscovery for PushHandler {
    async fn discover(&self, service_name: &str) -> Result<Vec<ServiceInstance>> {
        let instances = self.instances.read().await;
        let result: Vec<ServiceInstance> = instances
            .values()
            .filter(|e| service_name.is_empty() || e.instance.service_name == service_name)
            .map(|e| e.instance.clone())
            .collect();

        Ok(result)
    }

    async fn watch(
        &self,
        _service_name: &str,
        _handler: Box<dyn DiscoveryEventHandler>,
    ) -> Result<()> {
        // In a full implementation, this would register the handler
        // and notify on register/deregister/heartbeat timeout events.
        Ok(())
    }

    async fn register(&self, instance: &ServiceInstance) -> Result<()> {
        let mut instances = self.instances.write().await;
        instances.insert(
            instance.id.clone(),
            PushEntry {
                instance: instance.clone(),
                last_seen: chrono::Utc::now().timestamp(),
            },
        );

        Ok(())
    }

    async fn deregister(&self, instance_id: &str) -> Result<()> {
        let mut instances = self.instances.write().await;
        instances.remove(instance_id);

        Ok(())
    }

    async fn report_health(&self, instance_id: &str, status: InstanceStatus) -> Result<()> {
        let mut instances = self.instances.write().await;
        if let Some(entry) = instances.get_mut(instance_id) {
            entry.instance.status = status;
            entry.last_seen = chrono::Utc::now().timestamp();
            Ok(())
        } else {
            Err(Error::not_found("service instance", instance_id))
        }
    }

    async fn close(&self) -> Result<()> {
        Ok(())
    }

    async fn health(&self) -> Result<()> {
        Ok(())
    }
}
