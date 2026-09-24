package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type contractHealthScoreResponse struct {
	ContractID string           `json:"contract_id"`
	Score      int32            `json:"score"`
	Components healthComponents `json:"components"`
	ComputedAt string           `json:"computed_at"` // RFC3339 UTC
}

type healthComponents struct {
	Uptime      int32 `json:"uptime"`
	ErrorRate   int32 `json:"error_rate"`
	Performance int32 `json:"performance"`
	StorageTTL  int32 `json:"storage_ttl"`
}

func contractHealthScoreFromStore(h store.ContractHealthScore) contractHealthScoreResponse {
	return contractHealthScoreResponse{
		ContractID: h.ContractID,
		Score:      h.Score,
		Components: healthComponents{
			Uptime:      h.ComponentUptime,
			ErrorRate:   h.ComponentErrorRate,
			Performance: h.ComponentPerformance,
			StorageTTL:  h.ComponentStorageTTL,
		},
		ComputedAt: h.ComputedAt.UTC().Format(time.RFC3339),
	}
}

// GetContractHealthScore returns the last cached composite health score
// (issue #137) for a contract alongside its four component scores. The value
// is recomputed on every indexer poll; the API serves the cache, it does not
// re-derive the score.
func (h *Handler) GetContractHealthScore(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	if _, err := h.Store.GetContract(r.Context(), contractID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
			return
		}
		h.Logger.Error("get contract for health score", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load contract")
		return
	}

	health, err := h.Store.GetContractHealthScore(r.Context(), contractID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "health score not yet computed")
			return
		}
		h.Logger.Error("get contract health score", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to load health score")
		return
	}

	writeJSON(w, http.StatusOK, contractHealthScoreFromStore(health))
}
