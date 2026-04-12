import { describe, it, expect } from 'vitest';
import {
  isCompatible,
  getVersion,
  PROTOCOL_VERSION,
  PROTOCOL_MAJOR,
  PROTOCOL_MINOR,
  PROTOCOL_PATCH,
} from '../src/index';

describe('version', () => {
  describe('getVersion', () => {
    it('returns current protocol version info', () => {
      const v = getVersion();
      expect(v.version).toBe(PROTOCOL_VERSION);
      expect(v.major).toBe(PROTOCOL_MAJOR);
      expect(v.minor).toBe(PROTOCOL_MINOR);
      expect(v.patch).toBe(PROTOCOL_PATCH);
    });
  });

  describe('isCompatible', () => {
    it('same version is compatible', () => {
      expect(isCompatible(PROTOCOL_VERSION)).toBe(true);
    });

    it('same major, lower minor is compatible', () => {
      expect(isCompatible(`${PROTOCOL_MAJOR}.0.0`)).toBe(true);
    });

    it('same major, same minor, different patch is compatible', () => {
      expect(isCompatible(`${PROTOCOL_MAJOR}.${PROTOCOL_MINOR}.5`)).toBe(true);
    });

    it('different major version is incompatible', () => {
      expect(isCompatible('2.0.0')).toBe(false);
      expect(isCompatible('0.1.0')).toBe(false);
    });

    it('higher minor version is incompatible', () => {
      expect(isCompatible(`${PROTOCOL_MAJOR}.${PROTOCOL_MINOR + 1}.0`)).toBe(false);
    });

    it('invalid version strings return false', () => {
      expect(isCompatible('')).toBe(false);
      expect(isCompatible('abc')).toBe(false);
      expect(isCompatible('1.0')).toBe(false);
      expect(isCompatible('1.0.0.0')).toBe(false);
      expect(isCompatible('a.b.c')).toBe(false);
    });
  });
});
