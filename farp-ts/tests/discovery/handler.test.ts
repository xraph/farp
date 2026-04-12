import { describe, it, expect } from 'vitest';
import { FARPHandler } from '../../src/discovery/handler';
import { createManifest } from '../../src/index';

describe('FARPHandler', () => {
  function makeHandler() {
    const manifest = createManifest('test-svc', '1.0.0', 'inst-1');
    manifest.endpoints.health = '/health';
    const schemas = new Map<string, unknown>();
    schemas.set('openapi', { openapi: '3.1.0', paths: {} });
    return new FARPHandler(manifest, schemas);
  }

  describe('GET /_farp/manifest', () => {
    it('returns manifest as JSON', async () => {
      const handler = makeHandler();
      const req = new Request('http://localhost/_farp/manifest', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);

      const body = await resp.json();
      expect(body.service_name).toBe('test-svc');
      expect(body.instance_id).toBe('inst-1');
    });
  });

  describe('GET /_farp/health', () => {
    it('returns 200 when healthy', async () => {
      const handler = makeHandler();
      const req = new Request('http://localhost/_farp/health', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);

      const body = await resp.json();
      expect(body.status).toBe('healthy');
    });

    it('returns 503 when unhealthy', async () => {
      const handler = makeHandler();
      handler.setHealth('unhealthy');
      const req = new Request('http://localhost/_farp/health', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(503);

      const body = await resp.json();
      expect(body.status).toBe('unhealthy');
    });

    it('returns 200 when degraded', async () => {
      const handler = makeHandler();
      handler.setHealth('degraded');
      const req = new Request('http://localhost/_farp/health', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);
    });
  });

  describe('GET /_farp/schemas/{type}', () => {
    it('returns schema for known type', async () => {
      const handler = makeHandler();
      const req = new Request('http://localhost/_farp/schemas/openapi', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);

      const body = await resp.json();
      expect(body.openapi).toBe('3.1.0');
    });

    it('returns 404 for unknown schema type', async () => {
      const handler = makeHandler();
      const req = new Request('http://localhost/_farp/schemas/grpc', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(404);
    });
  });

  describe('unknown routes', () => {
    it('returns 404', async () => {
      const handler = makeHandler();
      const req = new Request('http://localhost/_farp/unknown', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(404);
    });
  });

  describe('updateManifest', () => {
    it('updates the served manifest', async () => {
      const handler = makeHandler();
      const newManifest = createManifest('updated-svc', '2.0.0', 'inst-2');
      newManifest.endpoints.health = '/health';
      handler.updateManifest(newManifest);

      const req = new Request('http://localhost/_farp/manifest', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      const body = await resp.json();
      expect(body.service_name).toBe('updated-svc');
    });
  });

  describe('updateSchema', () => {
    it('adds a new schema type', async () => {
      const handler = makeHandler();
      handler.updateSchema('graphql', { typeDefs: 'type Query { hello: String }' });

      const req = new Request('http://localhost/_farp/schemas/graphql', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);
    });
  });
});
