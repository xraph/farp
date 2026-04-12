/**
 * Discovery layer for FARP TypeScript client.
 *
 * Re-exports all discovery types, interfaces, and implementations.
 */

// Types
export type {
  ServiceInstance,
  DiscoveryEvent,
  DiscoveryEventHandler,
  PushRegistration,
  PushHeartbeat,
  PushRegistrationResponse,
  PushHeartbeatResponse,
} from './types';

// Interface
export type { ServiceDiscovery } from './interface';

// Push-based discovery (service-side client)
export { PushDiscovery } from './push';

// Push-based discovery (gateway-side handler)
export { PushHandler } from './push-handler';

// FARP HTTP handler (serves manifest, health, schemas)
export { FARPHandler } from './handler';

// Service node (full lifecycle management)
export { ServiceNode } from './service-node';
export type { ServiceNodeConfig } from './service-node';
