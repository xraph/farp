import type {
  SchemaManifest,
  SchemaDescriptor,
  RoutingConfig,
  InstanceMetadata,
  ManifestDiff,
  SchemaChangeDiff,
  SchemaType,
} from './types.js';
import { MountStrategy, LocationType, isValidSchemaType, isValidMountStrategy } from './types.js';
import { PROTOCOL_VERSION, isCompatible } from './version.js';
import {
  calculateManifestChecksum,
  calculateRoutesChecksum,
} from './checksum.js';
import {
  InvalidManifestError,
  IncompatibleVersionError,
  ChecksumMismatchError,
  UnsupportedTypeError,
  InvalidLocationError,
  ValidationError,
} from './errors.js';

/**
 * Creates a new schema manifest with default values.
 */
export function createManifest(
  serviceName: string,
  serviceVersion: string,
  instanceId: string,
): SchemaManifest {
  return {
    version: PROTOCOL_VERSION,
    service_name: serviceName,
    service_version: serviceVersion,
    instance_id: instanceId,
    schemas: [],
    capabilities: [],
    endpoints: { health: '' },
    routing: {
      strategy: MountStrategy.Service,
    },
    auth: undefined,
    webhook: undefined,
    updated_at: Math.floor(Date.now() / 1000),
    checksum: '',
  };
}

/**
 * Adds a schema descriptor to the manifest.
 */
export function addSchema(manifest: SchemaManifest, descriptor: SchemaDescriptor): void {
  manifest.schemas.push(descriptor);
}

/**
 * Adds a capability to the manifest (deduplicates).
 */
export function addCapability(manifest: SchemaManifest, capability: string): void {
  if (manifest.capabilities.includes(capability)) {
    return;
  }
  manifest.capabilities.push(capability);
}

/**
 * Recalculates the manifest checksum based on all schema hashes.
 */
export async function updateChecksum(manifest: SchemaManifest): Promise<void> {
  const checksum = await calculateManifestChecksum(manifest);
  manifest.checksum = checksum;
  manifest.updated_at = Math.floor(Date.now() / 1000);
}

/**
 * Recalculates the routes checksum based on the route table and routing configuration.
 */
export async function updateRoutesChecksum(manifest: SchemaManifest): Promise<void> {
  const checksum = await calculateRoutesChecksum(manifest);
  manifest.routes_checksum = checksum;
}

/**
 * Validates a schema location.
 */
function validateSchemaLocation(location: SchemaDescriptor['location']): void {
  if (!location.type || !(['http', 'registry', 'inline'] as string[]).includes(location.type)) {
    throw new InvalidLocationError(
      `invalid location type: ${location.type}`,
    );
  }

  switch (location.type) {
    case LocationType.HTTP:
      if (!location.url) {
        throw new InvalidLocationError('URL required for HTTP location');
      }
      break;
    case LocationType.Registry:
      if (!location.registry_path) {
        throw new InvalidLocationError('registry path required for registry location');
      }
      break;
    case LocationType.Inline:
      // No additional validation needed
      break;
  }
}

/**
 * Validates a schema descriptor.
 */
function validateSchemaDescriptor(sd: SchemaDescriptor): void {
  if (!isValidSchemaType(sd.type)) {
    throw new UnsupportedTypeError(`unsupported schema type: ${sd.type}`);
  }

  if (!sd.spec_version) {
    throw new ValidationError('spec_version', 'spec version is required');
  }

  validateSchemaLocation(sd.location);

  // For inline schemas, inline_schema must be present
  if (sd.location.type === LocationType.Inline && sd.inline_schema == null) {
    throw new ValidationError(
      'inline_schema',
      'inline schema is required for inline location type',
    );
  }

  if (!sd.hash) {
    throw new ValidationError('hash', 'schema hash is required');
  }

  // Validate hash format (should be 64 hex characters for SHA256)
  if (sd.hash.length !== 64) {
    throw new ValidationError('hash', 'invalid hash format (expected 64 hex characters)');
  }

  if (!sd.content_type) {
    throw new ValidationError('content_type', 'content type is required');
  }
}

/**
 * Validates instance metadata.
 */
function validateInstanceMetadata(im: InstanceMetadata): void {
  if (!im.address) {
    throw new ValidationError('address', 'instance address is required');
  }

  if (im.weight != null && (im.weight < 0 || im.weight > 100)) {
    throw new ValidationError('weight', 'instance weight must be between 0 and 100');
  }

  if (im.deployment) {
    if (!im.deployment.deployment_id) {
      throw new ValidationError(
        'deployment.deployment_id',
        'deployment ID is required',
      );
    }

    if (im.deployment.traffic_percent != null &&
        (im.deployment.traffic_percent < 0 || im.deployment.traffic_percent > 100)) {
      throw new ValidationError(
        'deployment.traffic_percent',
        'traffic percent must be between 0 and 100',
      );
    }
  }
}

/**
 * Validates routing configuration.
 */
function validateRoutingConfig(rc: RoutingConfig): void {
  if (rc.strategy && !isValidMountStrategy(rc.strategy)) {
    throw new ValidationError(
      'routing.strategy',
      `invalid mount strategy: ${rc.strategy}`,
    );
  }

  if (rc.strategy === MountStrategy.Custom && !rc.base_path) {
    throw new ValidationError(
      'routing.base_path',
      'base path is required for custom mount strategy',
    );
  }

  if (rc.strategy === MountStrategy.Subdomain && !rc.subdomain) {
    throw new ValidationError(
      'routing.subdomain',
      'subdomain is required for subdomain mount strategy',
    );
  }

  const priority = rc.priority ?? 0;
  if (priority < 0 || priority > 100) {
    throw new ValidationError(
      'routing.priority',
      'routing priority must be between 0 and 100',
    );
  }
}

/**
 * Validates the manifest for correctness. Throws on invalid.
 */
export async function validateManifest(manifest: SchemaManifest): Promise<void> {
  // Check protocol version compatibility
  if (!isCompatible(manifest.version)) {
    throw new IncompatibleVersionError(
      `incompatible protocol version: manifest version ${manifest.version}, protocol version ${PROTOCOL_VERSION}`,
    );
  }

  // Check required fields
  if (!manifest.service_name) {
    throw new ValidationError('service_name', 'service name is required');
  }

  if (!manifest.instance_id) {
    throw new ValidationError('instance_id', 'instance ID is required');
  }

  // Validate health endpoint
  if (!manifest.endpoints.health) {
    throw new ValidationError('endpoints.health', 'health endpoint is required');
  }

  // Validate instance metadata if present
  if (manifest.instance) {
    validateInstanceMetadata(manifest.instance);
  }

  // Validate routing configuration
  validateRoutingConfig(manifest.routing);

  // Validate each schema descriptor
  for (let i = 0; i < manifest.schemas.length; i++) {
    try {
      validateSchemaDescriptor(manifest.schemas[i]);
    } catch (err) {
      if (err instanceof Error) {
        throw new InvalidManifestError(`invalid schema at index ${i}: ${err.message}`);
      }
      throw err;
    }
  }

  // Verify checksum if present
  if (manifest.checksum) {
    const expectedChecksum = await calculateManifestChecksum(manifest);
    if (manifest.checksum !== expectedChecksum) {
      throw new ChecksumMismatchError(
        `checksum mismatch: expected ${expectedChecksum}, got ${manifest.checksum}`,
      );
    }
  }
}

/**
 * Retrieves a schema descriptor by type.
 */
export function getSchema(
  manifest: SchemaManifest,
  schemaType: SchemaType,
): SchemaDescriptor | undefined {
  return manifest.schemas.find((s) => s.type === schemaType);
}

/**
 * Checks if the manifest includes a specific capability.
 */
export function hasCapability(manifest: SchemaManifest, capability: string): boolean {
  return manifest.capabilities.includes(capability);
}

/**
 * Creates a deep copy of the manifest.
 */
export function cloneManifest(manifest: SchemaManifest): SchemaManifest {
  const clone: SchemaManifest = {
    version: manifest.version,
    service_name: manifest.service_name,
    service_version: manifest.service_version,
    instance_id: manifest.instance_id,
    schemas: manifest.schemas.map((s) => ({ ...s })),
    capabilities: [...manifest.capabilities],
    endpoints: { ...manifest.endpoints },
    routing: { ...manifest.routing },
    updated_at: manifest.updated_at,
    checksum: manifest.checksum,
    routes_checksum: manifest.routes_checksum,
  };

  if (manifest.instance) {
    clone.instance = { ...manifest.instance };
  }

  if (manifest.auth) {
    clone.auth = { ...manifest.auth };
  }

  if (manifest.webhook) {
    clone.webhook = { ...manifest.webhook };
  }

  if (manifest.hints) {
    clone.hints = { ...manifest.hints };
  }

  if (manifest.route_table && manifest.route_table.length > 0) {
    clone.route_table = manifest.route_table.map((r) => ({ ...r }));
  }

  return clone;
}

/**
 * Serializes the manifest to JSON.
 */
export function manifestToJSON(manifest: SchemaManifest): string {
  return JSON.stringify(manifest);
}

/**
 * Serializes the manifest to pretty-printed JSON.
 */
export function manifestToPrettyJSON(manifest: SchemaManifest): string {
  return JSON.stringify(manifest, null, 2);
}

/**
 * Deserializes a manifest from JSON.
 */
export function manifestFromJSON(data: string): SchemaManifest {
  try {
    return JSON.parse(data) as SchemaManifest;
  } catch (err) {
    throw new InvalidManifestError(
      `invalid manifest format: ${err instanceof Error ? err.message : String(err)}`,
    );
  }
}

/**
 * Compares two manifests and returns the differences.
 */
export function diffManifests(
  oldManifest: SchemaManifest,
  newManifest: SchemaManifest,
): ManifestDiff {
  const diff: ManifestDiff = {
    schemas_added: [],
    schemas_removed: [],
    schemas_changed: [],
    capabilities_added: [],
    capabilities_removed: [],
    endpoints_changed: false,
    routing_changed: false,
    routes_checksum_changed: false,
  };

  // Build maps for easier comparison
  const oldSchemas = new Map<SchemaType, SchemaDescriptor>();
  for (const s of oldManifest.schemas) {
    oldSchemas.set(s.type, s);
  }

  const newSchemas = new Map<SchemaType, SchemaDescriptor>();
  for (const s of newManifest.schemas) {
    newSchemas.set(s.type, s);
  }

  // Find added and changed schemas
  for (const [schemaType, newSchema] of newSchemas) {
    const oldSchema = oldSchemas.get(schemaType);
    if (oldSchema) {
      if (oldSchema.hash !== newSchema.hash) {
        diff.schemas_changed.push({
          type: schemaType,
          old_hash: oldSchema.hash,
          new_hash: newSchema.hash,
        } satisfies SchemaChangeDiff);
      }
    } else {
      diff.schemas_added.push(newSchema);
    }
  }

  // Find removed schemas
  for (const [schemaType, oldSchema] of oldSchemas) {
    if (!newSchemas.has(schemaType)) {
      diff.schemas_removed.push(oldSchema);
    }
  }

  // Compare capabilities
  const oldCaps = new Set(oldManifest.capabilities);
  const newCaps = new Set(newManifest.capabilities);

  for (const cap of newCaps) {
    if (!oldCaps.has(cap)) {
      diff.capabilities_added.push(cap);
    }
  }

  for (const cap of oldCaps) {
    if (!newCaps.has(cap)) {
      diff.capabilities_removed.push(cap);
    }
  }

  // Compare endpoints
  if (
    oldManifest.endpoints.health !== newManifest.endpoints.health ||
    oldManifest.endpoints.metrics !== newManifest.endpoints.metrics ||
    oldManifest.endpoints.openapi !== newManifest.endpoints.openapi ||
    oldManifest.endpoints.asyncapi !== newManifest.endpoints.asyncapi ||
    oldManifest.endpoints.grpc_reflection !== newManifest.endpoints.grpc_reflection ||
    oldManifest.endpoints.graphql !== newManifest.endpoints.graphql ||
    oldManifest.endpoints.documentation !== newManifest.endpoints.documentation ||
    oldManifest.endpoints.changelog !== newManifest.endpoints.changelog
  ) {
    diff.endpoints_changed = true;
  }

  // Compare routing configuration
  if (
    oldManifest.routing.strategy !== newManifest.routing.strategy ||
    oldManifest.routing.base_path !== newManifest.routing.base_path ||
    oldManifest.routing.subdomain !== newManifest.routing.subdomain ||
    oldManifest.routing.strip_prefix !== newManifest.routing.strip_prefix ||
    (oldManifest.routing.priority ?? 0) !== (newManifest.routing.priority ?? 0)
  ) {
    diff.routing_changed = true;
  }

  // Compare routes checksum
  if (oldManifest.routes_checksum !== newManifest.routes_checksum) {
    diff.routes_checksum_changed = true;
  }

  return diff;
}

/**
 * Returns true if a ManifestDiff has any changes.
 */
export function diffHasChanges(diff: ManifestDiff): boolean {
  return (
    diff.schemas_added.length > 0 ||
    diff.schemas_removed.length > 0 ||
    diff.schemas_changed.length > 0 ||
    diff.capabilities_added.length > 0 ||
    diff.capabilities_removed.length > 0 ||
    diff.endpoints_changed ||
    diff.routing_changed ||
    diff.routes_checksum_changed
  );
}

/**
 * Returns true only if route-affecting changes were detected.
 * Gateway implementations SHOULD use this to decide whether to remount routes.
 */
export function diffHasRouteChanges(diff: ManifestDiff): boolean {
  return (
    diff.schemas_added.length > 0 ||
    diff.schemas_removed.length > 0 ||
    diff.routing_changed ||
    diff.routes_checksum_changed
  );
}
