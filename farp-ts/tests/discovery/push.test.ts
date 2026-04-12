import { describe, it, expect, vi } from 'vitest';
import { PushDiscovery } from '../../src/discovery/push';
import type { ServiceInstance } from '../../src/discovery/types';

function makeInstance(id: string): ServiceInstance {
  return {
    id,
    serviceName: 'test-svc',
    address: 'localhost:8080',
    status: 'healthy',
    registeredAt: new Date().toISOString(),
    lastHealthCheck: new Date().toISOString(),
  };
}

function mockFetch(status: number, body: unknown = {}): typeof fetch {
  return vi.fn().mockResolvedValue({
    status,
    ok: status >= 200 && status < 300,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as Response);
}

describe('PushDiscovery', () => {
  describe('register', () => {
    it('sends POST to /register', async () => {
      const fetchFn = mockFetch(200, { status: 'registered', routes_checksum: '', schemas_applied: 0 });
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);
      const inst = makeInstance('i1');

      await discovery.register(inst);

      expect(fetchFn).toHaveBeenCalledWith(
        'http://gateway:9090/_farp/v1/register',
        expect.objectContaining({ method: 'POST' }),
      );
    });

    it('throws on non-200 response', async () => {
      const fetchFn = mockFetch(500);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await expect(discovery.register(makeInstance('i1'))).rejects.toThrow('HTTP 500');
    });

    it('throws on network error', async () => {
      const fetchFn = vi.fn().mockRejectedValue(new Error('network error'));
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await expect(discovery.register(makeInstance('i1'))).rejects.toThrow('fetch failed');
    });
  });

  describe('deregister', () => {
    it('sends DELETE to /deregister/{id}', async () => {
      const fetchFn = mockFetch(200);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await discovery.deregister('i1');

      expect(fetchFn).toHaveBeenCalledWith(
        'http://gateway:9090/_farp/v1/deregister/i1',
        expect.objectContaining({ method: 'DELETE' }),
      );
    });

    it('throws on failure', async () => {
      const fetchFn = mockFetch(500);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await expect(discovery.deregister('i1')).rejects.toThrow('HTTP 500');
    });
  });

  describe('reportHealth', () => {
    it('sends PUT to /heartbeat/{id}', async () => {
      const fetchFn = mockFetch(200, { status: 'ok', routes_checksum: '', schemas_applied: 0 });
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await discovery.reportHealth('i1', 'healthy');

      expect(fetchFn).toHaveBeenCalledWith(
        'http://gateway:9090/_farp/v1/heartbeat/i1',
        expect.objectContaining({ method: 'PUT' }),
      );
    });

    it('throws on non-200 response', async () => {
      const fetchFn = mockFetch(404);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await expect(discovery.reportHealth('i1', 'healthy')).rejects.toThrow('HTTP 404');
    });
  });

  describe('discover', () => {
    it('fetches instances from /services', async () => {
      const instances = [makeInstance('i1'), makeInstance('i2')];
      const fetchFn = mockFetch(200, instances);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      const result = await discovery.discover();

      expect(fetchFn).toHaveBeenCalledWith(
        'http://gateway:9090/_farp/v1/services',
        expect.objectContaining({ method: 'GET' }),
      );
      expect(result).toHaveLength(2);
    });

    it('fetches instances for specific service', async () => {
      const fetchFn = mockFetch(200, []);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await discovery.discover('my-svc');

      expect(fetchFn).toHaveBeenCalledWith(
        'http://gateway:9090/_farp/v1/services/my-svc',
        expect.objectContaining({ method: 'GET' }),
      );
    });
  });

  describe('health', () => {
    it('succeeds on 200', async () => {
      const fetchFn = mockFetch(200, []);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await expect(discovery.health()).resolves.toBeUndefined();
    });

    it('throws on 500', async () => {
      const fetchFn = mockFetch(500);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);

      await expect(discovery.health()).rejects.toThrow('HTTP 500');
    });
  });

  describe('close', () => {
    it('does not throw when called', () => {
      const fetchFn = mockFetch(200);
      const discovery = new PushDiscovery('http://gateway:9090/_farp/v1', fetchFn);
      expect(() => discovery.close()).not.toThrow();
    });
  });
});
