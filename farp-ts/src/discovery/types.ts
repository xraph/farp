/**
 * Discovery layer types for FARP TypeScript client.
 *
 * These types mirror the Go discovery package types used for
 * push-based and registry-based service discovery.
 */

import type {
  EventType,
  InstanceStatus,
  SchemaManifest,
} from '../types';

/**
 * ServiceInstance represents a discovered service instance in infrastructure.
 * This is the network-level presence of a service, distinct from SchemaManifest
 * which describes the API contracts.
 */
export interface ServiceInstance {
  /** Unique ID for this instance (maps to SchemaManifest.instanceId) */
  id: string;

  /** Service name (maps to SchemaManifest.serviceName) */
  serviceName: string;

  /** Service version (maps to SchemaManifest.serviceVersion) */
  version?: string;

  /** Network address (host or host:port) */
  address: string;

  /** Port number */
  port?: number;

  /** Health status */
  status: InstanceStatus;

  /** Tags for filtering and labeling */
  tags?: string[];

  /** Metadata tags (key-value pairs for filtering, labeling) */
  metadata?: Record<string, string>;

  /** When this instance was registered (ISO string or Unix ms) */
  registeredAt: string;

  /** When health was last reported (ISO string or Unix ms) */
  lastHealthCheck: string;
}

/**
 * DiscoveryEvent represents a change in service discovery.
 */
export interface DiscoveryEvent {
  type: EventType;
  instance: ServiceInstance;
  timestamp: string;

  /**
   * Manifest is populated when available (e.g., push mode includes it directly).
   * For registry-based discovery, this may be undefined and the gateway fetches it separately.
   */
  manifest?: SchemaManifest;
}

/**
 * DiscoveryEventHandler is called when a service discovery event occurs.
 */
export type DiscoveryEventHandler = (event: DiscoveryEvent) => void;

/**
 * PushRegistration is the body sent when a service pushes its manifest to the gateway.
 */
export interface PushRegistration {
  instance: ServiceInstance;
  manifest?: SchemaManifest;
}

/**
 * PushHeartbeat is the body sent for heartbeat updates.
 * routesChecksum is optional (section 17.4.1 reconciliation).
 */
export interface PushHeartbeat {
  status: InstanceStatus;
  routes_checksum?: string;
}

/**
 * PushRegistrationResponse is the gateway's response to a registration.
 */
export interface PushRegistrationResponse {
  status: string;
  routes_checksum: string;
  schemas_applied: number;
}

/**
 * PushHeartbeatResponse is the gateway's response to a heartbeat.
 */
export interface PushHeartbeatResponse {
  status: string;
  routes_checksum: string;
  schemas_applied: number;
}
