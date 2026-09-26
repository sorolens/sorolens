package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Limits keep a single note from exhausting the text column or overwhelming
// the contract detail page while still allowing long write-ups.
const (
	maxNoteBodyLen   = 10000
	maxNoteAuthorLen = 120
)

type contractNoteResponse struct {
	ID         string    `json:"id"`
	ContractID string    `json:"contract_id"`
	Author     string    `json:"author"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type createContractNoteRequest struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

func contractNoteFromStore(n store.ContractNote) contractNoteResponse {
	return contractNoteResponse{
		ID:         n.ID,
		ContractID: n.ContractID,
		Author:     n.Author,
		Body:       n.Body,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
	}
}

// newContractNoteID returns a collision-resistant, URL-safe note id.
func newContractNoteID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("note_%d", time.Now().UnixNano())
	}
	return "note_" + hex.EncodeToString(b[:])
}

// requireContract writes a 404/500 error and returns false when the contract
// does not exist (or the lookup failed). It keeps the notes handlers from
// orphaning rows or leaking an empty list for an unknown contract.
func (h *Handler) requireContract(w http.ResponseWriter, r *http.Request, contractID string) bool {
	_, err := h.Store.GetContract(r.Context(), contractID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return false
	}
	if err != nil {
		h.Logger.Error("get contract for note", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return false
	}
	return true
}

// CreateContractNote handles POST /api/v1/contracts/{id}/notes.
func (h *Handler) CreateContractNote(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	var req createContractNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}

	req.Author = strings.TrimSpace(req.Author)
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "body is required")
		return
	}
	if len(req.Body) > maxNoteBodyLen {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			fmt.Sprintf("body must be at most %d characters", maxNoteBodyLen))
		return
	}
	if req.Author == "" {
		req.Author = "anonymous"
	}
	if len(req.Author) > maxNoteAuthorLen {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput,
			fmt.Sprintf("author must be at most %d characters", maxNoteAuthorLen))
		return
	}

	if !h.requireContract(w, r, contractID) {
		return
	}

	now := time.Now().UTC()
	note := store.ContractNote{
		ID:         newContractNoteID(),
		ContractID: contractID,
		Author:     req.Author,
		Body:       req.Body,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := h.Store.CreateContractNote(r.Context(), note); err != nil {
		h.Logger.Error("create contract note", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to create note")
		return
	}
	writeJSON(w, http.StatusCreated, contractNoteFromStore(note))
}

// ListContractNotes handles GET /api/v1/contracts/{id}/notes.
func (h *Handler) ListContractNotes(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")

	if !h.requireContract(w, r, contractID) {
		return
	}

	limit := intQuery(r, "limit", 100)
	if limit > 200 {
		limit = 200
	}
	notes, err := h.Store.ListContractNotes(r.Context(), contractID, limit)
	if err != nil {
		h.Logger.Error("list contract notes", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list notes")
		return
	}
	resp := make([]contractNoteResponse, len(notes))
	for i, n := range notes {
		resp[i] = contractNoteFromStore(n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": resp})
}

// DeleteContractNote handles DELETE /api/v1/contracts/{id}/notes/{noteId}.
func (h *Handler) DeleteContractNote(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	noteID := chi.URLParam(r, "noteId")

	if err := h.Store.DeleteContractNote(r.Context(), contractID, noteID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, CodeNotFound, "note not found")
			return
		}
		h.Logger.Error("delete contract note", "err", err, "contract_id", contractID, "note_id", noteID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete note")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": noteID})
}
