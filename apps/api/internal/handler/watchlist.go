package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// ---- response types ---------------------------------------------------------

type watchlistItemResponse struct {
	ContractID string    `json:"contract_id"`
	AddedAt    time.Time `json:"added_at"`
}

type watchlistResponse struct {
	Items []watchlistItemResponse `json:"items"`
}

type watchlistStatusResponse struct {
	InWatchlist bool `json:"in_watchlist"`
}

// ---- helpers ----------------------------------------------------------------

// getUserID extracts the user ID from the X-User-ID header.
// Auth uses secure cookies with SameSite=Lax in production;
// this header-based approach is used for simplicity in this
// initial implementation.
func getUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

// ---- handlers ---------------------------------------------------------------

// AddToWatchlist handles POST /api/v1/watchlist.
// Body: {"contract_id": "..."}
func (h *Handler) AddToWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}

	var req struct {
		ContractID string `json:"contract_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if req.ContractID == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "contract_id is required")
		return
	}

	if err := h.Store.AddToWatchlist(r.Context(), userID, req.ContractID); err != nil {
		h.Logger.Error("add to watchlist", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to add to watchlist")
		return
	}
	writeJSON(w, http.StatusCreated, watchlistStatusResponse{InWatchlist: true})
}

// RemoveFromWatchlist handles DELETE /api/v1/watchlist/{contractId}.
func (h *Handler) RemoveFromWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}

	contractID := chi.URLParam(r, "contractId")
	if err := h.Store.RemoveFromWatchlist(r.Context(), userID, contractID); err != nil {
		h.Logger.Error("remove from watchlist", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to remove from watchlist")
		return
	}
	writeJSON(w, http.StatusOK, watchlistStatusResponse{InWatchlist: false})
}

// ListWatchlist handles GET /api/v1/watchlist.
func (h *Handler) ListWatchlist(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}

	contractIDs, err := h.Store.ListWatchlist(r.Context(), userID)
	if err != nil {
		h.Logger.Error("list watchlist", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list watchlist")
		return
	}

	resp := make([]watchlistItemResponse, len(contractIDs))
	for i, cid := range contractIDs {
		resp[i] = watchlistItemResponse{ContractID: cid, AddedAt: time.Now()}
	}
	writeJSON(w, http.StatusOK, watchlistResponse{Items: resp})
}

// WatchlistStatus handles GET /api/v1/watchlist/{contractId}/status.
func (h *Handler) WatchlistStatus(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	if userID == "" {
		writeError(w, r, http.StatusUnauthorized, CodeInvalidInput, "X-User-ID header is required")
		return
	}

	contractID := chi.URLParam(r, "contractId")
	inWatchlist, err := h.Store.IsInWatchlist(r.Context(), userID, contractID)
	if err != nil {
		h.Logger.Error("watchlist status", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to check watchlist status")
		return
	}
	writeJSON(w, http.StatusOK, watchlistStatusResponse{InWatchlist: inWatchlist})
}
