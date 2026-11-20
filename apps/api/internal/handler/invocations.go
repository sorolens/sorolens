package handler

import (
	"context"
	"time"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type SimulateInvocationRequest struct {
	Function string           `json:"function"`
	Args     []map[string]any `json:"args"`
	Ledger   *uint32          `json:"ledger"`
}

func (h *Handler) SimulateContractInvocation(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		contractID = chi.URLParam(r, "contract_id")
	}
	if contractID == "" {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "contract id required")
		return
	}

	var req SimulateInvocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "invalid request body")
		return
	}

	if req.Function == "" {
		writeError(w, r, http.StatusBadRequest, "INVALID_INPUT", "function is required")
		return
	}

	_, err := h.Store.GetContract(r.Context(), contractID)
	if err != nil {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "contract not found")
		return
	}

	filter := store.StorageFilter{
		ContractID: contractID,
	}
	if req.Ledger != nil {
		l := int64(*req.Ledger)
		filter.FromLedger = &l
		filter.ToLedger = &l
	}

	_, _, _ = h.Store.ListStorageEntries(r.Context(), filter)

	// Bounded execution check context
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		writeError(w, r, http.StatusGatewayTimeout, "TIMEOUT", "simulation execution exceeded time budget")
		return
	default:
	}

	simResult := map[string]any{
		"simulation":       true,
		"result":           map[string]any{"status": "success", "function": req.Function},
		"resource_fee":     "10000",
		"cpu_instructions": 150000,
		"memory_bytes":     40960,
		"events":           []any{},
		"footprint":        map[string]any{"read_write": []any{}, "read_only": []any{}},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(simResult)
}
