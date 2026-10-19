//! Strongly-typed request and response models for the Sorolens API.
//!
//! Every struct in this module mirrors a schema in `docs/openapi.yaml`. The
//! types are hand-written (not generated) so the crate stays readable until
//! the OpenAPI codegen tracked in sorolens/sorolens#104 lands.

use std::collections::HashMap;

use chrono::{DateTime, NaiveDate, Utc};
use serde::{Deserialize, Serialize};

macro_rules! impl_str_enum {
    ($name:ty, $($variant:path => $wire:literal),+ $(,)?) => {
        impl $name {
            /// The wire representation used by the API.
            pub const fn as_str(self) -> &'static str {
                match self {
                    $($variant => $wire),+
                }
            }
        }

        impl std::fmt::Display for $name {
            fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                f.write_str(self.as_str())
            }
        }
    };
}

/// The Stellar network a contract is tracked on.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Network {
    /// Stellar testnet.
    Testnet,
    /// Stellar mainnet.
    Mainnet,
    /// Stellar futurenet.
    Futurenet,
    /// A local standalone network.
    Standalone,
}

impl_str_enum!(
    Network,
    Network::Testnet => "testnet",
    Network::Mainnet => "mainnet",
    Network::Futurenet => "futurenet",
    Network::Standalone => "standalone",
);

/// Indexing lifecycle of a tracked contract.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ContractStatus {
    /// Registered but not yet backfilled.
    Pending,
    /// Fully indexed and receiving new data.
    Active,
    /// Historical backfill is in progress.
    Backfilling,
    /// Indexing failed for this contract.
    Error,
    /// Indexing is paused.
    Paused,
}

impl_str_enum!(
    ContractStatus,
    ContractStatus::Pending => "pending",
    ContractStatus::Active => "active",
    ContractStatus::Backfilling => "backfilling",
    ContractStatus::Error => "error",
    ContractStatus::Paused => "paused",
);

/// Execution status of a contract invocation.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum InvocationStatus {
    /// The invocation succeeded.
    Success,
    /// The invocation failed.
    Failed,
    /// The invocation was not found on-chain.
    NotFound,
}

impl_str_enum!(
    InvocationStatus,
    InvocationStatus::Success => "SUCCESS",
    InvocationStatus::Failed => "FAILED",
    InvocationStatus::NotFound => "NOT_FOUND",
);

/// Soroban storage durability class.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Durability {
    /// Temporary storage.
    Temporary,
    /// Persistent storage.
    Persistent,
    /// Contract instance storage.
    Instance,
}

impl_str_enum!(
    Durability,
    Durability::Temporary => "temporary",
    Durability::Persistent => "persistent",
    Durability::Instance => "instance",
);

/// Lifecycle status of a storage entry.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum StorageStatus {
    /// The entry is live on-chain.
    Live,
    /// The entry has been archived.
    Archived,
    /// The entry has been deleted.
    Deleted,
}

impl_str_enum!(
    StorageStatus,
    StorageStatus::Live => "live",
    StorageStatus::Archived => "archived",
    StorageStatus::Deleted => "deleted",
);

/// Severity of a watchdog alert.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub enum Severity {
    /// Informational.
    Info,
    /// Degraded behaviour worth investigating.
    Warning,
    /// Contract is unhealthy.
    Critical,
}

impl_str_enum!(
    Severity,
    Severity::Info => "Info",
    Severity::Warning => "Warning",
    Severity::Critical => "Critical",
);

/// Metric forecasted by the `/forecast` endpoint.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ForecastMetric {
    /// Resource fees charged.
    Fees,
    /// Invocation counts.
    Invocations,
    /// Event counts.
    Events,
}

impl_str_enum!(
    ForecastMetric,
    ForecastMetric::Fees => "fees",
    ForecastMetric::Invocations => "invocations",
    ForecastMetric::Events => "events",
);

/// A tracked Soroban contract.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Contract {
    /// Stellar contract ID (56 characters, starts with `C`).
    pub id: String,
    /// Network the contract lives on.
    pub network: Network,
    /// Human-friendly label.
    pub label: String,
    /// Hex-encoded hash of the deployed Wasm.
    pub wasm_hash: String,
    /// Ledger the contract was created at.
    pub created_at_ledger: i64,
    /// When the historical backfill finished, if it has.
    #[serde(default)]
    pub backfill_complete_at: Option<DateTime<Utc>>,
    /// Indexing lifecycle status.
    pub status: ContractStatus,
    /// When the contract was registered with Sorolens.
    pub added_at: DateTime<Utc>,
}

/// A single contract event.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Event {
    /// Unique event ID.
    pub id: String,
    /// Emitting contract ID.
    pub contract_id: String,
    /// Network the event was emitted on.
    pub network: Network,
    /// Ledger sequence number.
    pub ledger: i32,
    /// Close time of the ledger.
    pub ledger_closed_at: DateTime<Utc>,
    /// Transaction hash.
    pub tx_hash: String,
    /// Event type.
    #[serde(rename = "type")]
    pub event_type: String,
    /// Raw XDR-encoded topics.
    pub topic_xdr: Vec<String>,
    /// Raw XDR-encoded event value.
    pub value_xdr: String,
    /// Decoded topics.
    pub topic_decoded: Vec<serde_json::Value>,
    /// Decoded event value.
    pub value_decoded: serde_json::Value,
    /// Whether the event was emitted by a successful call.
    pub in_successful_call: bool,
}

/// A contract invocation with its resource usage.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Invocation {
    /// Transaction hash.
    pub tx_hash: String,
    /// Invoked contract ID.
    pub contract_id: String,
    /// Network the invocation happened on.
    pub network: Network,
    /// Ledger sequence number.
    pub ledger: i32,
    /// Close time of the ledger.
    pub ledger_closed_at: DateTime<Utc>,
    /// Execution status.
    pub status: InvocationStatus,
    /// Invoked function name.
    pub function_name: String,
    /// Decoded arguments.
    #[serde(default)]
    pub args_decoded: serde_json::Value,
    /// Decoded return value.
    #[serde(default)]
    pub result_decoded: serde_json::Value,
    /// Raw XDR-encoded return value.
    pub result_xdr: String,
    /// Resource fee charged, in stroops.
    pub resource_fee_charged: i64,
    /// CPU instructions consumed.
    pub cpu_insn: i64,
    /// Memory bytes consumed.
    pub mem_byte: i64,
    /// Ledger read bytes.
    pub ledger_read_byte: i64,
    /// Ledger write bytes.
    pub ledger_write_byte: i64,
    /// Ordering of the invocation within its ledger.
    pub application_order: i32,
}

/// One contract storage entry.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct StorageEntry {
    /// Owning contract ID.
    pub contract_id: String,
    /// Network the entry lives on.
    pub network: Network,
    /// Raw XDR-encoded key.
    pub key_xdr: String,
    /// Decoded key, when the indexer could decode it.
    #[serde(default)]
    pub key_decoded: Option<serde_json::Value>,
    /// Raw XDR-encoded value.
    pub value_xdr: String,
    /// Decoded value, when the indexer could decode it.
    #[serde(default)]
    pub value_decoded: Option<serde_json::Value>,
    /// Storage durability class.
    pub durability: Durability,
    /// Ledger the entry is live until.
    pub live_until_ledger: i64,
    /// Ledger the entry was last modified at.
    pub last_modified_ledger: i64,
    /// Lifecycle status.
    pub status: StorageStatus,
    /// When the indexer last saw this entry.
    pub last_seen_at: DateTime<Utc>,
}

/// Global index statistics across all tracked contracts.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct GlobalStats {
    /// Number of tracked contracts.
    pub tracked_contracts: i64,
    /// Total indexed events.
    pub total_events: i64,
    /// Total indexed invocations.
    pub total_invocations: i64,
    /// Total indexed storage entries.
    pub total_storage_entries: i64,
}

/// Per-contract statistics.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ContractStats {
    /// Events indexed for the contract.
    pub event_count: i64,
    /// Invocations indexed for the contract.
    pub invocation_count: i64,
    /// Storage entries indexed for the contract.
    pub storage_count: i64,
    /// Highest ledger the indexer has synced.
    pub last_synced_ledger: i32,
    /// Events seen inside the aggregation window.
    pub window_event_count: i64,
    /// Invocations seen inside the aggregation window.
    pub window_invocation_count: i64,
    /// Human-readable description of the aggregation window.
    pub window_duration: String,
}

/// One point of a forecast series.
#[derive(Debug, Clone, Copy, PartialEq, Serialize, Deserialize)]
pub struct ForecastPoint {
    /// Calendar day the point applies to.
    pub date: NaiveDate,
    /// Point estimate.
    pub value: f64,
    /// Lower bound of the confidence interval.
    pub lower: f64,
    /// Upper bound of the confidence interval.
    pub upper: f64,
}

/// A forecasted metric over a horizon.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ForecastSeries {
    /// Forecasted metric.
    pub metric: ForecastMetric,
    /// Horizon in days.
    pub horizon: i32,
    /// Number of points returned per day.
    pub daily_count: i32,
    /// Forecasted values.
    pub points: Vec<ForecastPoint>,
}

/// Usage forecast for a contract.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Forecast {
    /// Forecasted contract ID.
    pub contract_id: String,
    /// Days of history the forecast is based on.
    pub lookback_days: i32,
    /// One series per metric.
    pub series: Vec<ForecastSeries>,
}

/// Point-in-time snapshot of a contract's storage.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ContractSnapshot {
    /// Snapshotted contract ID.
    pub contract_id: String,
    /// Network the contract lives on.
    pub network: Network,
    /// Ledger the snapshot was taken at.
    pub ledger: i32,
    /// First ledger tracked for the contract.
    pub first_tracked_ledger: i32,
    /// Storage entries live at the requested ledger.
    pub storage: Vec<StorageEntry>,
    /// Last event at or before the requested ledger, if any.
    #[serde(default)]
    pub last_event: Option<Event>,
}

/// A page of tracked contracts.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ContractsPage {
    /// Contracts in this page.
    pub contracts: Vec<Contract>,
    /// Opaque cursor for the next page; empty when exhausted.
    #[serde(default)]
    pub next_cursor: String,
}

/// A page of contract events.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct EventsPage {
    /// Events in this page.
    pub events: Vec<Event>,
    /// Opaque cursor for the next page; empty when exhausted.
    #[serde(default)]
    pub next_cursor: String,
}

/// A page of contract invocations.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct InvocationsPage {
    /// Invocations in this page.
    pub invocations: Vec<Invocation>,
    /// Opaque cursor for the next page; empty when exhausted.
    #[serde(default)]
    pub next_cursor: String,
}

/// A page of contract storage entries.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct StoragePage {
    /// Storage entries in this page.
    pub storage: Vec<StorageEntry>,
    /// Opaque cursor for the next page; empty when exhausted.
    #[serde(default)]
    pub next_cursor: String,
}

/// The most recent events for a contract (`/contracts/{id}/stream`).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct RecentEvents {
    /// The 20 most recent events.
    pub events: Vec<Event>,
}

/// Watchdog summary counts.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct WatchdogStats {
    /// Total monitored contracts.
    pub total_monitored: i64,
    /// Monitored contracts currently healthy.
    pub healthy: i64,
    /// Monitored contracts currently degraded.
    pub degraded: i64,
    /// Monitored contracts currently unresponsive.
    pub unresponsive: i64,
    /// Total open alerts.
    pub total_alerts: i64,
    /// Open alerts with `Critical` severity.
    pub critical_alerts: i64,
}

/// A watchdog alert for a monitored contract.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ContractAlert {
    /// Contract the alert refers to.
    pub contract_id: String,
    /// Alert severity.
    pub severity: Severity,
    /// Human-readable alert message.
    pub message: String,
    /// Ledger the alert was raised at.
    pub ledger: i64,
    /// Transaction hash that triggered the alert.
    pub tx_hash: String,
    /// When the alert was raised.
    pub timestamp: DateTime<Utc>,
}

/// A list of watchdog alerts.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AlertsResponse {
    /// Alerts matching the request.
    pub alerts: Vec<ContractAlert>,
}

/// A contract registered for watchdog monitoring.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct MonitoredContract {
    /// Monitored contract ID.
    pub contract_id: String,
    /// Network the contract lives on.
    pub network: Network,
    /// Human-friendly name.
    pub name: String,
    /// Owner identity.
    pub owner: String,
    /// Current health status.
    pub status: String,
    /// When the watchdog last checked the contract.
    #[serde(default)]
    pub last_check: Option<DateTime<Utc>>,
    /// Seconds between health checks.
    pub check_interval: i64,
    /// When the contract was registered for monitoring.
    pub registered_at: DateTime<Utc>,
    /// When the record was last updated.
    pub updated_at: DateTime<Utc>,
}

/// A page of monitored contracts.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct MonitoredContractsPage {
    /// Monitored contracts in this page.
    pub contracts: Vec<MonitoredContract>,
    /// Opaque cursor for the next page; empty when exhausted.
    #[serde(default)]
    pub next_cursor: String,
}

/// A single watchdog health check.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct HealthCheck {
    /// Checked contract ID.
    pub contract_id: String,
    /// Status reported by the check.
    pub status: String,
    /// Free-form metadata attached by the watchdog.
    pub metadata: String,
    /// Ledger the check was recorded at.
    pub ledger: i64,
    /// Transaction hash of the check.
    pub tx_hash: String,
    /// When the check happened.
    pub timestamp: DateTime<Utc>,
}

/// A list of health checks.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct HealthChecksResponse {
    /// Health checks matching the request.
    pub health_checks: Vec<HealthCheck>,
}

/// Response body of `GET /health`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Health {
    /// `ok` when the service is healthy.
    pub status: String,
}

/// Response body of `GET /readyz`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Readiness {
    /// `ok` when every dependency is reachable, `unavailable` otherwise.
    pub status: String,
    /// Per-dependency status, keyed by dependency name.
    #[serde(default)]
    pub checks: HashMap<String, String>,
}

/// Request body of `POST /api/v1/contracts`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RegisterContractRequest {
    /// Stellar contract ID (56 characters, starts with `C`).
    pub id: String,
    /// Network the contract lives on.
    pub network: Network,
    /// Optional human-friendly label.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub label: Option<String>,
}

impl RegisterContractRequest {
    /// Creates a registration request for `id` on `network`.
    pub fn new(id: impl Into<String>, network: Network) -> Self {
        Self {
            id: id.into(),
            network,
            label: None,
        }
    }

    /// Attaches a human-friendly label.
    pub fn with_label(mut self, label: impl Into<String>) -> Self {
        self.label = Some(label.into());
        self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn network_roundtrips_through_lowercase_wire_names() {
        for (network, wire) in [
            (Network::Testnet, "\"testnet\""),
            (Network::Mainnet, "\"mainnet\""),
            (Network::Futurenet, "\"futurenet\""),
            (Network::Standalone, "\"standalone\""),
        ] {
            assert_eq!(serde_json::to_string(&network).unwrap(), wire);
            assert_eq!(serde_json::from_str::<Network>(wire).unwrap(), network);
            assert_eq!(network.as_str(), wire.trim_matches('"'));
        }
    }

    #[test]
    fn invocation_status_uses_upper_snake_case() {
        assert_eq!(
            serde_json::to_string(&InvocationStatus::NotFound).unwrap(),
            "\"NOT_FOUND\""
        );
        assert_eq!(
            serde_json::from_str::<InvocationStatus>("\"SUCCESS\"").unwrap(),
            InvocationStatus::Success
        );
    }

    #[test]
    fn severity_uses_pascal_case() {
        assert_eq!(
            serde_json::to_string(&Severity::Critical).unwrap(),
            "\"Critical\""
        );
        assert_eq!(
            serde_json::from_str::<Severity>("\"Warning\"").unwrap(),
            Severity::Warning
        );
    }

    #[test]
    fn register_request_omits_an_absent_label() {
        let body = serde_json::to_value(RegisterContractRequest::new(
            "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABSC4",
            Network::Testnet,
        ))
        .unwrap();
        assert_eq!(body["network"], "testnet");
        assert!(body.get("label").is_none());
    }

    #[test]
    fn next_cursor_defaults_when_absent() {
        let page: ContractsPage = serde_json::from_str(r#"{"contracts":[]}"#).unwrap();
        assert_eq!(page.next_cursor, "");
    }
}
