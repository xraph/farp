import { describe, it, expect } from 'vitest';
import {
  calculateSchemaChecksum,
  calculateManifestChecksum,
  calculateRoutesChecksum,
} from '../src/index';
import type { SchemaManifest } from '../src/index';

describe('checksum', () => {
  describe('calculateSchemaChecksum', () => {
    it('returns a 64-character hex string', async () => {
      const hash = await calculateSchemaChecksum({ foo: 'bar' });
      expect(hash).toHaveLength(64);
      expect(hash).toMatch(/^[0-9a-f]{64}$/);
    });

    it('same input produces same hash', async () => {
      const a = await calculateSchemaChecksum({ x: 1, y: 2 });
      const b = await calculateSchemaChecksum({ x: 1, y: 2 });
      expect(a).toBe(b);
    });

    it('different input produces different hash', async () => {
      const a = await calculateSchemaChecksum({ x: 1 });
      const b = await calculateSchemaChecksum({ x: 2 });
      expect(a).not.toBe(b);
    });

    it('handles empty objects', async () => {
      const hash = await calculateSchemaChecksum({});
      expect(hash).toHaveLength(64);
    });

    it('handles strings', async () => {
      const hash = await calculateSchemaChecksum('hello');
      expect(hash).toHaveLength(64);
    });
  });

  describe('calculateManifestChecksum', () => {
    it('returns empty string for manifest with no schemas', async () => {
      const manifest: SchemaManifest = {
        version: '1.1.0',
        service_name: 'test',
        service_version: '1.0.0',
        instance_id: 'inst-1',
        schemas: [],
        capabilities: [],
        endpoints: { health: '/health' },
        routing: { strategy: 'service' },
        updated_at: 0,
        checksum: '',
      };
      const hash = await calculateManifestChecksum(manifest);
      expect(hash).toBe('');
    });

    it('produces deterministic hash for schemas', async () => {
      const manifest: SchemaManifest = {
        version: '1.1.0',
        service_name: 'test',
        service_version: '1.0.0',
        instance_id: 'inst-1',
        schemas: [
          {
            type: 'openapi',
            spec_version: '3.1.0',
            location: { type: 'inline' },
            content_type: 'application/json',
            hash: 'a'.repeat(64),
            size: 100,
          },
        ],
        capabilities: [],
        endpoints: { health: '/health' },
        routing: { strategy: 'service' },
        updated_at: 0,
        checksum: '',
      };
      const a = await calculateManifestChecksum(manifest);
      const b = await calculateManifestChecksum(manifest);
      expect(a).toBe(b);
      expect(a).toHaveLength(64);
    });

    it('sorts schemas by type for deterministic hashing', async () => {
      const makeManifest = (schemas: SchemaManifest['schemas']): SchemaManifest => ({
        version: '1.1.0',
        service_name: 'test',
        service_version: '1.0.0',
        instance_id: 'inst-1',
        schemas,
        capabilities: [],
        endpoints: { health: '/health' },
        routing: { strategy: 'service' },
        updated_at: 0,
        checksum: '',
      });

      const schemaA = {
        type: 'asyncapi' as const,
        spec_version: '2.0.0',
        location: { type: 'inline' as const },
        content_type: 'application/json',
        hash: 'b'.repeat(64),
        size: 100,
      };
      const schemaO = {
        type: 'openapi' as const,
        spec_version: '3.1.0',
        location: { type: 'inline' as const },
        content_type: 'application/json',
        hash: 'a'.repeat(64),
        size: 100,
      };

      const m1 = makeManifest([schemaA, schemaO]);
      const m2 = makeManifest([schemaO, schemaA]);

      const h1 = await calculateManifestChecksum(m1);
      const h2 = await calculateManifestChecksum(m2);
      expect(h1).toBe(h2);
    });
  });

  describe('calculateRoutesChecksum', () => {
    it('produces a 64-char hex hash', async () => {
      const manifest: SchemaManifest = {
        version: '1.1.0',
        service_name: 'test',
        service_version: '1.0.0',
        instance_id: 'inst-1',
        schemas: [],
        capabilities: [],
        endpoints: { health: '/health' },
        routing: { strategy: 'service' },
        updated_at: 0,
        checksum: '',
      };
      const hash = await calculateRoutesChecksum(manifest);
      expect(hash).toHaveLength(64);
      expect(hash).toMatch(/^[0-9a-f]{64}$/);
    });

    it('changes when routing config changes', async () => {
      const m1: SchemaManifest = {
        version: '1.1.0',
        service_name: 'test',
        service_version: '1.0.0',
        instance_id: 'inst-1',
        schemas: [],
        capabilities: [],
        endpoints: { health: '/health' },
        routing: { strategy: 'service' },
        updated_at: 0,
        checksum: '',
      };
      const m2: SchemaManifest = {
        ...m1,
        routing: { strategy: 'root' },
      };
      const h1 = await calculateRoutesChecksum(m1);
      const h2 = await calculateRoutesChecksum(m2);
      expect(h1).not.toBe(h2);
    });

    it('changes when route table changes', async () => {
      const base: SchemaManifest = {
        version: '1.1.0',
        service_name: 'test',
        service_version: '1.0.0',
        instance_id: 'inst-1',
        schemas: [],
        capabilities: [],
        endpoints: { health: '/health' },
        routing: { strategy: 'service' },
        updated_at: 0,
        checksum: '',
      };
      const m1: SchemaManifest = {
        ...base,
        route_table: [{ path: '/users', methods: ['GET'], protocol: 'rest' }],
      };
      const m2: SchemaManifest = {
        ...base,
        route_table: [{ path: '/orders', methods: ['GET'], protocol: 'rest' }],
      };
      const h1 = await calculateRoutesChecksum(m1);
      const h2 = await calculateRoutesChecksum(m2);
      expect(h1).not.toBe(h2);
    });
  });
});
