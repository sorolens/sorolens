//! Global and per-contract statistics, plus usage forecasting.
//!
//! Covers the `Stats` and `Forecast` tags of `docs/openapi.yaml`
//! (`/api/v1/stats/global`, `/api/v1/contracts/{id}/stats`, and
//! `/api/v1/contracts/{id}/forecast`).

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};
use crate::types::{ContractStats, Forecast, GlobalStats};

/// Parameters for [`crate::Client::get_contract_stats`].
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct GetContractStatsParams {
    /// Aggregation window. `7d` and `30d` map to 7/30 days; anything else is
    /// treated as 24 hours.
    pub window: Option<String>,
}

impl GetContractStatsParams {
    /// Sets the aggregation window.
    pub fn window(mut self, window: impl Into<String>) -> Self {
        self.window = Some(window.into());
        self
    }
}

/// Parameters for [`crate::Client::get_contract_forecast`].
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct GetContractForecastParams {
    /// Days to forecast (`1..=365`, default 30).
    pub horizon: Option<i32>,
}

impl GetContractForecastParams {
    /// Sets the forecast horizon in days.
    pub fn horizon(mut self, horizon: i32) -> Self {
        self.horizon = Some(horizon);
        self
    }
}

pub(crate) fn global_request(_config: &ClientConfig) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/stats/global"))
}

pub(crate) fn contract_request(
    _config: &ClientConfig,
    id: &str,
    params: &GetContractStatsParams,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/contracts/{}/stats", contract_id(id)?))
            .query_opt("window", params.window.as_deref()),
    )
}

pub(crate) fn forecast_request(
    _config: &ClientConfig,
    id: &str,
    params: &GetContractForecastParams,
) -> Result<RequestSpec, Error> {
    if let Some(horizon) = params.horizon {
        if !(1..=365).contains(&horizon) {
            return Err(Error::InvalidRequest(format!(
                "forecast horizon must be between 1 and 365 days, got {horizon}"
            )));
        }
    }
    Ok(
        RequestSpec::get(format!("/api/v1/contracts/{}/forecast", contract_id(id)?))
            .query_opt("horizon", params.horizon),
    )
}

endpoints! {
    /// Global counts across every tracked contract.
    ///
    /// `GET /api/v1/stats/global`.
    fn get_global_stats() -> GlobalStats = global_request;

    /// Per-contract statistics.
    ///
    /// `GET /api/v1/contracts/{id}/stats`.
    fn get_contract_stats(id: &str, params: &GetContractStatsParams) -> ContractStats =
        contract_request;

    /// Forecasts contract usage over the next N days.
    ///
    /// `GET /api/v1/contracts/{id}/forecast`.
    fn get_contract_forecast(id: &str, params: &GetContractForecastParams) -> Forecast =
        forecast_request;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn global_request_has_no_query() {
        let config = ClientConfig::default();
        let spec = global_request(&config).unwrap();
        assert_eq!(spec.path, "/api/v1/stats/global");
        assert!(spec.query.is_empty());
    }

    #[test]
    fn contract_request_passes_the_window_through_verbatim() {
        let config = ClientConfig::default();
        let params = GetContractStatsParams::default().window("30d");
        let spec = contract_request(&config, "CABC", &params).unwrap();
        assert_eq!(spec.path, "/api/v1/contracts/CABC/stats");
        assert_eq!(spec.query, vec![("window".to_string(), "30d".to_string())]);
    }

    #[test]
    fn forecast_request_rejects_out_of_range_horizons() {
        let config = ClientConfig::default();
        for horizon in [0, -1, 366] {
            assert!(matches!(
                forecast_request(
                    &config,
                    "CABC",
                    &GetContractForecastParams::default().horizon(horizon)
                ),
                Err(Error::InvalidRequest(_))
            ));
        }
        let spec = forecast_request(
            &config,
            "CABC",
            &GetContractForecastParams::default().horizon(7),
        )
        .unwrap();
        assert_eq!(spec.query, vec![("horizon".to_string(), "7".to_string())]);
    }
}
