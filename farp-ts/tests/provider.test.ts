import { describe, it, expect } from 'vitest';
import {
  ProviderRegistry,
  BaseSchemaProvider,
} from '../src/index';
import type { SchemaProvider, SchemaType } from '../src/index';

function makeProvider(type: SchemaType): SchemaProvider {
  const base = new BaseSchemaProvider({
    type,
    specVersion: '3.1.0',
    contentType: 'application/json',
    endpoint: '/openapi.json',
  });
  return {
    ...base,
    type: () => type,
    specVersion: () => '3.1.0',
    contentType: () => 'application/json',
    endpoint: () => '/openapi.json',
    generate: async () => ({}),
    validate: () => {},
    hash: async (schema: unknown) => base.hash(schema),
    serialize: (schema: unknown) => base.serialize(schema),
  };
}

describe('ProviderRegistry', () => {
  it('registers and retrieves a provider', () => {
    const registry = new ProviderRegistry();
    const provider = makeProvider('openapi');
    registry.register(provider);
    expect(registry.get('openapi')).toBe(provider);
  });

  it('has() returns true for registered type', () => {
    const registry = new ProviderRegistry();
    registry.register(makeProvider('openapi'));
    expect(registry.has('openapi')).toBe(true);
    expect(registry.has('graphql')).toBe(false);
  });

  it('get() returns undefined for unregistered type', () => {
    const registry = new ProviderRegistry();
    expect(registry.get('graphql')).toBeUndefined();
  });

  it('list() returns all registered types', () => {
    const registry = new ProviderRegistry();
    registry.register(makeProvider('openapi'));
    registry.register(makeProvider('graphql'));
    const types = registry.list();
    expect(types).toContain('openapi');
    expect(types).toContain('graphql');
    expect(types).toHaveLength(2);
  });

  it('overwrites provider on re-register', () => {
    const registry = new ProviderRegistry();
    const p1 = makeProvider('openapi');
    const p2 = makeProvider('openapi');
    registry.register(p1);
    registry.register(p2);
    expect(registry.get('openapi')).toBe(p2);
    expect(registry.list()).toHaveLength(1);
  });
});

describe('BaseSchemaProvider', () => {
  it('returns correct type, specVersion, contentType, endpoint', () => {
    const p = new BaseSchemaProvider({
      type: 'graphql',
      specVersion: '1.0',
      contentType: 'application/graphql',
      endpoint: '/graphql',
    });
    expect(p.type()).toBe('graphql');
    expect(p.specVersion()).toBe('1.0');
    expect(p.contentType()).toBe('application/graphql');
    expect(p.endpoint()).toBe('/graphql');
  });

  it('hash() returns a 64-char hex hash', async () => {
    const p = new BaseSchemaProvider({
      type: 'openapi',
      specVersion: '3.1.0',
      contentType: 'application/json',
      endpoint: '/openapi',
    });
    const h = await p.hash({ foo: 'bar' });
    expect(h).toHaveLength(64);
    expect(h).toMatch(/^[0-9a-f]{64}$/);
  });

  it('serialize() returns JSON string', () => {
    const p = new BaseSchemaProvider({
      type: 'openapi',
      specVersion: '3.1.0',
      contentType: 'application/json',
      endpoint: '/openapi',
    });
    const result = p.serialize({ x: 1 });
    expect(result).toBe('{"x":1}');
  });

  it('validate() calls custom validate function', () => {
    let called = false;
    const p = new BaseSchemaProvider({
      type: 'openapi',
      specVersion: '3.1.0',
      contentType: 'application/json',
      endpoint: '/openapi',
      validateFunc: () => { called = true; },
    });
    p.validate({});
    expect(called).toBe(true);
  });

  it('validate() does nothing without validateFunc', () => {
    const p = new BaseSchemaProvider({
      type: 'openapi',
      specVersion: '3.1.0',
      contentType: 'application/json',
      endpoint: '/openapi',
    });
    expect(() => p.validate({})).not.toThrow();
  });
});
