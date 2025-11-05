//! gRPC schema merger for combining multiple service schemas

use super::*;
use crate::errors::Result;
use crate::types::{SchemaManifest, SchemaType};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// gRPC service definition (protobuf-based)
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct GRPCSpec {
    pub syntax: String, // proto3
    pub package: String,
    pub services: HashMap<String, GRPCService>,
    pub messages: HashMap<String, GRPCMessage>,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub enums: HashMap<String, GRPCEnum>,
    #[serde(
        skip_serializing_if = "HashMap::is_empty",
        default,
        rename = "securitySchemes"
    )]
    pub security_schemes: HashMap<String, GRPCSecurityScheme>,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    pub imports: Vec<String>,
}

/// gRPC authentication configuration
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GRPCSecurityScheme {
    #[serde(rename = "type")]
    pub scheme_type: String, // tls, oauth2, apiKey, custom
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub tls: Option<GRPCTLSConfig>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "tokenUrl")]
    pub token_url: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub scopes: Option<HashMap<String, String>>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "keyName")]
    pub key_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metadata: Option<HashMap<String, serde_json::Value>>,
}

/// TLS configuration for gRPC
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GRPCTLSConfig {
    #[serde(skip_serializing_if = "Option::is_none", rename = "serverName")]
    pub server_name: Option<String>,
    #[serde(rename = "requireClientCert")]
    pub require_client_cert: bool,
    #[serde(skip_serializing_if = "Option::is_none", rename = "insecureSkipVerify")]
    pub insecure_skip_verify: Option<bool>,
}

/// gRPC service definition
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct GRPCService {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub methods: HashMap<String, GRPCMethod>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub options: Option<HashMap<String, serde_json::Value>>,
}

/// gRPC method (RPC)
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GRPCMethod {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub input_type: String,
    pub output_type: String,
    pub client_streaming: bool,
    pub server_streaming: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub options: Option<HashMap<String, serde_json::Value>>,
}

/// Protobuf message
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct GRPCMessage {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub fields: HashMap<String, GRPCField>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub options: Option<HashMap<String, serde_json::Value>>,
}

/// Message field
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GRPCField {
    pub name: String,
    #[serde(rename = "type")]
    pub field_type: String,
    pub number: i32,
    pub repeated: bool,
    pub optional: bool,
}

/// Protobuf enum
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GRPCEnum {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    pub values: HashMap<String, i32>,
}

/// Service schema with gRPC context
#[derive(Debug, Clone)]
pub struct GRPCServiceSchema {
    pub manifest: SchemaManifest,
    pub schema: serde_json::Value,
    pub parsed: Option<GRPCSpec>,
}

/// gRPC merger
pub struct GRPCMerger {
    config: MergerConfig,
}

/// gRPC merge result
#[derive(Debug, Clone)]
pub struct GRPCMergeResult {
    pub spec: GRPCSpec,
    pub included_services: Vec<String>,
    pub excluded_services: Vec<String>,
    pub conflicts: Vec<Conflict>,
    pub warnings: Vec<String>,
}

impl GRPCMerger {
    pub fn new(config: MergerConfig) -> Self {
        Self { config }
    }

    pub fn merge(&self, schemas: Vec<GRPCServiceSchema>) -> Result<GRPCMergeResult> {
        let mut result = GRPCMergeResult {
            spec: GRPCSpec {
                syntax: "proto3".to_string(),
                package: self.config.merged_title.clone(),
                services: HashMap::new(),
                messages: HashMap::new(),
                enums: HashMap::new(),
                security_schemes: HashMap::new(),
                imports: Vec::new(),
            },
            included_services: Vec::new(),
            excluded_services: Vec::new(),
            conflicts: Vec::new(),
            warnings: Vec::new(),
        };

        let mut seen_services: HashMap<String, String> = HashMap::new();
        let mut seen_messages: HashMap<String, String> = HashMap::new();
        let mut seen_enums: HashMap<String, String> = HashMap::new();
        let mut seen_security_schemes: HashMap<String, String> = HashMap::new();

        for mut schema in schemas {
            let service_name = schema.manifest.service_name.clone();

            if !should_include_grpc(&schema) {
                result.excluded_services.push(service_name);
                continue;
            }

            result.included_services.push(service_name.clone());

            if schema.parsed.is_none() {
                match parse_grpc_schema(&schema.schema) {
                    Ok(parsed) => schema.parsed = Some(parsed),
                    Err(e) => {
                        result.warnings.push(format!(
                            "Failed to parse gRPC schema for {service_name}: {e}"
                        ));
                        continue;
                    }
                }
            }

            let parsed = schema.parsed.as_ref().unwrap();
            let strategy = self.config.default_conflict_strategy;

            let service_prefix = &schema.manifest.service_name;
            let message_prefix = &schema.manifest.service_name;

            // Merge services
            for (svc_name, service) in &parsed.services {
                let mut prefixed_name = format!("{service_prefix}_{svc_name}");

                if let Some(existing_service) = seen_services.get(&prefixed_name) {
                    let conflict = Conflict {
                        conflict_type: ConflictType::Component,
                        item: svc_name.clone(),
                        services: vec![existing_service.clone(), service_name.clone()],
                        resolution: String::new(),
                        strategy,
                    };

                    match strategy {
                        ConflictStrategy::Error => {
                            return Err(crate::errors::Error::Custom(format!(
                                "gRPC service conflict: {svc_name} exists in both {existing_service} and {service_name}"
                            )));
                        }
                        ConflictStrategy::Skip => {
                            let mut c = conflict;
                            c.resolution = format!("Skipped service from {service_name}");
                            result.conflicts.push(c);
                            continue;
                        }
                        ConflictStrategy::Overwrite => {
                            let mut c = conflict;
                            c.resolution = format!("Overwritten with {service_name} version");
                            result.conflicts.push(c);
                        }
                        ConflictStrategy::Prefix => {
                            prefixed_name = format!("{service_name}_{svc_name}");
                            let mut c = conflict;
                            c.resolution = format!("Prefixed to {prefixed_name}");
                            result.conflicts.push(c);
                        }
                        ConflictStrategy::Merge => {
                            let mut c = conflict;
                            c.resolution = "Merged".to_string();
                            result.conflicts.push(c);
                        }
                    }
                }

                result
                    .spec
                    .services
                    .insert(prefixed_name.clone(), service.clone());
                seen_services.insert(prefixed_name, service_name.clone());
            }

            // Merge messages
            for (msg_name, message) in &parsed.messages {
                let prefixed_name = format!("{message_prefix}_{msg_name}");
                if let Some(existing_service) = seen_messages.get(&prefixed_name) {
                    if strategy == ConflictStrategy::Skip {
                        result.conflicts.push(Conflict {
                            conflict_type: ConflictType::Component,
                            item: msg_name.clone(),
                            services: vec![existing_service.clone(), service_name.clone()],
                            resolution: format!("Skipped message from {service_name}"),
                            strategy,
                        });
                        continue;
                    }
                }

                result
                    .spec
                    .messages
                    .insert(prefixed_name.clone(), message.clone());
                seen_messages.insert(prefixed_name, service_name.clone());
            }

            // Merge enums
            for (enum_name, enum_def) in &parsed.enums {
                let prefixed_name = format!("{message_prefix}_{enum_name}");
                if let Some(existing_service) = seen_enums.get(&prefixed_name) {
                    result.warnings.push(format!(
                        "Enum {enum_name} from {service_name} overwrites {existing_service}"
                    ));
                }
                result
                    .spec
                    .enums
                    .insert(prefixed_name.clone(), enum_def.clone());
                seen_enums.insert(prefixed_name, service_name.clone());
            }

            // Merge security schemes
            for (name, sec_scheme) in &parsed.security_schemes {
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
                                "gRPC security scheme conflict: {name} exists in both {existing_service} and {service_name}"
                            )));
                        }
                        ConflictStrategy::Skip => {
                            let mut c = conflict;
                            c.resolution = format!("Skipped security scheme from {service_name}");
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
                            result
                                .spec
                                .security_schemes
                                .insert(prefixed_name.clone(), sec_scheme.clone());
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

                result
                    .spec
                    .security_schemes
                    .insert(name.clone(), sec_scheme.clone());
                seen_security_schemes.insert(name.clone(), service_name.clone());
            }
        }

        Ok(result)
    }
}

/// Parse gRPC schema from JSON
pub fn parse_grpc_schema(raw: &serde_json::Value) -> Result<GRPCSpec> {
    let schema_map = raw
        .as_object()
        .ok_or_else(|| crate::errors::Error::invalid_schema("schema must be an object"))?;

    let spec = GRPCSpec {
        syntax: "proto3".to_string(),
        package: schema_map
            .get("package")
            .and_then(|v| v.as_str())
            .unwrap_or("default")
            .to_string(),
        services: HashMap::new(),
        messages: HashMap::new(),
        enums: HashMap::new(),
        security_schemes: HashMap::new(),
        imports: Vec::new(),
    };

    Ok(spec)
}

fn should_include_grpc(schema: &GRPCServiceSchema) -> bool {
    schema
        .manifest
        .schemas
        .iter()
        .any(|s| s.schema_type == SchemaType::GRPC)
}
