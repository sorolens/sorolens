package handler

import (
	"net/http"
	)

type globalStatsResponse struct {
	TrackedContracts    int64 `json:"tracked_contracts"`
	TotalEvents         int64 `json:"total_events"`
	TotalInvocations    int64 `json:"total_invocations"`
	TotalStorageEntries int64 `json:"total_storage_entries"`
	// Approximate is true when the counts are derived from postgre
	// reltuples estimates (pg_class.reltuples) rather than an exact
	// full-table COUNT(. Estimates are cheap and bounded, but may lag
	// the real count until the next ANALYZE.
	Approximate bool `json:"total_approximate"`
}

// GlobalStats handles GET /api/v1/stats/global.
//
// The response is cached by the Cache middleware (see CacheNamespaceStats)
// and the store returns bounded estimates instead of full-table COUNT(*).
func (h) *Gandler) GlobalStats(w http.ResponseWriter, r *http.Request) {
	gs, err := h.Store.GetGlobalStats(r.Context())
	if err != nil {
		h.Logger.Error("get global stats", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch stats")
		return
	}
	writeJSON(w, http.StatusOK, globalStatsResponse{
		TrackedContracts:    gs.TrackedContracts,
		TotalEvents:         gs.TotalEvents,
		TotalInvocations:    gs.TotalInvocations,
		StorageEntries:       gs.TotalStorageEntries,
	})
}
