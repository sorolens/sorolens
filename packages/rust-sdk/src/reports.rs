//! Contract SLA reports and monthly history.

use chrono::{DateTime, Utc};
use serde::Deserialize;

use crate::client::ClientConfig;
use crate::error::Error;
use crate::macros::endpoints;
use crate::request::{contract_id, RequestSpec};

/// SLA metrics for one calendar month.
#[derive(Debug, Clone, Deserialize)]
pub struct MonthlySla {
    /// Contract these metrics describe.
    pub contract_id: String,
    /// Reporting period in `YYYY-MM` format.
    pub month: String,
    /// Percentage of checks that were healthy.
    pub uptime_pct: f64,
    /// Number of health checks recorded.
    pub total_checks: i64,
    /// Number of healthy checks.
    pub healthy_checks: i64,
    /// Number of outages that began during the month.
    pub incidents: i64,
    /// Mean recovery time in seconds.
    pub mttr_seconds: f64,
    /// Total outage time in seconds.
    pub total_downtime_seconds: f64,
    /// Longest outage in seconds.
    pub longest_outage_seconds: f64,
    /// Whether an outage was ongoing at the end of the reporting period.
    pub ongoing_outage: bool,
    /// Number of critical alerts.
    pub critical_alerts: i64,
    /// Number of warning alerts.
    pub warning_alerts: i64,
    /// Number of informational alerts.
    pub info_alerts: i64,
    /// Total number of alerts.
    pub total_alerts: i64,
    /// Timestamp of the first recorded check.
    pub first_check: Option<DateTime<Utc>>,
    /// Timestamp of the last recorded check.
    pub last_check: Option<DateTime<Utc>>,
}

/// Monthly contract report with signing information.
#[derive(Debug, Clone, Deserialize)]
pub struct ContractReport {
    /// Monthly SLA metrics.
    #[serde(flatten)]
    pub sla: MonthlySla,
    /// Whether the report was signed.
    pub signed: bool,
    /// Signature, when present.
    pub signature: Option<String>,
    /// Signature algorithm, when present.
    pub signature_algorithm: Option<String>,
    /// Server note about the health-check sampling cadence.
    pub sampling_note: Option<String>,
}

/// Monthly SLA history for a contract.
#[derive(Debug, Clone, Deserialize)]
pub struct ReportHistory {
    /// Contract these metrics describe.
    pub contract_id: String,
    /// Monthly metrics, ordered oldest first.
    pub months: Vec<MonthlySla>,
}

pub(crate) fn report_request(
    _config: &ClientConfig,
    id: &str,
    month: Option<&str>,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/reports/{}", contract_id(id)?))
            .query_opt("month", month)
            .query_opt("format", Some("json")),
    )
}

pub(crate) fn history_request(
    _config: &ClientConfig,
    id: &str,
    months: Option<u32>,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/reports/{}/history", contract_id(id)?))
            .query_opt("months", months),
    )
}

endpoints! {
    /// Fetches a monthly SLA report. Omit `month` to use the current month.
    fn get_contract_report(id: &str, month: Option<&str>) -> ContractReport = report_request;

    /// Fetches monthly SLA history. Omit `months` to use the server default.
    fn get_contract_report_history(id: &str, months: Option<u32>) -> ReportHistory = history_request;
}

/// Schedule for a report digest.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize, serde::Serialize)]
#[serde(rename_all = "lowercase")]
pub enum ReportFrequency {
    /// Send a digest every day.
    Daily,
    /// Send a digest once a week.
    Weekly,
}

/// Request body for a new report subscription.
#[derive(Debug, Clone, serde::Serialize)]
pub struct CreateReportSubscriptionRequest {
    /// Email address that receives the digest.
    pub email: String,
    /// How often to send the digest.
    pub frequency: ReportFrequency,
    /// Day for weekly digests, from 0 (Sunday) to 6 (Saturday).
    /// Omit to use the server default of Monday.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub day_of_week: Option<u8>,
}

/// An active report subscription.
#[derive(Debug, Clone, Deserialize)]
pub struct ReportSubscription {
    /// Identifier used to delete the subscription.
    pub id: String,
    /// Email address receiving the digest.
    pub email: String,
    /// Digest schedule.
    pub frequency: ReportFrequency,
    /// Scheduled day of the week, from 0 (Sunday) to 6 (Saturday).
    pub day_of_week: u8,
    /// When the subscription was created.
    pub created_at: DateTime<Utc>,
}

/// Active report subscriptions for an email address.
#[derive(Debug, Clone, Deserialize)]
pub struct ReportSubscriptions {
    /// Subscriptions matching the requested email.
    pub subscriptions: Vec<ReportSubscription>,
}

const SUBSCRIPTIONS_PATH: &str = "/api/v1/reports/subscriptions";

pub(crate) fn create_subscription_request(
    _config: &ClientConfig,
    request: &CreateReportSubscriptionRequest,
) -> Result<RequestSpec, Error> {
    RequestSpec::post(SUBSCRIPTIONS_PATH, request)
}

pub(crate) fn list_subscriptions_request(
    _config: &ClientConfig,
    email: &str,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get(SUBSCRIPTIONS_PATH).query_opt("email", Some(email)))
}

pub(crate) fn delete_subscription_request(
    _config: &ClientConfig,
    id: &str,
) -> Result<RequestSpec, Error> {
    let id = id.trim();
    if id.is_empty() || matches!(id, "." | "..") {
        return Err(Error::InvalidRequest(
            "invalid report subscription ID".into(),
        ));
    }

    let id = crate::request::encode_path_segment(id);
    Ok(RequestSpec::delete(format!("{SUBSCRIPTIONS_PATH}/{id}")))
}

endpoints! {
    /// Creates an email digest subscription.
    fn create_report_subscription(request: &CreateReportSubscriptionRequest) -> ReportSubscription = create_subscription_request;

    /// Lists active digest subscriptions for an email address.
    fn list_report_subscriptions(email: &str) -> ReportSubscriptions = list_subscriptions_request;

    /// Deletes an email digest subscription by ID.
    fn delete_report_subscription(id: &str) -> () = delete_subscription_request;
}

/// File format for a downloaded monthly report.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ReportDownloadFormat {
    /// Comma-separated values.
    Csv,
    /// Portable Document Format.
    Pdf,
}

impl ReportDownloadFormat {
    fn as_str(self) -> &'static str {
        match self {
            Self::Csv => "csv",
            Self::Pdf => "pdf",
        }
    }
}

pub(crate) fn download_request(
    _config: &ClientConfig,
    id: &str,
    month: Option<&str>,
    format: ReportDownloadFormat,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/reports/{}", contract_id(id)?))
            .query_opt("month", month)
            .query_opt("format", Some(format.as_str())),
    )
}

pub(crate) fn badge_request(
    _config: &ClientConfig,
    id: &str,
    month: Option<&str>,
) -> Result<RequestSpec, Error> {
    Ok(
        RequestSpec::get(format!("/api/v1/reports/{}/badge.svg", contract_id(id)?))
            .query_opt("month", month),
    )
}

pub(crate) fn unsubscribe_request(
    _config: &ClientConfig,
    token: &str,
) -> Result<RequestSpec, Error> {
    Ok(RequestSpec::get("/api/v1/reports/unsubscribe").query_opt("token", Some(token)))
}

endpoints! {
    bytes;

    /// Downloads a monthly SLA report as CSV or PDF bytes.
    fn download_contract_report(id: &str, month: Option<&str>, format: ReportDownloadFormat) -> Vec<u8> = download_request;

    /// Fetches the monthly SLA badge as SVG bytes.
    fn get_contract_sla_badge(id: &str, month: Option<&str>) -> Vec<u8> = badge_request;

    /// Unsubscribes using an email-link token and returns the plain-text confirmation as bytes.
    fn unsubscribe_report(token: &str) -> Vec<u8> = unsubscribe_request;
}
