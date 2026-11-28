package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type ttlOperationResponse struct {
	ID             string  `json:"id"`
	ContractID     string  `json:"contract_id"`
	Network        string  `json:"network"`
	TxHash         string  `json:"tx_hash"`
	Ledger         uint32  `json:"ledger"`
	LedgerClosedAt string  `json:"ledger_closed_at"`
	OpType         string  `json:"op_type"`
	KeyXDR         string  `json:"key_xdr,omitempty"`
	OldLiveLedger  *uint32 `json:"old_live_ledger,omitempty"`
	NewLiveLedger  *uint32 `json:"new_live_ledger,omitempty"`
	InsertedAt     string  `json:"inserted_at"`
}

func ttlOperationFromStore(op store.TTLOperation) ttlOperationResponse {
	return ttlOperationResponse{
		ID:             op.ID,
		ContractID:     op.ContractID,
		Network:        op.Network,
		TxHash:         op.TxHash,
		Ledger:         op.Ledger,
		LedgerClosedAt: op.LedgerClosedAt.UTC().Format("2006-01-02T15:04:05Z"),
		OpType:         op.OpType,
		KeyXDR:         op.KeyXDR,
		OldLiveLedger:  op.OldLiveLedger,
		NewLiveLedger:  op.NewLiveLedger,
		InsertedAt:     op.InsertedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// ListTTLHistory handles GET /api/v1/contracts/{id}/ttl-history.
//
// Returns a cursor-paginated list of TTL extension and restoration operations
// for the given contract, newest first.
func (h *Handler) ListTTLHistory(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	rawCursor, ok := cursor.Decode(r.URL.Query().Get("cursor"))
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid cursor")
		return
	}

	ops, nextRaw, err := h.Store.ListTTLHistory(r.Context(), contractID, rawCursor, pageLimit(r, DefaultPageSize))
	if err != nil {
		h.Logger.Error("list ttl history", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list TTL history")
		return
	}
	resp := make([]ttlOperationResponse, len(ops))
	for i, op := range ops {
		resp[i] = ttlOperationFromStore(op)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"operations":  resp,
		"next_cursor": cursor.Encode(nextRaw),
	})
}
