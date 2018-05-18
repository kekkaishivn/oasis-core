#![feature(clone_closures)]

#[cfg(not(target_env = "sgx"))]
extern crate grpcio;
#[cfg(not(target_env = "sgx"))]
extern crate rand;

extern crate futures;
extern crate protobuf;
extern crate serde;
extern crate serde_cbor;
extern crate sodalite;

extern crate ekiden_common;
#[cfg(not(target_env = "sgx"))]
extern crate ekiden_compute_api;
extern crate ekiden_enclave_common;
extern crate ekiden_rpc_common;

pub mod backend;
mod secure_channel;
mod client;
mod future;

#[doc(hidden)]
#[macro_use]
pub mod macros;

// Re-export.
pub use client::RpcClient;
pub use future::{ClientFuture, FutureExtra};