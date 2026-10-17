package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// resourceTrendBucketResponse is one day of averaged per-invocation resource
// usage, as returned by GET /api/v1/contracts/{id}/resource-trend.
type resourceTrendBucketResponse struct {
	Day         string  `json:"date"`
	AvgCPUInsn  float64 `json:"avg_cpu_insn"`
	AvgMemByte  float64 `json:"avg_mem_byte"`
	AvgFee      float64 `json:"avg_fee"`
	Count       int     `json:"count"`
}

// ResourceTrend handles GET /api/v1/contracts/{id}/resource-trend.
//
// It returns one bucket per calendar day for the most recent `days` days,
// averaging cpu_insn, mem_byte and resource_fee_charged across the contract's
// invocations. Days with no invocations are zero-filled so the chart's x-axis
// stays contiguous. The aggregation happens server-side in a single query,
// replacing the client-side paging over /invocations that fetched up to
// 2,000 invocations on every contract page load (issue #600).
func (h *Handler) ResourceTrend(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "days must be a positive integer")
			return
		}
		days = n
	}
	if days > 3650 {
		days = 3650
	}

	buckets, err := h.Store.ResourceTrend(r.Context(), contractID, days)
	if err != nil {
		h.Logger.Error("get resource trend", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch resource trend")
		return
	}

	out := make([]resourceTrendBucketResponse, 0, len(buckets))
	for _, b := range buckets {
		out = append(out, resourceTrendBucketResponse{
			Day:         b.Day.Format("2006-01-02"),
			AvgCPUInsn:  b.AvgCPUInsn,
			AvgMemByte:  b.AvgMemByte,
			AvgFee:      b.AvgFee,
			Count:       b.Count,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
