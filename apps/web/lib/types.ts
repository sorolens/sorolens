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

export interface EventsResponse {
  events: ContractEvent[];
  cursor: string | null;
  has_more: boolean;
}

export interface Invocation {
  tx_hash: string;
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
  cursor: string | null;
  has_more: boolean;
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

export interface ContractSummary {
  id: string;
  network: string;
  label: string | null;
  status: string;
  wasm_hash: string | null;
  added_at: string;
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

export interface CompareStats {
  event_count_24h: number;
  event_count_7d: number;
  invocation_count: number;
  avg_cpu: number;
  avg_fee: number;
  last_activity: string | null;
}

export interface ComparisonData {
  contract: ContractSummary;
  stats: CompareStats;
  health_status: string;
}

export interface ContractStatsApiResponse {
  event_count: number;
  invocation_count: number;
  storage_count: number;
  last_synced_ledger: number;
  window_event_count: number;
  window_invocation_count: number;
  window_duration: string;
}

export interface CreateSubscriptionRequest {
  contract_id: string;
  webhook_url: string;
  severity_filter?: string;
}

export interface AlertSubscription {
  id: string;
  contract_id: string;
  webhook_url: string;
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

