# FARP - Forge API Gateway Registration Protocol

[![License](https://img.shields.io/badge/license-MIT%2FApache--2.0-blue.svg)](LICENSE)
[![Rust](https://img.shields.io/badge/rust-1.75%2B-orange.svg)](https://www.rust-lang.org)

**FARP** is a Rust implementation of the Forge API Gateway Registration Protocol - a standardized mechanism for service instances to register their API schemas, health information, and capabilities with API gateways and service discovery systems.

## 🌟 Features

- **Schema-Aware Discovery**: Extend service discovery with API contract information
- **Multi-Protocol Support**: REST (OpenAPI), AsyncAPI, gRPC, GraphQL, oRPC, Avro, and Thrift
- **Gateway Automation**: Enable API gateways to auto-configure routes from schemas
- **Backend Agnostic**: Work with any service discovery backend (in-memory registry included)
- **Type-Safe**: Fully typed with comprehensive error handling
- **Async First**: Built on tokio for high-performance async I/O
- **Zero-Downtime Updates**: Support for schema versioning and compatibility checks
- **Production Ready**: Checksums, validation, compression, and observability built-in

## 📦 Installation

Add FARP to your `Cargo.toml`:

```toml
[dependencies]
farp = "1.0.0"

# With all features
farp = { version = "1.0.0", features = ["full"] }

# With specific providers
farp = { version = "1.0.0", features = ["providers-openapi", "providers-asyncapi", "gateway"] }
```

## 🚀 Quick Start

```rust
use farp::prelude::*;
use std::sync::Arc;

#[tokio::main]
async fn main() -> Result<()> {
    // 1. Create a schema manifest
    let mut manifest = new_manifest("user-service", "v1.0.0", "instance-123");
    manifest.add_capability("rest");
    manifest.endpoints.health = "/health".to_string();

    // 2. Add an OpenAPI schema
    let schema = serde_json::json!({
        "openapi": "3.1.0",
        "info": {"title": "User API", "version": "1.0.0"},
        "paths": {"/users": {"get": {}}}
    });

    let hash = calculate_schema_checksum(&schema)?;
    manifest.add_schema(SchemaDescriptor {
        schema_type: SchemaType::OpenAPI,
        spec_version: "3.1.0".to_string(),
        location: SchemaLocation {
            location_type: LocationType::Inline,
            inline_schema: Some(schema),
            ..Default::default()
        },
        hash,
        content_type: "application/json".to_string(),
        size: 1024,
        ..Default::default()
    });

    manifest.update_checksum()?;
    manifest.validate()?;

    // 3. Register with an in-memory registry
    use farp::registry::memory::MemoryRegistry;
    let registry = Arc::new(MemoryRegistry::new());
    registry.register_manifest(&manifest).await?;

    println!("✅ Manifest registered successfully!");
    Ok(())
}
```

## 🎯 Feature Flags

FARP uses feature flags to minimize dependencies:

- **`default`**: Core types + memory registry
- **`memory-registry`**: In-memory registry implementation
- **`providers-openapi`**: OpenAPI schema provider
- **`providers-asyncapi`**: AsyncAPI schema provider
- **`providers-grpc`**: gRPC/Protocol Buffer provider
- **`providers-graphql`**: GraphQL schema provider
- **`providers-orpc`**: oRPC provider
- **`providers-avro`**: Apache Avro provider
- **`providers-thrift`**: Apache Thrift provider
- **`providers-all`**: All schema providers
- **`gateway`**: Gateway client for route conversion
- **`full`**: Everything enabled

## 📚 Core Concepts

### Schema Manifest

A Schema Manifest describes all API contracts exposed by a service instance:

```rust
pub struct SchemaManifest {
    pub version: String,              // Protocol version
    pub service_name: String,         // Service name
    pub service_version: String,      // Service version
    pub instance_id: String,          // Instance ID
    pub schemas: Vec<SchemaDescriptor>, // API schemas
    pub capabilities: Vec<String>,    // Protocols supported
    pub endpoints: SchemaEndpoints,   // Health/metrics endpoints
    pub updated_at: i64,              // Unix timestamp
    pub checksum: String,             // SHA256 of schemas
}
```

### Schema Descriptor

A Schema Descriptor describes a single API schema:

```rust
pub struct SchemaDescriptor {
    pub schema_type: SchemaType,      // OpenAPI, AsyncAPI, etc.
    pub spec_version: String,         // Spec version
    pub location: SchemaLocation,     // Where to fetch schema
    pub content_type: String,         // MIME type
    pub hash: String,                 // SHA256 hash
    pub size: i64,                    // Size in bytes
}
```

### Location Strategies

Schemas can be retrieved via three strategies:

- **HTTP**: Gateway fetches from service HTTP endpoint
- **Registry**: Gateway fetches from backend KV store
- **Inline**: Schema embedded directly in manifest

## 🔧 Usage Examples

### Create and Register a Manifest

```rust
use farp::prelude::*;

let mut manifest = new_manifest("api-service", "v1.0.0", "instance-1");
manifest.add_capability("rest");
manifest.endpoints.health = "/health".to_string();
manifest.update_checksum()?;

// Register with registry
registry.register_manifest(&manifest).await?;
```

### Watch for Schema Changes

```rust
use farp::prelude::*;

registry.watch_manifests("api-service", Box::new(|event| {
    match event.event_type {
        EventType::Added => println!("Service added: {}", event.manifest.instance_id),
        EventType::Updated => println!("Service updated: {}", event.manifest.instance_id),
        EventType::Removed => println!("Service removed: {}", event.manifest.instance_id),
    }
})).await?;
```

### Gateway Integration

```rust
use farp::gateway::Client;

let client = Client::new(registry);

client.watch_services("api-service", |routes| {
    for route in routes {
        println!("Configure route: {} {} -> {}",
            route.methods.join(","),
            route.path,
            route.target_url
        );
    }
}).await?;
```

### Schema Providers

```rust
use farp::providers::openapi::OpenAPIProvider;
use farp::provider::{SchemaProvider, Application};

struct MyApp;

impl Application for MyApp {
    fn name(&self) -> &str { "my-app" }
    fn version(&self) -> &str { "1.0.0" }
    fn routes(&self) -> Box<dyn std::any::Any + Send + Sync> {
        Box::new(()) // Your route info
    }
}

let provider = OpenAPIProvider::default();
let schema = provider.generate(&MyApp).await?;
```

## 🧪 Testing

Run the test suite:

```bash
# Run all tests
cargo test

# Run tests with all features
cargo test --all-features

# Run specific test
cargo test test_manifest_creation

# Run with output
cargo test -- --nocapture
```

Run the example:

```bash
cargo run --example basic --all-features
```

## 📖 Documentation

Generate and view documentation:

```bash
cargo doc --all-features --open
```

## 🏗️ Architecture

FARP follows a layered architecture:

```
┌──────────────────────────────────────┐
│     Applications & Gateways          │
└─────────────┬────────────────────────┘
              │
┌─────────────▼────────────────────────┐
│        FARP Protocol Core            │
│  - Types & Manifests                 │
│  - Provider & Registry Traits        │
│  - Validation & Checksums            │
└─────────────┬────────────────────────┘
              │
        ┌─────┴──────┐
        │            │
┌───────▼──────┐  ┌─▼────────────────┐
│  Providers   │  │  Registries      │
│  - OpenAPI   │  │  - Memory        │
│  - AsyncAPI  │  │  - (Future: etcd,│
│  - gRPC      │  │    Consul, K8s)  │
│  - GraphQL   │  │                  │
│  - ...       │  │                  │
└──────────────┘  └──────────────────┘
```

## 🤝 Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📄 License

Licensed under either of:

- Apache License, Version 2.0 ([LICENSE-APACHE](LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](LICENSE-MIT) or http://opensource.org/licenses/MIT)

at your option.

## 🔗 Related Projects

- [FARP Go Implementation](https://github.com/xraph/farp) - Official Go implementation
- [Forge Framework](https://github.com/xraph/forge) - Full-featured Go web framework

## 📬 Contact

For questions or feedback, please [open an issue](https://github.com/xraph/farp/issues).

---

Built with ❤️ using Rust

