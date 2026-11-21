package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type ContractEventSchemasResponse struct {
	ContractID   string                      `json:"contract_id"`
	EventSchemas []store.ContractEventSchema `json:"event_schemas"`
	DeclaredSpec *json.RawMessage            `json:"declared_spec,omitempty"`
}

// GetContractEventSchemas returns the inferred event schemas for a contract,
// complemented by any declared SEP-48 contract spec.
func (h *Handler) GetContractEventSchemas(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	// Validate contract exists
	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("get contract for event schemas", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	schemas, err := h.Store.GetContractEventSchemas(r.Context(), contractID)
	if err != nil {
		h.Logger.Error("get contract event schemas", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load event schemas")
		return
	}
	if schemas == nil {
		schemas = []store.ContractEventSchema{}
	}

	resp := ContractEventSchemasResponse{
		ContractID:   contractID,
		EventSchemas: schemas,
	}

	// Check if a declared SEP-48 spec exists; if so, declared specs complement/take precedence
	if spec, err := h.Store.GetContractSpec(r.Context(), contractID); err == nil && len(spec.Spec) > 0 {
		raw := json.RawMessage(spec.Spec)
		resp.DeclaredSpec = &raw
	}

	writeJSON(w, http.StatusOK, resp)
}
