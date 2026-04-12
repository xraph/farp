import type { VersionInfo } from './types.js';

/** Current FARP protocol version (semver). */
export const PROTOCOL_VERSION = '1.1.0';

/** Major version number. */
export const PROTOCOL_MAJOR = 1;

/** Minor version number. */
export const PROTOCOL_MINOR = 1;

/** Patch version number. */
export const PROTOCOL_PATCH = 0;

/**
 * Returns the current protocol version information.
 */
export function getVersion(): VersionInfo {
  return {
    version: PROTOCOL_VERSION,
    major: PROTOCOL_MAJOR,
    minor: PROTOCOL_MINOR,
    patch: PROTOCOL_PATCH,
  };
}

/**
 * Checks if a manifest version is compatible with this protocol version.
 * Compatible means the major version matches and the manifest's minor version
 * is less than or equal to the protocol's minor version.
 */
export function isCompatible(manifestVersion: string): boolean {
  const parts = manifestVersion.split('.');
  if (parts.length !== 3) {
    return false;
  }

  const major = parseInt(parts[0], 10);
  const minor = parseInt(parts[1], 10);
  const patch = parseInt(parts[2], 10);

  if (isNaN(major) || isNaN(minor) || isNaN(patch)) {
    return false;
  }

  // Major version must match
  if (major !== PROTOCOL_MAJOR) {
    return false;
  }

  // Protocol must support manifest's minor version or higher
  return minor <= PROTOCOL_MINOR;
}
