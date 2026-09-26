package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// GetContractReportHistory handles
// GET /api/v1/reports/{contract_id}/history?months=12.
//
// Backs the trend chart: one SLA bucket per calendar month, oldest first, so
// the UI renders a series rather than issuing a request per month.
func (h *Handler) GetContractReportHistory(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contract_id")

	months := intQuery(r, "months", 12)
	if months < 1 || months > 24 {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "months must be between 1 and 24")
		return
	}

	s, ok := h.slaStore()
	if !ok {
		writeError(w, r, http.StatusNotImplemented, CodeInternal, errSLANotSupported.Error())
		return
	}

	now := time.Now()
	from, to := store.HistoryRange(months, now)

	checks, err := s.ListHealthChecksInRange(r.Context(), contractID, from, to)
	if err != nil {
		h.Logger.Error("report history: health checks", "contract_id", contractID, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch health checks")
		return
	}
	alerts, err := s.ListAlertsInRange(r.Context(), contractID, from, to)
	if err != nil {
		h.Logger.Error("report history: alerts", "contract_id", contractID, "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch alerts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"contract_id": contractID,
		"months":      store.ComputeMonthlySLAHistory(contractID, months, now, checks, alerts),
	})
}
