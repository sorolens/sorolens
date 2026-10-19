//! Health and readiness probes.
//!
//! Covers the `Health` tag of `docs/openapi.yaml` (`/health` and `/readyz`).

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::RequestSpec;
use crate::types::{Health, Readiness};

pub(crate) fn health_request(_config: &ClientConfig) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/health"))
}

pub(crate) fn readyz_request(_config: &ClientConfig) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/readyz"))
}

endpoints! {
    /// Liveness probe.
    ///
    /// `GET /health`. Never requires authentication.
    fn health() -> Health = health_request;

    /// Readiness probe covering Postgres and Redis.
    ///
    /// `GET /readyz`. When a dependency is unavailable the API answers `503`
    /// with a per-dependency `checks` map, surfaced as
    /// [`Error::Api`](crate::Error::Api).
    fn readyz() -> Readiness = readyz_request;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn probes_hit_the_root_paths() {
        let config = ClientConfig::default();
        assert_eq!(health_request(&config).unwrap().path, "/health");
        assert_eq!(readyz_request(&config).unwrap().path, "/readyz");
    }
}
