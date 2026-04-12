import { describe, it, expect } from 'vitest';
import { Merger, defaultMergerConfig } from '../../src/merger';
import type { MergerConfig, ServiceSchema } from '../../src/merger';
import { createManifest } from '../../src/index';
import type { SchemaManifest } from '../../src/index';

function makeServiceSchema(
  serviceName: string,
  openapi: Record<string, unknown>,
  opts?: {
    includeInMerge?: boolean;
    strategy?: string;
    basePath?: string;
  },
): ServiceSchema {
  const m = createManifest(serviceName, '1.0.0', `${serviceName}-inst`);
  m.endpoints.health = '/health';
  m.routing.strategy = (opts?.strategy as SchemaManifest['routing']['strategy']) ?? 'root';
  if (opts?.basePath) {
    m.routing.base_path = opts.basePath;
  }

  const includeInMerge = opts?.includeInMerge !== false;

  m.schemas.push({
    type: 'openapi',
    spec_version: '3.1.0',
    location: { type: 'inline' },
    content_type: 'application/json',
    hash: 'a'.repeat(64),
    size: 100,
    metadata: {
      openapi: {
        composition: {
          include_in_merged: includeInMerge,
          conflict_strategy: 'prefix',
          preserve_extensions: true,
        },
      },
    },
  });

  return {
    manifest: m,
    schema: openapi,
  };
}

describe('Merger', () => {
  describe('merge single service', () => {
    it('merges a single service schema', () => {
      const config = defaultMergerConfig();
      const merger = new Merger(config);

      const schema = makeServiceSchema('users', {
        openapi: '3.1.0',
        info: { title: 'Users API', version: '1.0.0' },
        paths: {
          '/users': {
            get: { operationId: 'listUsers', tags: ['users'] },
          },
        },
        tags: [{ name: 'users', description: 'User operations' }],
      });

      const result = merger.merge([schema]);

      expect(result.includedServices).toContain('users');
      expect(result.excludedServices).toHaveLength(0);
      expect(Object.keys(result.spec.paths)).toHaveLength(1);
      expect(result.spec.paths['/users']).toBeDefined();
    });
  });

  describe('merge multiple services', () => {
    it('merges paths from multiple services without conflict', () => {
      const config = defaultMergerConfig();
      const merger = new Merger(config);

      const s1 = makeServiceSchema('users', {
        openapi: '3.1.0',
        info: { title: 'Users', version: '1.0.0' },
        paths: {
          '/users': { get: { operationId: 'listUsers' } },
        },
      });
      const s2 = makeServiceSchema('orders', {
        openapi: '3.1.0',
        info: { title: 'Orders', version: '1.0.0' },
        paths: {
          '/orders': { get: { operationId: 'listOrders' } },
        },
      });

      const result = merger.merge([s1, s2]);

      expect(result.includedServices).toHaveLength(2);
      expect(result.spec.paths['/users']).toBeDefined();
      expect(result.spec.paths['/orders']).toBeDefined();
      expect(result.conflicts).toHaveLength(0);
    });

    it('detects and resolves path conflicts with prefix strategy', () => {
      const config: MergerConfig = {
        ...defaultMergerConfig(),
        defaultConflictStrategy: 'prefix',
      };
      const merger = new Merger(config);

      const s1 = makeServiceSchema('svc-a', {
        openapi: '3.1.0',
        info: { title: 'A', version: '1.0.0' },
        paths: {
          '/items': { get: { operationId: 'listItems' } },
        },
      });
      const s2 = makeServiceSchema('svc-b', {
        openapi: '3.1.0',
        info: { title: 'B', version: '1.0.0' },
        paths: {
          '/items': { get: { operationId: 'listItems' } },
        },
      });

      const result = merger.merge([s1, s2]);

      expect(result.conflicts.length).toBeGreaterThan(0);
      const pathConflict = result.conflicts.find(c => c.type === 'path');
      expect(pathConflict).toBeDefined();
      expect(pathConflict!.services).toContain('svc-a');
      expect(pathConflict!.services).toContain('svc-b');
    });
  });

  describe('collapseServiceTags', () => {
    it('groups operations under a single service tag', () => {
      const config: MergerConfig = {
        ...defaultMergerConfig(),
        collapseServiceTags: true,
      };
      const merger = new Merger(config);

      const schema = makeServiceSchema('users', {
        openapi: '3.1.0',
        info: { title: 'Users', version: '1.0.0' },
        paths: {
          '/users': {
            get: { operationId: 'listUsers', tags: ['admin', 'public'] },
          },
        },
        tags: [
          { name: 'admin', description: 'Admin ops' },
          { name: 'public', description: 'Public ops' },
        ],
      });

      const result = merger.merge([schema]);

      // When collapseServiceTags is true, operations get the service name as tag
      const pathItem = result.spec.paths['/users'];
      expect(pathItem.get?.tags).toEqual(['users']);

      // Only one tag in the spec (the service-level tag)
      expect(result.spec.tags).toBeDefined();
      const tagNames = result.spec.tags!.map(t => t.name);
      expect(tagNames).toContain('users');
    });
  });

  describe('conflict detection', () => {
    it('error strategy throws on path conflict', () => {
      const config: MergerConfig = {
        ...defaultMergerConfig(),
        defaultConflictStrategy: 'error',
      };
      const merger = new Merger(config);

      // Both schemas need composition config with error strategy
      const makeSchema = (name: string) => {
        const s = makeServiceSchema(name, {
          openapi: '3.1.0',
          info: { title: name, version: '1.0.0' },
          paths: { '/conflict': { get: {} } },
        });
        s.manifest.schemas[0].metadata = {
          openapi: {
            composition: {
              include_in_merged: true,
              conflict_strategy: 'error',
              preserve_extensions: true,
            },
          },
        };
        return s;
      };

      expect(() => merger.merge([makeSchema('a'), makeSchema('b')])).toThrow('path conflict');
    });

    it('skip strategy skips conflicting paths', () => {
      const config: MergerConfig = {
        ...defaultMergerConfig(),
        defaultConflictStrategy: 'skip',
      };
      const merger = new Merger(config);

      const makeSchema = (name: string) => {
        const s = makeServiceSchema(name, {
          openapi: '3.1.0',
          info: { title: name, version: '1.0.0' },
          paths: { '/shared': { get: { operationId: `${name}_get` } } },
        });
        s.manifest.schemas[0].metadata = {
          openapi: {
            composition: {
              include_in_merged: true,
              conflict_strategy: 'skip',
              preserve_extensions: true,
            },
          },
        };
        return s;
      };

      const result = merger.merge([makeSchema('first'), makeSchema('second')]);
      // First one wins, second is skipped
      expect(result.spec.paths['/shared']).toBeDefined();
      const conflict = result.conflicts.find(c => c.type === 'path');
      expect(conflict?.resolution).toContain('Skipped');
    });
  });

  describe('excluded services', () => {
    it('excludes services not marked for merge', () => {
      const config = defaultMergerConfig();
      const merger = new Merger(config);

      const included = makeServiceSchema('included-svc', {
        openapi: '3.1.0',
        info: { title: 'Included', version: '1.0.0' },
        paths: { '/yes': { get: {} } },
      });
      const excluded = makeServiceSchema('excluded-svc', {
        openapi: '3.1.0',
        info: { title: 'Excluded', version: '1.0.0' },
        paths: { '/no': { get: {} } },
      }, { includeInMerge: false });

      const result = merger.merge([included, excluded]);

      expect(result.includedServices).toContain('included-svc');
      expect(result.excludedServices).toContain('excluded-svc');
      expect(result.spec.paths['/yes']).toBeDefined();
      expect(result.spec.paths['/no']).toBeUndefined();
    });
  });

  describe('components merging', () => {
    it('merges component schemas with prefixed names', () => {
      const config = defaultMergerConfig();
      const merger = new Merger(config);

      const schema = makeServiceSchema('users', {
        openapi: '3.1.0',
        info: { title: 'Users', version: '1.0.0' },
        paths: { '/users': { get: {} } },
        components: {
          schemas: {
            User: { type: 'object', properties: { name: { type: 'string' } } },
          },
        },
      });

      const result = merger.merge([schema]);

      // Components should be prefixed with service name
      expect(result.spec.components?.schemas?.['users_User']).toBeDefined();
    });
  });

  describe('routing application', () => {
    it('applies service mount strategy to paths', () => {
      const config = defaultMergerConfig();
      const merger = new Merger(config);

      const schema = makeServiceSchema('users', {
        openapi: '3.1.0',
        info: { title: 'Users', version: '1.0.0' },
        paths: { '/list': { get: {} } },
      });
      schema.manifest.routing.strategy = 'service';

      const result = merger.merge([schema]);
      expect(result.spec.paths['/users/list']).toBeDefined();
    });
  });
});
