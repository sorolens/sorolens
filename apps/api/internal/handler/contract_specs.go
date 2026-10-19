package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// contractSpecResponse is the GET /api/v1/contracts/:id/spec body. Spec is a
// json.RawMessage so the parsed interface tree is embedded as-is rather than
// being re-encoded through an intermediate Go type.
type contractSpecResponse struct {
	ContractID string          `json:"contract_id"`
	WasmHash   string          `json:"wasm_hash,omitempty"`
	ParsedAt   time.Time       `json:"parsed_at"`
	Spec       json.RawMessage `json:"spec"`
}

// GetContractSpec returns the cached SEP-48 interface spec for a contract: a
// JSON tree of callable functions with their argument and return types.
//
// It answers 404 both for an untracked contract and for a tracked contract
// whose spec has not been parsed yet. The two cases are distinguished by the
// message so an operator can tell "wrong id" from "wait for the next pass".
func (h *Handler) GetContractSpec(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	spec, err := h.Store.GetContractSpec(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Confirm the contract itself exists so the error is actionable.
			if _, cErr := h.Store.GetContract(r.Context(), contractID); errors.Is(cErr, store.ErrNotFound) {
				writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
				return
			}
			writeError(w, r, http.StatusNotFound, CodeNotFound,
				"no parsed interface spec is available for this contract yet")
			return
		}
		h.Logger.Error("get contract spec", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract spec")
		return
	}

	writeJSON(w, http.StatusOK, contractSpecResponse{
		ContractID: spec.ContractID,
		WasmHash:   spec.WasmHash,
		ParsedAt:   spec.ParsedAt,
		Spec:       spec.Spec,
	})
}
