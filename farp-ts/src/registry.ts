import type {
  SchemaManifest,
  ManifestChangeHandler,
  SchemaChangeHandler,
  ManifestEvent,
  SchemaEvent,
  RegistryConfig,
} from './types.js';
import { EventType } from './types.js';
import {
  ManifestNotFoundError,
  SchemaNotFoundError,
} from './errors.js';

/**
 * SchemaRegistry manages schema manifests and schemas.
 * Implementations store data in various backends (Consul, etcd, Kubernetes, Redis, etc.).
 */
export interface SchemaRegistry {
  // Manifest operations
  registerManifest(manifest: SchemaManifest): Promise<void>;
  getManifest(instanceId: string): Promise<SchemaManifest>;
  updateManifest(manifest: SchemaManifest): Promise<void>;
  deleteManifest(instanceId: string): Promise<void>;
  listManifests(serviceName: string): Promise<SchemaManifest[]>;

  // Schema operations
  publishSchema(path: string, schema: unknown): Promise<void>;
  fetchSchema(path: string): Promise<unknown>;
  deleteSchema(path: string): Promise<void>;

  // Watch operations
  watchManifests(serviceName: string, onChange: ManifestChangeHandler): () => void;
  watchSchemas(path: string, onChange: SchemaChangeHandler): () => void;

  // Lifecycle
  close(): void;
  health(): Promise<void>;
}

/**
 * SchemaCache provides caching for fetched schemas.
 */
export interface SchemaCache {
  get(hash: string): unknown | undefined;
  set(hash: string, schema: unknown): void;
  delete(hash: string): void;
  clear(): void;
  size(): number;
}

/**
 * Returns the default registry configuration.
 */
export function defaultRegistryConfig(): RegistryConfig {
  return {
    backend: 'memory',
    namespace: 'farp',
    backend_config: {},
    max_schema_size: 1024 * 1024,      // 1MB
    compression_threshold: 100 * 1024,  // 100KB
    ttl: 0,                              // No expiry
  };
}

// =============================================================================
// Simple EventEmitter for watch support
// =============================================================================

type Listener<T> = (event: T) => void;

class EventEmitter<T> {
  private listeners: Map<string, Set<Listener<T>>> = new Map();

  on(key: string, listener: Listener<T>): () => void {
    if (!this.listeners.has(key)) {
      this.listeners.set(key, new Set());
    }
    this.listeners.get(key)!.add(listener);

    return () => {
      const set = this.listeners.get(key);
      if (set) {
        set.delete(listener);
        if (set.size === 0) {
          this.listeners.delete(key);
        }
      }
    };
  }

  emit(key: string, event: T): void {
    const set = this.listeners.get(key);
    if (set) {
      for (const listener of set) {
        listener(event);
      }
    }
  }
}

// =============================================================================
// InMemoryRegistry
// =============================================================================

/**
 * InMemoryRegistry is an in-memory implementation of SchemaRegistry.
 * Suitable for testing and development.
 */
export class InMemoryRegistry implements SchemaRegistry {
  private manifests = new Map<string, SchemaManifest>();
  private schemas = new Map<string, unknown>();
  private manifestEmitter = new EventEmitter<ManifestEvent>();
  private schemaEmitter = new EventEmitter<SchemaEvent>();
  private closed = false;

  async registerManifest(manifest: SchemaManifest): Promise<void> {
    this.ensureOpen();
    this.manifests.set(manifest.instance_id, manifest);

    this.manifestEmitter.emit(manifest.service_name, {
      type: EventType.Added,
      manifest,
      timestamp: Math.floor(Date.now() / 1000),
    });
  }

  async getManifest(instanceId: string): Promise<SchemaManifest> {
    this.ensureOpen();
    const manifest = this.manifests.get(instanceId);
    if (!manifest) {
      throw new ManifestNotFoundError(
        `schema manifest not found for instance: ${instanceId}`,
      );
    }
    return manifest;
  }

  async updateManifest(manifest: SchemaManifest): Promise<void> {
    this.ensureOpen();
    if (!this.manifests.has(manifest.instance_id)) {
      throw new ManifestNotFoundError(
        `schema manifest not found for instance: ${manifest.instance_id}`,
      );
    }
    this.manifests.set(manifest.instance_id, manifest);

    this.manifestEmitter.emit(manifest.service_name, {
      type: EventType.Updated,
      manifest,
      timestamp: Math.floor(Date.now() / 1000),
    });
  }

  async deleteManifest(instanceId: string): Promise<void> {
    this.ensureOpen();
    const manifest = this.manifests.get(instanceId);
    if (!manifest) {
      throw new ManifestNotFoundError(
        `schema manifest not found for instance: ${instanceId}`,
      );
    }
    this.manifests.delete(instanceId);

    this.manifestEmitter.emit(manifest.service_name, {
      type: EventType.Removed,
      manifest,
      timestamp: Math.floor(Date.now() / 1000),
    });
  }

  async listManifests(serviceName: string): Promise<SchemaManifest[]> {
    this.ensureOpen();
    const results: SchemaManifest[] = [];
    for (const manifest of this.manifests.values()) {
      if (serviceName === '' || manifest.service_name === serviceName) {
        results.push(manifest);
      }
    }
    return results;
  }

  async publishSchema(path: string, schema: unknown): Promise<void> {
    this.ensureOpen();
    const isNew = !this.schemas.has(path);
    this.schemas.set(path, schema);

    this.schemaEmitter.emit(path, {
      type: isNew ? EventType.Added : EventType.Updated,
      path,
      schema,
      timestamp: Math.floor(Date.now() / 1000),
    });
  }

  async fetchSchema(path: string): Promise<unknown> {
    this.ensureOpen();
    const schema = this.schemas.get(path);
    if (schema === undefined) {
      throw new SchemaNotFoundError(`schema not found at path: ${path}`);
    }
    return schema;
  }

  async deleteSchema(path: string): Promise<void> {
    this.ensureOpen();
    if (!this.schemas.has(path)) {
      throw new SchemaNotFoundError(`schema not found at path: ${path}`);
    }
    this.schemas.delete(path);

    this.schemaEmitter.emit(path, {
      type: EventType.Removed,
      path,
      schema: undefined,
      timestamp: Math.floor(Date.now() / 1000),
    });
  }

  watchManifests(serviceName: string, onChange: ManifestChangeHandler): () => void {
    this.ensureOpen();
    return this.manifestEmitter.on(serviceName, onChange);
  }

  watchSchemas(path: string, onChange: SchemaChangeHandler): () => void {
    this.ensureOpen();
    return this.schemaEmitter.on(path, onChange);
  }

  close(): void {
    this.closed = true;
    this.manifests.clear();
    this.schemas.clear();
  }

  async health(): Promise<void> {
    this.ensureOpen();
    // In-memory registry is always healthy if open
  }

  private ensureOpen(): void {
    if (this.closed) {
      throw new Error('registry is closed');
    }
  }
}
