/**
 * FARPHandler serves FARP protocol HTTP endpoints for a service.
 * Uses Web standard Request/Response -- framework-agnostic.
 *
 * Endpoints served (relative to mount point):
 *   GET /_farp/manifest         -- returns SchemaManifest JSON
 *   GET /_farp/health           -- returns health status (200 or 503)
 *   GET /_farp/schemas/{type}   -- returns schema by type (openapi, asyncapi, graphql, etc.)
 *
 * Ported from Go: discovery/handler.go
 */

import type {
  InstanceStatus,
  SchemaManifest,
  SchemaType,
} from '../types';

/**
 * FARPHandler serves FARP protocol HTTP endpoints for a service.
 */
export class FARPHandler {
  private manifest: SchemaManifest;
  private schemas: Map<string, unknown>;
  private status: InstanceStatus;

  /**
   * Creates a new FARP HTTP handler.
   * @param manifest - The SchemaManifest to serve.
   * @param schemas - Map of schema type to schema content.
   */
  constructor(
    manifest: SchemaManifest,
    schemas?: Map<string, unknown>,
  ) {
    this.manifest = manifest;
    this.schemas = schemas ?? new Map();
    this.status = 'healthy';
  }

  /**
   * handleRequest routes FARP HTTP requests using Web standard Request/Response.
   */
  async handleRequest(request: Request): Promise<Response> {
    const url = new URL(request.url, 'http://localhost');
    let path = url.pathname;

    // Strip /_farp prefix if present
    const stripped = path.replace(/^\/_farp/, '');
    if (stripped !== path) {
      path = stripped;
    }

    if (path === '/manifest' || path === '/_farp/manifest') {
      return this.serveManifest();
    }

    if (path === '/health' || path === '/_farp/health') {
      return this.serveHealth();
    }

    if (path.startsWith('/schemas/') || path.startsWith('/_farp/schemas/')) {
      let schemaType = path.replace(/^\/_farp\/schemas\//, '');
      schemaType = schemaType.replace(/^\/schemas\//, '');
      // Handle the case where we already stripped /_farp
      if (schemaType === path) {
        schemaType = path.replace(/^\/schemas\//, '');
      }
      return this.serveSchema(schemaType);
    }

    return new Response('Not Found', { status: 404 });
  }

  private serveManifest(): Response {
    return new Response(JSON.stringify(this.manifest), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  private serveHealth(): Response {
    let statusCode: number;

    switch (this.status) {
      case 'healthy':
      case 'degraded':
        statusCode = 200;
        break;
      default:
        statusCode = 503;
        break;
    }

    return new Response(
      JSON.stringify({ status: this.status }),
      {
        status: statusCode,
        headers: { 'Content-Type': 'application/json' },
      },
    );
  }

  private serveSchema(schemaType: string): Response {
    const schema = this.schemas.get(schemaType);

    if (schema === undefined) {
      return new Response(`schema not found: ${schemaType}`, { status: 404 });
    }

    return new Response(JSON.stringify(schema), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  /**
   * SetHealth sets the health status returned by /_farp/health.
   */
  setHealth(status: InstanceStatus): void {
    this.status = status;
  }

  /**
   * UpdateManifest updates the manifest served by /_farp/manifest.
   */
  updateManifest(manifest: SchemaManifest): void {
    this.manifest = manifest;
  }

  /**
   * UpdateSchema updates a schema served by /_farp/schemas/{type}.
   */
  updateSchema(schemaType: SchemaType | string, schema: unknown): void {
    this.schemas.set(String(schemaType), schema);
  }
}
