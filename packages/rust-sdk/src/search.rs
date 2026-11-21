//! Contract search by ID or label.

use serde::Deserialize;

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::RequestSpec;
use crate::types::Contract;

/// Query parameters for contract search.
#[derive(Debug, Clone)]
pub struct SearchContractsParams {
    /// Contract ID or label to search for.
    pub query: String,
    /// Maximum number of results. When omitted, the server uses its default.
    pub limit: Option<u32>,
}

/// Contracts returned by a search.
#[derive(Debug, Deserialize)]
pub struct SearchContractsResponse {
    /// Matching contracts.
    pub items: Vec<Contract>,
}

pub(crate) fn search_contracts_request(
    _config: &ClientConfig,
    params: &SearchContractsParams,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/search")
        .query_opt("q", Some(params.query.as_str()))
        .query_opt("limit", params.limit))
}

endpoints! {
    /// Search for contracts by ID or label.
    ///
    /// `GET /api/v1/search`.
    fn search_contracts(params: &SearchContractsParams) -> SearchContractsResponse = search_contracts_request;
}
