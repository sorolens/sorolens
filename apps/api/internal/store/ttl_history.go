package store

import (
	"context"
	"time"
)

// TTLOpType classifies the on-chain footprint operation recorded in the TTL
// history table.
const (
	TTLOpExtend  = "extend_ttl"
	TTLOpRestore = "restore_footprint"
)

// TTLOperation records one ExtendFootprintTTL or RestoreFootprint transaction
// that affected a contract's persistent storage.
type TTLOperation struct {
	ID            string     `json:"id"`
	ContractID    string     `json:"contract_id"`
	Network       string     `json:"network"`
	TxHash        string     `json:"tx_hash"`
	Ledger        uint32     `json:"ledger"`
	LedgerClosedAt time.Time `json:"ledger_closed_at"`
	// OpType is TTLOpExtend or TTLOpRestore.
	OpType        string     `json:"op_type"`
	// KeyXDR is the base64-encoded LedgerKey XDR for the entry that was
	// extended or restored, or empty when the operation applies to the whole
	// contract footprint.
	KeyXDR        string     `json:"key_xdr,omitempty"`
	OldLiveLedger *uint32    `json:"old_live_ledger,omitempty"`
	NewLiveLedger *uint32    `json:"new_live_ledger,omitempty"`
	InsertedAt    time.Time  `json:"inserted_at"`
}

// TTLHistoryStore is the read/write surface for per-contract TTL operation
// history (issue #804).
type TTLHistoryStore interface {
	// RecordTTLOperation inserts a TTL operation row, ignoring duplicates by
	// (contract_id, tx_hash, key_xdr) so re-indexing is idempotent.
	RecordTTLOperation(ctx context.Context, op TTLOperation) error
	// ListTTLHistory returns a cursor-paginated list of TTL operations for a
	// contract, newest first. cursor is the ID of the last row on the previous
	// page (empty for the first page). The returned cursor is empty when there
	// are no more pages.
	ListTTLHistory(ctx context.Context, contractID, cursor string, limit int) ([]TTLOperation, string, error)
}
