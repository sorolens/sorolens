export interface Contract {
  id: string;
  network: string;
  label: string | null;
  status: string;
  wasm_hash: string | null;
  backfill_complete_at: string | null;
  sync: {
    last_ledger: number;
    last_run_at: string;
  } | null;
  storage_entry_count: number;
  expiring_entry_count: number;
}

export interface ContractDetail extends Contract {
  added_at: string;
}

export interface ContractEvent {
  id: string;
  ledger: number;
  ledger_closed_at: string;
  tx_hash: string;
  type: string;
  topic_decoded: unknown[] | null;
  topic_xdr: string[];
  value_decoded: unknown | null;
  value_xdr: string;
  in_successful_call: boolean;
}

/** An event from the cross-contract feed (GET /api/v1/events). */
export interface GlobalEvent extends ContractEvent {
  contract_id: string;
  network: string;
}

export interface GlobalEventsResponse {
  events: GlobalEvent[];
  /** Opaque cursor for the next (older) page; empty on the last page. */
  next_cursor: string;
}

export type EventType = "contract" | "system" | "diagnostic";

export interface EventsResponse {
  events: ContractEvent[];
  cursor: string | null;
  has_more: boolean;
}

export interface Invocation {
  tx_hash: string;
  contract_id: string;
  network: string;
  ledger: number;
  ledger_closed_at: string;
  status: string;
  function_name: string | null;
  args_decoded: Record<string, unknown> | null;
  result_decoded: unknown | null;
  resource_fee_charged: number;
  cpu_insn: number;
  mem_byte: number;
  ledger_read_byte: number;
  ledger_write_byte: number;
}

export interface InvocationsResponse {
  invocations: Invocation[];
  // The API returns `next_cursor` (empty when there are no further pages).
  next_cursor: string | null;
}

export interface StorageEntry {
  key_xdr: string;
  key_decoded: string | null;
  value_xdr: string | null;
  value_decoded: unknown | null;
  durability: string;
  live_until_ledger: number | null;
  ledgers_until_expiry: number | null;
  status: string;
  last_modified_ledger: number | null;
}

export interface StorageResponse {
  current_ledger: number;
  entries: StorageEntry[];
  cursor: string | null;
  has_more: boolean;
}

export interface ContractStats {
  total_events: number;
  total_invocations: number;
  storage_entry_count: number;
  expiring_entry_count: number;
}

export interface VolumePoint {
  date: string;
  ledger: number;
  count: number;
}

export interface StatsResponse {
  event_volume: VolumePoint[];
  invocation_count: VolumePoint[];
  stats: ContractStats;
}

/** One day of averaged per-invocation resource usage (issue #184). */
export interface ResourceTrendPoint {
  date: string;
  avg_cpu_insn: number;
  avg_mem_byte: number;
  avg_fee: number;
  count: number;
}

export interface ContractSummary {
  id: string;
  network: string;
  label: string | null;
  status: string;
  wasm_hash: string | null;
  added_at: string;
  last_activity_at: string | null;
}

export interface ContractsListResponse {
  contracts: ContractSummary[];
  cursor: string | null;
  has_more: boolean;
}

export interface TrackContractRequest {
  id: string;
  label?: string;
}

export type TimeWindow = "24h" | "7d" | "30d" | "all";

// ---- watchdog --------------------------------------------------------------

export type HealthStatus = "Healthy" | "Degraded" | "Unresponsive" | string;
export type AlertSeverity = "Info" | "Warning" | "Critical";
export type UptimeWindow = "24h" | "7d" | "30d";

export interface UptimeResponse {
  contract_id: string;
  window: UptimeWindow;
  /** Uptime percentage in the range [0, 100] with up to 2 decimal places. */
  uptime_pct: number;
}

/**
 * One contract's service-level summary for a calendar month (issue #266).
 * Derived from watchdog health checks and alerts, not from a separate source.
 */
export interface MonthlySLA {
  contract_id: string;
  /** Reporting period, YYYY-MM (UTC). */
  month: string;
  /** Healthy checks / total checks * 100. Zero when there are no checks. */
  uptime_pct: number;
  total_checks: number;
  healthy_checks: number;
  /** Outages: transitions from Healthy into any other status. */
  incidents: number;
  /** Mean time to recovery in seconds, across incidents that recovered. */
  mttr_seconds: number;
  total_downtime_seconds: number;
  longest_outage_seconds: number;
  /** True when the month ends mid-incident, so MTTR excludes that incident. */
  ongoing_outage: boolean;
  critical_alerts: number;
  warning_alerts: number;
  info_alerts: number;
  total_alerts: number;
  first_check: string | null;
  last_check: string | null;
}

export interface SLAHistoryResponse {
  contract_id: string;
  /** Oldest first, so it maps straight onto a chart's x-axis. */
  months: MonthlySLA[];
}

export type ReportFormat = "json" | "csv" | "pdf";

export interface MonitoredContract {
  contract_id: string;
  network: string;
  name: string;
  owner: string;
  status: HealthStatus;
  last_check: string | null;
  check_interval: number;
  registered_at: string;
  updated_at: string;
}

export interface MonitoredContractsResponse {
  contracts: MonitoredContract[];
  next_cursor: string;
}

export interface HealthCheck {
  contract_id: string;
  status: HealthStatus;
  metadata: string;
  ledger: number;
  tx_hash: string;
  timestamp: string;
}

export interface HealthChecksResponse {
  health_checks: HealthCheck[];
}

export interface ContractAlert {
  contract_id: string;
  severity: AlertSeverity;
  message: string;
  ledger: number;
  tx_hash: string;
  timestamp: string;
}

export interface AlertsResponse {
  alerts: ContractAlert[];
  /** Cursor for the next page; empty when the feed is exhausted. */
  next_cursor: string;
}

export interface WatchdogStats {
  total_monitored: number;
  healthy: number;
  degraded: number;
  unresponsive: number;
  total_alerts: number;
  critical_alerts: number;
}

// ---- snapshot / replay ------------------------------------------------------

export interface SnapshotStorageEntry extends StorageEntry {
  network: string;
}

export interface HealthScoreResponse {
  contract_id: string;
  score: number;
  components: {
    uptime: number;
    error_rate: number;
    performance: number;
    storage_ttl: number;
  };
  computed_at: string;
}

export interface ContractSnapshot {
  contract_id: string;
  network: string;
  ledger: number;
  first_tracked_ledger: number;
  storage: SnapshotStorageEntry[];
  last_event: {
    id: string;
    ledger: number;
    tx_hash: string;
    type: string;
    ledger_closed_at: string;
    value_decoded: unknown;
    value_xdr: string;
  } | null;
}

export interface GlobalStats {
  tracked_contracts: number;
  total_events: number;
  total_invocations: number;
  total_storage_entries: number;
}

export interface WatchlistItem {
  contract_id: string;
  added_at: string;
}

export interface WatchlistResponse {
  items: WatchlistItem[];
}

export interface WatchlistStatusResponse {
  in_watchlist: boolean;
}

// ---- comparison ------------------------------------------------------------

/** One hour bucket of event volume for the comparison sparkline. */
export interface CompareVolumePoint {
  /** RFC3339 UTC hour start. */
  timestamp: string;
  count: number;
}

/** One contract's unified comparison metrics from GET /api/v1/compare. */
export interface CompareContractEntry {
  id: string;
  network: string;
  label: string;
  status: string;
  /** Whether the contract is registered in Sorolens. */
  tracked: boolean;
  /** Whether any indexed data (or a health score) exists yet. */
  has_data: boolean;
  event_count: number;
  invocation_count: number;
  avg_cpu: number;
  avg_fee: number;
  /** Cached composite health score, or null when not computed yet. */
  health_score: number | null;
  last_synced_ledger: number;
  event_volume: CompareVolumePoint[];
  /** Set when this contract's lookups failed while others succeeded. */
  error?: string;
}

export interface CompareResponse {
  window: string;
  contracts: CompareContractEntry[];
}

export type ChannelType = "webhook" | "slack" | "discord" | "pagerduty";

export interface CreateSubscriptionRequest {
  contract_id: string;
  channel_type?: ChannelType;
  /** Required for webhook, slack and discord; optional for pagerduty. */
  webhook_url?: string;
  /** PagerDuty integration key (pagerduty only). */
  routing_key?: string;
  severity_filter?: string;
}

/** Secrets are never returned: webhook_url is masked for slack/discord. */
export interface AlertSubscription {
  id: string;
  contract_id: string;
  channel_type: ChannelType;
  webhook_url: string;
  has_routing_key: boolean;
  severity_filter: string;
  created_at: string;
  updated_at: string;
}

export interface SubscriptionsResponse {
  subscriptions: AlertSubscription[];
}

// ---- source verification ---------------------------------------------------

export interface VerificationDiagnostic {
  code: string;
  severity: string;
  message: string;
  hint?: string;
}

export interface ContractVerification {
  contract_id: string;
  status: string;
  matched: boolean;
  on_chain_hash?: string;
  compiled_wasm_hash?: string;
  source: {
    kind: string;
    ref?: string;
    digest?: string;
  };
  toolchain: {
    stellar?: string;
    rustc?: string;
    cargo?: string;
  };
  diagnostics: VerificationDiagnostic[];
  build_log?: string;
  submitted_at: string;
  verified_at?: string;
  updated_at: string;
}

