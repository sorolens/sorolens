//! Watchdog monitoring: summaries, alerts, monitored contracts, and health checks.
//!
//! Covers the `Watchdog` tag of `docs/openapi.yaml` under `/api/v1/watchdog`.

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};
use crate::types::{
    AlertsResponse, HealthChecksResponse, MonitoredContract, MonitoredContractsPage, Network,
    Severity, WatchdogStats,
};

const MAX_ALERT_LIMIT: u32 = 500;

/// Parameters for [`crate::Client::get_watchdog_stats`].
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct GetWatchdogStatsParams {
    /// Restrict the summary to one network.
    pub network: Option<Network>,
}

impl GetWatchdogStatsParams {
    /// Restricts the summary to one network.
    pub fn network(mut self, network: Network) -> Self {
        self.network = Some(network);
        self
    }
}

/// Filters for [`crate::Client::list_watchdog_alerts`].
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct ListWatchdogAlertsParams {
    /// Severity filter.
    pub severity: Option<Severity>,
    /// Restrict to one network.
    pub network: Option<Network>,
    /// Maximum alerts to return (`1..=500`, default 100).
    pub limit: Option<u32>,
}

impl ListWatchdogAlertsParams {
    /// Filters by severity.
    pub fn severity(mut self, severity: Severity) -> Self {
        self.severity = Some(severity);
        self
    }

    /// Restricts the page to one network.
    pub fn network(mut self, network: Network) -> Self {
        self.network = Some(network);
        self
    }

    /// Sets the maximum number of alerts.
    pub fn limit(mut self, limit: u32) -> Self {
        self.limit = Some(limit);
        self
    }
}

/// Filters for [`crate::Client::list_monitored_contracts`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ListMonitoredContractsParams {
    /// Opaque cursor from a previous page.
    pub cursor: Option<String>,
    /// Page size (`1..=200`, default 50).
    pub limit: Option<u32>,
    /// Restrict to one network.
    pub network: Option<Network>,
}

impl ListMonitoredContractsParams {
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
}

/// Parameters for [`crate::Client::list_contract_health_checks`].
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct ListContractHealthChecksParams {
    /// Maximum checks to return (`1..=500`, default 100).
    pub limit: Option<u32>,
}

impl ListContractHealthChecksParams {
    /// Sets the maximum number of health checks.
    pub fn limit(mut self, limit: u32) -> Self {
        self.limit = Some(limit);
        self
    }
}

/// Filters for [`crate::Client::list_contract_alerts`].
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct ListContractAlertsParams {
    /// Severity filter.
    pub severity: Option<Severity>,
    /// Restrict to one network.
    pub network: Option<Network>,
    /// Maximum alerts to return (`1..=500`, default 100).
    pub limit: Option<u32>,
}

impl ListContractAlertsParams {
    /// Filters by severity.
    pub fn severity(mut self, severity: Severity) -> Self {
        self.severity = Some(severity);
        self
    }

    /// Restricts the page to one network.
    pub fn network(mut self, network: Network) -> Self {
        self.network = Some(network);
        self
    }

    /// Sets the maximum number of alerts.
    pub fn limit(mut self, limit: u32) -> Self {
        self.limit = Some(limit);
        self
    }
}

fn check_limit(limit: Option<u32>) -> Result<(), Error> {
    if let Some(limit) = limit {
        if !(1..=MAX_ALERT_LIMIT).contains(&limit) {
            return Err(Error::InvalidRequest(format!(
                "limit must be between 1 and {MAX_ALERT_LIMIT}, got {limit}"
            )));
        }
    }
    Ok(())
}

pub(crate) fn stats_request(
    _config: &ClientConfig,
    params: &GetWatchdogStatsParams,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/watchdog/stats")
        .query_opt("network", params.network.map(Network::as_str)))
}

pub(crate) fn alerts_request(
    _config: &ClientConfig,
    params: &ListWatchdogAlertsParams,
) -> Result<RequestSpec, Error> {
    check_limit(params.limit)?;
    Ok(RequestSpec::get("/api/v1/watchdog/alerts")
        .query_opt("severity", params.severity.map(Severity::as_str))
        .query_opt("network", params.network.map(Network::as_str))
        .query_opt("limit", params.limit))
}

pub(crate) fn contracts_request(
    _config: &ClientConfig,
    params: &ListMonitoredContractsParams,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/watchdog/contracts")
        .query_opt("cursor", params.cursor.as_deref())
        .query_opt("limit", params.limit)
        .query_opt("network", params.network.map(Network::as_str)))
}

pub(crate) fn contract_request(_config: &ClientConfig, id: &str) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get(format!(
        "/api/v1/watchdog/contracts/{}",
        contract_id(id)?
    )))
}

pub(crate) fn health_checks_request(
    _config: &ClientConfig,
    id: &str,
    params: &ListContractHealthChecksParams,
) -> Result<RequestSpec, Error> {
    check_limit(params.limit)?;
    Ok(RequestSpec::get(format!(
        "/api/v1/watchdog/contracts/{}/health",
        contract_id(id)?
    ))
    .query_opt("limit", params.limit))
}

pub(crate) fn contract_alerts_request(
    _config: &ClientConfig,
    id: &str,
    params: &ListContractAlertsParams,
) -> Result<RequestSpec, Error> {
    check_limit(params.limit)?;
    Ok(RequestSpec::get(format!(
        "/api/v1/watchdog/contracts/{}/alerts",
        contract_id(id)?
    ))
    .query_opt("severity", params.severity.map(Severity::as_str))
    .query_opt("network", params.network.map(Network::as_str))
    .query_opt("limit", params.limit))
}

endpoints! {
    /// Watchdog summary counts.
    ///
    /// `GET /api/v1/watchdog/stats`.
    fn get_watchdog_stats(params: &GetWatchdogStatsParams) -> WatchdogStats = stats_request;

    /// Lists watchdog alerts.
    ///
    /// `GET /api/v1/watchdog/alerts`.
    fn list_watchdog_alerts(params: &ListWatchdogAlertsParams) -> AlertsResponse = alerts_request;

    /// Lists contracts registered for watchdog monitoring.
    ///
    /// `GET /api/v1/watchdog/contracts`.
    fn list_monitored_contracts(
        params: &ListMonitoredContractsParams,
    ) -> MonitoredContractsPage = contracts_request;

    /// Fetches one monitored contract.
    ///
    /// `GET /api/v1/watchdog/contracts/{id}`.
    fn get_monitored_contract(id: &str) -> MonitoredContract = contract_request;

    /// Lists health checks recorded for a monitored contract.
    ///
    /// `GET /api/v1/watchdog/contracts/{id}/health`.
    fn list_contract_health_checks(
        id: &str,
        params: &ListContractHealthChecksParams,
    ) -> HealthChecksResponse = health_checks_request;

    /// Lists alerts raised for a monitored contract.
    ///
    /// `GET /api/v1/watchdog/contracts/{id}/alerts`.
    fn list_contract_alerts(
        id: &str,
        params: &ListContractAlertsParams,
    ) -> AlertsResponse = contract_alerts_request;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn alerts_request_encodes_filters() {
        let config = ClientConfig::default();
        let params = ListWatchdogAlertsParams::default()
            .severity(Severity::Critical)
            .network(Network::Testnet)
            .limit(25);
        let spec = alerts_request(&config, &params).unwrap();
        assert_eq!(spec.path, "/api/v1/watchdog/alerts");
        assert_eq!(
            spec.query,
            vec![
                ("severity".to_string(), "Critical".to_string()),
                ("network".to_string(), "testnet".to_string()),
                ("limit".to_string(), "25".to_string()),
            ]
        );
    }

    #[test]
    fn alerts_request_rejects_out_of_range_limits() {
        let config = ClientConfig::default();
        assert!(matches!(
            alerts_request(&config, &ListWatchdogAlertsParams::default().limit(501)),
            Err(Error::InvalidRequest(_))
        ));
    }

    #[test]
    fn contract_routes_are_nested_under_watchdog() {
        let config = ClientConfig::default();
        assert_eq!(
            contract_request(&config, "CABC").unwrap().path,
            "/api/v1/watchdog/contracts/CABC"
        );
        assert_eq!(
            health_checks_request(&config, "CABC", &ListContractHealthChecksParams::default())
                .unwrap()
                .path,
            "/api/v1/watchdog/contracts/CABC/health"
        );
        assert_eq!(
            contract_alerts_request(&config, "CABC", &ListContractAlertsParams::default())
                .unwrap()
                .path,
            "/api/v1/watchdog/contracts/CABC/alerts"
        );
    }

    #[test]
    fn contracts_request_has_no_query_by_default() {
        let config = ClientConfig::default();
        let spec = contracts_request(&config, &ListMonitoredContractsParams::default()).unwrap();
        assert_eq!(spec.path, "/api/v1/watchdog/contracts");
        assert!(spec.query.is_empty());
    }
}
