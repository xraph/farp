/**
 * ServiceDiscovery interface for FARP TypeScript client.
 *
 * Provides pluggable service discovery operations.
 * Implementations wrap infrastructure-specific discovery mechanisms
 * (Consul, etcd, Kubernetes, Redis, mDNS, or push-based).
 */

import type { InstanceStatus } from '../types';
import type { ServiceInstance, DiscoveryEventHandler } from './types';

/**
 * ServiceDiscovery provides pluggable service discovery operations.
 */
export interface ServiceDiscovery {
  /**
   * Discover returns all known instances of a service.
   * If serviceName is empty or undefined, returns all instances across all services.
   */
  discover(serviceName?: string): Promise<ServiceInstance[]>;

  /**
   * Watch watches for changes to instances of a service.
   * The handler is called when instances are added, removed, or change health status.
   * If serviceName is empty or undefined, watches all services.
   * Returns a Promise that resolves when watching stops (e.g. via AbortSignal or close).
   */
  watch(serviceName: string | undefined, handler: DiscoveryEventHandler): Promise<void>;

  /**
   * Register registers a service instance in the discovery backend.
   */
  register(instance: ServiceInstance): Promise<void>;

  /**
   * Deregister removes a service instance from the discovery backend.
   */
  deregister(instanceId: string): Promise<void>;

  /**
   * ReportHealth reports the health status of a registered instance.
   */
  reportHealth(instanceId: string, status: InstanceStatus): Promise<void>;

  /**
   * Close closes the discovery backend connection and releases resources.
   */
  close(): void;

  /**
   * Health returns without throwing if the discovery backend itself is reachable.
   * Throws an error if the backend is unreachable.
   */
  health(): Promise<void>;
}
