import { describe, it, expect } from 'vitest';
import {
  matchPath,
  shouldIncludePath,
  buildPathRules,
  INTERNAL_PATH_RULES,
  PathRuleAction,
} from '../src/index';
import type { PathRule } from '../src/index';

describe('path-match', () => {
  describe('matchPath', () => {
    it('matches exact paths', () => {
      expect(matchPath('/api/v1/users', '/api/v1/users')).toBe(true);
      expect(matchPath('/api/v1/users', '/api/v1/orders')).toBe(false);
    });

    it('matches single wildcard *', () => {
      expect(matchPath('/api/*/status', '/api/users/status')).toBe(true);
      expect(matchPath('/api/*/status', '/api/orders/status')).toBe(true);
      expect(matchPath('/api/*/status', '/api/users/health')).toBe(false);
    });

    it('single wildcard does not match multiple segments', () => {
      expect(matchPath('/api/*/status', '/api/v1/users/status')).toBe(false);
    });

    it('matches double wildcard ** at end', () => {
      expect(matchPath('/api/**', '/api')).toBe(true);
      expect(matchPath('/api/**', '/api/v1')).toBe(true);
      expect(matchPath('/api/**', '/api/v1/users/123')).toBe(true);
    });

    it('matches double wildcard ** in middle', () => {
      expect(matchPath('/api/**/status', '/api/status')).toBe(true);
      expect(matchPath('/api/**/status', '/api/v1/status')).toBe(true);
      expect(matchPath('/api/**/status', '/api/v1/v2/status')).toBe(true);
    });

    it('matches inline wildcards', () => {
      expect(matchPath('/openapi*', '/openapi.json')).toBe(true);
      expect(matchPath('/openapi*', '/openapi.yaml')).toBe(true);
      expect(matchPath('/openapi*', '/openapi')).toBe(true);
      expect(matchPath('/*json', '/openapi.json')).toBe(true);
    });

    it('matches root path', () => {
      expect(matchPath('/', '/')).toBe(true);
      expect(matchPath('/', '/api')).toBe(false);
    });

    it('handles trailing slashes', () => {
      expect(matchPath('/api/v1/', '/api/v1')).toBe(true);
    });
  });

  describe('shouldIncludePath', () => {
    it('returns true by default when no rules match', () => {
      const rules: PathRule[] = [
        { pattern: '/admin/**', action: PathRuleAction.Exclude },
      ];
      expect(shouldIncludePath('/api/users', rules)).toBe(true);
    });

    it('first matching rule wins - exclude', () => {
      const rules: PathRule[] = [
        { pattern: '/api/**', action: PathRuleAction.Exclude },
        { pattern: '/api/users', action: PathRuleAction.Include },
      ];
      expect(shouldIncludePath('/api/users', rules)).toBe(false);
    });

    it('first matching rule wins - include', () => {
      const rules: PathRule[] = [
        { pattern: '/api/users', action: PathRuleAction.Include },
        { pattern: '/api/**', action: PathRuleAction.Exclude },
      ];
      expect(shouldIncludePath('/api/users', rules)).toBe(true);
    });

    it('exclude rule prevents inclusion', () => {
      const rules: PathRule[] = [
        { pattern: '/internal/**', action: PathRuleAction.Exclude },
      ];
      expect(shouldIncludePath('/internal/metrics', rules)).toBe(false);
    });
  });

  describe('buildPathRules', () => {
    it('returns only user rules when excludeInternal is false', () => {
      const userRules: PathRule[] = [
        { pattern: '/custom/**', action: PathRuleAction.Exclude },
      ];
      const result = buildPathRules(false, userRules);
      expect(result).toEqual(userRules);
    });

    it('prepends internal rules when excludeInternal is true', () => {
      const userRules: PathRule[] = [
        { pattern: '/custom/**', action: PathRuleAction.Exclude },
      ];
      const result = buildPathRules(true, userRules);
      expect(result.length).toBe(INTERNAL_PATH_RULES.length + userRules.length);
      expect(result.slice(0, INTERNAL_PATH_RULES.length)).toEqual(INTERNAL_PATH_RULES);
      expect(result.slice(INTERNAL_PATH_RULES.length)).toEqual(userRules);
    });
  });

  describe('INTERNAL_PATH_RULES', () => {
    it('excludes root path', () => {
      expect(shouldIncludePath('/', INTERNAL_PATH_RULES)).toBe(false);
    });

    it('excludes _farp paths', () => {
      expect(shouldIncludePath('/_farp/manifest', INTERNAL_PATH_RULES)).toBe(false);
      expect(shouldIncludePath('/_farp/health', INTERNAL_PATH_RULES)).toBe(false);
    });

    it('excludes docs paths', () => {
      expect(shouldIncludePath('/docs', INTERNAL_PATH_RULES)).toBe(false);
      expect(shouldIncludePath('/docs/api', INTERNAL_PATH_RULES)).toBe(false);
    });

    it('excludes openapi and asyncapi files', () => {
      expect(shouldIncludePath('/openapi.json', INTERNAL_PATH_RULES)).toBe(false);
      expect(shouldIncludePath('/asyncapi.json', INTERNAL_PATH_RULES)).toBe(false);
    });

    it('includes normal API paths', () => {
      expect(shouldIncludePath('/api/users', INTERNAL_PATH_RULES)).toBe(true);
      expect(shouldIncludePath('/v1/orders', INTERNAL_PATH_RULES)).toBe(true);
    });
  });
});
