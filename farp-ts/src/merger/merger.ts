/**
 * OpenAPI schema merger for FARP TypeScript client.
 *
 * Ported from Go: merger/merger.go
 * Handles merging multiple OpenAPI schemas from service manifests into a
 * single unified specification with conflict detection and resolution.
 */

import type {
  SchemaManifest,
  CompositionConfig,
  ConflictStrategy,
} from '../types';
import type {
  OpenAPISpec,
  PathItem,
  Components,
  Tag,
  Conflict,
  MergeResult,
  ServiceSchema,
  Server,
} from './openapi-types';
import {
  prefixTags,
  sortTags,
  applyRouting,
  prefixComponentNames,
  parseOpenAPISchema,
  applyOperationPrefixes,
} from './utils';

// =============================================================================
// MergerConfig
// =============================================================================

/** Configuration for the Merger. */
export interface MergerConfig {
  /** Default conflict strategy if not specified in metadata */
  defaultConflictStrategy?: ConflictStrategy;

  /** Title for the merged OpenAPI spec */
  mergedTitle?: string;

  /** Description for the merged OpenAPI spec */
  mergedDescription?: string;

  /** Version for the merged OpenAPI spec */
  mergedVersion?: string;

  /** Whether to include service tags in operations */
  includeServiceTags?: boolean;

  /**
   * When true, all operations from a service are grouped under a single
   * tag matching the service name, instead of prefixing individual tags.
   * Takes precedence over includeServiceTags when both are true.
   */
  collapseServiceTags?: boolean;

  /** Whether to sort merged content alphabetically */
  sortOutput?: boolean;

  /** Custom server URLs for the merged spec */
  servers?: Server[];
}

/** Returns default merger configuration. */
export function defaultMergerConfig(): MergerConfig {
  return {
    defaultConflictStrategy: 'prefix',
    mergedTitle: 'Federated API',
    mergedDescription: 'Merged API specification from multiple services',
    mergedVersion: '1.0.0',
    includeServiceTags: true,
    collapseServiceTags: false,
    sortOutput: true,
    servers: [],
  };
}

// =============================================================================
// Merger
// =============================================================================

/** Handles OpenAPI schema composition / merging. */
export class Merger {
  private readonly config: MergerConfig;

  constructor(config: MergerConfig) {
    this.config = config;
  }

  /**
   * Merges multiple OpenAPI schemas from service manifests.
   * Returns the merged spec, included/excluded services, conflicts, and warnings.
   */
  merge(schemas: ServiceSchema[]): MergeResult {
    const result: MergeResult = {
      spec: {
        openapi: '3.1.0',
        info: {
          title: this.config.mergedTitle ?? 'Federated API',
          description: this.config.mergedDescription ?? 'Merged API specification from multiple services',
          version: this.config.mergedVersion ?? '1.0.0',
        },
        servers: this.config.servers ?? [],
        paths: {},
        components: {
          schemas: {},
          responses: {},
          parameters: {},
          requestBodies: {},
          securitySchemes: {},
        },
        tags: [],
        extensions: {},
      },
      includedServices: [],
      excludedServices: [],
      conflicts: [],
      warnings: [],
    };

    // Track what we've seen for conflict detection
    const seenPaths = new Map<string, string>(); // path -> service
    const seenComponents = new Map<string, string>(); // component -> service
    const seenOperationIds = new Map<string, string>(); // operationID -> service
    const seenTags = new Map<string, Tag>(); // tag name -> tag
    const seenSecuritySchemes = new Map<string, string>(); // security scheme -> service

    // Process each schema
    for (let schema of schemas) {
      const serviceName = schema.manifest.service_name;

      // Check if this schema should be included
      if (!shouldIncludeInMerge(schema)) {
        result.excludedServices.push(serviceName);
        continue;
      }

      result.includedServices.push(serviceName);

      // Parse the schema if not already parsed
      if (!schema.parsed) {
        const parsed = parseOpenAPISchema(schema.schema);
        if (!parsed) {
          result.warnings.push(
            `Failed to parse schema for ${serviceName}`,
          );
          continue;
        }
        schema = { ...schema, parsed };
      }

      // Get composition config
      const compConfig = getCompositionConfig(schema.manifest);
      const strategy = this.getConflictStrategy(compConfig);

      // Determine prefixes
      const componentPrefix = getComponentPrefix(schema.manifest, compConfig);
      const tagPrefix = getTagPrefix(schema.manifest, compConfig);
      const operationIdPrefix = getOperationIdPrefix(schema.manifest, compConfig);

      // Merge paths
      const paths = applyRouting(schema.parsed!.paths, schema.manifest);

      for (let [path, pathItem] of Object.entries(paths)) {
        // Check for path conflicts
        const existingService = seenPaths.get(path);
        if (existingService) {
          const conflict: Conflict = {
            type: 'path',
            item: path,
            services: [existingService, serviceName],
            resolution: '',
            strategy,
          };

          switch (strategy) {
            case 'error':
              throw new Error(
                `path conflict: ${path} exists in both ${existingService} and ${serviceName}`,
              );

            case 'skip':
              conflict.resolution = 'Skipped path from ' + serviceName;
              result.conflicts.push(conflict);
              continue;

            case 'overwrite':
              conflict.resolution = `Overwritten with ${serviceName} version`;
              result.conflicts.push(conflict);
              // Continue to overwrite
              break;

            case 'prefix': {
              // Add service prefix to path
              const newPath = `/${serviceName}${path}`;
              conflict.resolution = 'Prefixed to ' + newPath;
              result.conflicts.push(conflict);
              path = newPath;
              break;
            }

            case 'merge':
              // Attempt to merge operations
              pathItem = mergePathItems(result.spec.paths[path], pathItem);
              conflict.resolution = 'Merged operations';
              result.conflicts.push(conflict);
              break;
          }
        }

        // Apply prefixes to operation IDs and tags
        pathItem = applyOperationPrefixes(
          pathItem,
          operationIdPrefix,
          tagPrefix,
          serviceName,
          this.config.collapseServiceTags ?? false,
          seenOperationIds,
          result,
        );

        result.spec.paths[path] = pathItem;
        seenPaths.set(path, serviceName);
      }

      // Merge components
      if (schema.parsed!.components) {
        const prefixedComponents = prefixComponentNames(
          schema.parsed!.components,
          componentPrefix,
        );

        // Merge schemas
        if (prefixedComponents.schemas) {
          for (const [name, schemaObj] of Object.entries(prefixedComponents.schemas)) {
            const existingCompService = seenComponents.get(name);
            if (existingCompService) {
              const conflict: Conflict = {
                type: 'component',
                item: name,
                services: [existingCompService, serviceName],
                resolution: '',
                strategy,
              };

              if (strategy === 'skip') {
                conflict.resolution = 'Skipped component from ' + serviceName;
                result.conflicts.push(conflict);
                continue;
              }

              conflict.resolution = `Overwritten with ${serviceName} version`;
              result.conflicts.push(conflict);
            }

            result.spec.components!.schemas![name] = schemaObj;
            seenComponents.set(name, serviceName);
          }
        }

        // Merge other component types
        if (prefixedComponents.responses) {
          Object.assign(result.spec.components!.responses!, prefixedComponents.responses);
        }
        if (prefixedComponents.parameters) {
          Object.assign(result.spec.components!.parameters!, prefixedComponents.parameters);
        }
        if (prefixedComponents.requestBodies) {
          Object.assign(result.spec.components!.requestBodies!, prefixedComponents.requestBodies);
        }

        // Merge security schemes (with conflict detection)
        if (prefixedComponents.securitySchemes) {
          for (const [name, scheme] of Object.entries(prefixedComponents.securitySchemes)) {
            const existingSecService = seenSecuritySchemes.get(name);
            if (existingSecService) {
              const conflict: Conflict = {
                type: 'securityScheme',
                item: name,
                services: [existingSecService, serviceName],
                resolution: '',
                strategy,
              };

              switch (strategy) {
                case 'error':
                  throw new Error(
                    `security scheme conflict: ${name} exists in both ${existingSecService} and ${serviceName}`,
                  );

                case 'skip':
                  conflict.resolution = 'Skipped security scheme from ' + serviceName;
                  result.conflicts.push(conflict);
                  continue;

                case 'overwrite':
                  conflict.resolution = `Overwritten with ${serviceName} version`;
                  result.conflicts.push(conflict);
                  break;

                case 'prefix': {
                  const prefixedName = `${componentPrefix}_${name}`;
                  conflict.resolution = 'Prefixed to ' + prefixedName;
                  result.conflicts.push(conflict);
                  result.spec.components!.securitySchemes![prefixedName] = scheme;
                  seenSecuritySchemes.set(prefixedName, serviceName);
                  continue;
                }

                case 'merge':
                  conflict.resolution = `Merged (overwritten) with ${serviceName} version`;
                  result.conflicts.push(conflict);
                  break;
              }
            }

            result.spec.components!.securitySchemes![name] = scheme;
            seenSecuritySchemes.set(name, serviceName);
          }
        }
      }

      // Merge tags
      if (this.config.collapseServiceTags) {
        // Collapse all tags into a single service-level tag
        const serviceTag: Tag = {
          name: serviceName,
          description: 'Routes from ' + serviceName,
        };
        if (!seenTags.has(serviceTag.name)) {
          seenTags.set(serviceTag.name, serviceTag);
          result.spec.tags!.push(serviceTag);
        }
      } else {
        const parsedTags = schema.parsed!.tags ?? [];
        for (const tag of parsedTags) {
          let tagName = tag.name;
          if (tagPrefix && this.config.includeServiceTags) {
            tagName = `${tagPrefix}_${tag.name}`;
          }

          const existing = seenTags.get(tagName);
          if (existing) {
            // Merge descriptions
            if (tag.description && !existing.description) {
              existing.description = tag.description;
              seenTags.set(tagName, existing);
            }
          } else {
            const newTag: Tag = { name: tagName, description: tag.description };
            seenTags.set(tagName, newTag);
            result.spec.tags!.push(newTag);
          }
        }
      }
    }

    // Sort output if requested
    if (this.config.sortOutput) {
      result.spec.tags = sortTags(result.spec.tags ?? []);
    }

    return result;
  }

  /** Resolves the conflict strategy from composition config or default. */
  private getConflictStrategy(
    config: CompositionConfig | undefined,
  ): ConflictStrategy {
    if (config && config.conflict_strategy) {
      return config.conflict_strategy;
    }
    return this.config.defaultConflictStrategy ?? 'prefix';
  }
}

// =============================================================================
// Helper functions
// =============================================================================

/**
 * Determines whether a service schema should be included in the merged output.
 */
function shouldIncludeInMerge(schema: ServiceSchema): boolean {
  // Check OpenAPI metadata for composition config
  for (const schemaDesc of schema.manifest.schemas) {
    if (
      schemaDesc.type === 'openapi' &&
      schemaDesc.metadata?.openapi?.composition
    ) {
      return schemaDesc.metadata.openapi.composition.include_in_merged;
    }
  }

  // Default: include if OpenAPI schema is present
  for (const schemaDesc of schema.manifest.schemas) {
    if (schemaDesc.type === 'openapi') {
      return true;
    }
  }

  return false;
}

/**
 * Extracts the composition config from a manifest's OpenAPI schema metadata.
 */
function getCompositionConfig(
  manifest: SchemaManifest,
): CompositionConfig | undefined {
  for (const schemaDesc of manifest.schemas) {
    if (
      schemaDesc.type === 'openapi' &&
      schemaDesc.metadata?.openapi
    ) {
      return schemaDesc.metadata.openapi.composition;
    }
  }
  return undefined;
}

/**
 * Gets the component prefix for a service (from composition config or service name).
 */
function getComponentPrefix(
  manifest: SchemaManifest,
  config: CompositionConfig | undefined,
): string {
  if (config && config.component_prefix) {
    return config.component_prefix;
  }
  return manifest.service_name;
}

/**
 * Gets the tag prefix for a service (from composition config or service name).
 */
function getTagPrefix(
  manifest: SchemaManifest,
  config: CompositionConfig | undefined,
): string {
  if (config && config.tag_prefix) {
    return config.tag_prefix;
  }
  return manifest.service_name;
}

/**
 * Gets the operation ID prefix for a service (from composition config or service name).
 */
function getOperationIdPrefix(
  manifest: SchemaManifest,
  config: CompositionConfig | undefined,
): string {
  if (config && config.operation_id_prefix) {
    return config.operation_id_prefix;
  }
  return manifest.service_name;
}

/**
 * Merges two path items by preferring non-null operations from the new item.
 */
function mergePathItems(existing: PathItem, newPath: PathItem): PathItem {
  const merged = { ...existing };

  if (newPath.get) merged.get = newPath.get;
  if (newPath.post) merged.post = newPath.post;
  if (newPath.put) merged.put = newPath.put;
  if (newPath.delete) merged.delete = newPath.delete;
  if (newPath.patch) merged.patch = newPath.patch;
  if (newPath.options) merged.options = newPath.options;
  if (newPath.head) merged.head = newPath.head;
  if (newPath.trace) merged.trace = newPath.trace;

  return merged;
}
