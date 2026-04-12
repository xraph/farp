import type { SchemaType } from './types.js';

/**
 * FarpError is the base error class for all FARP errors.
 */
export class FarpError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'FarpError';
  }
}

/**
 * ManifestNotFoundError is returned when a schema manifest is not found.
 */
export class ManifestNotFoundError extends FarpError {
  constructor(message = 'schema manifest not found', options?: ErrorOptions) {
    super(message, options);
    this.name = 'ManifestNotFoundError';
  }
}

/**
 * SchemaNotFoundError is returned when a schema is not found.
 */
export class SchemaNotFoundError extends FarpError {
  constructor(message = 'schema not found', options?: ErrorOptions) {
    super(message, options);
    this.name = 'SchemaNotFoundError';
  }
}

/**
 * InvalidManifestError is returned when a manifest has invalid format.
 */
export class InvalidManifestError extends FarpError {
  constructor(message = 'invalid manifest format', options?: ErrorOptions) {
    super(message, options);
    this.name = 'InvalidManifestError';
  }
}

/**
 * InvalidSchemaError is returned when a schema has invalid format.
 */
export class InvalidSchemaError extends FarpError {
  constructor(message = 'invalid schema format', options?: ErrorOptions) {
    super(message, options);
    this.name = 'InvalidSchemaError';
  }
}

/**
 * SchemaTooLargeError is returned when a schema exceeds size limits.
 */
export class SchemaTooLargeError extends FarpError {
  constructor(message = 'schema exceeds size limit', options?: ErrorOptions) {
    super(message, options);
    this.name = 'SchemaTooLargeError';
  }
}

/**
 * ChecksumMismatchError is returned when a schema checksum doesn't match.
 */
export class ChecksumMismatchError extends FarpError {
  constructor(message = 'schema checksum mismatch', options?: ErrorOptions) {
    super(message, options);
    this.name = 'ChecksumMismatchError';
  }
}

/**
 * UnsupportedTypeError is returned when a schema type is not supported.
 */
export class UnsupportedTypeError extends FarpError {
  constructor(message = 'unsupported schema type', options?: ErrorOptions) {
    super(message, options);
    this.name = 'UnsupportedTypeError';
  }
}

/**
 * BackendUnavailableError is returned when the backend is unavailable.
 */
export class BackendUnavailableError extends FarpError {
  constructor(message = 'backend unavailable', options?: ErrorOptions) {
    super(message, options);
    this.name = 'BackendUnavailableError';
  }
}

/**
 * IncompatibleVersionError is returned when protocol versions are incompatible.
 */
export class IncompatibleVersionError extends FarpError {
  constructor(message = 'incompatible protocol version', options?: ErrorOptions) {
    super(message, options);
    this.name = 'IncompatibleVersionError';
  }
}

/**
 * InvalidLocationError is returned when a schema location is invalid.
 */
export class InvalidLocationError extends FarpError {
  constructor(message = 'invalid schema location', options?: ErrorOptions) {
    super(message, options);
    this.name = 'InvalidLocationError';
  }
}

/**
 * ProviderNotFoundError is returned when a schema provider is not found.
 */
export class ProviderNotFoundError extends FarpError {
  constructor(message = 'schema provider not found', options?: ErrorOptions) {
    super(message, options);
    this.name = 'ProviderNotFoundError';
  }
}

/**
 * RegistryNotConfiguredError is returned when no registry is configured.
 */
export class RegistryNotConfiguredError extends FarpError {
  constructor(message = 'schema registry not configured', options?: ErrorOptions) {
    super(message, options);
    this.name = 'RegistryNotConfiguredError';
  }
}

/**
 * SchemaFetchFailedError is returned when schema fetch fails.
 */
export class SchemaFetchFailedError extends FarpError {
  constructor(message = 'failed to fetch schema', options?: ErrorOptions) {
    super(message, options);
    this.name = 'SchemaFetchFailedError';
  }
}

/**
 * ValidationFailedError is returned when schema validation fails.
 */
export class ValidationFailedError extends FarpError {
  constructor(message = 'schema validation failed', options?: ErrorOptions) {
    super(message, options);
    this.name = 'ValidationFailedError';
  }
}

/**
 * InstanceNotFoundError is returned when a service instance is not found.
 */
export class InstanceNotFoundError extends FarpError {
  constructor(message = 'service instance not found', options?: ErrorOptions) {
    super(message, options);
    this.name = 'InstanceNotFoundError';
  }
}

/**
 * RegistrationFailedError is returned when service registration fails.
 */
export class RegistrationFailedError extends FarpError {
  constructor(message = 'service registration failed', options?: ErrorOptions) {
    super(message, options);
    this.name = 'RegistrationFailedError';
  }
}

/**
 * DeregistrationFailedError is returned when service deregistration fails.
 */
export class DeregistrationFailedError extends FarpError {
  constructor(message = 'service deregistration failed', options?: ErrorOptions) {
    super(message, options);
    this.name = 'DeregistrationFailedError';
  }
}

/**
 * HealthCheckFailedError is returned when a health check report fails.
 */
export class HealthCheckFailedError extends FarpError {
  constructor(message = 'health check reporting failed', options?: ErrorOptions) {
    super(message, options);
    this.name = 'HealthCheckFailedError';
  }
}

/**
 * DiscoveryUnavailableError is returned when the discovery backend is unreachable.
 */
export class DiscoveryUnavailableError extends FarpError {
  constructor(message = 'discovery backend unavailable', options?: ErrorOptions) {
    super(message, options);
    this.name = 'DiscoveryUnavailableError';
  }
}

/**
 * ManifestFetchFailedError is returned when fetching a manifest from a service fails.
 */
export class ManifestFetchFailedError extends FarpError {
  constructor(message = 'manifest fetch failed', options?: ErrorOptions) {
    super(message, options);
    this.name = 'ManifestFetchFailedError';
  }
}

/**
 * ManifestError represents a manifest-specific error.
 */
export class ManifestError extends FarpError {
  public readonly serviceName: string;
  public readonly instanceId: string;

  constructor(serviceName: string, instanceId: string, cause: Error) {
    super(
      `manifest error for service=${serviceName} instance=${instanceId}: ${cause.message}`,
      { cause },
    );
    this.name = 'ManifestError';
    this.serviceName = serviceName;
    this.instanceId = instanceId;
  }
}

/**
 * SchemaError represents a schema-specific error.
 */
export class SchemaError extends FarpError {
  public readonly schemaType: SchemaType;
  public readonly path: string;

  constructor(schemaType: SchemaType, path: string, cause: Error) {
    super(
      `schema error type=${schemaType} path=${path}: ${cause.message}`,
      { cause },
    );
    this.name = 'SchemaError';
    this.schemaType = schemaType;
    this.path = path;
  }
}

/**
 * ValidationError represents a validation error with field information.
 */
export class ValidationError extends FarpError {
  public readonly field: string;

  constructor(field: string, message: string) {
    super(`validation error: field=${field} message=${message}`);
    this.name = 'ValidationError';
    this.field = field;
  }
}
