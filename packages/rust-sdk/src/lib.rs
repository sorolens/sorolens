//! # Sorolens Rust SDK
//!
//! A typed client for the [Sorolens](https://github.com/sorolens/sorolens) API,
//! which indexes Soroban smart contracts: events, invocations, storage,
//! storage TTL, contract analytics, and on-chain watchdog monitoring.
//!
//! The crate is hand-written from [`docs/openapi.yaml`] and mirrors the surface
//! of the generated Go client at `packages/go-client`. It is designed for
//! Soroban test harnesses, off-chain workers, and CLIs written in Rust.
//!
//! [`docs/openapi.yaml`]: https://github.com/sorolens/sorolens/blob/main/docs/openapi.yaml
//!
//! ## Quickstart
//!
//! ```no_run
//! use sorolens_sdk::{
//!     Client, ClientConfig, Error, ListContractsParams, ListWatchdogAlertsParams,
//! };
//!
//! # async fn run() -> Result<(), Error> {
//! let client = Client::new(
//!     ClientConfig::builder()
//!         .base_url("https://api.sorolens.xyz")
//!         .api_key("sl_your_api_key")
//!         .timeout(std::time::Duration::from_secs(10))
//!         .build()?,
//! )?;
//!
//! // Page through tracked contracts.
//! let page = client.list_contracts(&ListContractsParams::default()).await?;
//! for contract in &page.contracts {
//!     println!("{} is {}", contract.id, contract.status);
//! }
//!
//! // Or just grab the critical watchdog alerts.
//! let alerts = client
//!     .list_watchdog_alerts(&ListWatchdogAlertsParams::default())
//!     .await?;
//! println!("{} alerts", alerts.alerts.len());
//! # Ok(())
//! # }
//! ```
//!
//! ## Feature flags
//!
//! | Feature | Default | Description |
//! | --- | --- | --- |
//! | `async` | yes | Asynchronous [`Client`] built on `reqwest`. |
//! | `blocking` | no | Blocking `BlockingClient` for CLIs and harnesses without an async runtime. |
//!
//! Enable both to use either transport in the same build:
//!
//! ```toml
//! sorolens-sdk = { version = "0.1", features = ["blocking"] }
//! ```
//!
//! With `blocking` enabled, the synchronous client mirrors the asynchronous one
//! method for method:
//!
//! ```no_run
//! # #[cfg(feature = "blocking")]
//! # fn main() -> Result<(), sorolens_sdk::Error> {
//! use sorolens_sdk::{GlobalStats, BlockingClient};
//!
//! let client = BlockingClient::builder()
//!     .base_url("https://api.sorolens.xyz")
//!     .build()?;
//! let GlobalStats { tracked_contracts, .. } = client.get_global_stats()?;
//! println!("{tracked_contracts} contracts tracked");
//! # Ok(())
//! # }
//! # #[cfg(not(feature = "blocking"))]
//! # fn main() {}
//! ```
//!
//! ## Authentication
//!
//! Most read endpoints accept an optional API key, sent as
//! `Authorization: Bearer <token>` (or `X-API-Key`). Configure it with
//! [`ClientConfigBuilder::api_key`]. Anonymous reads are allowed on the public
//! endpoints, and the header is simply omitted when no key is set.
//!
//! ## Endpoint coverage
//!
//! | Module | Endpoints |
//! | --- | --- |
//! | `contracts` | list, get, register |
//! | `events` | list, stream |
//! | `invocations` | list |
//! | `storage` | list, snapshot |
//! | `stats` | global stats, contract stats, forecast |
//! | `watchdog` | stats, alerts, monitored contracts, health checks |
//! | `health` | `/health`, `/readyz` |
//!
//! Admin API-key management and the per-user watchlist routes are not covered
//! yet; they are tracked separately.
//!
//! ## Errors
//!
//! Every operation returns [`Result<T, Error>`]. Non-2xx responses become
//! [`Error::Api`] carrying an [`ApiError`] with the HTTP status, machine code,
//! message, and server request id.

#![warn(missing_docs)]
#![forbid(unsafe_code)]

pub mod error;
pub mod types;

mod client;
mod macros;
mod request;

#[cfg(feature = "blocking")]
mod blocking;

#[cfg(any(feature = "async", feature = "blocking"))]
mod contracts;
#[cfg(any(feature = "async", feature = "blocking"))]
mod events;
#[cfg(any(feature = "async", feature = "blocking"))]
mod health;
#[cfg(any(feature = "async", feature = "blocking"))]
mod invocations;
#[cfg(any(feature = "async", feature = "blocking"))]
mod stats;
#[cfg(any(feature = "async", feature = "blocking"))]
mod storage;
#[cfg(any(feature = "async", feature = "blocking"))]
mod watchdog;

pub use client::{ClientConfig, ClientConfigBuilder, DEFAULT_BASE_URL, DEFAULT_USER_AGENT};
pub use error::{ApiError, Error};
pub use types::*;

#[cfg(feature = "async")]
pub use client::{Client, ClientBuilder};

#[cfg(feature = "blocking")]
pub use blocking::{BlockingClient, BlockingClientBuilder};

#[cfg(any(feature = "async", feature = "blocking"))]
pub use contracts::ListContractsParams;
#[cfg(any(feature = "async", feature = "blocking"))]
pub use events::ListContractEventsParams;
#[cfg(any(feature = "async", feature = "blocking"))]
pub use invocations::ListContractInvocationsParams;
#[cfg(any(feature = "async", feature = "blocking"))]
pub use stats::{GetContractForecastParams, GetContractStatsParams};
#[cfg(any(feature = "async", feature = "blocking"))]
pub use storage::{GetContractSnapshotParams, ListContractStorageParams};
#[cfg(any(feature = "async", feature = "blocking"))]
pub use watchdog::{
    GetWatchdogStatsParams, ListContractAlertsParams, ListContractHealthChecksParams,
    ListMonitoredContractsParams, ListWatchdogAlertsParams,
};

/// The crate version, taken from `Cargo.toml`.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_matches_manifest() {
        assert_eq!(VERSION, "0.1.0");
    }

    #[cfg(feature = "async")]
    #[test]
    fn default_base_url_is_production() {
        assert_eq!(DEFAULT_BASE_URL, "https://api.sorolens.xyz");
        assert!(DEFAULT_USER_AGENT.starts_with("sorolens-sdk/"));
    }

    #[cfg(feature = "blocking")]
    #[test]
    fn blocking_and_async_clients_share_configuration() {
        let blocking = BlockingClient::builder()
            .base_url("http://localhost:8080")
            .build()
            .unwrap()
            .into_config();
        let r#async = Client::builder()
            .base_url("http://localhost:8080")
            .build()
            .unwrap()
            .into_config();
        assert_eq!(blocking, r#async);
    }
}
