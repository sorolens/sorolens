//! Contract invocations and their resource usage.
//!
//! Covers the `Invocations` tag of `docs/openapi.yaml`
//! (`/api/v1/contracts/{id}/invocations`).

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};
use crate::types::{InvocationStatus, InvocationsPage, Network};

/// Filters for [`crate::Client::list_contract_invocations`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ListContractInvocationsParams {
    /// Opaque cursor from a previous page.
    pub cursor: Option<String>,
    /// Page size (`1..=200`, default 50).
    pub limit: Option<u32>,
    /// Restrict to one network.
    pub network: Option<Network>,
    /// Execution status filter.
    pub status: Option<InvocationStatus>,
    /// Function name filter (wire name: `fn`).
    pub function_name: Option<String>,
    /// Inclusive lower ledger bound.
    pub from: Option<i32>,
    /// Inclusive upper ledger bound.
    pub to: Option<i32>,
}

impl ListContractInvocationsParams {
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

    /// Filters by execution status.
    pub fn status(mut self, status: InvocationStatus) -> Self {
        self.status = Some(status);
        self
    }

    /// Filters by invoked function name.
    pub fn function_name(mut self, function_name: impl Into<String>) -> Self {
        self.function_name = Some(function_name.into());
        self
    }

    /// Restricts the page to ledgers within `from..=to`.
    pub fn ledger_range(mut self, from: i32, to: i32) -> Self {
        self.from = Some(from);
        self.to = Some(to);
        self
    }
}

pub(crate) fn list_request(
    _config: &ClientConfig,
    id: &str,
    params: &ListContractInvocationsParams,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get(format!(
        "/api/v1/contracts/{}/invocations",
        contract_id(id)?
    ))
    .query_opt("cursor", params.cursor.as_deref())
    .query_opt("limit", params.limit)
    .query_opt("network", params.network.map(Network::as_str))
    .query_opt("status", params.status.map(InvocationStatus::as_str))
    .query_opt("fn", params.function_name.as_deref())
    .query_opt("from", params.from)
    .query_opt("to", params.to))
}

endpoints! {
    /// Lists contract invocations, newest first.
    ///
    /// `GET /api/v1/contracts/{id}/invocations`.
    fn list_contract_invocations(
        id: &str,
        params: &ListContractInvocationsParams,
    ) -> InvocationsPage = list_request;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn list_request_maps_function_name_to_fn() {
        let config = ClientConfig::default();
        let params = ListContractInvocationsParams::default()
            .function_name("transfer")
            .status(InvocationStatus::Failed);
        let spec = list_request(&config, "CABC", &params).unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/CABC/invocations");
        assert_eq!(
            spec.query,
            vec![
                ("status".to_string(), "FAILED".to_string()),
                ("fn".to_string(), "transfer".to_string()),
            ]
        );
    }
}
