/**
 * ServiceNode manages the full FARP lifecycle for a service:
 * schema generation, manifest building, HTTP endpoint serving,
 * discovery registration, health reporting, and graceful shutdown.
 *
 * Ported from Go: discovery/node.go and discovery/options.go
 */

import type {
  InstanceStatus,
  SchemaManifest,
  SchemaType,
  SchemaEndpoints,
  MountStrategy,
  PathRule,
  ServiceHints,
  SchemaDescriptor,
} from '../types';
import type { SchemaProvider } from '../provider';
import {
  RegistrationFailedError,
  DeregistrationFailedError,
} from '../errors';
import { PROTOCOL_VERSION } from '../version';
import { calculateSchemaChecksum } from '../checksum';
import { createManifest } from '../manifest';
import type { ServiceDiscovery } from './interface';
import type { ServiceInstance } from './types';
import { PushDiscovery } from './push';
import { FARPHandler } from './handler';

/**
 * Type for the fetch function, allowing injection for testing or custom runtimes.
 */
type FetchFn = typeof globalThis.fetch;

/**
 * ServiceNodeConfig configures a ServiceNode.
 */
export interface ServiceNodeConfig {
  /** Required: Service name */
  serviceName: string;

  /** Service version */
  serviceVersion?: string;

  /** Instance ID (auto-generated if empty) */
  instanceId?: string;

  /** host:port this service listens on */
  address: string;

  /**
   * Discovery mode, Option A: Registry-based -- register in external backend.
   * Provide a ServiceDiscovery implementation.
   */
  discovery?: ServiceDiscovery;

  /**
   * Discovery mode, Option B: Push-based -- push manifest directly to gateway.
   * e.g. "http://gateway:9090/_farp/v1"
   */
  gatewayURL?: string;

  /** Optional: Schema providers for auto-generating schemas */
  providers?: SchemaProvider[];

  /** Health interval in ms. Default: 10000 */
  healthInterval?: number;

  /** TTL in ms. Default: 30000 */
  ttl?: number;

  /** Max retries for re-registration. Default: 5 */
  maxRetries?: number;

  /** Retry backoff in ms. Default: 2000 */
  retryBackoff?: number;

  /** Tags for service instance filtering/labeling */
  tags?: string[];

  /**
   * Metadata is additional key-value metadata to include on the registered
   * ServiceInstance. Merged with auto-generated FARP metadata keys;
   * user-provided keys take precedence.
   */
  metadata?: Record<string, string>;

  /** Endpoints configures the service's introspection endpoints. */
  endpoints?: Partial<SchemaEndpoints>;

  /** Routing configuration (flows into manifest) */
  mountStrategy?: MountStrategy;
  basePath?: string;
  pathRules?: PathRule[];

  /**
   * Routes provides route information for OpenAPI schema generation.
   * Can be any type that the configured schema provider understands.
   */
  routes?: unknown;

  /** Service hints (flows into manifest) */
  hints?: ServiceHints;

  /** Custom fetch function for push mode (optional) */
  fetchFn?: FetchFn;
}

/**
 * Apply defaults to ServiceNodeConfig.
 */
function applyDefaults(config: ServiceNodeConfig): Required<
  Pick<ServiceNodeConfig, 'healthInterval' | 'ttl' | 'maxRetries' | 'retryBackoff' | 'mountStrategy'>
> & ServiceNodeConfig {
  return {
    ...config,
    healthInterval: config.healthInterval ?? 10_000,
    ttl: config.ttl ?? 30_000,
    maxRetries: config.maxRetries ?? 5,
    retryBackoff: config.retryBackoff ?? 2_000,
    mountStrategy: config.mountStrategy ?? 'service',
  };
}

/**
 * Generate a random instance ID.
 */
function generateInstanceId(): string {
  const bytes = new Uint8Array(8);
  if (typeof globalThis.crypto !== 'undefined' && globalThis.crypto.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes);
  } else {
    // Fallback for environments without crypto
    for (let i = 0; i < bytes.length; i++) {
      bytes[i] = Math.floor(Math.random() * 256);
    }
  }
  const hex = Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
  return `i-${hex}`;
}

/**
 * Simple Application adapter wrapping ServiceNodeConfig for SchemaProvider.Generate().
 */
class ServiceApp {
  constructor(private config: ServiceNodeConfig) {}
  name(): string {
    return this.config.serviceName;
  }
  version(): string {
    return this.config.serviceVersion ?? '';
  }
  routes(): unknown {
    return this.config.routes;
  }
}

/**
 * ServiceNode manages the full FARP lifecycle for a service.
 */
export class ServiceNode {
  private config: ReturnType<typeof applyDefaults>;
  private _manifest: SchemaManifest;
  private _handler: FARPHandler;
  private schemas: Map<string, unknown> = new Map();
  private healthTimer: ReturnType<typeof setInterval> | null = null;
  private stopped: boolean = false;
  private discovery: ServiceDiscovery | null = null;

  constructor(config: ServiceNodeConfig) {
    const cfg = applyDefaults(config);

    if (!cfg.serviceName) {
      throw new Error('service name is required');
    }
    if (!cfg.address) {
      throw new Error('address is required');
    }
    if (!cfg.discovery && !cfg.gatewayURL) {
      throw new Error('either discovery or gatewayURL must be set');
    }
    if (!cfg.instanceId) {
      cfg.instanceId = generateInstanceId();
    }

    this.config = cfg;

    // Build manifest
    const manifest = createManifest(
      cfg.serviceName,
      cfg.serviceVersion ?? '',
      cfg.instanceId,
    );

    manifest.routing = {
      ...manifest.routing,
      strategy: cfg.mountStrategy,
      base_path: cfg.basePath ?? '',
      path_rules: cfg.pathRules ?? [],
    };

    // Wire endpoints from config, falling back to FARP defaults
    manifest.endpoints = {
      ...manifest.endpoints,
      health: cfg.endpoints?.health ?? '/_farp/health',
    };
    if (cfg.endpoints?.openapi) {
      manifest.endpoints.openapi = cfg.endpoints.openapi;
    }
    if (cfg.endpoints?.asyncapi) {
      manifest.endpoints.asyncapi = cfg.endpoints.asyncapi;
    }
    if (cfg.endpoints?.graphql) {
      manifest.endpoints.graphql = cfg.endpoints.graphql;
    }
    if (cfg.endpoints?.metrics) {
      manifest.endpoints.metrics = cfg.endpoints.metrics;
    }
    if (cfg.endpoints?.grpc_reflection !== undefined) {
      manifest.endpoints.grpc_reflection = cfg.endpoints.grpc_reflection;
    }

    if (cfg.hints) {
      manifest.hints = cfg.hints;
    }

    manifest.instance = {
      address: cfg.address,
      status: 'starting' as InstanceStatus,
      started_at: Math.floor(Date.now() / 1000),
    };

    this._manifest = manifest;

    const schemaMap = new Map<string, unknown>();
    this._handler = new FARPHandler(manifest, schemaMap);
    this.schemas = schemaMap;
  }

  /**
   * Start registers the service and begins the health reporting loop.
   * It generates schemas from providers (if configured), builds the manifest,
   * registers in the discovery backend (or pushes to gateway), and starts
   * a background timer for health reporting and TTL renewal.
   */
  async start(): Promise<void> {
    // Generate schemas from providers
    await this.generateSchemas();

    // Update checksums
    this._manifest.checksum = await calculateSchemaChecksum(this._manifest.schemas ?? []);
    this.updateRoutesChecksum();

    // Mark as healthy
    if (this._manifest.instance) {
      this._manifest.instance.status = 'healthy';
    }
    this._handler.setHealth('healthy');
    this._handler.updateManifest(this._manifest);

    // Register in discovery backend
    const instance = this.buildInstance();

    let discovery = this.config.discovery ?? null;
    if (!discovery && this.config.gatewayURL) {
      discovery = new PushDiscovery(this.config.gatewayURL, this.config.fetchFn);
    }
    this.discovery = discovery;

    if (discovery) {
      try {
        await discovery.register(instance);
      } catch (err) {
        throw new RegistrationFailedError(
          `registration failed: ${err}`,
        );
      }
    }

    // Start background health loop
    this.stopped = false;
    this.healthTimer = setInterval(
      () => this.healthLoop(),
      this.config.healthInterval,
    );
  }

  /**
   * Stop gracefully deregisters the service and stops the health loop.
   */
  async stop(): Promise<void> {
    // Cancel health loop
    this.stopped = true;
    if (this.healthTimer !== null) {
      clearInterval(this.healthTimer);
      this.healthTimer = null;
    }

    // Mark as stopping
    this._handler.setHealth('stopping');

    // Deregister from discovery
    let discovery = this.config.discovery ?? null;
    if (!discovery && this.config.gatewayURL) {
      discovery = new PushDiscovery(this.config.gatewayURL, this.config.fetchFn);
    }

    if (discovery && this.config.instanceId) {
      try {
        await discovery.deregister(this.config.instanceId);
      } catch (err) {
        throw new DeregistrationFailedError(
          `deregistration failed: ${err}`,
        );
      }
    }
  }

  /**
   * Returns the current SchemaManifest.
   */
  manifest(): SchemaManifest {
    return this._manifest;
  }

  /**
   * Returns the FARPHandler instance for mounting on an HTTP server.
   */
  handler(): FARPHandler {
    return this._handler;
  }

  // =========================================================================
  // Internal
  // =========================================================================

  private async generateSchemas(): Promise<void> {
    const providers = this.config.providers ?? [];
    const app = new ServiceApp(this.config);

    for (const provider of providers) {
      const schema = await provider.generate(app);

      const hash = await calculateSchemaChecksum(schema);

      this.schemas.set(String(provider.type()), schema);
      this._handler.updateSchema(provider.type(), schema);

      // Update or add schema descriptor in manifest
      let found = false;
      const schemas = this._manifest.schemas ?? [];

      for (let i = 0; i < schemas.length; i++) {
        if (schemas[i].type === provider.type()) {
          schemas[i].hash = hash;
          found = true;
          break;
        }
      }

      if (!found) {
        const descriptor: SchemaDescriptor = {
          type: provider.type() as SchemaType,
          spec_version: provider.specVersion(),
          location: {
            type: 'http',
            url: `http://${this.config.address}/_farp/schemas/${provider.type()}`,
          },
          content_type: provider.contentType(),
          hash,
          size: 0,
        };

        if (!this._manifest.schemas) {
          this._manifest.schemas = [];
        }
        this._manifest.schemas.push(descriptor);

        if (!this._manifest.capabilities) {
          this._manifest.capabilities = [];
        }
        if (!this._manifest.capabilities.includes(String(provider.type()))) {
          this._manifest.capabilities.push(String(provider.type()));
        }

        // Auto-populate manifest endpoint paths from providers
        switch (provider.type()) {
          case 'openapi':
            if (!this._manifest.endpoints.openapi) {
              this._manifest.endpoints.openapi = provider.endpoint();
            }
            break;
          case 'asyncapi':
            if (!this._manifest.endpoints.asyncapi) {
              this._manifest.endpoints.asyncapi = provider.endpoint();
            }
            break;
          case 'graphql':
            if (!this._manifest.endpoints.graphql) {
              this._manifest.endpoints.graphql = provider.endpoint();
            }
            break;
        }
      }
    }
  }

  private buildInstance(): ServiceInstance {
    const baseURL = `http://${this.config.address}`;

    // Build metadata from the manifest
    const metadata: Record<string, string> = {
      'farp.enabled': 'true',
      'farp.version': PROTOCOL_VERSION,
      'farp.manifest': `${baseURL}/_farp/manifest`,
    };

    const eps = this._manifest.endpoints;
    if (eps.health) {
      metadata['farp.health'] = eps.health;
      metadata['farp.health.url'] = `${baseURL}${eps.health}`;
    }
    if (eps.openapi) {
      metadata['farp.openapi'] = `${baseURL}${eps.openapi}`;
      metadata['farp.openapi.path'] = eps.openapi;
    }
    if (eps.asyncapi) {
      metadata['farp.asyncapi'] = `${baseURL}${eps.asyncapi}`;
      metadata['farp.asyncapi.path'] = eps.asyncapi;
    }
    if (eps.graphql) {
      metadata['farp.graphql'] = `${baseURL}${eps.graphql}`;
      metadata['farp.graphql.path'] = eps.graphql;
    }
    if (eps.grpc_reflection) {
      metadata['farp.grpc.reflection'] = 'true';
    }

    // Advertise capabilities from the manifest
    if (this._manifest.capabilities && this._manifest.capabilities.length > 0) {
      metadata['farp.capabilities'] = this._manifest.capabilities.join(',');
    }

    // Merge user-provided metadata last so it can override auto-generated keys
    if (this.config.metadata) {
      for (const [k, v] of Object.entries(this.config.metadata)) {
        metadata[k] = v;
      }
    }

    const now = new Date().toISOString();
    return {
      id: this.config.instanceId!,
      serviceName: this.config.serviceName,
      version: this.config.serviceVersion,
      address: this.config.address,
      status: 'healthy',
      tags: this.config.tags,
      metadata,
      registeredAt: now,
      lastHealthCheck: now,
    };
  }

  private healthLoop(): void {
    if (this.stopped || !this.discovery) {
      return;
    }

    // In push mode, use checksum reconciliation (section 17.4.1)
    if (this.discovery instanceof PushDiscovery) {
      this.pushHealthCheck(this.discovery);
    } else {
      this.discovery
        .reportHealth(this.config.instanceId!, 'healthy')
        .catch(() => {
          this.retryRegister(this.discovery!);
        });
    }
  }

  /**
   * pushHealthCheck sends a heartbeat with routes_checksum and handles
   * reconciliation if the gateway's state doesn't match (section 17.4.1).
   *
   * Gateway restart recovery: if the heartbeat returns an error (e.g. 404
   * because the gateway lost all state), we re-register with the full
   * manifest so the gateway can rebuild its route table immediately.
   */
  private pushHealthCheck(pushDisc: PushDiscovery): void {
    const expectedChecksum = this._manifest.routes_checksum ?? '';

    pushDisc
      .reportHealthWithChecksum(
        this.config.instanceId!,
        'healthy',
        expectedChecksum,
      )
      .then((resp) => {
        // Reconciliation: if gateway checksum doesn't match, re-register WITH
        // manifest so the gateway can apply schemas without fetching.
        if (resp.routes_checksum !== expectedChecksum) {
          this.pushReRegisterWithManifest(pushDisc);
        }
      })
      .catch(() => {
        // Gateway unreachable or returned 404 (service unknown after restart).
        // Re-register with full manifest so the gateway can rebuild immediately.
        this.pushReRegisterWithManifest(pushDisc);
      });
  }

  /**
   * pushReRegisterWithManifest re-registers with the gateway, sending the
   * full manifest inline. Handles both gateway restarts (404) and
   * checksum mismatches without requiring the gateway to fetch back.
   */
  private pushReRegisterWithManifest(pushDisc: PushDiscovery): void {
    const manifest = this._manifest;
    const instance = this.buildInstance();
    const maxRetries = this.config.maxRetries;
    const retryBackoff = this.config.retryBackoff;

    const attempt = (n: number): void => {
      if (n >= maxRetries || this.stopped) {
        return;
      }

      if (n > 0) {
        setTimeout(() => {
          pushDisc
            .registerWithManifest(instance, manifest)
            .then(() => {
              // success
            })
            .catch(() => {
              attempt(n + 1);
            });
        }, retryBackoff * n);
      } else {
        pushDisc
          .registerWithManifest(instance, manifest)
          .then(() => {
            // success
          })
          .catch(() => {
            attempt(n + 1);
          });
      }
    };

    attempt(0);
  }

  /**
   * retryRegister attempts re-registration with exponential backoff.
   */
  private retryRegister(discovery: ServiceDiscovery): void {
    const maxRetries = this.config.maxRetries;
    const retryBackoff = this.config.retryBackoff;

    const attempt = (n: number): void => {
      if (n >= maxRetries || this.stopped) {
        return;
      }

      setTimeout(() => {
        const instance = this.buildInstance();
        discovery.register(instance).catch(() => {
          attempt(n + 1);
        });
      }, retryBackoff * (n + 1));
    };

    attempt(0);
  }

  /**
   * Update routes checksum on the manifest.
   */
  private updateRoutesChecksum(): void {
    // Build a deterministic representation of the route table for checksumming
    const routeData = {
      strategy: this._manifest.routing?.strategy,
      base_path: this._manifest.routing?.base_path,
      route_table: this._manifest.route_table,
      path_rules: this._manifest.routing?.path_rules,
    };

    // Use a synchronous hash approach for the routes checksum
    const routeString = JSON.stringify(routeData);
    // Simple hash -- in production, the manifest module provides this
    let hash = 0;
    for (let i = 0; i < routeString.length; i++) {
      const char = routeString.charCodeAt(i);
      hash = ((hash << 5) - hash + char) | 0;
    }
    this._manifest.routes_checksum = Math.abs(hash).toString(16).padStart(8, '0');
  }
}
