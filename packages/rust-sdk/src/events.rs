//! Contract events.
//!
//! Covers the `Events` tag of `docs/openapi.yaml`
//! (`/api/v1/contracts/{id}/events` and `/api/v1/contracts/{id}/stream`).

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};
use crate::types::{EventsPage, Network, RecentEvents};

/// Filters for [`crate::Client::list_contract_events`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ListContractEventsParams {
    /// Opaque cursor from a previous page.
    pub cursor: Option<String>,
    /// Page size (`1..=200`, default 50).
    pub limit: Option<u32>,
    /// Restrict to one network.
    pub network: Option<Network>,
    /// Event type filter (wire name: `type`).
    pub event_type: Option<String>,
    /// Inclusive lower ledger bound.
    pub from: Option<i32>,
    /// Inclusive upper ledger bound.
    pub to: Option<i32>,
}

impl ListContractEventsParams {
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

    /// Filters by event type.
    pub fn event_type(mut self, event_type: impl Into<String>) -> Self {
        self.event_type = Some(event_type.into());
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
    params: &ListContractEventsParams,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/contracts/{}/events", contract_id(id)?))
            .query_opt("cursor", params.cursor.as_deref())
            .query_opt("limit", params.limit)
            .query_opt("network", params.network.map(Network::as_str))
            .query_opt("type", params.event_type.as_deref())
            .query_opt("from", params.from)
            .query_opt("to", params.to),
    )
}

pub(crate) fn stream_request(_config: &ClientConfig, id: &str) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get(format!(
        "/api/v1/contracts/{}/stream",
        contract_id(id)?
    )))
}

endpoints! {
    /// Lists contract events, newest first.
    ///
    /// `GET /api/v1/contracts/{id}/events`.
    fn list_contract_events(id: &str, params: &ListContractEventsParams) -> EventsPage =
        list_request;

    /// Fetches the 20 most recent events for a contract.
    ///
    /// `GET /api/v1/contracts/{id}/stream`. This is a short-poll convenience
    /// endpoint, not server-sent events.
    fn stream_contract_events(id: &str) -> RecentEvents = stream_request;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn list_request_includes_ledger_bounds_and_type() {
        let config = ClientConfig::default();
        let params = ListContractEventsParams::default()
            .event_type("transfer")
            .ledger_range(100, 200)
            .limit(5);
        let spec = list_request(&config, "CABC", &params).unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/CABC/events");
        assert_eq!(
            spec.query,
            vec![
                ("limit".to_string(), "5".to_string()),
                ("type".to_string(), "transfer".to_string()),
                ("from".to_string(), "100".to_string()),
                ("to".to_string(), "200".to_string()),
            ]
        );
    }

    #[test]
    fn stream_request_targets_the_stream_route() {
        let config = ClientConfig::default();
        let spec = stream_request(&config, "CABC").unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/CABC/stream");
        assert!(spec.query.is_empty());
    }
}
