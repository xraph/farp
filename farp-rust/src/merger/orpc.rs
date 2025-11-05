//! oRPC schema merger for combining multiple service schemas

use super::*;
use crate::errors::Result;
use crate::types::{SchemaManifest, SchemaType};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// oRPC specification
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ORPCSpec {
    pub orpc: String,
    pub info: super::types::Info,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    pub servers: Vec<super::types::Server>,
    pub procedures: HashMap<String, ORPCProcedure>,
    #[serde(skip_serializing_if = "HashMap::is_empty", default)]
    pub schemas: HashMap<String, serde_json::Value>,
    #[serde(
        skip_serializing_if = "HashMap::is_empty",
        default,
        rename = "securitySchemes"
    )]
    pub security_schemes: HashMap<String, ORPCSecurityScheme>,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    pub security: Vec<HashMap<String, Vec<String>>>,
    #[serde(flatten)]
    pub extensions: HashMap<String, serde_json::Value>,
}

/// oRPC security scheme
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ORPCSecurityScheme {
    #[serde(rename = "type")]
    pub scheme_type: String,
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

/// oRPC procedure definition
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ORPCProcedure {
    pub name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Vec::is_empty", default)]
    pub errors: Vec<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub streaming: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub batch: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub options: Option<HashMap<String, serde_json::Value>>,
    #[serde(flatten)]
    pub extensions: HashMap<String, serde_json::Value>,
}

/// Service schema with oRPC context
#[derive(Debug, Clone)]
pub struct ORPCServiceSchema {
    pub manifest: SchemaManifest,
    pub schema: serde_json::Value,
    pub parsed: Option<ORPCSpec>,
}

/// oRPC merger
pub struct ORPCMerger {
    config: MergerConfig,
}

/// oRPC merge result
#[derive(Debug, Clone)]
pub struct ORPCMergeResult {
    pub spec: ORPCSpec,
    pub included_services: Vec<String>,
    pub excluded_services: Vec<String>,
    pub conflicts: Vec<Conflict>,
    pub warnings: Vec<String>,
}

impl ORPCMerger {
    pub fn new(config: MergerConfig) -> Self {
        Self { config }
    }

    pub fn merge(&self, schemas: Vec<ORPCServiceSchema>) -> Result<ORPCMergeResult> {
        let mut result = ORPCMergeResult {
            spec: ORPCSpec {
                orpc: "1.0.0".to_string(),
                info: super::types::Info {
                    title: self.config.merged_title.clone(),
                    description: Some(self.config.merged_description.clone()),
                    version: self.config.merged_version.clone(),
                    terms_of_service: None,
                    contact: None,
                    license: None,
                    extensions: HashMap::new(),
                },
                servers: self.config.servers.clone(),
                procedures: HashMap::new(),
                schemas: HashMap::new(),
                security_schemes: HashMap::new(),
                security: Vec::new(),
                extensions: HashMap::new(),
            },
            included_services: Vec::new(),
            excluded_services: Vec::new(),
            conflicts: Vec::new(),
            warnings: Vec::new(),
        };

        let mut seen_procedures: HashMap<String, String> = HashMap::new();
        let mut seen_schemas: HashMap<String, String> = HashMap::new();
        let mut seen_security_schemes: HashMap<String, String> = HashMap::new();

        for mut schema in schemas {
            let service_name = schema.manifest.service_name.clone();

            if !should_include_orpc(&schema) {
                result.excluded_services.push(service_name);
                continue;
            }

            result.included_services.push(service_name.clone());

            if schema.parsed.is_none() {
                match parse_orpc_schema(&schema.schema) {
                    Ok(parsed) => schema.parsed = Some(parsed),
                    Err(e) => {
                        result.warnings.push(format!(
                            "Failed to parse oRPC schema for {service_name}: {e}"
                        ));
                        continue;
                    }
                }
            }

            let parsed = schema.parsed.as_ref().unwrap();
            let strategy = self.config.default_conflict_strategy;

            let procedure_prefix = &schema.manifest.service_name;
            let schema_prefix = &schema.manifest.service_name;

            // Merge procedures
            for (proc_name, procedure) in &parsed.procedures {
                let mut prefixed_name = format!("{procedure_prefix}.{proc_name}");

                if let Some(existing_service) = seen_procedures.get(&prefixed_name) {
                    let conflict = Conflict {
                        conflict_type: ConflictType::Component,
                        item: proc_name.clone(),
                        services: vec![existing_service.clone(), service_name.clone()],
                        resolution: String::new(),
                        strategy,
                    };

                    match strategy {
                        ConflictStrategy::Error => {
                            return Err(crate::errors::Error::Custom(format!(
                                "oRPC procedure conflict: {proc_name} exists in both {existing_service} and {service_name}"
                            )));
                        }
                        ConflictStrategy::Skip => {
                            let mut c = conflict;
                            c.resolution = format!("Skipped procedure from {service_name}");
                            result.conflicts.push(c);
                            continue;
                        }
                        ConflictStrategy::Overwrite => {
                            let mut c = conflict;
                            c.resolution = format!("Overwritten with {service_name} version");
                            result.conflicts.push(c);
                        }
                        ConflictStrategy::Prefix => {
                            prefixed_name = format!("{service_name}.{proc_name}");
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
                    .procedures
                    .insert(prefixed_name.clone(), procedure.clone());
                seen_procedures.insert(prefixed_name, service_name.clone());
            }

            // Merge schemas
            for (schema_name, schema_obj) in &parsed.schemas {
                let prefixed_name = format!("{schema_prefix}_{schema_name}");
                if let Some(existing_service) = seen_schemas.get(&prefixed_name) {
                    if strategy == ConflictStrategy::Skip {
                        result.conflicts.push(Conflict {
                            conflict_type: ConflictType::Component,
                            item: schema_name.clone(),
                            services: vec![existing_service.clone(), service_name.clone()],
                            resolution: format!("Skipped schema from {service_name}"),
                            strategy,
                        });
                        continue;
                    }
                }

                result
                    .spec
                    .schemas
                    .insert(prefixed_name.clone(), schema_obj.clone());
                seen_schemas.insert(prefixed_name, service_name.clone());
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
                                "oRPC security scheme conflict: {name} exists in both {existing_service} and {service_name}"
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

/// Parse oRPC schema from JSON
pub fn parse_orpc_schema(raw: &serde_json::Value) -> Result<ORPCSpec> {
    serde_json::from_value(raw.clone()).map_err(|e| {
        crate::errors::Error::invalid_schema(format!("Failed to parse oRPC schema: {e}"))
    })
}

fn should_include_orpc(schema: &ORPCServiceSchema) -> bool {
    schema
        .manifest
        .schemas
        .iter()
        .any(|s| s.schema_type == SchemaType::ORPC)
}
