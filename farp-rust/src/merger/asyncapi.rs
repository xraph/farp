//! AsyncAPI schema merger for combining multiple service schemas

use super::types::*;
use super::*;
use crate::errors::Result;
use crate::types::{SchemaManifest, SchemaType};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// AsyncAPI 2.x/3.x specification
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AsyncAPISpec {
    pub asyncapi: String,
    pub info: Info,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub servers: HashMap<String, AsyncServer>,
    pub channels: HashMap<String, Channel>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub components: Option<AsyncComponents>,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    pub security: Vec<HashMap<String, Vec<String>>>,
    #[serde(flatten)]
    pub extensions: HashMap<String, serde_json::Value>,
}

/// AsyncAPI server (broker connection)
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AsyncServer {
    pub url: String,
    pub protocol: String, // kafka, amqp, mqtt, ws, etc.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub variables: Option<HashMap<String, serde_json::Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bindings: Option<HashMap<String, serde_json::Value>>,
}

/// AsyncAPI channel
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Channel {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub subscribe: Option<Operation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub publish: Option<Operation>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parameters: Option<HashMap<String, serde_json::Value>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bindings: Option<HashMap<String, serde_json::Value>>,
    #[serde(flatten)]
    pub extensions: HashMap<String, serde_json::Value>,
}

/// AsyncAPI components
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AsyncComponents {
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub messages: HashMap<String, serde_json::Value>,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub schemas: HashMap<String, serde_json::Value>,
    #[serde(
        skip_serializing_if = "HashMap::is_empty",
        default,
        rename = "securitySchemes"
    )]
    pub security_schemes: HashMap<String, AsyncSecurityScheme>,
}

/// AsyncAPI security scheme
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AsyncSecurityScheme {
    #[serde(rename = "type")]
    pub scheme_type: String, // userPassword, apiKey, X509, etc.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "in")]
    pub in_: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub scheme: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "bearerFormat")]
    pub bearer_format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "openIdConnectUrl")]
    pub openid_connect_url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub flows: Option<HashMap<String, serde_json::Value>>,
}

/// Service schema with AsyncAPI context
#[derive(Debug, Clone)]
pub struct AsyncAPIServiceSchema {
    pub manifest: SchemaManifest,
    pub schema: serde_json::Value,
    pub parsed: Option<AsyncAPISpec>,
}

/// AsyncAPI merger
pub struct AsyncAPIMerger {
    config: MergerConfig,
}

/// AsyncAPI merge result
#[derive(Debug, Clone)]
pub struct AsyncAPIMergeResult {
    pub spec: AsyncAPISpec,
    pub included_services: Vec<String>,
    pub excluded_services: Vec<String>,
    pub conflicts: Vec<Conflict>,
    pub warnings: Vec<String>,
}

impl AsyncAPIMerger {
    pub fn new(config: MergerConfig) -> Self {
        Self { config }
    }

    pub fn merge(&self, schemas: Vec<AsyncAPIServiceSchema>) -> Result<AsyncAPIMergeResult> {
        let mut result = AsyncAPIMergeResult {
            spec: AsyncAPISpec {
                asyncapi: "2.6.0".to_string(),
                info: Info {
                    title: self.config.merged_title.clone(),
                    description: Some(self.config.merged_description.clone()),
                    version: self.config.merged_version.clone(),
                    terms_of_service: None,
                    contact: None,
                    license: None,
                    extensions: HashMap::new(),
                },
                servers: HashMap::new(),
                channels: HashMap::new(),
                components: Some(AsyncComponents {
                    messages: HashMap::new(),
                    schemas: HashMap::new(),
                    security_schemes: HashMap::new(),
                }),
                security: Vec::new(),
                extensions: HashMap::new(),
            },
            included_services: Vec::new(),
            excluded_services: Vec::new(),
            conflicts: Vec::new(),
            warnings: Vec::new(),
        };

        let mut seen_channels: HashMap<String, String> = HashMap::new();
        let mut seen_messages: HashMap<String, String> = HashMap::new();
        let mut seen_servers: HashMap<String, String> = HashMap::new();
        let mut seen_security_schemes: HashMap<String, String> = HashMap::new();

        for mut schema in schemas {
            let service_name = schema.manifest.service_name.clone();

            if !should_include_asyncapi(&schema) {
                result.excluded_services.push(service_name);
                continue;
            }

            result.included_services.push(service_name.clone());

            if schema.parsed.is_none() {
                match parse_asyncapi_schema(&schema.schema) {
                    Ok(parsed) => schema.parsed = Some(parsed),
                    Err(e) => {
                        result.warnings.push(format!(
                            "Failed to parse AsyncAPI schema for {service_name}: {e}"
                        ));
                        continue;
                    }
                }
            }

            let parsed = schema.parsed.as_ref().unwrap();
            let strategy = self.config.default_conflict_strategy;

            let channel_prefix = &schema.manifest.service_name;
            let message_prefix = &schema.manifest.service_name;

            // Merge channels
            for (channel_name, channel) in &parsed.channels {
                let mut prefixed_name = format!("{channel_prefix}.{channel_name}");

                if let Some(existing_service) = seen_channels.get(&prefixed_name) {
                    let conflict = Conflict {
                        conflict_type: ConflictType::Component,
                        item: channel_name.clone(),
                        services: vec![existing_service.clone(), service_name.clone()],
                        resolution: String::new(),
                        strategy,
                    };

                    match strategy {
                        ConflictStrategy::Error => {
                            return Err(crate::errors::Error::Custom(format!(
                                "channel conflict: {channel_name} exists in both {existing_service} and {service_name}"
                            )));
                        }
                        ConflictStrategy::Skip => {
                            let mut c = conflict;
                            c.resolution = format!("Skipped channel from {service_name}");
                            result.conflicts.push(c);
                            continue;
                        }
                        ConflictStrategy::Overwrite => {
                            let mut c = conflict;
                            c.resolution = format!("Overwritten with {service_name} version");
                            result.conflicts.push(c);
                        }
                        ConflictStrategy::Prefix => {
                            prefixed_name = format!("{service_name}.{channel_name}");
                            let mut c = conflict;
                            c.resolution = format!("Prefixed to {prefixed_name}");
                            result.conflicts.push(c);
                        }
                        ConflictStrategy::Merge => {
                            let existing = result.spec.channels.get(&prefixed_name).cloned();
                            if let Some(existing) = existing {
                                let merged = merge_channels(existing, channel.clone());
                                result.spec.channels.insert(prefixed_name.clone(), merged);
                            }
                            let mut c = conflict;
                            c.resolution = "Merged operations".to_string();
                            result.conflicts.push(c);
                            continue;
                        }
                    }
                }

                result
                    .spec
                    .channels
                    .insert(prefixed_name.clone(), channel.clone());
                seen_channels.insert(prefixed_name, service_name.clone());
            }

            // Merge components
            if let Some(components) = &parsed.components {
                // Merge messages
                for (name, message) in &components.messages {
                    let prefixed_name = format!("{message_prefix}_{name}");
                    if let Some(existing_service) = seen_messages.get(&prefixed_name) {
                        if strategy == ConflictStrategy::Skip {
                            result.conflicts.push(Conflict {
                                conflict_type: ConflictType::Component,
                                item: name.clone(),
                                services: vec![existing_service.clone(), service_name.clone()],
                                resolution: format!("Skipped message from {service_name}"),
                                strategy,
                            });
                            continue;
                        }
                    }

                    if let Some(spec_components) = result.spec.components.as_mut() {
                        spec_components
                            .messages
                            .insert(prefixed_name.clone(), message.clone());
                    }
                    seen_messages.insert(prefixed_name, service_name.clone());
                }

                // Merge schemas
                for (name, schema_obj) in &components.schemas {
                    let prefixed_name = format!("{message_prefix}_{name}");
                    if let Some(spec_components) = result.spec.components.as_mut() {
                        spec_components
                            .schemas
                            .insert(prefixed_name, schema_obj.clone());
                    }
                }

                // Merge security schemes
                for (name, sec_scheme) in &components.security_schemes {
                    if let Some(existing_service) = seen_security_schemes.get(name) {
                        let conflict = Conflict {
                            conflict_type: ConflictType::SecurityScheme,
                            item: name.clone(),
                            services: vec![existing_service.clone(), service_name.clone()],
                            resolution: String::new(),
                            strategy,
                        };

                        match strategy {
                            ConflictStrategy::Error => {
                                return Err(crate::errors::Error::Custom(format!(
                                    "security scheme conflict: {name} exists in both {existing_service} and {service_name}"
                                )));
                            }
                            ConflictStrategy::Skip => {
                                let mut c = conflict;
                                c.resolution =
                                    format!("Skipped security scheme from {service_name}");
                                result.conflicts.push(c);
                                continue;
                            }
                            ConflictStrategy::Overwrite => {
                                let mut c = conflict;
                                c.resolution = format!("Overwritten with {service_name} version");
                                result.conflicts.push(c);
                            }
                            ConflictStrategy::Prefix => {
                                let prefixed_name = format!("{service_name}_{name}");
                                let mut c = conflict;
                                c.resolution = format!("Prefixed to {prefixed_name}");
                                result.conflicts.push(c);
                                if let Some(spec_components) = result.spec.components.as_mut() {
                                    spec_components
                                        .security_schemes
                                        .insert(prefixed_name.clone(), sec_scheme.clone());
                                }
                                seen_security_schemes.insert(prefixed_name, service_name.clone());
                                continue;
                            }
                            ConflictStrategy::Merge => {
                                let mut c = conflict;
                                c.resolution =
                                    format!("Merged (overwritten) with {service_name} version");
                                result.conflicts.push(c);
                            }
                        }
                    }

                    if let Some(spec_components) = result.spec.components.as_mut() {
                        spec_components
                            .security_schemes
                            .insert(name.clone(), sec_scheme.clone());
                    }
                    seen_security_schemes.insert(name.clone(), service_name.clone());
                }
            }

            // Merge servers
            for (server_name, server) in &parsed.servers {
                let prefixed_name = format!("{service_name}_{server_name}");
                if let Some(existing_service) = seen_servers.get(&prefixed_name) {
                    result.warnings.push(format!(
                        "Server {server_name} from {service_name} overwrites {existing_service}"
                    ));
                }
                result
                    .spec
                    .servers
                    .insert(prefixed_name.clone(), server.clone());
                seen_servers.insert(prefixed_name, service_name.clone());
            }
        }

        Ok(result)
    }
}

/// Parse AsyncAPI schema from JSON
pub fn parse_asyncapi_schema(raw: &serde_json::Value) -> Result<AsyncAPISpec> {
    let schema_map = raw
        .as_object()
        .ok_or_else(|| crate::errors::Error::invalid_schema("schema must be an object"))?;

    let asyncapi = schema_map
        .get("asyncapi")
        .and_then(|v| v.as_str())
        .ok_or_else(|| crate::errors::Error::invalid_schema("missing asyncapi version"))?
        .to_string();

    let info = super::openapi::parse_info_public(schema_map.get("info"))?;

    let servers = schema_map
        .get("servers")
        .and_then(|v| v.as_object())
        .map(parse_async_servers)
        .unwrap_or_default();

    let channels = schema_map
        .get("channels")
        .and_then(|v| v.as_object())
        .map(parse_channels)
        .unwrap_or_default();

    let components = schema_map
        .get("components")
        .and_then(|v| v.as_object())
        .map(parse_async_components);

    Ok(AsyncAPISpec {
        asyncapi,
        info,
        servers,
        channels,
        components,
        security: Vec::new(),
        extensions: HashMap::new(),
    })
}

fn parse_async_servers(
    obj: &serde_json::Map<String, serde_json::Value>,
) -> HashMap<String, AsyncServer> {
    obj.iter()
        .filter_map(|(name, server)| {
            server.as_object().and_then(|s| {
                Some((
                    name.clone(),
                    AsyncServer {
                        url: s.get("url")?.as_str()?.to_string(),
                        protocol: s.get("protocol")?.as_str()?.to_string(),
                        description: s
                            .get("description")
                            .and_then(|v| v.as_str())
                            .map(String::from),
                        variables: None,
                        bindings: None,
                    },
                ))
            })
        })
        .collect()
}

fn parse_channels(obj: &serde_json::Map<String, serde_json::Value>) -> HashMap<String, Channel> {
    obj.iter()
        .filter_map(|(name, channel)| {
            channel.as_object().map(|c| {
                (
                    name.clone(),
                    Channel {
                        description: c
                            .get("description")
                            .and_then(|v| v.as_str())
                            .map(String::from),
                        subscribe: c
                            .get("subscribe")
                            .and_then(|v| v.as_object())
                            .map(super::openapi::parse_operation_public),
                        publish: c
                            .get("publish")
                            .and_then(|v| v.as_object())
                            .map(super::openapi::parse_operation_public),
                        parameters: None,
                        bindings: None,
                        extensions: HashMap::new(),
                    },
                )
            })
        })
        .collect()
}

fn parse_async_components(obj: &serde_json::Map<String, serde_json::Value>) -> AsyncComponents {
    AsyncComponents {
        messages: obj
            .get("messages")
            .and_then(|v| v.as_object())
            .map(|m| m.iter().map(|(k, v)| (k.clone(), v.clone())).collect())
            .unwrap_or_default(),
        schemas: obj
            .get("schemas")
            .and_then(|v| v.as_object())
            .map(|s| s.iter().map(|(k, v)| (k.clone(), v.clone())).collect())
            .unwrap_or_default(),
        security_schemes: HashMap::new(),
    }
}

fn merge_channels(existing: Channel, new: Channel) -> Channel {
    Channel {
        description: new.description.or(existing.description),
        subscribe: new.subscribe.or(existing.subscribe),
        publish: new.publish.or(existing.publish),
        parameters: new.parameters.or(existing.parameters),
        bindings: new.bindings.or(existing.bindings),
        extensions: {
            let mut ext = existing.extensions;
            ext.extend(new.extensions);
            ext
        },
    }
}

fn should_include_asyncapi(schema: &AsyncAPIServiceSchema) -> bool {
    schema
        .manifest
        .schemas
        .iter()
        .any(|s| s.schema_type == SchemaType::AsyncAPI)
}
