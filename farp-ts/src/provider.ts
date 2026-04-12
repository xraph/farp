import type { SchemaType } from './types.js';
import { calculateSchemaChecksum } from './checksum.js';

/**
 * Application represents an application that can have schemas generated from it.
 * This is an abstraction to avoid direct dependency on any framework.
 */
export interface Application {
  /** Returns the application/service name. */
  name(): string;

  /** Returns the application version. */
  version(): string;

  /** Returns route information for schema generation. */
  routes(): unknown;
}

/**
 * SchemaProvider generates schemas from application code.
 * Implementations generate protocol-specific schemas (OpenAPI, AsyncAPI, gRPC, GraphQL).
 */
export interface SchemaProvider {
  /** Returns the schema type this provider generates. */
  type(): SchemaType;

  /** Generates a schema from the application. */
  generate(app: Application): Promise<unknown>;

  /** Validates a generated schema for correctness. */
  validate(schema: unknown): void;

  /** Calculates the SHA256 hash of a schema. */
  hash(schema: unknown): Promise<string>;

  /** Converts schema to bytes for storage/transmission. */
  serialize(schema: unknown): string;

  /** Returns the HTTP endpoint where the schema is served. Empty string if not served via HTTP. */
  endpoint(): string;

  /** Returns the specification version (e.g., "3.1.0" for OpenAPI). */
  specVersion(): string;

  /** Returns the content type for the schema. */
  contentType(): string;
}

/**
 * BaseSchemaProvider provides common functionality for schema providers.
 */
export class BaseSchemaProvider implements Omit<SchemaProvider, 'generate'> {
  private readonly _type: SchemaType;
  private readonly _specVersion: string;
  private readonly _contentType: string;
  private readonly _endpoint: string;
  private readonly _validateFunc?: (schema: unknown) => void;

  constructor(options: {
    type: SchemaType;
    specVersion: string;
    contentType: string;
    endpoint: string;
    validateFunc?: (schema: unknown) => void;
  }) {
    this._type = options.type;
    this._specVersion = options.specVersion;
    this._contentType = options.contentType;
    this._endpoint = options.endpoint;
    this._validateFunc = options.validateFunc;
  }

  type(): SchemaType {
    return this._type;
  }

  specVersion(): string {
    return this._specVersion;
  }

  contentType(): string {
    return this._contentType;
  }

  endpoint(): string {
    return this._endpoint;
  }

  async hash(schema: unknown): Promise<string> {
    return calculateSchemaChecksum(schema);
  }

  serialize(schema: unknown): string {
    return JSON.stringify(schema);
  }

  validate(schema: unknown): void {
    if (this._validateFunc) {
      this._validateFunc(schema);
    }
  }
}

/**
 * ProviderRegistry manages registered schema providers.
 */
export class ProviderRegistry {
  private readonly providers = new Map<SchemaType, SchemaProvider>();

  /** Registers a schema provider. */
  register(provider: SchemaProvider): void {
    this.providers.set(provider.type(), provider);
  }

  /** Retrieves a provider by schema type. */
  get(schemaType: SchemaType): SchemaProvider | undefined {
    return this.providers.get(schemaType);
  }

  /** Checks if a provider exists for a schema type. */
  has(schemaType: SchemaType): boolean {
    return this.providers.has(schemaType);
  }

  /** Returns all registered schema types. */
  list(): SchemaType[] {
    return Array.from(this.providers.keys());
  }
}

// Global provider registry
const globalRegistry = new ProviderRegistry();

/** Registers a schema provider globally. */
export function registerProvider(provider: SchemaProvider): void {
  globalRegistry.register(provider);
}

/** Retrieves a provider from the global registry. */
export function getProvider(schemaType: SchemaType): SchemaProvider | undefined {
  return globalRegistry.get(schemaType);
}

/** Checks if a provider exists in the global registry. */
export function hasProvider(schemaType: SchemaType): boolean {
  return globalRegistry.has(schemaType);
}

/** Returns all registered schema types from the global registry. */
export function listProviders(): SchemaType[] {
  return globalRegistry.list();
}
