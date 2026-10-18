//! Contracts: registration, lookup, and lifecycle.
//!
//! Covers the `Contracts` tag of `docs/openapi.yaml`
//! (`/api/v1/contracts` and `/api/v1/contracts/{id}`).

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};
use crate::types::{Contract, ContractStatus, ContractsPage, Network, RegisterContractRequest};

/// Filters for [`crate::Client::list_contracts`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ListContractsParams {
    /// Opaque cursor from a previous page.
    pub cursor: Option<String>,
    /// Page size (`1..=200`, default 50).
    pub limit: Option<u32>,
    /// Restrict to one network.
    pub network: Option<Network>,
    /// Restrict to one contract status.
    pub status: Option<ContractStatus>,
}

impl ListContractsParams {
    /// Sets the pagination cursor.
    pub fn cursor(mut self, cursor: impl Into<String>) -> Self {
        self.cursor = Some(cursor.into());
        self
    }

    /// Sets the page size.
    pub fn limit(mut self, limit: u32) -> Self {
        self.limit = Some(limit);
        self
    }

    /// Restricts the page to one network.
    pub fn network(mut self, network: Network) -> Self {
        self.network = Some(network);
        self
    }

    /// Restricts the page to one status.
    pub fn status(mut self, status: ContractStatus) -> Self {
        self.status = Some(status);
        self
    }
}

pub(crate) fn list_request(
    _config: &ClientConfig,
    params: &ListContractsParams,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/contracts")
        .query_opt("cursor", params.cursor.as_deref())
        .query_opt("limit", params.limit)
        .query_opt("network", params.network.map(Network::as_str))
        .query_opt("status", params.status.map(ContractStatus::as_str)))
}

pub(crate) fn get_request(_config: &ClientConfig, id: &str) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get(format!(
        "/api/v1/contracts/{}",
        contract_id(id)?
    )))
}

pub(crate) fn register_request(
    _config: &ClientConfig,
    request: &RegisterContractRequest,
) -> Result<RequestSpec, Error> {
    RequestSpec::post("/api/v1/contracts", request)
}

endpoints! {
    /// Lists tracked contracts.
    ///
    /// `GET /api/v1/contracts`.
    fn list_contracts(params: &ListContractsParams) -> ContractsPage = list_request;

    /// Fetches a single tracked contract.
    ///
    /// `GET /api/v1/contracts/{id}`. Returns
    /// [`Error::Api`](crate::Error::Api) with status 404 when unknown.
    fn get_contract(id: &str) -> Contract = get_request;

    /// Registers a contract for tracking.
    ///
    /// `POST /api/v1/contracts`. The contract starts in
    /// [`ContractStatus::Pending`] until backfill completes.
    fn register_contract(request: &RegisterContractRequest) -> Contract = register_request;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn list_request_encodes_filters() {
        let config = ClientConfig::default();
        let params = ListContractsParams::default()
            .limit(10)
            .network(Network::Mainnet)
            .status(ContractStatus::Active);
        let spec = list_request(&config, &params).unwrap();
        assert_eq!(spec.path, "/api/v1/contracts");
        assert_eq!(
            spec.query,
            vec![
                ("limit".to_string(), "10".to_string()),
                ("network".to_string(), "mainnet".to_string()),
                ("status".to_string(), "active".to_string()),
            ]
        );
    }

    #[test]
    fn get_request_rejects_an_empty_id() {
        let config = ClientConfig::default();
        assert!(matches!(
            get_request(&config, " "),
            Err(Error::InvalidRequest(_))
        ));
    }

    #[test]
    fn get_request_encodes_the_id() {
        let config = ClientConfig::default();
        let spec = get_request(&config, "a/b").unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/a%2Fb");
    }
}
