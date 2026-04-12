/**
 * OpenAPI 3.1 types for the FARP merger.
 *
 * These types mirror the Go merger/openapi.go types for structured
 * OpenAPI specification manipulation during schema merging.
 */

import type { SchemaManifest, ConflictStrategy } from '../types';

// =============================================================================
// OpenAPI 3.1 Specification Types
// =============================================================================

/** Represents a simplified OpenAPI 3.x specification. */
export interface OpenAPISpec {
  openapi: string;
  info: Info;
  servers?: Server[];
  paths: Record<string, PathItem>;
  components?: Components;
  security?: SecurityRequirement[];
  tags?: Tag[];
  externalDocs?: ExternalDocumentation;
  webhooks?: Record<string, PathItem>;
  extensions?: Record<string, unknown>;
}

/** Represents OpenAPI info object. */
export interface Info {
  title: string;
  description?: string;
  version: string;
  termsOfService?: string;
  contact?: Contact;
  license?: License;
  extensions?: Record<string, unknown>;
}

/** Represents contact information. */
export interface Contact {
  name?: string;
  url?: string;
  email?: string;
}

/** Represents license information. */
export interface License {
  name: string;
  url?: string;
}

/** Represents an OpenAPI server. */
export interface Server {
  url: string;
  description?: string;
  variables?: Record<string, ServerVariable>;
}

/** Represents a server variable. */
export interface ServerVariable {
  default: string;
  enum?: string[];
  description?: string;
}

/** Represents an OpenAPI path item. */
export interface PathItem {
  summary?: string;
  description?: string;
  get?: Operation;
  put?: Operation;
  post?: Operation;
  delete?: Operation;
  options?: Operation;
  head?: Operation;
  patch?: Operation;
  trace?: Operation;
  parameters?: Parameter[];
  extensions?: Record<string, unknown>;
}

/** Represents an OpenAPI operation. */
export interface Operation {
  operationId?: string;
  summary?: string;
  description?: string;
  tags?: string[];
  parameters?: Parameter[];
  requestBody?: RequestBody;
  responses?: Record<string, Response>;
  security?: SecurityRequirement[];
  deprecated?: boolean;
  extensions?: Record<string, unknown>;
}

/** Represents an OpenAPI parameter. */
export interface Parameter {
  name: string;
  in: 'query' | 'header' | 'path' | 'cookie';
  description?: string;
  required?: boolean;
  schema?: Record<string, unknown>;
  example?: unknown;
}

/** Represents an OpenAPI request body. */
export interface RequestBody {
  description?: string;
  content: Record<string, MediaType>;
  required?: boolean;
  extensions?: Record<string, unknown>;
}

/** Represents an OpenAPI response. */
export interface Response {
  description: string;
  content?: Record<string, MediaType>;
  headers?: Record<string, Header>;
  extensions?: Record<string, unknown>;
}

/** Represents a media type object. */
export interface MediaType {
  schema?: Record<string, unknown>;
  example?: unknown;
  examples?: Record<string, Example>;
}

/** Represents an example object. */
export interface Example {
  summary?: string;
  description?: string;
  value?: unknown;
  externalValue?: string;
}

/** Represents a header object. */
export interface Header {
  description?: string;
  schema?: Record<string, unknown>;
}

/** Represents OpenAPI components. */
export interface Components {
  schemas?: Record<string, Record<string, unknown>>;
  responses?: Record<string, Response>;
  parameters?: Record<string, Parameter>;
  requestBodies?: Record<string, RequestBody>;
  headers?: Record<string, Header>;
  securitySchemes?: Record<string, SecurityScheme>;
}

/** Represents a security scheme. */
export interface SecurityScheme {
  type: string; // apiKey, http, oauth2, openIdConnect
  description?: string;
  name?: string; // For apiKey
  in?: string; // For apiKey: query, header, cookie
  scheme?: string; // For http: bearer, basic
  bearerFormat?: string; // For http bearer
  openIdConnectUrl?: string; // For openIdConnect
}

/** Represents a security requirement (map of scheme name to scopes). */
export type SecurityRequirement = Record<string, string[]>;

/** Represents an OpenAPI tag. */
export interface Tag {
  name: string;
  description?: string;
  extensions?: Record<string, unknown>;
}

/** Represents external documentation. */
export interface ExternalDocumentation {
  url: string;
  description?: string;
}

// =============================================================================
// Merger Result Types
// =============================================================================

/** ConflictType represents the type of conflict. */
export type ConflictType =
  | 'path'
  | 'component'
  | 'tag'
  | 'operationId'
  | 'securityScheme';

/** Conflict represents a conflict encountered during merging. */
export interface Conflict {
  /** Type of conflict (path, component, tag, etc.) */
  type: ConflictType;

  /** Path or name that conflicted */
  item: string;

  /** Services involved in the conflict */
  services: string[];

  /** How the conflict was resolved */
  resolution: string;

  /** Conflict strategy that was applied */
  strategy: ConflictStrategy;
}

/** MergeResult contains the merged OpenAPI spec and metadata. */
export interface MergeResult {
  /** The merged OpenAPI specification */
  spec: OpenAPISpec;

  /** Services that were included in the merge */
  includedServices: string[];

  /** Services that were excluded (not marked for inclusion) */
  excludedServices: string[];

  /** Conflicts that were encountered during merge */
  conflicts: Conflict[];

  /** Warnings (non-fatal issues) */
  warnings: string[];
}

/** ServiceSchema wraps a schema with its service context. */
export interface ServiceSchema {
  manifest: SchemaManifest;
  schema: unknown; // Raw OpenAPI schema (Record<string, unknown>)
  parsed?: OpenAPISpec;
}
