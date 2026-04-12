/**
 * Utility functions for OpenAPI schema merging.
 *
 * Ported from Go: merger/openapi.go
 * Provides tag prefixing, routing application, component prefixing,
 * schema parsing, and operation prefix application.
 */

import type { SchemaManifest } from '../types';
import type {
  OpenAPISpec,
  Info,
  Server,
  PathItem,
  Operation,
  Components,
  SecurityScheme,
  SecurityRequirement,
  Tag,
  Parameter,
  RequestBody,
  Response,
  MediaType,
  Example,
  Header,
  MergeResult,
} from './openapi-types';

// =============================================================================
// PrefixTags
// =============================================================================

/**
 * Adds prefix to operation tags.
 * Each tag is transformed to `prefix_tag`.
 */
export function prefixTags(tags: string[], prefix: string): string[] {
  if (!prefix) {
    return tags;
  }

  return tags.map(tag => `${prefix}_${tag}`);
}

// =============================================================================
// SortTags
// =============================================================================

/**
 * Sorts tags alphabetically by name.
 */
export function sortTags(tags: Tag[]): Tag[] {
  const sorted = [...tags];
  sorted.sort((a, b) => a.name.localeCompare(b.name));
  return sorted;
}

// =============================================================================
// ApplyRouting
// =============================================================================

/**
 * Applies routing configuration to paths based on the manifest's mount strategy.
 * Returns a new paths record with updated path keys.
 */
export function applyRouting(
  paths: Record<string, PathItem>,
  manifest: SchemaManifest,
): Record<string, PathItem> {
  const result: Record<string, PathItem> = {};

  for (const [path, item] of Object.entries(paths)) {
    const newPath = applyMountStrategy(path, manifest);
    result[newPath] = item;
  }

  return result;
}

/**
 * Applies the mount strategy to a single path.
 */
function applyMountStrategy(path: string, manifest: SchemaManifest): string {
  const routing = manifest.routing;

  switch (routing.strategy) {
    case 'root':
      return path;

    case 'instance':
      return `/${manifest.instance_id.toLowerCase()}${path}`;

    case 'service':
      return `/${manifest.service_name.toLowerCase()}${path}`;

    case 'versioned':
      return `/${manifest.service_name.toLowerCase()}/${manifest.service_version.toLowerCase()}${path}`;

    case 'custom':
      if (routing.base_path) {
        return routing.base_path + path;
      }
      return path;

    case 'subdomain':
      // Subdomain routing doesn't change path
      return path;

    default:
      // Default to instance strategy
      return `/${manifest.instance_id}${path}`;
  }
}

// =============================================================================
// PrefixComponentNames
// =============================================================================

/**
 * Adds prefix to component schema names, responses, parameters, and request bodies.
 * Security schemes are copied without prefixing (shared across services).
 */
export function prefixComponentNames(
  components: Components | undefined,
  prefix: string,
): Components {
  if (!components || !prefix) {
    return components ?? createEmptyComponents();
  }

  const result = createEmptyComponents();

  // Prefix schema names
  if (components.schemas) {
    for (const [name, schema] of Object.entries(components.schemas)) {
      result.schemas![`${prefix}_${name}`] = schema;
    }
  }

  // Prefix other components
  if (components.responses) {
    for (const [name, response] of Object.entries(components.responses)) {
      result.responses![`${prefix}_${name}`] = response;
    }
  }

  if (components.parameters) {
    for (const [name, param] of Object.entries(components.parameters)) {
      result.parameters![`${prefix}_${name}`] = param;
    }
  }

  if (components.requestBodies) {
    for (const [name, body] of Object.entries(components.requestBodies)) {
      result.requestBodies![`${prefix}_${name}`] = body;
    }
  }

  if (components.headers) {
    for (const [name, header] of Object.entries(components.headers)) {
      result.headers![`${prefix}_${name}`] = header;
    }
  }

  // Security schemes typically don't need prefixing (shared across services)
  if (components.securitySchemes) {
    Object.assign(result.securitySchemes!, components.securitySchemes);
  }

  return result;
}

/** Creates an empty Components object with all maps initialized. */
function createEmptyComponents(): Components {
  return {
    schemas: {},
    responses: {},
    parameters: {},
    requestBodies: {},
    headers: {},
    securitySchemes: {},
  };
}

// =============================================================================
// ParseOpenAPISchema
// =============================================================================

/**
 * Parses a raw OpenAPI schema (typically a plain object) into the structured
 * OpenAPISpec format. Returns null if the schema is invalid.
 */
export function parseOpenAPISchema(raw: unknown): OpenAPISpec | null {
  const schemaMap = raw as Record<string, unknown> | undefined;
  if (!schemaMap || typeof schemaMap !== 'object') {
    return null;
  }

  const spec: OpenAPISpec = {
    openapi: '',
    info: { title: '', version: '' },
    paths: {},
    extensions: {},
  };

  // Parse OpenAPI version
  if (typeof schemaMap['openapi'] === 'string') {
    spec.openapi = schemaMap['openapi'];
  } else {
    return null; // missing openapi version
  }

  // Parse info
  if (schemaMap['info'] && typeof schemaMap['info'] === 'object') {
    spec.info = parseInfo(schemaMap['info'] as Record<string, unknown>);
  }

  // Parse servers
  if (Array.isArray(schemaMap['servers'])) {
    spec.servers = parseServers(schemaMap['servers']);
  }

  // Parse paths
  if (schemaMap['paths'] && typeof schemaMap['paths'] === 'object') {
    spec.paths = parsePaths(schemaMap['paths'] as Record<string, unknown>);
  }

  // Parse components
  if (schemaMap['components'] && typeof schemaMap['components'] === 'object') {
    spec.components = parseComponents(
      schemaMap['components'] as Record<string, unknown>,
    );
  }

  // Parse tags
  if (Array.isArray(schemaMap['tags'])) {
    spec.tags = parseTags(schemaMap['tags']);
  }

  // Parse extensions (x-*)
  for (const [key, value] of Object.entries(schemaMap)) {
    if (key.startsWith('x-')) {
      spec.extensions![key] = value;
    }
  }

  return spec;
}

// =============================================================================
// Internal parsing helpers
// =============================================================================

function parseInfo(info: Record<string, unknown>): Info {
  const result: Info = {
    title: '',
    version: '',
    extensions: {},
  };

  if (typeof info['title'] === 'string') {
    result.title = info['title'];
  }
  if (typeof info['description'] === 'string') {
    result.description = info['description'];
  }
  if (typeof info['version'] === 'string') {
    result.version = info['version'];
  }

  // Parse extensions
  for (const [key, value] of Object.entries(info)) {
    if (key.startsWith('x-')) {
      result.extensions![key] = value;
    }
  }

  return result;
}

function parseServers(servers: unknown[]): Server[] {
  const result: Server[] = [];

  for (const s of servers) {
    const serverMap = s as Record<string, unknown> | undefined;
    if (!serverMap || typeof serverMap !== 'object') {
      continue;
    }

    const server: Server = {
      url: '',
    };

    if (typeof serverMap['url'] === 'string') {
      server.url = serverMap['url'];
    }
    if (typeof serverMap['description'] === 'string') {
      server.description = serverMap['description'];
    }

    result.push(server);
  }

  return result;
}

function parsePaths(paths: Record<string, unknown>): Record<string, PathItem> {
  const result: Record<string, PathItem> = {};

  for (const [path, item] of Object.entries(paths)) {
    const pathMap = item as Record<string, unknown> | undefined;
    if (pathMap && typeof pathMap === 'object') {
      result[path] = parsePathItem(pathMap);
    }
  }

  return result;
}

function parsePathItem(item: Record<string, unknown>): PathItem {
  const pathItem: PathItem = {
    extensions: {},
  };

  if (typeof item['summary'] === 'string') {
    pathItem.summary = item['summary'];
  }
  if (typeof item['description'] === 'string') {
    pathItem.description = item['description'];
  }

  if (item['get'] && typeof item['get'] === 'object') {
    pathItem.get = parseOperation(item['get'] as Record<string, unknown>);
  }
  if (item['post'] && typeof item['post'] === 'object') {
    pathItem.post = parseOperation(item['post'] as Record<string, unknown>);
  }
  if (item['put'] && typeof item['put'] === 'object') {
    pathItem.put = parseOperation(item['put'] as Record<string, unknown>);
  }
  if (item['delete'] && typeof item['delete'] === 'object') {
    pathItem.delete = parseOperation(item['delete'] as Record<string, unknown>);
  }
  if (item['patch'] && typeof item['patch'] === 'object') {
    pathItem.patch = parseOperation(item['patch'] as Record<string, unknown>);
  }
  if (item['options'] && typeof item['options'] === 'object') {
    pathItem.options = parseOperation(item['options'] as Record<string, unknown>);
  }
  if (item['head'] && typeof item['head'] === 'object') {
    pathItem.head = parseOperation(item['head'] as Record<string, unknown>);
  }
  if (item['trace'] && typeof item['trace'] === 'object') {
    pathItem.trace = parseOperation(item['trace'] as Record<string, unknown>);
  }

  // Parse path-level parameters.
  if (Array.isArray(item['parameters'])) {
    pathItem.parameters = parseParameters(item['parameters']);
  }

  // Parse extensions
  for (const [key, value] of Object.entries(item)) {
    if (key.startsWith('x-')) {
      pathItem.extensions![key] = value;
    }
  }

  return pathItem;
}

function parseOperation(op: Record<string, unknown>): Operation {
  const operation: Operation = {
    extensions: {},
  };

  if (typeof op['operationId'] === 'string') {
    operation.operationId = op['operationId'];
  }
  if (typeof op['summary'] === 'string') {
    operation.summary = op['summary'];
  }
  if (typeof op['description'] === 'string') {
    operation.description = op['description'];
  }

  // Parse tags
  if (Array.isArray(op['tags'])) {
    operation.tags = [];
    for (const tag of op['tags']) {
      if (typeof tag === 'string') {
        operation.tags.push(tag);
      }
    }
  }

  // Parse parameters.
  if (Array.isArray(op['parameters'])) {
    operation.parameters = parseParameters(op['parameters']);
  }

  // Parse requestBody.
  if (op['requestBody'] && typeof op['requestBody'] === 'object') {
    operation.requestBody = parseRequestBody(op['requestBody'] as Record<string, unknown>);
  }

  // Parse responses.
  if (op['responses'] && typeof op['responses'] === 'object') {
    operation.responses = parseResponses(op['responses'] as Record<string, unknown>);
  }

  // Parse security.
  if (Array.isArray(op['security'])) {
    operation.security = parseSecurity(op['security']);
  }

  // Parse deprecated.
  if (typeof op['deprecated'] === 'boolean') {
    operation.deprecated = op['deprecated'];
  }

  // Parse extensions
  for (const [key, value] of Object.entries(op)) {
    if (key.startsWith('x-')) {
      operation.extensions![key] = value;
    }
  }

  return operation;
}

function parseComponents(components: Record<string, unknown>): Components {
  const result: Components = {
    schemas: {},
    responses: {},
    parameters: {},
    requestBodies: {},
    headers: {},
    securitySchemes: {},
  };

  // Parse schemas
  if (components['schemas'] && typeof components['schemas'] === 'object') {
    const schemas = components['schemas'] as Record<string, unknown>;
    for (const [name, schema] of Object.entries(schemas)) {
      if (schema && typeof schema === 'object') {
        result.schemas![name] = schema as Record<string, unknown>;
      }
    }
  }

  // Parse responses
  if (components['responses'] && typeof components['responses'] === 'object') {
    result.responses = parseResponses(components['responses'] as Record<string, unknown>);
  }

  // Parse parameters
  if (components['parameters'] && typeof components['parameters'] === 'object') {
    const params = components['parameters'] as Record<string, unknown>;
    for (const [name, param] of Object.entries(params)) {
      const paramMap = param as Record<string, unknown> | undefined;
      if (!paramMap || typeof paramMap !== 'object') {
        continue;
      }
      const p: Parameter = {
        name: (typeof paramMap['name'] === 'string' ? paramMap['name'] : '') as string,
        in: (typeof paramMap['in'] === 'string' ? paramMap['in'] : 'query') as Parameter['in'],
      };
      if (typeof paramMap['description'] === 'string') {
        p.description = paramMap['description'];
      }
      if (typeof paramMap['required'] === 'boolean') {
        p.required = paramMap['required'];
      }
      if (paramMap['schema'] && typeof paramMap['schema'] === 'object') {
        p.schema = paramMap['schema'] as Record<string, unknown>;
      }
      if (paramMap['example'] !== undefined) {
        p.example = paramMap['example'];
      }
      result.parameters![name] = p;
    }
  }

  // Parse requestBodies
  if (components['requestBodies'] && typeof components['requestBodies'] === 'object') {
    const bodies = components['requestBodies'] as Record<string, unknown>;
    for (const [name, body] of Object.entries(bodies)) {
      const bodyMap = body as Record<string, unknown> | undefined;
      if (!bodyMap || typeof bodyMap !== 'object') {
        continue;
      }
      result.requestBodies![name] = parseRequestBody(bodyMap);
    }
  }

  // Parse headers
  if (components['headers'] && typeof components['headers'] === 'object') {
    result.headers = parseHeaders(components['headers'] as Record<string, unknown>);
  }

  // Parse security schemes
  if (components['securitySchemes'] && typeof components['securitySchemes'] === 'object') {
    const secSchemes = components['securitySchemes'] as Record<string, unknown>;
    for (const [name, scheme] of Object.entries(secSchemes)) {
      const schemeMap = scheme as Record<string, unknown> | undefined;
      if (!schemeMap || typeof schemeMap !== 'object') {
        continue;
      }

      const sec: SecurityScheme = {
        type: '',
      };

      if (typeof schemeMap['type'] === 'string') {
        sec.type = schemeMap['type'];
      }
      if (typeof schemeMap['description'] === 'string') {
        sec.description = schemeMap['description'];
      }
      if (typeof schemeMap['name'] === 'string') {
        sec.name = schemeMap['name'];
      }
      if (typeof schemeMap['in'] === 'string') {
        sec.in = schemeMap['in'];
      }
      if (typeof schemeMap['scheme'] === 'string') {
        sec.scheme = schemeMap['scheme'];
      }
      if (typeof schemeMap['bearerFormat'] === 'string') {
        sec.bearerFormat = schemeMap['bearerFormat'];
      }
      if (typeof schemeMap['openIdConnectUrl'] === 'string') {
        sec.openIdConnectUrl = schemeMap['openIdConnectUrl'];
      }

      result.securitySchemes![name] = sec;
    }
  }

  return result;
}

function parseTags(tags: unknown[]): Tag[] {
  const result: Tag[] = [];

  for (const t of tags) {
    const tagMap = t as Record<string, unknown> | undefined;
    if (!tagMap || typeof tagMap !== 'object') {
      continue;
    }

    const tag: Tag = {
      name: '',
      extensions: {},
    };

    if (typeof tagMap['name'] === 'string') {
      tag.name = tagMap['name'];
    }
    if (typeof tagMap['description'] === 'string') {
      tag.description = tagMap['description'];
    }

    result.push(tag);
  }

  return result;
}

// =============================================================================
// OpenAPI field parsing helpers
// =============================================================================

function parseParameters(params: unknown[]): Parameter[] {
  const result: Parameter[] = [];
  for (const p of params) {
    const paramMap = p as Record<string, unknown> | undefined;
    if (!paramMap || typeof paramMap !== 'object') {
      continue;
    }
    const param: Parameter = {
      name: (typeof paramMap['name'] === 'string' ? paramMap['name'] : '') as string,
      in: (typeof paramMap['in'] === 'string' ? paramMap['in'] : 'query') as Parameter['in'],
    };
    if (typeof paramMap['description'] === 'string') {
      param.description = paramMap['description'];
    }
    if (typeof paramMap['required'] === 'boolean') {
      param.required = paramMap['required'];
    }
    if (paramMap['schema'] && typeof paramMap['schema'] === 'object') {
      param.schema = paramMap['schema'] as Record<string, unknown>;
    }
    if (paramMap['example'] !== undefined) {
      param.example = paramMap['example'];
    }
    result.push(param);
  }
  return result;
}

function parseRequestBody(body: Record<string, unknown>): RequestBody {
  const rb: RequestBody = {
    content: {},
    extensions: {},
  };
  if (typeof body['description'] === 'string') {
    rb.description = body['description'];
  }
  if (typeof body['required'] === 'boolean') {
    rb.required = body['required'];
  }
  if (body['content'] && typeof body['content'] === 'object') {
    rb.content = parseMediaTypes(body['content'] as Record<string, unknown>);
  }
  for (const [key, value] of Object.entries(body)) {
    if (key.startsWith('x-')) {
      rb.extensions![key] = value;
    }
  }
  return rb;
}

function parseMediaTypes(content: Record<string, unknown>): Record<string, MediaType> {
  const result: Record<string, MediaType> = {};
  for (const [mediaType, mt] of Object.entries(content)) {
    const mtMap = mt as Record<string, unknown> | undefined;
    if (!mtMap || typeof mtMap !== 'object') {
      continue;
    }
    const m: MediaType = {};
    if (mtMap['schema'] && typeof mtMap['schema'] === 'object') {
      m.schema = mtMap['schema'] as Record<string, unknown>;
    }
    if (mtMap['example'] !== undefined) {
      m.example = mtMap['example'];
    }
    if (mtMap['examples'] && typeof mtMap['examples'] === 'object') {
      m.examples = {};
      const examples = mtMap['examples'] as Record<string, unknown>;
      for (const [name, ex] of Object.entries(examples)) {
        const exMap = ex as Record<string, unknown> | undefined;
        if (!exMap || typeof exMap !== 'object') {
          continue;
        }
        const e: Example = {};
        if (typeof exMap['summary'] === 'string') {
          e.summary = exMap['summary'];
        }
        if (typeof exMap['description'] === 'string') {
          e.description = exMap['description'];
        }
        if (exMap['value'] !== undefined) {
          e.value = exMap['value'];
        }
        if (typeof exMap['externalValue'] === 'string') {
          e.externalValue = exMap['externalValue'];
        }
        m.examples[name] = e;
      }
    }
    result[mediaType] = m;
  }
  return result;
}

function parseResponses(responses: Record<string, unknown>): Record<string, Response> {
  const result: Record<string, Response> = {};
  for (const [status, resp] of Object.entries(responses)) {
    const respMap = resp as Record<string, unknown> | undefined;
    if (!respMap || typeof respMap !== 'object') {
      continue;
    }
    const r: Response = {
      description: typeof respMap['description'] === 'string' ? respMap['description'] : '',
    };
    if (respMap['content'] && typeof respMap['content'] === 'object') {
      r.content = parseMediaTypes(respMap['content'] as Record<string, unknown>);
    }
    if (respMap['headers'] && typeof respMap['headers'] === 'object') {
      r.headers = parseHeaders(respMap['headers'] as Record<string, unknown>);
    }
    result[status] = r;
  }
  return result;
}

function parseHeaders(headers: Record<string, unknown>): Record<string, Header> {
  const result: Record<string, Header> = {};
  for (const [name, h] of Object.entries(headers)) {
    const hMap = h as Record<string, unknown> | undefined;
    if (!hMap || typeof hMap !== 'object') {
      continue;
    }
    const header: Header = {};
    if (typeof hMap['description'] === 'string') {
      header.description = hMap['description'];
    }
    if (hMap['schema'] && typeof hMap['schema'] === 'object') {
      header.schema = hMap['schema'] as Record<string, unknown>;
    }
    result[name] = header;
  }
  return result;
}

function parseSecurity(security: unknown[]): SecurityRequirement[] {
  const result: SecurityRequirement[] = [];
  for (const s of security) {
    const sMap = s as Record<string, unknown> | undefined;
    if (!sMap || typeof sMap !== 'object') {
      continue;
    }
    const req: SecurityRequirement = {};
    for (const [name, scopes] of Object.entries(sMap)) {
      if (Array.isArray(scopes)) {
        req[name] = scopes.filter((scope): scope is string => typeof scope === 'string');
      } else {
        req[name] = [];
      }
    }
    result.push(req);
  }
  return result;
}

// =============================================================================
// applyOperationPrefixes
// =============================================================================

/**
 * Applies operation ID and tag prefixes to all operations in a PathItem.
 * When collapseServiceTags is true, all operation tags are replaced with
 * just the service name. Otherwise, tags are individually prefixed.
 *
 * Also detects operation ID conflicts and records them in the MergeResult.
 */
export function applyOperationPrefixes(
  item: PathItem,
  opIdPrefix: string,
  tagPrefix: string,
  serviceName: string,
  collapseServiceTags: boolean,
  seenOperationIds: Map<string, string>,
  result: MergeResult,
): PathItem {
  const applyToOp = (op: Operation | undefined): void => {
    if (!op) {
      return;
    }

    // Prefix operation ID
    if (op.operationId) {
      const originalId = op.operationId;

      if (opIdPrefix) {
        op.operationId = `${opIdPrefix}_${op.operationId}`;
      }

      // Check for conflicts
      const existingService = seenOperationIds.get(op.operationId);
      if (existingService) {
        result.conflicts.push({
          type: 'operationId',
          item: originalId,
          services: [existingService, serviceName],
          resolution: 'Prefixed to ' + op.operationId,
          strategy: 'prefix',
        });
      }

      seenOperationIds.set(op.operationId, serviceName);
    }

    // Handle tags: collapse all to service name, or prefix individually
    if (collapseServiceTags) {
      op.tags = [serviceName];
    } else if (tagPrefix) {
      op.tags = prefixTags(op.tags ?? [], tagPrefix);
    }
  };

  applyToOp(item.get);
  applyToOp(item.post);
  applyToOp(item.put);
  applyToOp(item.delete);
  applyToOp(item.patch);
  applyToOp(item.options);
  applyToOp(item.head);
  applyToOp(item.trace);

  return item;
}
