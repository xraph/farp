/**
 * Merger module for FARP TypeScript client.
 *
 * Provides OpenAPI schema merging utilities for composing multiple service
 * schemas into a single unified specification.
 */

export type {
  OpenAPISpec,
  Info,
  Contact,
  License,
  Server,
  ServerVariable,
  PathItem,
  Operation,
  Parameter,
  RequestBody,
  Response,
  MediaType,
  Example,
  Header,
  Components,
  SecurityScheme,
  SecurityRequirement,
  Tag,
  ExternalDocumentation,
  ConflictType,
  Conflict,
  MergeResult,
  ServiceSchema,
} from './openapi-types';

export { Merger, defaultMergerConfig } from './merger';
export type { MergerConfig } from './merger';

export {
  prefixTags,
  sortTags,
  applyRouting,
  prefixComponentNames,
  parseOpenAPISchema,
  applyOperationPrefixes,
} from './utils';
