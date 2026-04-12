/**
 * Gateway module for FARP TypeScript client.
 *
 * Provides API gateway integration utilities including service route conversion,
 * schema watching, and atomic route swapping.
 */

export type { ServiceRoute, RouteUpdateHandler } from './types';
export { GatewayClient } from './client';
export type { GatewayClientConfig } from './client';
