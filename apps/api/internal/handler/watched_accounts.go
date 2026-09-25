package handler

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- response types ---------------------------------------------------------

type watchedAccountResponse struct {
	AccountID       string    `json:"account_id"`
	AddedBy         string    `json:"added_by"`
	DiscoveredCount int64     `json:"discovered_count"`
	CreatedAt       time.Time `json:"created_at"`
}

func watchedAccountFromStore(a store.WatchedAccount) watchedAccountResponse {
	return watchedAccountResponse{
		AccountID:       a.AccountID,
		AddedBy:         a.AddedBy,
		DiscoveredCount: a.DiscoveredCount,
		CreatedAt:       a.CreatedAt,
	}
}

// ---- helpers ----------------------------------------------------------------

// strkeyAccountVersion is the strkey version byte for an ed25519 public key
// (account IDs render with a leading 'G').
const strkeyAccountVersion = 6 << 3

// validateAccountID reports whether id is a well-formed Stellar account
// strkey: 'G' + base32(version byte, 32-byte key, CRC16-XModem checksum).
func validateAccountID(id string) bool {
	if len(id) != 56 || !strings.HasPrefix(id, "G") {
		return false
	}
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(id)
	if err != nil || len(raw) != 35 || raw[0] != strkeyAccountVersion {
		return false
	}
	want := crc16XModem(raw[:33])
	got := uint16(raw[33]) | uint16(raw[34])<<8 // little-endian
	return want == got
}

func crc16XModem(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// ---- handlers ---------------------------------------------------------------

// AddWatchedAccount handles POST /api/v1/watched-accounts.
// Body: {"account_id": "G..."}. Returns 201 when the account is newly
// watched and 200 when it already was.
func (h *Handler) AddWatchedAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	req.AccountID = strings.TrimSpace(req.AccountID)
	if !validateAccountID(req.AccountID) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "account_id must be a valid Stellar account ID (G...)")
		return
	}

	a, created, err := h.Store.AddWatchedAccount(r.Context(), store.WatchedAccount{
		AccountID: req.AccountID,
		AddedBy:   middleware.Actor(r),
	})
	if err != nil {
		h.Logger.Error("add watched account", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to add watched account")
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, watchedAccountFromStore(a))
}

// ListWatchedAccounts handles GET /api/v1/watched-accounts.
func (h *Handler) ListWatchedAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.Store.ListWatchedAccounts(r.Context())
	if err != nil {
		h.Logger.Error("list watched accounts", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list watched accounts")
		return
	}
	resp := make([]watchedAccountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = watchedAccountFromStore(a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"watched_accounts": resp})
}

// DeleteWatchedAccount handles DELETE /api/v1/watched-accounts/{id}.
// Contracts the account already discovered stay tracked.
func (h *Handler) DeleteWatchedAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := h.Store.DeleteWatchedAccount(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "watched account not found")
		return
	}
	if err != nil {
		h.Logger.Error("delete watched account", "err", err)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete watched account")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
