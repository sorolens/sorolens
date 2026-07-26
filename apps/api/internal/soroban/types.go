package soroban

// Package soroban provides a typed JSON-RPC 2.0 client for the Stellar Soroban RPC.
//
// Retention window: the public SDF endpoints retain events for approximately
// 24 hours. The maximum queryable range is roughly 7 days (about 100,000
// ledgers at 5-6 seconds per ledger). A newly tracked contract must start its
// backfill from max(latestLedger - 100_000, 1), not from ledger 0. The
// getEvents startLedger must be >= oldestLedger returned by the RPC, or the
// call will return an error. See RESEARCH.md section 1.2 for the full spec.

// ---- JSON-RPC 2.0 envelope ------------------------------------------------

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse[T any] struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      int       `json:"id"`
	Result  T         `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError is an application-level JSON-RPC error returned by the node.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return e.Message
}

// ---- getLatestLedger ------------------------------------------------------

// LatestLedger is the result of getLatestLedger.
type LatestLedger struct {
	ID              string `json:"id"`
	ProtocolVersion int    `json:"protocolVersion"`
	Sequence        uint32 `json:"sequence"`
	// CloseTime is a Unix timestamp as a string.
	CloseTime   string `json:"closeTime"`
	HeaderXDR   string `json:"headerXdr"`
	MetadataXDR string `json:"metadataXdr"`
}

// ---- getEvents ------------------------------------------------------------

// EventFilter constrains which events getEvents returns.
type EventFilter struct {
	// Type is "contract" or "system".
	Type        string     `json:"type"`
	ContractIDs []string   `json:"contractIds,omitempty"`
	Topics      [][]string `json:"topics,omitempty"`
}

type getEventsParams struct {
	StartLedger uint32        `json:"startLedger,omitempty"`
	EndLedger   uint32        `json:"endLedger,omitempty"`
	Filters     []EventFilter `json:"filters"`
	Pagination  *pagination   `json:"pagination,omitempty"`
}

type pagination struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// RPCEvent is a single event as returned by getEvents.
type RPCEvent struct {
	Type string `json:"type"`
	// Ledger is the sequence number of the ledger that closed this event.
	Ledger          uint32 `json:"ledger"`
	LedgerClosedAt  string `json:"ledgerClosedAt"`
	ContractID      string `json:"contractId"`
	ID              string `json:"id"`
	PagingToken     string `json:"pagingToken"`
	// InSuccessfulContractCall is deprecated in recent protocol versions;
	// failed-invocation events are excluded at the protocol level.
	InSuccessfulContractCall bool     `json:"inSuccessfulContractCall"`
	TxHash                   string   `json:"txHash"`
	Topic                    []string `json:"topic"`
	Value                    string   `json:"value"`
	TransactionIndex         int      `json:"transactionIndex"`
	OperationIndex           int      `json:"operationIndex"`
}

// GetEventsResult is the result of getEvents.
type GetEventsResult struct {
	Events        []RPCEvent `json:"events"`
	LatestLedger  uint32     `json:"latestLedger"`
	Cursor        string     `json:"cursor"`
}

// ---- getLedgerEntries -----------------------------------------------------

// LedgerEntry is a single entry returned by getLedgerEntries.
type LedgerEntry struct {
	Key                   string `json:"key"`
	XDR                   string `json:"xdr"`
	LastModifiedLedgerSeq uint32 `json:"lastModifiedLedgerSeq"`
	// LiveUntilLedgerSeq is present only for ContractData and ContractCode
	// entries. Zero means the field was absent (e.g. the entry is archived).
	LiveUntilLedgerSeq uint32 `json:"liveUntilLedgerSeq"`
}

// GetLedgerEntriesResult is the result of getLedgerEntries.
type GetLedgerEntriesResult struct {
	Entries      []LedgerEntry `json:"entries"`
	LatestLedger uint32        `json:"latestLedger"`
}

// ---- getTransaction -------------------------------------------------------

// TransactionResult is the result of getTransaction.
type TransactionResult struct {
	// Status is "SUCCESS", "FAILED", or "NOT_FOUND".
	Status                string `json:"status"`
	LatestLedger          uint32 `json:"latestLedger"`
	LatestLedgerCloseTime int64  `json:"latestLedgerCloseTime"`
	OldestLedger          uint32 `json:"oldestLedger"`
	OldestLedgerCloseTime int64  `json:"oldestLedgerCloseTime"`
	// Fields below are absent when Status is NOT_FOUND.
	Ledger           uint32 `json:"ledger"`
	CreatedAt        int64  `json:"createdAt"`
	ApplicationOrder int    `json:"applicationOrder"`
	FeeBump          bool   `json:"feeBump"`
	EnvelopeXDR      string `json:"envelopeXdr"`
	ResultXDR        string `json:"resultXdr"`
	ResultMetaXDR    string `json:"resultMetaXdr"`
	// DiagnosticEventsXDR is only populated when the RPC node has
	// ENABLE_SOROBAN_DIAGNOSTIC_EVENTS=true. Most public nodes do not.
	DiagnosticEventsXDR []string          `json:"diagnosticEventsXdr"`
	Events              *TransactionEvents `json:"events"`
}

// TransactionEvents holds the event arrays nested under the "events" key.
type TransactionEvents struct {
	TransactionEventsXDR []string   `json:"transactionEventsXdr"`
	ContractEventsXDR    [][]string `json:"contractEventsXdr"`
}

// ---- getNetwork -----------------------------------------------------------

// NetworkInfo is the result of getNetwork.
type NetworkInfo struct {
	// FriendBotURL is only present on Testnet and Futurenet.
	FriendBotURL    string `json:"friendbotUrl"`
	Passphrase      string `json:"passphrase"`
	ProtocolVersion int    `json:"protocolVersion"`
}
