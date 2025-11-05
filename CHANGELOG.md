# Changelog

All notable changes to FARP will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 1.0.0 (2025-11-05)


### Features

* add Apache License 2.0 to the project ([5b391f2](https://github.com/xraph/farp/commit/5b391f2db7db51187d3d35f1a7f051c59307a358))
* Add initial implementation of FARP protocol ([fc0162b](https://github.com/xraph/farp/commit/fc0162ba09a047b149df347b71d7af0693cca2e6))
* enhance base URL resolution in gateway client ([d7a6ecc](https://github.com/xraph/farp/commit/d7a6ecccd1817e193289bcfff39ff4752de822c2))
* enhance schema merging capabilities in AsyncAPI, OpenAPI, and oRPC providers ([e22c4ea](https://github.com/xraph/farp/commit/e22c4ea8cf0f1d76968f03ca6c75be705177cd9c))
* implement HTTP schema fetching in gateway client ([fdb3dfa](https://github.com/xraph/farp/commit/fdb3dfa42373e36a541f68bebe0e36e8e7c15087))
* Initialize FARP protocol with core components ([2148b74](https://github.com/xraph/farp/commit/2148b74009a67a5210bca5efe751aa22e49475a4))


### Bug Fixes

* improve CI checks for TODOs and debug statements ([b8ca709](https://github.com/xraph/farp/commit/b8ca70916aa9aed97586d1a83041f99acb584616))


### Documentation

* Update README with author information ([6cc17c2](https://github.com/xraph/farp/commit/6cc17c208c5938ef2adcb15be7ff9af8fda0d831))

## [1.0.1](https://github.com/xraph/farp/compare/v1.0.0...v1.0.1) (2025-11-05)


### Documentation

* Update README with author information ([6cc17c2](https://github.com/xraph/farp/commit/6cc17c208c5938ef2adcb15be7ff9af8fda0d831))

## 1.0.0 (2025-11-05)


### Features

* Add initial implementation of FARP protocol ([fc0162b](https://github.com/xraph/farp/commit/fc0162ba09a047b149df347b71d7af0693cca2e6))
* Initialize FARP protocol with core components ([2148b74](https://github.com/xraph/farp/commit/2148b74009a67a5210bca5efe751aa22e49475a4))

# Changelog

All notable changes to FARP will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/0.0.1/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.1] - 2025-11-01

### Added

- Core FARP protocol specification
- Schema provider interface
- Registry interface and in-memory implementation
- Multi-protocol support (OpenAPI, AsyncAPI, gRPC, GraphQL, oRPC, Thrift, Avro)
- Backend-agnostic discovery integration
- Gateway client library
- Comprehensive documentation
- Examples and integration tests
- Zero-config mDNS/Bonjour support
- Schema manifest validation
- Checksum-based schema verification
- Health and metrics endpoint registration

### Documentation

- Complete protocol specification
- Architecture documentation
- Provider implementation guide
- Gateway integration examples
- mDNS service type configuration guide

[0.0.1]: https://github.com/xraph/farp/releases/tag/v0.0.1
