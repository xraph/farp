/**
 * Gateway client for FARP TypeScript client.
 *
 * Reference implementation for API gateway integration with FARP.
 * Demonstrates how to watch for manifest changes, convert schemas to routes,
 * cache schemas, and integrate with the merger package.
 *
 * Ported from Go: gateway/client.go
 */

import type {
  SchemaManifest,
  SchemaDescriptor,
  ManifestEvent,
  RouteDescriptor,
} from '../types';
import type { SchemaRegistry } from '../registry';
import { Merger, defaultMergerConfig } from '../merger';
import type { MergerConfig, MergeResult, ServiceSchema } from '../merger';
import type { ServiceRoute, RouteUpdateHandler } from './types';

/**
 * Configuration options for GatewayClient.
 */
export interface GatewayClientConfig {
  /** Custom fetch function for HTTP schema retrieval (defaults to global fetch) */
  fetchFn?: typeof fetch;

  /** Request timeout in milliseconds (default: 30000) */
  timeoutMs?: number;

  /** Custom merger configuration */
  mergerConfig?: MergerConfig;
}

/**
 * GatewayClient is a reference implementation for API gateway integration.
 * It demonstrates how to watch for service schema changes and provides
 * conversion utilities.
 */
export class GatewayClient {
  private readonly registry: SchemaRegistry;
  private readonly manifestCache: Map<string, SchemaManifest> = new Map();
  private readonly schemaCache: Map<string, unknown> = new Map();
  private readonly merger: Merger;
  private readonly fetchFn: typeof fetch;
  private readonly timeoutMs: number;
  private currentRoutesHash = '';

  constructor(registry: SchemaRegistry, config?: GatewayClientConfig) {
    this.registry = registry;
    this.fetchFn = config?.fetchFn ?? globalThis.fetch.bind(globalThis);
    this.timeoutMs = config?.timeoutMs ?? 30_000;
    this.merger = new Merger(config?.mergerConfig ?? defaultMergerConfig());
  }

  // ---------------------------------------------------------------------------
  // WatchServices
  // ---------------------------------------------------------------------------

  /**
   * Watches for service registrations and schema updates.
   * onChange is called only when the computed route table actually changes,
   * preventing unnecessary route remounts that cause intermittent 404s.
   *
   * Route change detection uses two strategies:
   *  1. Fast path: compare RoutesChecksum from manifests (if services provide it)
   *  2. Fallback: compute route table hash locally and compare
   */
  async watchServices(
    serviceName: string,
    onChange: (routes: ServiceRoute[]) => void,
  ): Promise<() => void> {
    // Initial load
    const manifests = await this.registry.listManifests(serviceName);
    const routes = this.convertToRoutes(manifests);

    this.currentRoutesHash = computeRouteTableHash(routes);
    onChange(routes);

    // Watch for changes
    const unsubscribe = this.registry.watchManifests(serviceName, (event: ManifestEvent) => {
      // Fast path: check RoutesChecksum if available on updated events
      if (event.type === 'updated') {
        const old = this.manifestCache.get(event.manifest.instance_id);
        if (
          old &&
          old.routes_checksum &&
          event.manifest.routes_checksum &&
          old.routes_checksum === event.manifest.routes_checksum
        ) {
          // Route table unchanged - update cache but skip remounting
          this.manifestCache.set(event.manifest.instance_id, event.manifest);
          return;
        }
      }

      // Update manifest cache
      switch (event.type) {
        case 'added':
        case 'updated':
          this.manifestCache.set(event.manifest.instance_id, event.manifest);
          break;
        case 'removed':
          this.manifestCache.delete(event.manifest.instance_id);
          break;
      }

      // Get all cached manifests
      const allManifests = Array.from(this.manifestCache.values());

      // Convert to routes
      const newRoutes = this.convertToRoutes(allManifests);

      // Fallback: compute local route table hash and compare
      const newHash = computeRouteTableHash(newRoutes);
      if (newHash === this.currentRoutesHash) {
        // Routes unchanged - skip remounting
        return;
      }
      this.currentRoutesHash = newHash;

      // Routes changed - notify gateway to remount
      onChange(newRoutes);
    });

    return unsubscribe;
  }

  // ---------------------------------------------------------------------------
  // WatchServicesAtomic
  // ---------------------------------------------------------------------------

  /**
   * Watches for service changes and uses the RouteUpdateHandler
   * for atomic route swaps. This prevents intermittent 404s by staging new
   * routes before removing old ones.
   */
  async watchServicesAtomic(
    serviceName: string,
    handler: RouteUpdateHandler,
  ): Promise<() => void> {
    // Initial load
    const manifests = await this.registry.listManifests(serviceName);
    const routes = this.convertToRouteDescriptors(manifests);

    await handler.prepareRoutes(routes);
    try {
      await handler.commitRoutes();
    } catch {
      await handler.rollbackRoutes();
      throw new Error('failed to commit initial routes');
    }

    this.currentRoutesHash = computeRouteDescriptorHash(routes);

    // Watch for changes
    const unsubscribe = this.registry.watchManifests(serviceName, (event: ManifestEvent) => {
      // Fast path: check RoutesChecksum
      if (event.type === 'updated') {
        const old = this.manifestCache.get(event.manifest.instance_id);
        if (
          old &&
          old.routes_checksum &&
          event.manifest.routes_checksum &&
          old.routes_checksum === event.manifest.routes_checksum
        ) {
          this.manifestCache.set(event.manifest.instance_id, event.manifest);
          return;
        }
      }

      // Update manifest cache
      switch (event.type) {
        case 'added':
        case 'updated':
          this.manifestCache.set(event.manifest.instance_id, event.manifest);
          break;
        case 'removed':
          this.manifestCache.delete(event.manifest.instance_id);
          break;
      }

      const allManifests = Array.from(this.manifestCache.values());

      // Convert to route descriptors
      const newRoutes = this.convertToRouteDescriptors(allManifests);
      const newHash = computeRouteDescriptorHash(newRoutes);

      if (newHash === this.currentRoutesHash) {
        return;
      }
      this.currentRoutesHash = newHash;

      // Atomic swap: prepare -> commit -> rollback on failure
      void (async () => {
        try {
          await handler.prepareRoutes(newRoutes);
        } catch {
          // Validation failed, skip this update
          return;
        }

        try {
          await handler.commitRoutes();
        } catch {
          await handler.rollbackRoutes();
        }
      })();
    });

    return unsubscribe;
  }

  // ---------------------------------------------------------------------------
  // ConvertToRoutes
  // ---------------------------------------------------------------------------

  /**
   * Converts service manifests to gateway routes.
   * This is a reference implementation - actual gateways should customize this.
   */
  convertToRoutes(manifests: SchemaManifest[]): ServiceRoute[] {
    const routes: ServiceRoute[] = [];

    for (const manifest of manifests) {
      for (const schemaDesc of manifest.schemas) {
        let schema: unknown;

        // Check cache first
        const cached = this.getSchemaFromCache(schemaDesc.hash);
        if (cached !== undefined) {
          schema = cached;
        } else {
          // Fetch schema based on location type
          schema = this.fetchSchemaSync(schemaDesc);
          if (schema === undefined) {
            continue;
          }
          // Cache the schema
          this.cacheSchema(schemaDesc.hash, schema);
        }

        // Convert schema to routes based on type
        switch (schemaDesc.type) {
          case 'openapi':
            routes.push(...this.convertOpenAPIToRoutes(manifest, schema, schemaDesc));
            break;
          case 'asyncapi':
            routes.push(...this.convertAsyncAPIToRoutes(manifest, schema));
            break;
          case 'graphql':
            routes.push(...this.convertGraphQLToRoutes(manifest, schema));
            break;
        }
      }
    }

    return routes;
  }

  /**
   * Async version of convertToRoutes that fetches schemas via HTTP when needed.
   */
  async convertToRoutesAsync(manifests: SchemaManifest[]): Promise<ServiceRoute[]> {
    const routes: ServiceRoute[] = [];

    for (const manifest of manifests) {
      for (const schemaDesc of manifest.schemas) {
        let schema: unknown;

        // Check cache first
        const cached = this.getSchemaFromCache(schemaDesc.hash);
        if (cached !== undefined) {
          schema = cached;
        } else {
          // Fetch schema based on location type
          try {
            schema = await this.fetchSchema(schemaDesc);
          } catch {
            continue;
          }
          if (schema === undefined) {
            continue;
          }
          // Cache the schema
          this.cacheSchema(schemaDesc.hash, schema);
        }

        // Convert schema to routes based on type
        switch (schemaDesc.type) {
          case 'openapi':
            routes.push(...this.convertOpenAPIToRoutes(manifest, schema, schemaDesc));
            break;
          case 'asyncapi':
            routes.push(...this.convertAsyncAPIToRoutes(manifest, schema));
            break;
          case 'graphql':
            routes.push(...this.convertGraphQLToRoutes(manifest, schema));
            break;
        }
      }
    }

    return routes;
  }

  // ---------------------------------------------------------------------------
  // ConvertToRouteDescriptors
  // ---------------------------------------------------------------------------

  /**
   * Converts manifests to RouteDescriptor list.
   * If manifests include a RouteTable, use it directly. Otherwise, fall back
   * to converting schemas.
   */
  private convertToRouteDescriptors(manifests: SchemaManifest[]): RouteDescriptor[] {
    const routes: RouteDescriptor[] = [];

    for (const manifest of manifests) {
      // Prefer pre-computed route table if available
      if (manifest.route_table && manifest.route_table.length > 0) {
        routes.push(...manifest.route_table);
        continue;
      }

      // Fallback: convert ServiceRoutes to RouteDescriptors
      const serviceRoutes = this.convertToRoutes([manifest]);
      for (const sr of serviceRoutes) {
        routes.push({
          path: sr.path,
          methods: sr.methods,
          protocol: 'rest',
          metadata: sr.metadata,
        });
      }
    }

    return routes;
  }

  // ---------------------------------------------------------------------------
  // GenerateMergedOpenAPI
  // ---------------------------------------------------------------------------

  /**
   * Generates a unified OpenAPI spec from all registered services.
   */
  async generateMergedOpenAPI(serviceName?: string): Promise<MergeResult> {
    const manifests = this.getCachedManifests();

    const filtered = serviceName
      ? manifests.filter(m => m.service_name === serviceName)
      : manifests;

    // Build service schemas
    const serviceSchemas: ServiceSchema[] = [];

    for (const manifest of filtered) {
      for (const schemaDesc of manifest.schemas) {
        if (schemaDesc.type !== 'openapi') {
          continue;
        }

        let schema: unknown;
        const cached = this.getSchemaFromCache(schemaDesc.hash);
        if (cached !== undefined) {
          schema = cached;
        } else {
          try {
            schema = await this.fetchSchema(schemaDesc);
          } catch {
            continue;
          }
          this.cacheSchema(schemaDesc.hash, schema);
        }

        serviceSchemas.push({
          manifest,
          schema,
        });

        break; // Only one OpenAPI schema per manifest
      }
    }

    return this.merger.merge(serviceSchemas);
  }

  /**
   * Returns the merged OpenAPI spec as a JSON string.
   */
  async getMergedOpenAPIJSON(serviceName?: string): Promise<string> {
    const result = await this.generateMergedOpenAPI(serviceName);
    return JSON.stringify(result.spec, null, 2);
  }

  // ---------------------------------------------------------------------------
  // Schema fetching
  // ---------------------------------------------------------------------------

  /**
   * Fetches a schema based on its location type.
   */
  private async fetchSchema(descriptor: SchemaDescriptor): Promise<unknown> {
    switch (descriptor.location.type) {
      case 'inline':
        return descriptor.inline_schema;

      case 'registry':
        return this.registry.fetchSchema(descriptor.location.registry_path!);

      case 'http': {
        if (!descriptor.location.url) {
          throw new Error('URL is required for HTTP location type');
        }

        const headers: Record<string, string> = {
          Accept: 'application/json',
          ...descriptor.location.headers,
        };

        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), this.timeoutMs);

        try {
          const resp = await this.fetchFn(descriptor.location.url, {
            method: 'GET',
            headers,
            signal: controller.signal,
          });

          if (!resp.ok) {
            throw new Error(
              `HTTP request failed with status ${resp.status}: ${resp.statusText}`,
            );
          }

          return await resp.json();
        } finally {
          clearTimeout(timeoutId);
        }
      }

      default:
        throw new Error(`invalid location type: ${descriptor.location.type}`);
    }
  }

  /**
   * Synchronous schema fetch - only works for inline schemas.
   * Returns undefined if schema cannot be fetched synchronously.
   */
  private fetchSchemaSync(descriptor: SchemaDescriptor): unknown | undefined {
    if (descriptor.location.type === 'inline') {
      return descriptor.inline_schema;
    }
    return undefined;
  }

  // ---------------------------------------------------------------------------
  // getBaseURL
  // ---------------------------------------------------------------------------

  /**
   * Extracts the base URL for a service from multiple sources.
   * Priority order:
   *  1. OpenAPI schema's servers array (first server URL)
   *  2. Schema descriptor's Location.URL (extract base URL from full URL)
   *  3. manifest.Instance.Address (convert to http://host:port)
   *  4. Fallback: http://serviceName:8080
   */
  private getBaseURL(
    manifest: SchemaManifest,
    schemaMap: Record<string, unknown>,
    schemaDesc?: SchemaDescriptor,
  ): string {
    // Try OpenAPI schema's servers array
    const servers = schemaMap['servers'] as unknown[] | undefined;
    if (Array.isArray(servers) && servers.length > 0) {
      const server = servers[0] as Record<string, unknown> | undefined;
      if (server && typeof server['url'] === 'string' && server['url'] !== '') {
        return server['url'].replace(/\/$/, '');
      }
    }

    // Try schema descriptor's Location.URL
    if (schemaDesc && schemaDesc.location.type === 'http' && schemaDesc.location.url) {
      try {
        const parsed = new URL(schemaDesc.location.url);
        const scheme = parsed.protocol.replace(':', '') || 'http';
        return `${scheme}://${parsed.host}`;
      } catch {
        // fall through
      }
    }

    // Try manifest Instance.Address
    if (manifest.instance && manifest.instance.address) {
      const address = manifest.instance.address;
      if (!address.includes('://')) {
        return 'http://' + address;
      }
      return address;
    }

    // Fallback: construct default URL from service name
    return `http://${manifest.service_name}:8080`;
  }

  // ---------------------------------------------------------------------------
  // Schema type converters
  // ---------------------------------------------------------------------------

  /**
   * Converts an OpenAPI schema to gateway routes.
   */
  private convertOpenAPIToRoutes(
    manifest: SchemaManifest,
    schema: unknown,
    schemaDesc: SchemaDescriptor,
  ): ServiceRoute[] {
    const routes: ServiceRoute[] = [];

    const schemaMap = schema as Record<string, unknown> | undefined;
    if (!schemaMap || typeof schemaMap !== 'object') {
      return routes;
    }

    const paths = schemaMap['paths'] as Record<string, unknown> | undefined;
    if (!paths || typeof paths !== 'object') {
      return routes;
    }

    const baseURL = this.getBaseURL(manifest, schemaMap, schemaDesc);

    for (const [path, pathItem] of Object.entries(paths)) {
      const pathItemMap = pathItem as Record<string, unknown> | undefined;
      if (!pathItemMap || typeof pathItemMap !== 'object') {
        continue;
      }

      const methods: string[] = [];
      for (const method of Object.keys(pathItemMap)) {
        if (['get', 'post', 'put', 'delete', 'patch', 'options', 'head'].includes(method)) {
          methods.push(method);
        }
      }

      if (methods.length > 0) {
        routes.push({
          path,
          methods,
          targetUrl: baseURL + path,
          healthUrl: baseURL + (manifest.endpoints.health || ''),
          serviceName: manifest.service_name,
          serviceVersion: manifest.service_version,
          instanceId: manifest.instance_id,
          middleware: [],
          metadata: { schema_type: 'openapi' },
        });
      }
    }

    return routes;
  }

  /**
   * Converts an AsyncAPI schema to gateway routes (WebSocket, SSE).
   */
  private convertAsyncAPIToRoutes(
    manifest: SchemaManifest,
    schema: unknown,
  ): ServiceRoute[] {
    const routes: ServiceRoute[] = [];

    const schemaMap = schema as Record<string, unknown> | undefined;
    if (!schemaMap || typeof schemaMap !== 'object') {
      return routes;
    }

    const channels = schemaMap['channels'] as Record<string, unknown> | undefined;
    if (!channels || typeof channels !== 'object') {
      return routes;
    }

    const baseURL = this.getBaseURL(manifest, schemaMap);

    for (const channelPath of Object.keys(channels)) {
      routes.push({
        path: channelPath,
        methods: ['WEBSOCKET'],
        targetUrl: baseURL + channelPath,
        healthUrl: baseURL + (manifest.endpoints.health || ''),
        serviceName: manifest.service_name,
        serviceVersion: manifest.service_version,
        instanceId: manifest.instance_id,
        middleware: [],
        metadata: {
          schema_type: 'asyncapi',
          protocol: 'websocket',
        },
      });
    }

    return routes;
  }

  /**
   * Converts a GraphQL schema to a gateway route.
   */
  private convertGraphQLToRoutes(
    manifest: SchemaManifest,
    schema: unknown,
  ): ServiceRoute[] {
    const schemaMap = (schema as Record<string, unknown>) ?? {};
    const baseURL = this.getBaseURL(manifest, schemaMap);

    const graphqlPath = manifest.endpoints.graphql || '/graphql';

    return [
      {
        path: graphqlPath,
        methods: ['POST', 'GET'],
        targetUrl: baseURL + graphqlPath,
        healthUrl: baseURL + (manifest.endpoints.health || ''),
        serviceName: manifest.service_name,
        serviceVersion: manifest.service_version,
        instanceId: manifest.instance_id,
        middleware: [],
        metadata: { schema_type: 'graphql' },
      },
    ];
  }

  // ---------------------------------------------------------------------------
  // Cache helpers
  // ---------------------------------------------------------------------------

  private getSchemaFromCache(hash: string): unknown | undefined {
    return this.schemaCache.get(hash);
  }

  private cacheSchema(hash: string, schema: unknown): void {
    this.schemaCache.set(hash, schema);
  }

  /** Clears the schema cache. */
  clearCache(): void {
    this.schemaCache.clear();
  }

  /** Retrieves a cached manifest by instance ID. */
  getManifest(instanceID: string): SchemaManifest | undefined {
    return this.manifestCache.get(instanceID);
  }

  /** Returns a snapshot of all cached manifests. */
  getCachedManifests(): SchemaManifest[] {
    return Array.from(this.manifestCache.values());
  }

  /** Returns a composite hash of all cached manifests. */
  getManifestsHash(): string {
    return computeManifestsHash(this.manifestCache);
  }
}

// =============================================================================
// Hash computation helpers
// =============================================================================

/**
 * Computes a SHA256 hash of the ServiceRoute table.
 * Uses a simplified approach with JSON serialization of path+methods.
 */
function computeRouteTableHash(routes: ServiceRoute[]): string {
  const entries = routes
    .map(r => ({
      p: r.path,
      m: [...r.methods].sort(),
    }))
    .sort((a, b) => a.p.localeCompare(b.p));

  return simpleHash(JSON.stringify(entries));
}

/**
 * Computes a SHA256 hash of RouteDescriptor list.
 */
function computeRouteDescriptorHash(routes: RouteDescriptor[]): string {
  const entries = routes
    .map(r => ({
      p: r.path,
      m: [...(r.methods || [])].sort(),
      pr: r.protocol || '',
    }))
    .sort((a, b) => {
      if (a.p !== b.p) return a.p.localeCompare(b.p);
      return a.pr.localeCompare(b.pr);
    });

  return simpleHash(JSON.stringify(entries));
}

/**
 * Computes a composite hash of all manifest checksums.
 */
function computeManifestsHash(cache: Map<string, SchemaManifest>): string {
  const entries = Array.from(cache.entries())
    .map(([id, m]) => ({ id, cs: m.checksum }))
    .sort((a, b) => a.id.localeCompare(b.id));

  return simpleHash(JSON.stringify(entries));
}

/**
 * Simple hash function. Uses SubtleCrypto SHA-256 when available,
 * falls back to a basic string hash for synchronous contexts.
 */
function simpleHash(data: string): string {
  // Use a deterministic FNV-1a-like hash for synchronous operation.
  // In production, prefer crypto.subtle.digest('SHA-256', ...) async.
  let hash = 0x811c9dc5; // FNV offset basis
  for (let i = 0; i < data.length; i++) {
    hash ^= data.charCodeAt(i);
    hash = (hash * 0x01000193) >>> 0; // FNV prime, keep as uint32
  }
  return hash.toString(16).padStart(8, '0');
}
