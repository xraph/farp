//! OpenAPI schema merger for combining multiple service schemas

pub mod asyncapi;
pub mod grpc;
pub mod openapi;
pub mod orpc;
pub mod types;

pub use asyncapi::*;
pub use grpc::*;
pub use openapi::*;
pub use orpc::*;
pub use types::*;

use crate::errors::Result;
use crate::types::{ConflictStrategy, SchemaManifest, SchemaType};
use std::collections::HashMap;

/// OpenAPI schema merger
pub struct Merger {
    config: MergerConfig,
}

/// Merger configuration
#[derive(Debug, Clone)]
pub struct MergerConfig {
    /// Default conflict strategy if not specified in metadata
    pub default_conflict_strategy: ConflictStrategy,
    /// Title for the merged OpenAPI spec
    pub merged_title: String,
    /// Description for the merged OpenAPI spec
    pub merged_description: String,
    /// Version for the merged OpenAPI spec
    pub merged_version: String,
    /// Whether to include service tags in operations
    pub include_service_tags: bool,
    /// Whether to sort merged content alphabetically
    pub sort_output: bool,
    /// Custom server URLs for the merged spec
    pub servers: Vec<Server>,
}

impl Default for MergerConfig {
    fn default() -> Self {
        Self {
            default_conflict_strategy: ConflictStrategy::Prefix,
            merged_title: "Federated API".to_string(),
            merged_description: "Merged API specification from multiple services".to_string(),
            merged_version: "1.0.0".to_string(),
            include_service_tags: true,
            sort_output: true,
            servers: Vec::new(),
        }
    }
}

/// Service schema with manifest context
#[derive(Debug, Clone)]
pub struct ServiceSchema {
    /// Service manifest
    pub manifest: SchemaManifest,
    /// Raw OpenAPI schema
    pub schema: serde_json::Value,
    /// Parsed OpenAPI spec
    pub parsed: Option<OpenAPISpec>,
}

/// Result of merging multiple schemas
#[derive(Debug, Clone)]
pub struct MergeResult {
    /// The merged OpenAPI specification
    pub spec: OpenAPISpec,
    /// Services that were included in the merge
    pub included_services: Vec<String>,
    /// Services that were excluded (not marked for inclusion)
    pub excluded_services: Vec<String>,
    /// Conflicts that were encountered during merge
    pub conflicts: Vec<Conflict>,
    /// Warnings (non-fatal issues)
    pub warnings: Vec<String>,
}

/// Conflict encountered during merging
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Conflict {
    /// Type of conflict
    pub conflict_type: ConflictType,
    /// Path or name that conflicted
    pub item: String,
    /// Services involved in the conflict
    pub services: Vec<String>,
    /// How the conflict was resolved
    pub resolution: String,
    /// Conflict strategy that was applied
    pub strategy: ConflictStrategy,
}

/// Type of conflict
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum ConflictType {
    /// Path conflict
    Path,
    /// Component name conflict
    Component,
    /// Tag conflict
    Tag,
    /// Operation ID conflict
    OperationID,
    /// Security scheme conflict
    SecurityScheme,
}

impl Merger {
    /// Creates a new merger with the given configuration
    pub fn new(config: MergerConfig) -> Self {
        Self { config }
    }

    /// Creates a new merger with default configuration
    pub fn default() -> Self {
        Self::new(MergerConfig::default())
    }

    /// Merges multiple OpenAPI schemas from service manifests
    pub fn merge(&self, schemas: Vec<ServiceSchema>) -> Result<MergeResult> {
        let mut result = MergeResult {
            spec: OpenAPISpec {
                openapi: "3.1.0".to_string(),
                info: Info {
                    title: self.config.merged_title.clone(),
                    description: Some(self.config.merged_description.clone()),
                    version: self.config.merged_version.clone(),
                    terms_of_service: None,
                    contact: None,
                    license: None,
                    extensions: HashMap::new(),
                },
                servers: self.config.servers.clone(),
                paths: HashMap::new(),
                components: Some(Components {
                    schemas: HashMap::new(),
                    responses: HashMap::new(),
                    parameters: HashMap::new(),
                    request_bodies: HashMap::new(),
                    headers: HashMap::new(),
                    security_schemes: HashMap::new(),
                }),
                security: Vec::new(),
                tags: Vec::new(),
                extensions: HashMap::new(),
            },
            included_services: Vec::new(),
            excluded_services: Vec::new(),
            conflicts: Vec::new(),
            warnings: Vec::new(),
        };

        // Track what we've seen for conflict detection
        let mut seen_paths: HashMap<String, String> = HashMap::new();
        let mut seen_components: HashMap<String, String> = HashMap::new();
        let mut seen_operation_ids: HashMap<String, String> = HashMap::new();
        let mut seen_tags: HashMap<String, Tag> = HashMap::new();
        let mut seen_security_schemes: HashMap<String, String> = HashMap::new();

        // Process each schema
        for mut schema in schemas {
            let service_name = schema.manifest.service_name.clone();

            // Check if this schema should be included
            if !should_include_in_merge(&schema) {
                result.excluded_services.push(service_name);
                continue;
            }

            result.included_services.push(service_name.clone());

            // Parse the schema if not already parsed
            if schema.parsed.is_none() {
                match parse_openapi_schema(&schema.schema) {
                    Ok(parsed) => schema.parsed = Some(parsed),
                    Err(e) => {
                        result
                            .warnings
                            .push(format!("Failed to parse schema for {service_name}: {e}"));
                        continue;
                    }
                }
            }

            let parsed = schema.parsed.as_ref().unwrap();

            // Get composition config
            let comp_config = get_composition_config(&schema.manifest);
            let strategy = self.get_conflict_strategy(comp_config.as_ref());

            // Determine prefixes
            let component_prefix = get_component_prefix(&schema.manifest, comp_config.as_ref());
            let tag_prefix = get_tag_prefix(&schema.manifest, comp_config.as_ref());
            let operation_id_prefix =
                get_operation_id_prefix(&schema.manifest, comp_config.as_ref());

            // Merge paths
            let paths = apply_routing(&parsed.paths, &schema.manifest);
            for (mut path, mut path_item) in paths {
                // Check for path conflicts
                if let Some(existing_service) = seen_paths.get(&path) {
                    let conflict = Conflict {
                        conflict_type: ConflictType::Path,
                        item: path.clone(),
                        services: vec![existing_service.clone(), service_name.clone()],
                        resolution: String::new(),
                        strategy,
                    };

                    match strategy {
                        ConflictStrategy::Error => {
                            return Err(crate::errors::Error::Custom(format!(
                                "path conflict: {path} exists in both {existing_service} and {service_name}"
                            )));
                        }
                        ConflictStrategy::Skip => {
                            let mut c = conflict;
                            c.resolution = format!("Skipped path from {service_name}");
                            result.conflicts.push(c);
                            continue;
                        }
                        ConflictStrategy::Overwrite => {
                            let mut c = conflict;
                            c.resolution = format!("Overwritten with {service_name} version");
                            result.conflicts.push(c);
                        }
                        ConflictStrategy::Prefix => {
                            let new_path = format!("/{service_name}{path}");
                            let mut c = conflict;
                            c.resolution = format!("Prefixed to {new_path}");
                            result.conflicts.push(c);
                            path = new_path;
                        }
                        ConflictStrategy::Merge => {
                            let existing = result.spec.paths.get(&path).cloned();
                            if let Some(existing) = existing {
                                path_item = merge_path_items(existing, path_item);
                            }
                            let mut c = conflict;
                            c.resolution = "Merged operations".to_string();
                            result.conflicts.push(c);
                        }
                    }
                }

                // Apply prefixes to operation IDs and tags
                path_item = apply_operation_prefixes(
                    path_item,
                    &operation_id_prefix,
                    &tag_prefix,
                    &service_name,
                    &mut seen_operation_ids,
                    &mut result,
                );

                result.spec.paths.insert(path.clone(), path_item);
                seen_paths.insert(path, service_name.clone());
            }

            // Merge components
            if let Some(components) = &parsed.components {
                let prefixed = prefix_component_names(components, &component_prefix);

                for (name, schema_obj) in &prefixed.schemas {
                    if let Some(existing_service) = seen_components.get(name) {
                        let conflict = Conflict {
                            conflict_type: ConflictType::Component,
                            item: name.clone(),
                            services: vec![existing_service.clone(), service_name.clone()],
                            resolution: if strategy == ConflictStrategy::Skip {
                                format!("Skipped component from {service_name}")
                            } else {
                                format!("Overwritten with {service_name} version")
                            },
                            strategy,
                        };

                        result.conflicts.push(conflict);

                        if strategy == ConflictStrategy::Skip {
                            continue;
                        }
                    }

                    if let Some(spec_components) = result.spec.components.as_mut() {
                        spec_components
                            .schemas
                            .insert(name.clone(), schema_obj.clone());
                    }
                    seen_components.insert(name.clone(), service_name.clone());
                }

                // Merge other component types
                if let Some(spec_components) = result.spec.components.as_mut() {
                    for (name, response) in &prefixed.responses {
                        spec_components
                            .responses
                            .insert(name.clone(), response.clone());
                    }
                    for (name, param) in &prefixed.parameters {
                        spec_components
                            .parameters
                            .insert(name.clone(), param.clone());
                    }
                    for (name, body) in &prefixed.request_bodies {
                        spec_components
                            .request_bodies
                            .insert(name.clone(), body.clone());
                    }
                    // Merge security schemes (with conflict detection)
                    for (name, scheme) in &prefixed.security_schemes {
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
                                    c.resolution =
                                        format!("Overwritten with {service_name} version");
                                    result.conflicts.push(c);
                                }
                                ConflictStrategy::Prefix => {
                                    let prefixed_name = format!("{service_name}_{name}");
                                    let mut c = conflict;
                                    c.resolution = format!("Prefixed to {prefixed_name}");
                                    result.conflicts.push(c);
                                    spec_components
                                        .security_schemes
                                        .insert(prefixed_name.clone(), scheme.clone());
                                    seen_security_schemes
                                        .insert(prefixed_name, service_name.clone());
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

                        spec_components
                            .security_schemes
                            .insert(name.clone(), scheme.clone());
                        seen_security_schemes.insert(name.clone(), service_name.clone());
                    }
                }
            }

            // Merge tags
            for mut tag in parsed.tags.clone() {
                if !tag_prefix.is_empty() && self.config.include_service_tags {
                    tag.name = format!("{}_{}", tag_prefix, tag.name);
                }

                if let Some(existing) = seen_tags.get(&tag.name) {
                    // Merge descriptions
                    if tag.description.is_some() && existing.description.is_none() {
                        let mut updated = existing.clone();
                        updated.description = tag.description;
                        seen_tags.insert(tag.name.clone(), updated.clone());
                        // Update in result as well
                        if let Some(pos) = result.spec.tags.iter().position(|t| t.name == tag.name)
                        {
                            result.spec.tags[pos] = updated;
                        }
                    }
                } else {
                    seen_tags.insert(tag.name.clone(), tag.clone());
                    result.spec.tags.push(tag);
                }
            }
        }

        // Sort output if requested
        if self.config.sort_output {
            result.spec.tags.sort_by(|a, b| a.name.cmp(&b.name));
        }

        Ok(result)
    }

    fn get_conflict_strategy(
        &self,
        config: Option<&crate::types::CompositionConfig>,
    ) -> ConflictStrategy {
        config
            .map(|c| c.conflict_strategy)
            .unwrap_or(self.config.default_conflict_strategy)
    }
}

// Helper functions

fn should_include_in_merge(schema: &ServiceSchema) -> bool {
    for schema_desc in &schema.manifest.schemas {
        if schema_desc.schema_type == SchemaType::OpenAPI {
            if let Some(metadata) = &schema_desc.metadata {
                if let Some(openapi_metadata) = &metadata.openapi {
                    if let Some(composition) = &openapi_metadata.composition {
                        return composition.include_in_merged;
                    }
                }
            }
            // Default: include if OpenAPI schema is present
            return true;
        }
    }
    false
}

fn get_composition_config(manifest: &SchemaManifest) -> Option<crate::types::CompositionConfig> {
    for schema_desc in &manifest.schemas {
        if schema_desc.schema_type == SchemaType::OpenAPI {
            if let Some(metadata) = &schema_desc.metadata {
                if let Some(openapi_metadata) = &metadata.openapi {
                    return openapi_metadata.composition.clone();
                }
            }
        }
    }
    None
}

fn get_component_prefix(
    manifest: &SchemaManifest,
    config: Option<&crate::types::CompositionConfig>,
) -> String {
    config
        .and_then(|c| c.component_prefix.clone())
        .unwrap_or_else(|| manifest.service_name.clone())
}

fn get_tag_prefix(
    manifest: &SchemaManifest,
    config: Option<&crate::types::CompositionConfig>,
) -> String {
    config
        .and_then(|c| c.tag_prefix.clone())
        .unwrap_or_else(|| manifest.service_name.clone())
}

fn get_operation_id_prefix(
    manifest: &SchemaManifest,
    config: Option<&crate::types::CompositionConfig>,
) -> String {
    config
        .and_then(|c| c.operation_id_prefix.clone())
        .unwrap_or_else(|| manifest.service_name.clone())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_merger_default_config() {
        let config = MergerConfig::default();
        assert_eq!(config.merged_title, "Federated API");
        assert_eq!(config.default_conflict_strategy, ConflictStrategy::Prefix);
        assert!(config.include_service_tags);
    }

    #[test]
    fn test_conflict_type() {
        let conflict = Conflict {
            conflict_type: ConflictType::Path,
            item: "/users".to_string(),
            services: vec!["service-a".to_string(), "service-b".to_string()],
            resolution: "Prefixed".to_string(),
            strategy: ConflictStrategy::Prefix,
        };

        assert_eq!(conflict.conflict_type, ConflictType::Path);
        assert_eq!(conflict.services.len(), 2);
    }
}
