import { describe, it, expect, afterEach } from 'vitest';
import { PushHandler } from '../../src/discovery/push-handler';
import type { ServiceInstance } from '../../src/discovery/types';

function makeInstance(id: string, serviceName: string): ServiceInstance {
  return {
    id,
    serviceName,
    address: 'localhost:8080',
    status: 'healthy',
    registeredAt: new Date().toISOString(),
    lastHealthCheck: new Date().toISOString(),
  };
}

describe('PushHandler', () => {
  let handler: PushHandler;

  afterEach(() => {
    handler?.close();
  });

  describe('POST /register', () => {
    it('registers a service and returns 200', async () => {
      handler = new PushHandler(60_000);
      const inst = makeInstance('i1', 'svc');
      const req = new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance: inst }),
      });

      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);

      const body = await resp.json();
      expect(body.status).toBe('registered');
    });

    it('returns 400 when instance ID is missing', async () => {
      handler = new PushHandler(60_000);
      const req = new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance: { serviceName: 'svc' } }),
      });

      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(400);
    });

    it('returns 400 for invalid JSON', async () => {
      handler = new PushHandler(60_000);
      const req = new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: 'not json',
      });

      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(400);
    });
  });

  describe('PUT /heartbeat/{id}', () => {
    it('updates heartbeat for registered instance', async () => {
      handler = new PushHandler(60_000);
      // Register first
      const inst = makeInstance('i1', 'svc');
      await handler.handleRequest(new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance: inst }),
      }));

      // Heartbeat
      const req = new Request('http://localhost/heartbeat/i1', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: 'healthy' }),
      });

      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);

      const body = await resp.json();
      expect(body.status).toBe('ok');
    });

    it('returns 404 for unknown instance', async () => {
      handler = new PushHandler(60_000);
      const req = new Request('http://localhost/heartbeat/unknown', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: 'healthy' }),
      });

      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(404);
    });
  });

  describe('GET /services', () => {
    it('returns registered services', async () => {
      handler = new PushHandler(60_000);
      await handler.handleRequest(new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance: makeInstance('i1', 'svc-a') }),
      }));
      await handler.handleRequest(new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance: makeInstance('i2', 'svc-b') }),
      }));

      const req = new Request('http://localhost/services', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      expect(resp.status).toBe(200);

      const body = await resp.json();
      expect(body).toHaveLength(2);
    });

    it('returns empty array when no services registered', async () => {
      handler = new PushHandler(60_000);
      const req = new Request('http://localhost/services', { method: 'GET' });
      const resp = await handler.handleRequest(req);
      const body = await resp.json();
      expect(body).toEqual([]);
    });
  });

  describe('DELETE /deregister/{id}', () => {
    it('deregisters a service', async () => {
      handler = new PushHandler(60_000);
      await handler.handleRequest(new Request('http://localhost/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ instance: makeInstance('i1', 'svc') }),
      }));

      const resp = await handler.handleRequest(
        new Request('http://localhost/deregister/i1', { method: 'DELETE' }),
      );
      expect(resp.status).toBe(200);

      // Verify it's gone
      const listResp = await handler.handleRequest(
        new Request('http://localhost/services', { method: 'GET' }),
      );
      const body = await listResp.json();
      expect(body).toHaveLength(0);
    });

    it('returns 404 for unknown instance', async () => {
      handler = new PushHandler(60_000);
      const resp = await handler.handleRequest(
        new Request('http://localhost/deregister/unknown', { method: 'DELETE' }),
      );
      expect(resp.status).toBe(404);
    });
  });

  describe('unknown routes', () => {
    it('returns 404 for unknown paths', async () => {
      handler = new PushHandler(60_000);
      const resp = await handler.handleRequest(
        new Request('http://localhost/unknown', { method: 'GET' }),
      );
      expect(resp.status).toBe(404);
    });
  });
});
