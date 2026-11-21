package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// tokenTransferResponse is one entry in the transfers list.
type tokenTransferResponse struct {
	ID             int64  `json:"id"`
	ContractID     string `json:"contract_id"`
	Network        string `json:"network"`
	TxHash         string `json:"tx_hash"`
	Ledger         uint32 `json:"ledger"`
	LedgerClosedAt string `json:"ledger_closed_at"`
	EventID        string `json:"event_id"`
	TransferType   string `json:"transfer_type"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Amount         string `json:"amount"`
}

// tokenHolderResponse is one entry in the holders list.
type tokenHolderResponse struct {
	Holder    string `json:"holder"`
	Balance   string `json:"balance"`
	UpdatedAt string `json:"updated_at"`
}

func tokenTransferFromStore(t store.TokenTransfer) tokenTransferResponse {
	return tokenTransferResponse{
		ID:             t.ID,
		ContractID:     t.ContractID,
		Network:        t.Network,
		TxHash:         t.TxHash,
		Ledger:         t.Ledger,
		LedgerClosedAt: t.LedgerClosedAt.UTC().Format(time.RFC3339),
		EventID:        t.EventID,
		TransferType:   t.TransferType,
		From:           t.FromAddress,
		To:             t.ToAddress,
		Amount:         t.Amount,
	}
}

// ListTokenTransfers handles GET /api/v1/contracts/{id}/transfers.
// Returns paginated SEP-41 token transfer events for the contract, newest
// first. Supports ?network, ?type (transfer|mint|burn|clawback|approve),
// ?from and ?to ledger bounds, ?limit, and ?cursor.
func (h *Handler) ListTokenTransfers(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	transferType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if transferType != "" {
		valid := map[string]bool{"transfer": true, "mint": true, "burn": true, "clawback": true, "approve": true}
		if !valid[transferType] {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "type must be one of: transfer, mint, burn, clawback, approve")
			return
		}
	}

	from, hasFrom, err := parseLedgerParam(r, "from")
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	to, hasTo, err := parseLedgerParam(r, "to")
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, err.Error())
		return
	}
	if hasFrom && hasTo && from > to {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "from must not exceed to")
		return
	}

	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	limit := pageLimit(r, DefaultPageSize)

	f := store.TokenTransferFilters{
		Network:      network,
		TransferType: transferType,
		From:         from,
		To:           to,
	}

	transfers, next, err := h.Store.ListTokenTransfers(r.Context(), contractID, cursor, limit, f)
	if err != nil {
		h.Logger.Error("list token transfers", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list token transfers")
		return
	}

	resp := make([]tokenTransferResponse, len(transfers))
	for i, t := range transfers {
		resp[i] = tokenTransferFromStore(t)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"transfers":   resp,
		"next_cursor": next,
	})
}

// ListTokenHolders handles GET /api/v1/contracts/{id}/holders.
// Returns token holders sorted by balance descending. Supports ?network,
// ?limit, and ?cursor (last holder address from previous page).
func (h *Handler) ListTokenHolders(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	network, ok := networkParam(r)
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "network must be one of: testnet, mainnet, futurenet, standalone")
		return
	}

	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	limit := pageLimit(r, DefaultPageSize)

	holders, next, err := h.Store.ListTokenHolders(r.Context(), contractID, network, cursor, limit)
	if err != nil {
		h.Logger.Error("list token holders", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list token holders")
		return
	}

	resp := make([]tokenHolderResponse, len(holders))
	for i, h := range holders {
		resp[i] = tokenHolderResponse{
			Holder:    h.Holder,
			Balance:   h.Balance,
			UpdatedAt: h.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"holders":     resp,
		"next_cursor": next,
	})
}
