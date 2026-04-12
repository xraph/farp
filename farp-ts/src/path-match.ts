import { PathRuleAction } from './types.js';
import type { PathRule } from './types.js';

/**
 * Splits a URL path into segments, stripping leading/trailing slashes.
 */
function splitPath(p: string): string[] {
  const trimmed = p.replace(/^\/+|\/+$/g, '');
  if (trimmed === '') {
    return [];
  }
  return trimmed.split('/');
}

/**
 * Matches a single path segment against a pattern segment containing
 * inline wildcards (e.g., "openapi*" matches "openapi.json").
 */
function matchSegment(pattern: string, segment: string): boolean {
  // Simple prefix matching: "openapi*"
  if (pattern.endsWith('*') && !pattern.startsWith('*')) {
    const prefix = pattern.slice(0, -1);
    return segment.startsWith(prefix);
  }

  // Simple suffix matching: "*json"
  if (pattern.startsWith('*') && !pattern.endsWith('*')) {
    const suffix = pattern.slice(1);
    return segment.endsWith(suffix);
  }

  // * in the middle: "open*json"
  const idx = pattern.indexOf('*');
  if (idx >= 0) {
    const prefix = pattern.slice(0, idx);
    const suffix = pattern.slice(idx + 1);
    return (
      segment.startsWith(prefix) &&
      segment.endsWith(suffix) &&
      segment.length >= prefix.length + suffix.length
    );
  }

  return pattern === segment;
}

/**
 * Recursively matches pattern parts against path parts.
 */
function matchParts(pattern: string[], path: string[]): boolean {
  let pi = 0;
  let pa = 0;

  while (pi < pattern.length && pa < path.length) {
    const seg = pattern[pi];

    if (seg === '**') {
      // If ** is the last pattern segment, it matches everything remaining
      if (pi === pattern.length - 1) {
        return true;
      }

      // Try matching ** against zero or more path segments
      for (let tryPa = pa; tryPa <= path.length; tryPa++) {
        if (matchParts(pattern.slice(pi + 1), path.slice(tryPa))) {
          return true;
        }
      }

      return false;
    } else if (seg === '*') {
      // Matches exactly one segment
      pi++;
      pa++;
    } else if (seg.includes('*')) {
      // Segment contains inline wildcard
      if (!matchSegment(seg, path[pa])) {
        return false;
      }
      pi++;
      pa++;
    } else {
      if (seg !== path[pa]) {
        return false;
      }
      pi++;
      pa++;
    }
  }

  // Handle trailing ** which can match zero segments
  while (pi < pattern.length && pattern[pi] === '**') {
    pi++;
  }

  return pi === pattern.length && pa === path.length;
}

/**
 * Checks whether a URL path matches a glob pattern.
 *
 * Pattern syntax:
 *   - Literal segments match exactly: "/api/v1/users" matches "/api/v1/users"
 *   - "*" matches exactly one path segment: "/api/* /status" matches "/api/users/status"
 *   - "**" matches zero or more path segments: "/api/**" matches "/api", "/api/v1", "/api/v1/users/123"
 */
export function matchPath(pattern: string, path: string): boolean {
  const patternParts = splitPath(pattern);
  const pathParts = splitPath(path);
  return matchParts(patternParts, pathParts);
}

/**
 * Evaluates path rules in order and returns whether the path should be included.
 * First matching rule wins. If no rule matches, the path is included by default.
 */
export function shouldIncludePath(path: string, rules: PathRule[]): boolean {
  for (const rule of rules) {
    if (matchPath(rule.pattern, path)) {
      return rule.action === PathRuleAction.Include;
    }
  }
  return true; // default: include
}

/**
 * Default rules for excluding framework introspection endpoints.
 */
export const INTERNAL_PATH_RULES: PathRule[] = [
  { pattern: '/', action: PathRuleAction.Exclude },
  { pattern: '/_farp/**', action: PathRuleAction.Exclude },
  { pattern: '/_/**', action: PathRuleAction.Exclude },
  { pattern: '/docs/**', action: PathRuleAction.Exclude },
  { pattern: '/docs', action: PathRuleAction.Exclude },
  { pattern: '/openapi*', action: PathRuleAction.Exclude },
  { pattern: '/asyncapi*', action: PathRuleAction.Exclude },
];

/**
 * Combines internal rules (if enabled) with user-defined rules.
 * Internal rules are prepended so they run first.
 */
export function buildPathRules(excludeInternal: boolean, userRules: PathRule[]): PathRule[] {
  if (!excludeInternal) {
    return userRules;
  }
  return [...INTERNAL_PATH_RULES, ...userRules];
}
