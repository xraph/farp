//! OpenAPI schema parsing and manipulation

use super::types::*;
use super::*;
use crate::types::{MountStrategy, SchemaManifest};
use std::collections::HashMap;

/// Parses a raw OpenAPI schema into structured format
pub fn parse_openapi_schema(raw: &serde_json::Value) -> Result<OpenAPISpec> {
    let schema_map = raw
        .as_object()
        .ok_or_else(|| crate::errors::Error::invalid_schema("schema must be an object"))?;

    let openapi = schema_map
        .get("openapi")
        .and_then(|v| v.as_str())
        .ok_or_else(|| crate::errors::Error::invalid_schema("missing openapi version"))?
        .to_string();

    let info = parse_info_public(schema_map.get("info"))?;

    let servers = schema_map
        .get("servers")
        .and_then(|v| v.as_array())
        .map(|arr| parse_servers(arr))
        .unwrap_or_default();

    let paths = schema_map
        .get("paths")
        .and_then(|v| v.as_object())
        .map(parse_paths)
        .unwrap_or_default();

    let components = schema_map
        .get("components")
        .and_then(|v| v.as_object())
        .map(parse_components);

    let tags = schema_map
        .get("tags")
        .and_then(|v| v.as_array())
        .map(|arr| parse_tags(arr))
        .unwrap_or_default();

    // Parse extensions (x-*)
    let extensions = schema_map
        .iter()
        .filter(|(k, _)| k.starts_with("x-"))
        .map(|(k, v)| (k.clone(), v.clone()))
        .collect();

    Ok(OpenAPISpec {
        openapi,
        info,
        servers,
        paths,
        components,
        security: Vec::new(),
        tags,
        extensions,
    })
}

pub(crate) fn parse_info_public(value: Option<&serde_json::Value>) -> Result<Info> {
    let info = value
        .and_then(|v| v.as_object())
        .ok_or_else(|| crate::errors::Error::invalid_schema("missing or invalid info"))?;

    Ok(Info {
        title: info
            .get("title")
            .and_then(|v| v.as_str())
            .ok_or_else(|| crate::errors::Error::invalid_schema("missing info.title"))?
            .to_string(),
        description: info
            .get("description")
            .and_then(|v| v.as_str())
            .map(String::from),
        version: info
            .get("version")
            .and_then(|v| v.as_str())
            .ok_or_else(|| crate::errors::Error::invalid_schema("missing info.version"))?
            .to_string(),
        terms_of_service: info
            .get("termsOfService")
            .and_then(|v| v.as_str())
            .map(String::from),
        contact: None,
        license: None,
        extensions: info
            .iter()
            .filter(|(k, _)| k.starts_with("x-"))
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect(),
    })
}

fn parse_servers(arr: &[serde_json::Value]) -> Vec<Server> {
    arr.iter()
        .filter_map(|v| v.as_object())
        .filter_map(|obj| {
            Some(Server {
                url: obj.get("url")?.as_str()?.to_string(),
                description: obj
                    .get("description")
                    .and_then(|v| v.as_str())
                    .map(String::from),
                variables: None,
            })
        })
        .collect()
}

fn parse_paths(obj: &serde_json::Map<String, serde_json::Value>) -> HashMap<String, PathItem> {
    obj.iter()
        .filter_map(|(path, item)| {
            item.as_object()
                .map(|item_obj| (path.clone(), parse_path_item(item_obj)))
        })
        .collect()
}

fn parse_path_item(obj: &serde_json::Map<String, serde_json::Value>) -> PathItem {
    PathItem {
        summary: obj
            .get("summary")
            .and_then(|v| v.as_str())
            .map(String::from),
        description: obj
            .get("description")
            .and_then(|v| v.as_str())
            .map(String::from),
        get: obj
            .get("get")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        put: obj
            .get("put")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        post: obj
            .get("post")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        delete: obj
            .get("delete")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        options: obj
            .get("options")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        head: obj
            .get("head")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        patch: obj
            .get("patch")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        trace: obj
            .get("trace")
            .and_then(|v| v.as_object())
            .map(parse_operation_public),
        parameters: Vec::new(),
        extensions: obj
            .iter()
            .filter(|(k, _)| k.starts_with("x-"))
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect(),
    }
}

pub(crate) fn parse_operation_public(
    obj: &serde_json::Map<String, serde_json::Value>,
) -> Operation {
    Operation {
        operation_id: obj
            .get("operationId")
            .and_then(|v| v.as_str())
            .map(String::from),
        summary: obj
            .get("summary")
            .and_then(|v| v.as_str())
            .map(String::from),
        description: obj
            .get("description")
            .and_then(|v| v.as_str())
            .map(String::from),
        tags: obj
            .get("tags")
            .and_then(|v| v.as_array())
            .map(|arr| {
                arr.iter()
                    .filter_map(|v| v.as_str().map(String::from))
                    .collect()
            })
            .unwrap_or_default(),
        parameters: Vec::new(),
        request_body: None,
        responses: None,
        security: Vec::new(),
        deprecated: obj.get("deprecated").and_then(|v| v.as_bool()),
        extensions: obj
            .iter()
            .filter(|(k, _)| k.starts_with("x-"))
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect(),
    }
}

fn parse_components(obj: &serde_json::Map<String, serde_json::Value>) -> Components {
    let schemas = obj
        .get("schemas")
        .and_then(|v| v.as_object())
        .map(|schemas_obj| {
            schemas_obj
                .iter()
                .map(|(k, v)| (k.clone(), v.clone()))
                .collect()
        })
        .unwrap_or_default();

    Components {
        schemas,
        responses: HashMap::new(),
        parameters: HashMap::new(),
        request_bodies: HashMap::new(),
        headers: HashMap::new(),
        security_schemes: HashMap::new(),
    }
}

fn parse_tags(arr: &[serde_json::Value]) -> Vec<Tag> {
    arr.iter()
        .filter_map(|v| v.as_object())
        .filter_map(|obj| {
            Some(Tag {
                name: obj.get("name")?.as_str()?.to_string(),
                description: obj
                    .get("description")
                    .and_then(|v| v.as_str())
                    .map(String::from),
                extensions: obj
                    .iter()
                    .filter(|(k, _)| k.starts_with("x-"))
                    .map(|(k, v)| (k.clone(), v.clone()))
                    .collect(),
            })
        })
        .collect()
}

/// Applies routing configuration to paths
pub fn apply_routing(
    paths: &HashMap<String, PathItem>,
    manifest: &SchemaManifest,
) -> HashMap<String, PathItem> {
    paths
        .iter()
        .map(|(path, item)| {
            let new_path = apply_mount_strategy(path, manifest);
            (new_path, item.clone())
        })
        .collect()
}

fn apply_mount_strategy(path: &str, manifest: &SchemaManifest) -> String {
    let routing = &manifest.routing;

    match routing.strategy {
        MountStrategy::Root => path.to_string(),
        MountStrategy::Instance => format!("/{}{}", manifest.instance_id, path),
        MountStrategy::Service => format!("/{}{}", manifest.service_name, path),
        MountStrategy::Versioned => {
            format!(
                "/{}/{}{}",
                manifest.service_name, manifest.service_version, path
            )
        }
        MountStrategy::Custom => {
            if let Some(base_path) = &routing.base_path {
                format!("{base_path}{path}")
            } else {
                path.to_string()
            }
        }
        MountStrategy::Subdomain => path.to_string(),
    }
}

/// Adds prefix to component schema names
pub fn prefix_component_names(components: &Components, prefix: &str) -> Components {
    if prefix.is_empty() {
        return components.clone();
    }

    Components {
        schemas: components
            .schemas
            .iter()
            .map(|(name, schema)| (format!("{prefix}_{name}"), schema.clone()))
            .collect(),
        responses: components
            .responses
            .iter()
            .map(|(name, response)| (format!("{prefix}_{name}"), response.clone()))
            .collect(),
        parameters: components
            .parameters
            .iter()
            .map(|(name, param)| (format!("{prefix}_{name}"), param.clone()))
            .collect(),
        request_bodies: components
            .request_bodies
            .iter()
            .map(|(name, body)| (format!("{prefix}_{name}"), body.clone()))
            .collect(),
        headers: HashMap::new(),
        security_schemes: components.security_schemes.clone(), // Don't prefix security schemes
    }
}

/// Applies prefixes to operation IDs and tags
pub fn apply_operation_prefixes(
    mut item: PathItem,
    op_id_prefix: &str,
    tag_prefix: &str,
    service_name: &str,
    seen_operation_ids: &mut HashMap<String, String>,
    result: &mut MergeResult,
) -> PathItem {
    let mut apply_to_op = |op: &mut Option<Operation>| {
        if let Some(operation) = op {
            // Prefix operation ID
            if let Some(original_id) = &operation.operation_id {
                let new_id = if !op_id_prefix.is_empty() {
                    format!("{op_id_prefix}_{original_id}")
                } else {
                    original_id.clone()
                };

                // Check for conflicts
                if let Some(existing_service) = seen_operation_ids.get(&new_id) {
                    result.conflicts.push(Conflict {
                        conflict_type: ConflictType::OperationID,
                        item: original_id.clone(),
                        services: vec![existing_service.clone(), service_name.to_string()],
                        resolution: format!("Prefixed to {new_id}"),
                        strategy: ConflictStrategy::Prefix,
                    });
                }
                seen_operation_ids.insert(new_id.clone(), service_name.to_string());
                operation.operation_id = Some(new_id);
            }

            // Prefix tags
            if !tag_prefix.is_empty() {
                operation.tags = operation
                    .tags
                    .iter()
                    .map(|tag| format!("{tag_prefix}_{tag}"))
                    .collect();
            }
        }
    };

    apply_to_op(&mut item.get);
    apply_to_op(&mut item.post);
    apply_to_op(&mut item.put);
    apply_to_op(&mut item.delete);
    apply_to_op(&mut item.patch);
    apply_to_op(&mut item.options);
    apply_to_op(&mut item.head);
    apply_to_op(&mut item.trace);

    item
}

/// Merges two path items, preferring non-None operations
pub fn merge_path_items(existing: PathItem, new: PathItem) -> PathItem {
    PathItem {
        summary: new.summary.or(existing.summary),
        description: new.description.or(existing.description),
        get: new.get.or(existing.get),
        post: new.post.or(existing.post),
        put: new.put.or(existing.put),
        delete: new.delete.or(existing.delete),
        patch: new.patch.or(existing.patch),
        options: new.options.or(existing.options),
        head: new.head.or(existing.head),
        trace: new.trace.or(existing.trace),
        parameters: {
            let mut params = existing.parameters;
            params.extend(new.parameters);
            params
        },
        extensions: {
            let mut ext = existing.extensions;
            ext.extend(new.extensions);
            ext
        },
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_openapi_schema() {
        let schema = serde_json::json!({
            "openapi": "3.1.0",
            "info": {
                "title": "Test API",
                "version": "1.0.0"
            },
            "paths": {}
        });

        let parsed = parse_openapi_schema(&schema).unwrap();
        assert_eq!(parsed.openapi, "3.1.0");
        assert_eq!(parsed.info.title, "Test API");
    }

    #[test]
    fn test_prefix_component_names() {
        let components = Components {
            schemas: vec![("User".to_string(), serde_json::json!({"type": "object"}))]
                .into_iter()
                .collect(),
            responses: HashMap::new(),
            parameters: HashMap::new(),
            request_bodies: HashMap::new(),
            headers: HashMap::new(),
            security_schemes: HashMap::new(),
        };

        let prefixed = prefix_component_names(&components, "service");
        assert!(prefixed.schemas.contains_key("service_User"));
        assert!(!prefixed.schemas.contains_key("User"));
    }

    #[test]
    fn test_merge_path_items() {
        let existing = PathItem {
            get: Some(Operation {
                operation_id: Some("getUser".to_string()),
                summary: None,
                description: None,
                tags: Vec::new(),
                parameters: Vec::new(),
                request_body: None,
                responses: None,
                security: Vec::new(),
                deprecated: None,
                extensions: HashMap::new(),
            }),
            post: None,
            put: None,
            delete: None,
            patch: None,
            options: None,
            head: None,
            trace: None,
            summary: None,
            description: None,
            parameters: Vec::new(),
            extensions: HashMap::new(),
        };

        let new = PathItem {
            get: None,
            post: Some(Operation {
                operation_id: Some("createUser".to_string()),
                summary: None,
                description: None,
                tags: Vec::new(),
                parameters: Vec::new(),
                request_body: None,
                responses: None,
                security: Vec::new(),
                deprecated: None,
                extensions: HashMap::new(),
            }),
            put: None,
            delete: None,
            patch: None,
            options: None,
            head: None,
            trace: None,
            summary: None,
            description: None,
            parameters: Vec::new(),
            extensions: HashMap::new(),
        };

        let merged = merge_path_items(existing, new);
        assert!(merged.get.is_some());
        assert!(merged.post.is_some());
    }
}
