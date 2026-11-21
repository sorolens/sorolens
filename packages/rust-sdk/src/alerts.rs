//! Grouped and individual alerts from the global alerts endpoint.

use chrono::{DateTime, Utc};
use serde::Deserialize;

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::RequestSpec;
use crate::types::{ContractAlert, Network, Severity};

/// Filters shared by grouped and individual alerts.
#[derive(Debug, Clone, Default)]
pub struct ListAlertsParams {
    /// Restrict results to this contract.
    pub contract_id: Option<String>,
    /// Restrict results to this severity.
    pub severity: Option<Severity>,
    /// Restrict results to this network.
    pub network: Option<Network>,
    /// Continuation cursor from the previous page.
    pub cursor: Option<String>,
    /// Maximum number of results to return.
    pub limit: Option<u32>,
}

/// Deduplicated alert group.
#[derive(Debug, Clone, Deserialize)]
pub struct AlertGroup {
    /// Group ID.
    pub id: i64,
    /// Deduplication key.
    pub group_key: String,
    /// Contract associated with the group.
    pub contract_id: String,
    /// Severity of the grouped alerts.
    pub severity: Severity,
    /// Rule that triggered the alerts.
    pub rule: String,
    /// Number of alerts in the group.
    pub count: i64,
    /// Deduplication window in seconds.
    pub dedupe_window_secs: i64,
    /// When the first alert was seen.
    pub first_seen: DateTime<Utc>,
    /// When the most recent alert was seen.
    pub last_seen: DateTime<Utc>,
    /// Message from the most recent alert.
    pub last_message: String,
    /// Whether the group is eligible for backfill.
    pub backfill_eligible: bool,
}

/// A page of deduplicated alert groups.
#[derive(Debug, Clone, Deserialize)]
pub struct AlertGroupsPage {
    /// Groups in this page.
    pub groups: Vec<AlertGroup>,
    /// Cursor for the next page, or an empty string at the end.
    pub next_cursor: String,
}

/// A page of individual alerts.
#[derive(Debug, Clone, Deserialize)]
pub struct FlatAlertsPage {
    /// Individual alerts in this page.
    pub alerts: Vec<ContractAlert>,
    /// Cursor for the next page, or an empty string at the end.
    pub next_cursor: String,
}

fn alerts_request(params: &ListAlertsParams) -> RequestSpec {
    RequestSpec::get("/api/v1/alerts")
        .query_opt("contract_id", params.contract_id.as_deref())
        .query_opt("severity", params.severity.map(Severity::as_str))
        .query_opt("network", params.network.map(Network::as_str))
        .query_opt("cursor", params.cursor.as_deref())
        .query_opt("limit", params.limit)
}

pub(crate) fn grouped_request(
    _config: &ClientConfig,
    params: &ListAlertsParams,
) -> Result<RequestSpec, Error> {
    Ok(alerts_request(params))
}

pub(crate) fn flat_request(
    _config: &ClientConfig,
    params: &ListAlertsParams,
) -> Result<RequestSpec, Error> {
    Ok(alerts_request(params).query_opt("flat", Some(true)))
}

endpoints! {
    /// Lists deduplicated alert groups.
    fn list_alerts(params: &ListAlertsParams) -> AlertGroupsPage = grouped_request;

    /// Lists individual alerts without deduplication.
    fn list_flat_alerts(params: &ListAlertsParams) -> FlatAlertsPage = flat_request;
}
