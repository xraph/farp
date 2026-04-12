/**
 * PushHandler is the gateway-side HTTP handler for push-based service registration.
 * Services POST their manifests directly to this handler.
 *
 * It also implements ServiceDiscovery so GatewayNode can use it like any other backend.
 *
 * Endpoints:
 *   POST   /register              -- service pushes instance + manifest
 *   PUT    /heartbeat/{id}        -- service sends heartbeat
 *   DELETE /deregister/{id}       -- service deregisters
 *   GET    /services              -- list all registered services
 *   GET    /services/{name}       -- get instances of a service
 *
 * Ported from Go: discovery/push_handler.go
 */

import type { InstanceStatus, SchemaManifest } from '../types';
import { InstanceNotFoundError } from '../errors';
import type { ServiceDiscovery } from './interface';
import type {
  ServiceInstance,
  DiscoveryEvent,
  DiscoveryEventHandler,
  PushRegistration,
  PushHeartbeat,
} from './types';

interface PushEntry {
  instance: ServiceInstance;
  manifest?: SchemaManifest;
  lastSeen: number; // Date.now() ms
}

/**
 * PushHandler is the gateway-side handler for push-based service registration.
 * It receives push registrations via handleRequest() using Web standard Request/Response,
 * stores instances in a Map, and implements ServiceDiscovery.
 */
export class PushHandler implements ServiceDiscovery {
  private instances: Map<string, PushEntry> = new Map();
  private watchers: DiscoveryEventHandler[] = [];
  private heartbeatTimeout: number; // ms
  private reaperInterval: ReturnType<typeof setInterval> | null = null;
  private watchAbortControllers: AbortController[] = [];

  /**
   * Creates a new push registration handler.
   * @param heartbeatTimeoutMs - How long (ms) before an instance with no heartbeat is evicted. Default: 30000.
   */
  constructor(heartbeatTimeoutMs: number = 30_000) {
    this.heartbeatTimeout = heartbeatTimeoutMs;

    // Start reaper interval (runs at half the timeout interval)
    this.reaperInterval = setInterval(
      () => this.evictStale(),
      this.heartbeatTimeout / 2,
    );
  }

  /**
   * handleRequest routes push API requests using Web standard Request/Response.
   */
  async handleRequest(request: Request): Promise<Response> {
    const url = new URL(request.url, 'http://localhost');
    const path = url.pathname;
    const method = request.method;

    if (method === 'POST' && path.endsWith('/register')) {
      return this.handleRegister(request);
    }

    if (method === 'PUT' && path.includes('/heartbeat/')) {
      const id = path.substring(path.lastIndexOf('/') + 1);
      return this.handleHeartbeat(request, id);
    }

    if (method === 'DELETE' && path.includes('/deregister/')) {
      const id = path.substring(path.lastIndexOf('/') + 1);
      return this.handleDeregister(id);
    }

    if (method === 'GET' && path.endsWith('/services')) {
      return this.handleListServices();
    }

    if (method === 'GET' && path.includes('/services/')) {
      const name = path.substring(path.lastIndexOf('/') + 1);
      return this.handleGetService(name);
    }

    return new Response('Not Found', { status: 404 });
  }

  private async handleRegister(request: Request): Promise<Response> {
    let reg: PushRegistration;
    try {
      reg = await request.json() as PushRegistration;
    } catch (err) {
      return new Response(`invalid JSON: ${err}`, { status: 400 });
    }

    if (!reg.instance?.id) {
      return new Response('instance ID is required', { status: 400 });
    }

    const existing = this.instances.get(reg.instance.id);
    const eventType: string = existing ? 'updated' : 'added';

    const now = new Date().toISOString();
    reg.instance.lastHealthCheck = now;
    if (!reg.instance.registeredAt) {
      reg.instance.registeredAt = now;
    }

    this.instances.set(reg.instance.id, {
      instance: reg.instance,
      manifest: reg.manifest,
      lastSeen: Date.now(),
    });

    // Copy watchers for notification
    const watchers = [...this.watchers];

    // Notify watchers
    const event: DiscoveryEvent = {
      type: eventType as DiscoveryEvent['type'],
      instance: reg.instance,
      manifest: reg.manifest,
      timestamp: now,
    };

    for (const watcher of watchers) {
      watcher(event);
    }

    // Build ack with checksum (section 17.4.1 reconciliation)
    let routesChecksum = '';
    let schemasApplied = 0;

    const entry = this.instances.get(reg.instance.id);
    if (entry?.manifest) {
      routesChecksum = entry.manifest.routes_checksum ?? '';
      schemasApplied = entry.manifest.schemas?.length ?? 0;
    }

    return new Response(
      JSON.stringify({
        status: 'registered',
        routes_checksum: routesChecksum,
        schemas_applied: schemasApplied,
      }),
      {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      },
    );
  }

  private async handleHeartbeat(
    request: Request,
    instanceId: string,
  ): Promise<Response> {
    let hb: PushHeartbeat;
    try {
      hb = await request.json() as PushHeartbeat;
    } catch {
      return new Response('invalid JSON', { status: 400 });
    }

    const entry = this.instances.get(instanceId);
    if (!entry) {
      return new Response('instance not found', { status: 404 });
    }

    entry.lastSeen = Date.now();
    entry.instance.status = hb.status;
    entry.instance.lastHealthCheck = new Date().toISOString();

    // Respond with gateway state for reconciliation (section 17.4.1)
    let routesChecksum = '';
    let schemasApplied = 0;

    const current = this.instances.get(instanceId);
    if (current?.manifest) {
      routesChecksum = current.manifest.routes_checksum ?? '';
      schemasApplied = current.manifest.schemas?.length ?? 0;
    }

    return new Response(
      JSON.stringify({
        status: 'ok',
        routes_checksum: routesChecksum,
        schemas_applied: schemasApplied,
      }),
      {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      },
    );
  }

  private handleDeregister(instanceId: string): Response {
    const entry = this.instances.get(instanceId);
    if (!entry) {
      return new Response('instance not found', { status: 404 });
    }

    this.instances.delete(instanceId);

    // Copy watchers for notification
    const watchers = [...this.watchers];

    // Notify watchers
    const event: DiscoveryEvent = {
      type: 'removed',
      instance: entry.instance,
      timestamp: new Date().toISOString(),
    };

    for (const watcher of watchers) {
      watcher(event);
    }

    return new Response(null, { status: 200 });
  }

  private handleListServices(): Response {
    const instances: ServiceInstance[] = [];
    for (const entry of this.instances.values()) {
      instances.push(entry.instance);
    }

    return new Response(JSON.stringify(instances), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  private handleGetService(serviceName: string): Response {
    const instances: ServiceInstance[] = [];
    for (const entry of this.instances.values()) {
      if (entry.instance.serviceName === serviceName) {
        instances.push(entry.instance);
      }
    }

    return new Response(JSON.stringify(instances), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  /**
   * Evict instances that have missed heartbeats.
   */
  private evictStale(): void {
    const now = Date.now();
    const evicted: PushEntry[] = [];

    for (const [id, entry] of this.instances.entries()) {
      if (now - entry.lastSeen > this.heartbeatTimeout) {
        evicted.push(entry);
        this.instances.delete(id);
      }
    }

    // Copy watchers for notification
    const watchers = [...this.watchers];

    // Notify watchers about evicted instances
    for (const entry of evicted) {
      const event: DiscoveryEvent = {
        type: 'removed',
        instance: entry.instance,
        timestamp: new Date().toISOString(),
      };

      for (const watcher of watchers) {
        watcher(event);
      }
    }
  }

  // =========================================================================
  // ServiceDiscovery interface implementation
  // =========================================================================

  /**
   * Discover returns all known instances, optionally filtered by service name.
   */
  async discover(serviceName?: string): Promise<ServiceInstance[]> {
    const instances: ServiceInstance[] = [];
    for (const entry of this.instances.values()) {
      if (!serviceName || entry.instance.serviceName === serviceName) {
        instances.push(entry.instance);
      }
    }
    return instances;
  }

  /**
   * Watch registers a watcher for discovery events.
   * Blocks until the returned promise is resolved via close().
   */
  async watch(
    _serviceName: string | undefined,
    handler: DiscoveryEventHandler,
  ): Promise<void> {
    this.watchers.push(handler);

    const ac = new AbortController();
    this.watchAbortControllers.push(ac);

    return new Promise<void>((resolve) => {
      ac.signal.addEventListener('abort', () => resolve());
    });
  }

  /**
   * Register adds an instance (same as receiving a push via HTTP).
   */
  async register(instance: ServiceInstance): Promise<void> {
    this.instances.set(instance.id, {
      instance,
      lastSeen: Date.now(),
    });
  }

  /**
   * Deregister removes an instance.
   */
  async deregister(instanceId: string): Promise<void> {
    this.instances.delete(instanceId);
  }

  /**
   * ReportHealth updates the health status of an instance.
   */
  async reportHealth(
    instanceId: string,
    status: InstanceStatus,
  ): Promise<void> {
    const entry = this.instances.get(instanceId);
    if (!entry) {
      throw new InstanceNotFoundError(`instance ${instanceId} not found`);
    }

    entry.instance.status = status;
    entry.lastSeen = Date.now();
  }

  /**
   * Close stops the reaper interval and aborts any pending watch promises.
   */
  close(): void {
    if (this.reaperInterval !== null) {
      clearInterval(this.reaperInterval);
      this.reaperInterval = null;
    }

    for (const ac of this.watchAbortControllers) {
      ac.abort();
    }
    this.watchAbortControllers = [];
  }

  /**
   * Health always returns successfully (push handler is always "healthy").
   */
  async health(): Promise<void> {
    // no-op -- push handler is always healthy
  }
}
