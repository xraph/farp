/**
 * PushDiscovery implements ServiceDiscovery by pushing manifests directly
 * to a gateway via HTTP. No external service registry needed.
 *
 * This is the service-side component of push-based discovery.
 * The gateway-side component is PushHandler.
 *
 * Ported from Go: discovery/push.go
 */

import type { InstanceStatus, SchemaManifest } from '../types';
import {
  RegistrationFailedError,
  DeregistrationFailedError,
  HealthCheckFailedError,
  DiscoveryUnavailableError,
} from '../errors';
import type { ServiceDiscovery } from './interface';
import type {
  ServiceInstance,
  DiscoveryEventHandler,
  PushRegistration,
  PushHeartbeat,
  PushRegistrationResponse,
  PushHeartbeatResponse,
} from './types';

/**
 * Type for the fetch function, allowing injection for testing or custom runtimes.
 */
type FetchFn = typeof globalThis.fetch;

/**
 * PushDiscovery implements ServiceDiscovery by pushing manifests directly
 * to a gateway via HTTP. No external service registry needed.
 */
export class PushDiscovery implements ServiceDiscovery {
  private readonly gatewayURL: string;
  private readonly fetchFn: FetchFn;
  private abortController: AbortController | null = null;

  /**
   * Creates a new push-based discovery client.
   * @param gatewayURL - Base URL of the gateway's FARP push endpoint (e.g. "http://gateway:9090/_farp/v1")
   * @param fetchFn - Optional custom fetch function (defaults to globalThis.fetch)
   */
  constructor(gatewayURL: string, fetchFn?: FetchFn) {
    this.gatewayURL = gatewayURL.replace(/\/+$/, '');
    this.fetchFn = fetchFn ?? globalThis.fetch.bind(globalThis);
  }

  /**
   * Register pushes the service instance to the gateway.
   */
  async register(instance: ServiceInstance): Promise<void> {
    await this.doRegister({ instance });
  }

  /**
   * RegisterWithManifest pushes both the instance and manifest to the gateway.
   * Use this when the gateway needs the manifest directly (e.g. after a
   * heartbeat reconciliation mismatch -- see section 17.4.1).
   */
  async registerWithManifest(
    instance: ServiceInstance,
    manifest: SchemaManifest,
  ): Promise<PushRegistrationResponse> {
    return this.doRegister({ instance, manifest });
  }

  private async doRegister(
    payload: PushRegistration,
  ): Promise<PushRegistrationResponse> {
    let body: string;
    try {
      body = JSON.stringify(payload);
    } catch (err) {
      throw new RegistrationFailedError(
        `failed to serialize registration: ${err}`,
      );
    }

    let resp: Response;
    try {
      resp = await this.fetchFn(`${this.gatewayURL}/register`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body,
      });
    } catch (err) {
      throw new RegistrationFailedError(
        `fetch failed: ${err}`,
      );
    }

    if (resp.status !== 200 && resp.status !== 201) {
      throw new RegistrationFailedError(
        `HTTP ${resp.status}`,
      );
    }

    try {
      const ack: PushRegistrationResponse = await resp.json();
      return ack;
    } catch {
      // Gateway may not support ack yet -- treat as success with empty state
      return { status: 'registered', routes_checksum: '', schemas_applied: 0 };
    }
  }

  /**
   * Deregister removes the service from the gateway.
   */
  async deregister(instanceId: string): Promise<void> {
    let resp: Response;
    try {
      resp = await this.fetchFn(
        `${this.gatewayURL}/deregister/${instanceId}`,
        { method: 'DELETE' },
      );
    } catch (err) {
      throw new DeregistrationFailedError(
        `fetch failed: ${err}`,
      );
    }

    if (resp.status !== 200 && resp.status !== 204) {
      throw new DeregistrationFailedError(
        `HTTP ${resp.status}`,
      );
    }
  }

  /**
   * ReportHealth sends a heartbeat to the gateway (without checksum).
   * Satisfies the ServiceDiscovery interface.
   */
  async reportHealth(
    instanceId: string,
    status: InstanceStatus,
  ): Promise<void> {
    await this.reportHealthWithChecksum(instanceId, status, '');
  }

  /**
   * ReportHealthWithChecksum sends a heartbeat with the service's expected
   * routes_checksum for reconciliation (FARP spec section 17.4.1).
   * The response includes the gateway's applied checksum and schema count.
   */
  async reportHealthWithChecksum(
    instanceId: string,
    status: InstanceStatus,
    routesChecksum: string,
  ): Promise<PushHeartbeatResponse> {
    const payload: PushHeartbeat = {
      status,
      routes_checksum: routesChecksum || undefined,
    };

    let body: string;
    try {
      body = JSON.stringify(payload);
    } catch (err) {
      throw new HealthCheckFailedError(
        `failed to serialize heartbeat: ${err}`,
      );
    }

    let resp: Response;
    try {
      resp = await this.fetchFn(
        `${this.gatewayURL}/heartbeat/${instanceId}`,
        {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body,
        },
      );
    } catch (err) {
      throw new HealthCheckFailedError(
        `fetch failed: ${err}`,
      );
    }

    if (resp.status !== 200) {
      throw new HealthCheckFailedError(
        `HTTP ${resp.status}`,
      );
    }

    try {
      const ack: PushHeartbeatResponse = await resp.json();
      return ack;
    } catch {
      // Gateway may not support ack yet
      return { status: 'ok', routes_checksum: '', schemas_applied: 0 };
    }
  }

  /**
   * Discover queries the gateway for instances of a service.
   */
  async discover(serviceName?: string): Promise<ServiceInstance[]> {
    let url = `${this.gatewayURL}/services`;
    if (serviceName) {
      url += `/${serviceName}`;
    }

    let resp: Response;
    try {
      resp = await this.fetchFn(url, { method: 'GET' });
    } catch (err) {
      throw new DiscoveryUnavailableError(
        `fetch failed: ${err}`,
      );
    }

    const instances: ServiceInstance[] = await resp.json();
    return instances;
  }

  /**
   * Watch is not natively supported in push mode from the service side.
   * Services push to the gateway; they don't watch it.
   * This blocks until the AbortController is aborted (via close()).
   */
  async watch(
    _serviceName: string | undefined,
    _handler: DiscoveryEventHandler,
  ): Promise<void> {
    this.abortController = new AbortController();
    return new Promise<void>((resolve) => {
      this.abortController!.signal.addEventListener('abort', () => {
        resolve();
      });
    });
  }

  /**
   * Close is a no-op for push discovery (aborts any pending watch).
   */
  close(): void {
    if (this.abortController) {
      this.abortController.abort();
      this.abortController = null;
    }
  }

  /**
   * Health checks if the gateway is reachable.
   */
  async health(): Promise<void> {
    let resp: Response;
    try {
      resp = await this.fetchFn(`${this.gatewayURL}/services`, {
        method: 'GET',
      });
    } catch (err) {
      throw new DiscoveryUnavailableError(
        `fetch failed: ${err}`,
      );
    }

    if (resp.status >= 500) {
      throw new DiscoveryUnavailableError(
        `HTTP ${resp.status}`,
      );
    }
  }
}
