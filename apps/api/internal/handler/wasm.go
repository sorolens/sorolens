package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// GetContractWasm handles GET /api/v1/contracts/{id}/wasm.
//
// Returns the cached Wasm binary for the contract with
// Content-Type: application/wasm and Cache-Control: immutable.
// Responds 404 when the contract is unknown or the Wasm has not been
// fetched/cached yet (issue #162).
func (h *Handler) GetContractWasm(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("get contract for wasm", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	cached, err := h.Store.GetContractWasm(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "wasm not fetched yet")
			return
		}
		h.Logger.Error("get contract wasm", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load wasm")
		return
	}

	w.Header().Set("Content-Type", "application/wasm")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Wasm-Hash", cached.WasmHash)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(cached.Code)
}
