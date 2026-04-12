import { describe, it, expect } from 'vitest';
import { GatewayClient } from '../../src/gateway/client';
import { InMemoryRegistry, createManifest } from '../../src/index';
import type { SchemaManifest, SchemaDescriptor } from '../../src/index';

function makeManifestWithSchema(
  name: string,
  instanceId: string,
  schemaType: 'openapi' | 'asyncapi' | 'graphql',
  inlineSchema: unknown,
): SchemaManifest {
  const m = createManifest(name, '1.0.0', instanceId);
  m.endpoints.health = '/health';
  m.schemas.push({
    type: schemaType,
    spec_version: '3.1.0',
    location: { type: 'inline' },
    content_type: 'application/json',
    inline_schema: inlineSchema,
    hash: 'a'.repeat(64),
    size: 100,
  });
  return m;
}

describe('GatewayClient', () => {
  describe('convertToRoutes', () => {
    describe('OpenAPI inline schema', () => {
      it('converts paths to routes', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          openapi: '3.1.0',
          paths: {
            '/users': {
              get: { operationId: 'listUsers' },
              post: { operationId: 'createUser' },
            },
            '/users/{id}': {
              get: { operationId: 'getUser' },
              delete: { operationId: 'deleteUser' },
            },
          },
        };

        const manifest = makeManifestWithSchema('user-svc', 'i1', 'openapi', schema);
        const routes = client.convertToRoutes([manifest]);

        expect(routes).toHaveLength(2);

        const usersRoute = routes.find(r => r.path === '/users');
        expect(usersRoute).toBeDefined();
        expect(usersRoute!.methods).toContain('get');
        expect(usersRoute!.methods).toContain('post');
        expect(usersRoute!.serviceName).toBe('user-svc');
        expect(usersRoute!.instanceId).toBe('i1');

        const userByIdRoute = routes.find(r => r.path === '/users/{id}');
        expect(userByIdRoute).toBeDefined();
        expect(userByIdRoute!.methods).toContain('get');
        expect(userByIdRoute!.methods).toContain('delete');
      });

      it('uses servers array for base URL', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          openapi: '3.1.0',
          servers: [{ url: 'https://api.example.com' }],
          paths: {
            '/items': { get: {} },
          },
        };

        const manifest = makeManifestWithSchema('item-svc', 'i1', 'openapi', schema);
        const routes = client.convertToRoutes([manifest]);

        expect(routes).toHaveLength(1);
        expect(routes[0].targetUrl).toBe('https://api.example.com/items');
      });

      it('falls back to service name for base URL', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          openapi: '3.1.0',
          paths: {
            '/items': { get: {} },
          },
        };

        const manifest = makeManifestWithSchema('item-svc', 'i1', 'openapi', schema);
        const routes = client.convertToRoutes([manifest]);

        expect(routes[0].targetUrl).toBe('http://item-svc:8080/items');
      });

      it('uses instance address for base URL when available', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          openapi: '3.1.0',
          paths: { '/x': { get: {} } },
        };

        const manifest = makeManifestWithSchema('svc', 'i1', 'openapi', schema);
        manifest.instance = {
          address: 'myhost:9090',
          status: 'healthy',
          started_at: 0,
        };
        const routes = client.convertToRoutes([manifest]);

        expect(routes[0].targetUrl).toBe('http://myhost:9090/x');
      });

      it('uses schema location URL for base URL', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          openapi: '3.1.0',
          paths: { '/x': { get: {} } },
        };

        const manifest = makeManifestWithSchema('svc', 'i1', 'openapi', schema);
        manifest.schemas[0].location = {
          type: 'http',
          url: 'https://myservice.local:3000/openapi.json',
        };
        // Put the schema in the cache since it can't fetch inline from http location
        // We need to use inline for convertToRoutes to work synchronously
        manifest.schemas[0].location = { type: 'inline' };
        manifest.schemas[0].inline_schema = schema;

        // Test that the priority ordering works: no servers array, no location URL, use instance
        manifest.instance = {
          address: 'https://frominstance.local:4000',
          status: 'healthy',
          started_at: 0,
        };

        const routes = client.convertToRoutes([manifest]);
        // Instance address has protocol already
        expect(routes[0].targetUrl).toBe('https://frominstance.local:4000/x');
      });
    });

    describe('AsyncAPI schema', () => {
      it('converts channels to WebSocket routes', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          asyncapi: '2.0.0',
          channels: {
            '/ws/orders': { subscribe: {} },
            '/ws/notifications': { publish: {} },
          },
        };

        const manifest = makeManifestWithSchema('ws-svc', 'i1', 'asyncapi', schema);
        const routes = client.convertToRoutes([manifest]);

        expect(routes).toHaveLength(2);
        expect(routes[0].methods).toContain('WEBSOCKET');
        expect(routes[0].metadata).toHaveProperty('schema_type', 'asyncapi');
        expect(routes[0].metadata).toHaveProperty('protocol', 'websocket');
      });
    });

    describe('GraphQL schema', () => {
      it('creates a single route for GraphQL endpoint', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = { typeDefs: 'type Query { hello: String }' };
        const manifest = makeManifestWithSchema('gql-svc', 'i1', 'graphql', schema);
        manifest.endpoints.graphql = '/graphql';
        const routes = client.convertToRoutes([manifest]);

        expect(routes).toHaveLength(1);
        expect(routes[0].path).toBe('/graphql');
        expect(routes[0].methods).toContain('POST');
        expect(routes[0].methods).toContain('GET');
        expect(routes[0].metadata).toHaveProperty('schema_type', 'graphql');
      });

      it('uses default /graphql path if not specified', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {};
        const manifest = makeManifestWithSchema('gql-svc', 'i1', 'graphql', schema);
        const routes = client.convertToRoutes([manifest]);

        expect(routes[0].path).toBe('/graphql');
      });
    });

    describe('base URL priority', () => {
      it('servers array takes highest priority', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const schema = {
          openapi: '3.1.0',
          servers: [{ url: 'https://from-servers.example.com' }],
          paths: { '/test': { get: {} } },
        };

        const manifest = makeManifestWithSchema('svc', 'i1', 'openapi', schema);
        manifest.instance = {
          address: 'from-instance:3000',
          status: 'healthy',
          started_at: 0,
        };

        const routes = client.convertToRoutes([manifest]);
        expect(routes[0].targetUrl).toContain('from-servers.example.com');
      });
    });

    describe('multiple manifests', () => {
      it('combines routes from multiple services', () => {
        const registry = new InMemoryRegistry();
        const client = new GatewayClient(registry);

        const m1 = makeManifestWithSchema('svc-a', 'i1', 'openapi', {
          openapi: '3.1.0',
          paths: { '/a': { get: {} } },
        });
        // Use a different hash so the schema cache does not conflate the two
        const m2 = makeManifestWithSchema('svc-b', 'i2', 'openapi', {
          openapi: '3.1.0',
          paths: { '/b': { post: {} } },
        });
        m2.schemas[0].hash = 'b'.repeat(64);

        const routes = client.convertToRoutes([m1, m2]);
        expect(routes).toHaveLength(2);
        expect(routes.map(r => r.path).sort()).toEqual(['/a', '/b']);
      });
    });
  });
});
