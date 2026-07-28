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

