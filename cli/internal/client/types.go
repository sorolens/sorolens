package client

import "time"

// Contract mirrors the API contract response.
type Contract struct {
	ID                 string     `json:"id"`
	Network            string     `json:"network"`
	Label              string     `json:"label"`
	WasmHash           string     `json:"wasm_hash"`
	CreatedAtLedger    int64      `json:"created_at_ledger"`
	BackfillCompleteAt *time.Time `json:"backfill_complete_at"`
	Status             string     `json:"status"`
	AddedAt            time.Time  `json:"added_at"`
}

// Event mirrors the API event response.
type Event struct {
	ID               string    `json:"id"`
	ContractID       string    `json:"contract_id"`
	Ledger           uint32    `json:"ledger"`
	LedgerClosedAt   time.Time `json:"ledger_closed_at"`
	TxHash           string    `json:"tx_hash"`
	Type             string    `json:"type"`
	TopicXDR         []string  `json:"topic_xdr"`
	ValueXDR         string    `json:"value_xdr"`
	TopicDecoded     []any     `json:"topic_decoded"`
	ValueDecoded     any       `json:"value_decoded"`
	InSuccessfulCall bool      `json:"in_successful_call"`
}

// StorageEntry mirrors the API storage entry response.
type StorageEntry struct {
	ContractID         string    `json:"contract_id"`
	KeyXDR             string    `json:"key_xdr"`
	KeyDecoded         any       `json:"key_decoded"`
	ValueXDR           string    `json:"value_xdr"`
	ValueDecoded       any       `json:"value_decoded"`
	Durability         string    `json:"durability"`
	LiveUntilLedger    int64     `json:"live_until_ledger"`
	LastModifiedLedger int64     `json:"last_modified_ledger"`
	Status             string    `json:"status"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

// Invocation mirrors the API invocation response.
type Invocation struct {
	TxHash             string         `json:"tx_hash"`
	ContractID         string         `json:"contract_id"`
	Ledger             uint32         `json:"ledger"`
	LedgerClosedAt     time.Time      `json:"ledger_closed_at"`
	Status             string         `json:"status"`
	FunctionName       string         `json:"function_name"`
	ArgsDecoded        map[string]any `json:"args_decoded"`
	ResultDecoded      any            `json:"result_decoded"`
	ResultXDR          string         `json:"result_xdr"`
	ResourceFeeCharged int64          `json:"resource_fee_charged"`
	CPUInsn            int64          `json:"cpu_insn"`
	MemByte            int64          `json:"mem_byte"`
	LedgerReadByte     int64          `json:"ledger_read_byte"`
	LedgerWriteByte    int64          `json:"ledger_write_byte"`
	ApplicationOrder   int            `json:"application_order"`
}

// GlobalStats mirrors the API global stats response.
type GlobalStats struct {
	TrackedContracts    int64 `json:"tracked_contracts"`
	TotalEvents         int64 `json:"total_events"`
	TotalInvocations    int64 `json:"total_invocations"`
	TotalStorageEntries int64 `json:"total_storage_entries"`
}

// ContractStats mirrors the API contract stats response.
type ContractStats struct {
	EventCount            int64  `json:"event_count"`
	InvocationCount       int64  `json:"invocation_count"`
	StorageCount          int64  `json:"storage_count"`
	LastSyncedLedger      uint32 `json:"last_synced_ledger"`
	WindowEventCount      int64  `json:"window_event_count"`
	WindowInvocationCount int64  `json:"window_invocation_count"`
	WindowDuration        string `json:"window_duration"`
}

// EventsResponse wraps the paginated events API response.
type EventsResponse struct {
	Events     []Event `json:"events"`
	NextCursor string  `json:"next_cursor"`
}

// StorageResponse wraps the paginated storage API response.
type StorageResponse struct {
	Storage    []StorageEntry `json:"storage"`
	NextCursor string         `json:"next_cursor"`
}

// SorolensError carries the error envelope returned by the API on non-2xx responses.
type SorolensError struct {
	Code      string
	Message   string
	RequestID string
	Status    int
}

func (e *SorolensError) Error() string {
	return e.Message
}
