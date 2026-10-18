package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types ---------------------------------------------------------

type contractUpgradeResponse struct {
	ContractID string    `json:"contract_id"`
	FromHash   string    `json:"from_hash"`
	ToHash     string    `json:"to_hash"`
	Ledger     int64     `json:"ledger"`
	TxHash     string    `json:"tx_hash,omitempty"`
	At         time.Time `json:"at"`
}

// --- converters --------------------------------------------------------------

func contractUpgradeFromStore(u store.ContractUpgrade) contractUpgradeResponse {
	return contractUpgradeResponse{
		ContractID: u.ContractID,
		FromHash:   u.FromHash,
		ToHash:     u.ToHash,
		Ledger:     u.Ledger,
		TxHash:     u.TxHash,
		At:         u.At,
	}
}

// ListContractUpgrades returns the Wasm-hash upgrade history for a contract,
// newest first. A 404 is returned when no contract with this ID is tracked.
func (h *Handler) ListContractUpgrades(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("get contract", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	upgrades, err := h.Store.ListContractUpgrades(r.Context(), contractID, intQuery(r, "limit", 50))
	if err != nil {
		h.Logger.Error("list contract upgrades", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list contract upgrades")
		return
	}

	resp := make([]contractUpgradeResponse, len(upgrades))
	for i, u := range upgrades {
		resp[i] = contractUpgradeFromStore(u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"upgrades": resp})
}
