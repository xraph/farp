import { describe, it, expect } from 'vitest';
import {
  createManifest,
  addSchema,
  addCapability,
  validateManifest,
  updateChecksum,
  cloneManifest,
  diffManifests,
  manifestToJSON,
  manifestFromJSON,
  PROTOCOL_VERSION,
  calculateSchemaChecksum,
} from '../src/index';
import type { SchemaManifest, SchemaDescriptor } from '../src/index';

function makeValidDescriptor(hash: string): SchemaDescriptor {
  return {
    type: 'openapi',
    spec_version: '3.1.0',
    location: { type: 'http', url: 'http://localhost/openapi.json' },
    content_type: 'application/json',
    hash,
    size: 100,
  };
}

describe('manifest', () => {
  describe('createManifest', () => {
    it('creates a manifest with correct defaults', () => {
      const m = createManifest('my-service', '1.0.0', 'inst-1');
      expect(m.version).toBe(PROTOCOL_VERSION);
      expect(m.service_name).toBe('my-service');
      expect(m.service_version).toBe('1.0.0');
      expect(m.instance_id).toBe('inst-1');
      expect(m.schemas).toEqual([]);
      expect(m.capabilities).toEqual([]);
      expect(m.endpoints.health).toBe('');
      expect(m.routing.strategy).toBe('service');
      expect(m.checksum).toBe('');
      expect(m.updated_at).toBeGreaterThan(0);
    });
  });

  describe('addSchema', () => {
    it('adds a schema descriptor to the manifest', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      const desc = makeValidDescriptor('a'.repeat(64));
      addSchema(m, desc);
      expect(m.schemas).toHaveLength(1);
      expect(m.schemas[0].type).toBe('openapi');
    });

    it('allows multiple schemas', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addSchema(m, makeValidDescriptor('a'.repeat(64)));
      addSchema(m, {
        ...makeValidDescriptor('b'.repeat(64)),
        type: 'asyncapi',
        spec_version: '2.0.0',
      });
      expect(m.schemas).toHaveLength(2);
    });
  });

  describe('addCapability', () => {
    it('adds a capability', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addCapability(m, 'rest');
      expect(m.capabilities).toContain('rest');
    });

    it('deduplicates capabilities', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addCapability(m, 'rest');
      addCapability(m, 'rest');
      expect(m.capabilities).toHaveLength(1);
    });
  });

  describe('validateManifest', () => {
    it('valid manifest passes validation', async () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      m.endpoints.health = '/health';
      // No schemas, no checksum needed
      await expect(validateManifest(m)).resolves.toBeUndefined();
    });

    it('missing service_name fails', async () => {
      const m = createManifest('', '1.0.0', 'i1');
      m.endpoints.health = '/health';
      await expect(validateManifest(m)).rejects.toThrow('service name is required');
    });

    it('missing instance_id fails', async () => {
      const m = createManifest('svc', '1.0.0', '');
      m.endpoints.health = '/health';
      await expect(validateManifest(m)).rejects.toThrow('instance ID is required');
    });

    it('missing health endpoint fails', async () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      await expect(validateManifest(m)).rejects.toThrow('health endpoint is required');
    });

    it('incompatible version fails', async () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      m.version = '2.0.0';
      m.endpoints.health = '/health';
      await expect(validateManifest(m)).rejects.toThrow('incompatible protocol version');
    });

    it('manifest with schemas and valid checksum passes', async () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      m.endpoints.health = '/health';
      const hash = await calculateSchemaChecksum({ openapi: '3.1.0' });
      addSchema(m, makeValidDescriptor(hash));
      await updateChecksum(m);
      await expect(validateManifest(m)).resolves.toBeUndefined();
    });

    it('manifest with wrong checksum fails', async () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      m.endpoints.health = '/health';
      const hash = await calculateSchemaChecksum({ openapi: '3.1.0' });
      addSchema(m, makeValidDescriptor(hash));
      m.checksum = 'f'.repeat(64);
      await expect(validateManifest(m)).rejects.toThrow('checksum mismatch');
    });
  });

  describe('updateChecksum', () => {
    it('updates the checksum and updated_at', async () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addSchema(m, makeValidDescriptor('a'.repeat(64)));
      const oldUpdated = m.updated_at;
      await updateChecksum(m);
      expect(m.checksum).toHaveLength(64);
      expect(m.updated_at).toBeGreaterThanOrEqual(oldUpdated);
    });
  });

  describe('cloneManifest', () => {
    it('creates a deep copy', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addSchema(m, makeValidDescriptor('a'.repeat(64)));
      addCapability(m, 'rest');
      m.route_table = [{ path: '/users', methods: ['GET'], protocol: 'rest' }];

      const clone = cloneManifest(m);
      expect(clone).toEqual(m);

      // Verify it is a deep copy
      clone.service_name = 'other';
      expect(m.service_name).toBe('svc');

      clone.schemas[0].type = 'graphql';
      expect(m.schemas[0].type).toBe('openapi');

      clone.capabilities.push('graphql');
      expect(m.capabilities).toHaveLength(1);
    });
  });

  describe('diffManifests', () => {
    it('detects no changes for identical manifests', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addSchema(m, makeValidDescriptor('a'.repeat(64)));
      const diff = diffManifests(m, m);
      expect(diff.schemas_added).toHaveLength(0);
      expect(diff.schemas_removed).toHaveLength(0);
      expect(diff.schemas_changed).toHaveLength(0);
      expect(diff.capabilities_added).toHaveLength(0);
      expect(diff.capabilities_removed).toHaveLength(0);
      expect(diff.endpoints_changed).toBe(false);
      expect(diff.routing_changed).toBe(false);
    });

    it('detects added schemas', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      const newM = createManifest('svc', '1.0.0', 'i1');
      addSchema(newM, makeValidDescriptor('a'.repeat(64)));

      const diff = diffManifests(old, newM);
      expect(diff.schemas_added).toHaveLength(1);
      expect(diff.schemas_added[0].type).toBe('openapi');
    });

    it('detects removed schemas', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      addSchema(old, makeValidDescriptor('a'.repeat(64)));
      const newM = createManifest('svc', '1.0.0', 'i1');

      const diff = diffManifests(old, newM);
      expect(diff.schemas_removed).toHaveLength(1);
    });

    it('detects changed schemas', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      addSchema(old, makeValidDescriptor('a'.repeat(64)));
      const newM = createManifest('svc', '1.0.0', 'i1');
      addSchema(newM, makeValidDescriptor('b'.repeat(64)));

      const diff = diffManifests(old, newM);
      expect(diff.schemas_changed).toHaveLength(1);
      expect(diff.schemas_changed[0].old_hash).toBe('a'.repeat(64));
      expect(diff.schemas_changed[0].new_hash).toBe('b'.repeat(64));
    });

    it('detects capability changes', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      addCapability(old, 'rest');
      const newM = createManifest('svc', '1.0.0', 'i1');
      addCapability(newM, 'graphql');

      const diff = diffManifests(old, newM);
      expect(diff.capabilities_added).toContain('graphql');
      expect(diff.capabilities_removed).toContain('rest');
    });

    it('detects endpoint changes', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      old.endpoints.health = '/health';
      const newM = createManifest('svc', '1.0.0', 'i1');
      newM.endpoints.health = '/healthz';

      const diff = diffManifests(old, newM);
      expect(diff.endpoints_changed).toBe(true);
    });

    it('detects routing changes', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      old.routing.strategy = 'service';
      const newM = createManifest('svc', '1.0.0', 'i1');
      newM.routing.strategy = 'root';

      const diff = diffManifests(old, newM);
      expect(diff.routing_changed).toBe(true);
    });

    it('detects routes_checksum changes', () => {
      const old = createManifest('svc', '1.0.0', 'i1');
      old.routes_checksum = 'aaa';
      const newM = createManifest('svc', '1.0.0', 'i1');
      newM.routes_checksum = 'bbb';

      const diff = diffManifests(old, newM);
      expect(diff.routes_checksum_changed).toBe(true);
    });
  });

  describe('manifestToJSON / manifestFromJSON', () => {
    it('round-trips a manifest', () => {
      const m = createManifest('svc', '1.0.0', 'i1');
      addSchema(m, makeValidDescriptor('a'.repeat(64)));
      addCapability(m, 'rest');

      const json = manifestToJSON(m);
      const parsed = manifestFromJSON(json);

      expect(parsed.service_name).toBe(m.service_name);
      expect(parsed.schemas).toHaveLength(1);
      expect(parsed.capabilities).toContain('rest');
    });

    it('manifestFromJSON throws on invalid JSON', () => {
      expect(() => manifestFromJSON('not json')).toThrow('invalid manifest format');
    });
  });
});
