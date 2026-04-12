import { describe, it, expect } from 'vitest';
import { InMemoryRegistry, createManifest } from '../src/index';
import type { SchemaManifest, ManifestEvent } from '../src/index';

function makeManifest(name: string, instanceId: string): SchemaManifest {
  const m = createManifest(name, '1.0.0', instanceId);
  m.endpoints.health = '/health';
  return m;
}

describe('InMemoryRegistry', () => {
  describe('registerManifest / getManifest', () => {
    it('registers and retrieves a manifest', async () => {
      const reg = new InMemoryRegistry();
      const m = makeManifest('svc', 'i1');
      await reg.registerManifest(m);
      const result = await reg.getManifest('i1');
      expect(result.service_name).toBe('svc');
      expect(result.instance_id).toBe('i1');
    });

    it('getManifest throws for unknown instance', async () => {
      const reg = new InMemoryRegistry();
      await expect(reg.getManifest('unknown')).rejects.toThrow('schema manifest not found');
    });
  });

  describe('updateManifest', () => {
    it('updates an existing manifest', async () => {
      const reg = new InMemoryRegistry();
      const m = makeManifest('svc', 'i1');
      await reg.registerManifest(m);

      const updated = { ...m, service_version: '2.0.0' };
      await reg.updateManifest(updated);

      const result = await reg.getManifest('i1');
      expect(result.service_version).toBe('2.0.0');
    });

    it('throws when updating non-existent manifest', async () => {
      const reg = new InMemoryRegistry();
      const m = makeManifest('svc', 'i1');
      await expect(reg.updateManifest(m)).rejects.toThrow('schema manifest not found');
    });
  });

  describe('deleteManifest', () => {
    it('deletes a manifest', async () => {
      const reg = new InMemoryRegistry();
      const m = makeManifest('svc', 'i1');
      await reg.registerManifest(m);
      await reg.deleteManifest('i1');
      await expect(reg.getManifest('i1')).rejects.toThrow('schema manifest not found');
    });

    it('throws when deleting non-existent manifest', async () => {
      const reg = new InMemoryRegistry();
      await expect(reg.deleteManifest('unknown')).rejects.toThrow('schema manifest not found');
    });
  });

  describe('listManifests', () => {
    it('lists manifests by service name', async () => {
      const reg = new InMemoryRegistry();
      await reg.registerManifest(makeManifest('svc-a', 'i1'));
      await reg.registerManifest(makeManifest('svc-a', 'i2'));
      await reg.registerManifest(makeManifest('svc-b', 'i3'));

      const results = await reg.listManifests('svc-a');
      expect(results).toHaveLength(2);
    });

    it('lists all manifests with empty service name', async () => {
      const reg = new InMemoryRegistry();
      await reg.registerManifest(makeManifest('svc-a', 'i1'));
      await reg.registerManifest(makeManifest('svc-b', 'i2'));

      const results = await reg.listManifests('');
      expect(results).toHaveLength(2);
    });

    it('returns empty array for unknown service', async () => {
      const reg = new InMemoryRegistry();
      const results = await reg.listManifests('unknown');
      expect(results).toEqual([]);
    });
  });

  describe('publishSchema / fetchSchema', () => {
    it('publishes and fetches a schema', async () => {
      const reg = new InMemoryRegistry();
      const schema = { openapi: '3.1.0', paths: {} };
      await reg.publishSchema('/schemas/openapi', schema);
      const result = await reg.fetchSchema('/schemas/openapi');
      expect(result).toEqual(schema);
    });

    it('throws when fetching non-existent schema', async () => {
      const reg = new InMemoryRegistry();
      await expect(reg.fetchSchema('/unknown')).rejects.toThrow('schema not found');
    });

    it('overwrites existing schema on re-publish', async () => {
      const reg = new InMemoryRegistry();
      await reg.publishSchema('/s1', { v: 1 });
      await reg.publishSchema('/s1', { v: 2 });
      const result = await reg.fetchSchema('/s1');
      expect(result).toEqual({ v: 2 });
    });
  });

  describe('watchManifests', () => {
    it('notifies on manifest register', async () => {
      const reg = new InMemoryRegistry();
      const events: ManifestEvent[] = [];
      reg.watchManifests('svc', (e) => events.push(e));

      await reg.registerManifest(makeManifest('svc', 'i1'));

      expect(events).toHaveLength(1);
      expect(events[0].type).toBe('added');
      expect(events[0].manifest.instance_id).toBe('i1');
    });

    it('notifies on manifest update', async () => {
      const reg = new InMemoryRegistry();
      const events: ManifestEvent[] = [];

      const m = makeManifest('svc', 'i1');
      await reg.registerManifest(m);

      reg.watchManifests('svc', (e) => events.push(e));
      await reg.updateManifest({ ...m, service_version: '2.0.0' });

      expect(events).toHaveLength(1);
      expect(events[0].type).toBe('updated');
    });

    it('notifies on manifest delete', async () => {
      const reg = new InMemoryRegistry();
      const events: ManifestEvent[] = [];

      await reg.registerManifest(makeManifest('svc', 'i1'));
      reg.watchManifests('svc', (e) => events.push(e));
      await reg.deleteManifest('i1');

      expect(events).toHaveLength(1);
      expect(events[0].type).toBe('removed');
    });

    it('unsubscribe stops notifications', async () => {
      const reg = new InMemoryRegistry();
      const events: ManifestEvent[] = [];

      const unsub = reg.watchManifests('svc', (e) => events.push(e));
      await reg.registerManifest(makeManifest('svc', 'i1'));
      expect(events).toHaveLength(1);

      unsub();
      await reg.registerManifest(makeManifest('svc', 'i2'));
      expect(events).toHaveLength(1); // no new event
    });
  });

  describe('close', () => {
    it('prevents further operations after close', async () => {
      const reg = new InMemoryRegistry();
      reg.close();
      await expect(reg.registerManifest(makeManifest('svc', 'i1'))).rejects.toThrow('registry is closed');
    });
  });

  describe('health', () => {
    it('succeeds when open', async () => {
      const reg = new InMemoryRegistry();
      await expect(reg.health()).resolves.toBeUndefined();
    });

    it('throws when closed', async () => {
      const reg = new InMemoryRegistry();
      reg.close();
      await expect(reg.health()).rejects.toThrow('registry is closed');
    });
  });
});
