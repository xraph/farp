//! Schema provider implementations

#[cfg(feature = "providers-openapi")]
pub mod openapi;

#[cfg(feature = "providers-asyncapi")]
pub mod asyncapi;

#[cfg(feature = "providers-grpc")]
pub mod grpc;

#[cfg(feature = "providers-graphql")]
pub mod graphql;

#[cfg(feature = "providers-orpc")]
pub mod orpc;

#[cfg(feature = "providers-avro")]
pub mod avro;

#[cfg(feature = "providers-thrift")]
pub mod thrift;
