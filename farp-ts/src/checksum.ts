import type { SchemaManifest, PathRewrite } from './types.js';

/**
 * Computes a SHA-256 hex digest of the given string using the Web Crypto API.
 */
async function sha256(data: string): Promise<string> {
  const encoder = new TextEncoder();
  const buffer = encoder.encode(data);
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer);
  const hashArray = new Uint8Array(hashBuffer);
  return Array.from(hashArray)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/**
 * Calculates the SHA256 checksum of a schema by JSON-serializing it first.
 */
export async function calculateSchemaChecksum(schema: unknown): Promise<string> {
  const data = JSON.stringify(schema);
  return sha256(data);
}

/**
 * Calculates the SHA256 checksum of a manifest by combining all schema hashes
 * in a deterministic order (sorted by schema type).
 */
export async function calculateManifestChecksum(manifest: SchemaManifest): Promise<string> {
  if (manifest.schemas.length === 0) {
    return '';
  }

  // Sort schemas by type for deterministic hashing
  const sorted = [...manifest.schemas].sort((a, b) => {
    if (a.type < b.type) return -1;
    if (a.type > b.type) return 1;
    return 0;
  });

  // Concatenate all schema hashes
  const combined = sorted.map((s) => s.hash).join('');

  return sha256(combined);
}

/**
 * Calculates a SHA256 hash of the route-affecting fields.
 * Covers: routing config (strategy, base_path, subdomain, rewrite, strip_prefix)
 * and sorted route table entries (path + methods + protocol).
 * This hash is used by gateways to detect whether route remounting is needed.
 */
export async function calculateRoutesChecksum(manifest: SchemaManifest): Promise<string> {
  interface RouteEntry {
    path: string;
    methods?: string[];
    protocol: string;
  }

  interface RouteCanonical {
    strategy: string;
    base_path: string;
    subdomain: string;
    rewrite: PathRewrite[];
    strip_prefix: boolean;
    routes: RouteEntry[];
    health: string;
    graphql: string;
    openapi: string;
    asyncapi: string;
  }

  const routeTable = manifest.route_table ?? [];

  // Sort route table entries for deterministic hashing
  const sortedRoutes: RouteEntry[] = routeTable
    .map((r) => ({
      path: r.path,
      methods: r.methods ? [...r.methods].sort() : undefined,
      protocol: r.protocol,
    }))
    .sort((a, b) => {
      if (a.path !== b.path) return a.path < b.path ? -1 : 1;
      return a.protocol < b.protocol ? -1 : a.protocol > b.protocol ? 1 : 0;
    });

  const canonical: RouteCanonical = {
    strategy: manifest.routing.strategy ?? '',
    base_path: manifest.routing.base_path ?? '',
    subdomain: manifest.routing.subdomain ?? '',
    rewrite: manifest.routing.rewrite ?? [],
    strip_prefix: manifest.routing.strip_prefix ?? false,
    routes: sortedRoutes,
    health: manifest.endpoints.health ?? '',
    graphql: manifest.endpoints.graphql ?? '',
    openapi: manifest.endpoints.openapi ?? '',
    asyncapi: manifest.endpoints.asyncapi ?? '',
  };

  const data = JSON.stringify(canonical);
  return sha256(data);
}
