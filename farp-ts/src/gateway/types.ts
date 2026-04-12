/**
 * Gateway types for FARP TypeScript client.
 *
 * These types mirror the Go gateway package types used for
 * API gateway integration with FARP service discovery.
 */

import type { RouteDescriptor } from '../types';

/**
 * ServiceRoute represents a route configuration for the gateway.
 * Produced by converting service manifests/schemas into actionable routes.
 */
export interface ServiceRoute {
  /** Route path pattern (e.g., "/users/{id}") */
  path: string;

  /** HTTP methods for this route (e.g., ["GET", "POST"]) */
  methods: string[];

  /** Backend service URL to proxy requests to */
  targetUrl: string;

  /** Health check URL for the backend service */
  healthUrl: string;

  /** Middleware names to apply to this route */
  middleware: string[];

  /** Additional route metadata */
  metadata: Record<string, unknown>;

  /** Name of the backend service */
  serviceName: string;

  /** Version of the backend service */
  serviceVersion: string;

  /** ID of the service instance that produced this route */
  instanceId: string;
}

/**
 * RouteUpdateHandler provides atomic route update callbacks.
 * Gateway implementations should implement this interface to prevent
 * intermittent 404s during route remounting.
 *
 * The flow is:
 *  1. prepareRoutes - validate new routes (may fail)
 *  2. commitRoutes - atomically swap route table (should not fail)
 *  3. rollbackRoutes - revert if commit fails
 */
export interface RouteUpdateHandler {
  /** Called with new routes for validation. Throw/reject to reject the update. */
  prepareRoutes(routes: RouteDescriptor[]): Promise<void>;

  /** Atomically swaps the route table. Called only after prepareRoutes succeeds. */
  commitRoutes(): Promise<void>;

  /** Reverts a failed commit. */
  rollbackRoutes(): Promise<void>;
}
