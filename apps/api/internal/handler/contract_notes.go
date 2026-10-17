package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Contract notes (issue #164). A note is a markdown write-up attached to a
// contract — migration history, ownership, upgrade context — that would
// otherwise live in a chat thread. Reads are open like the rest of the v0.1
// surface; authoring needs the contributor role, and the note's author is the
// X-User-ID caller, which is also what later authorizes edits and deletes.

const (
	// invalidNoteBodyMessage is the shared 422 message for a malformed body.
	invalidNoteBodyMessage = "body must be 1-8192 characters of markdown"
	// invalidNoteIDMessage is the shared 422 message for a malformed note id.
	invalidNoteIDMessage = "note id must be a UUID"
	// noteAuthorMismatchMessage is the 403 message for editing someone else's note.
	noteAuthorMismatchMessage = "only the author of a note may change it"
)

// noteIDRe matches a canonical 8-4-4-4-12 UUID. Note ids are uuid columns in
// postgres, so a malformed path parameter is rejected before the query rather
// than surfacing as a 500 from the cast.
var noteIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type contractNoteResponse struct {
	ID         string    `json:"id"`
	ContractID string    `json:"contract_id"`
	Author     string    `json:"author"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type contractNotesResponse struct {
	ContractID string                 `json:"contract_id"`
	Notes      []contractNoteResponse `json:"notes"`
}

func noteResponse(n store.ContractNote) contractNoteResponse {
	return contractNoteResponse{
		ID:         n.ID,
		ContractID: n.ContractID,
		Author:     n.Author,
		Body:       n.Body,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
	}
}

// validNoteID reports whether id is a well-formed UUID.
func validNoteID(id string) bool {
	return noteIDRe.MatchString(id)
}

// validNoteBody reports whether body is acceptable note markdown. The check is
// run on the trimmed value so a body of only whitespace is rejected, while the
// stored body keeps its original leading indentation (which markdown uses for
// code blocks).
func validNoteBody(body string) bool {
	if strings.TrimSpace(body) == "" {
		return false
	}
	return utf8.RuneCountInString(body) <= store.MaxContractNoteBodyLen
}

// contractForNote resolves the {id} path parameter, writing the 404/500
// response itself when the contract is missing. It reports whether the caller
// should continue.
func (h *Handler) contractForNote(w http.ResponseWriter, r *http.Request, contractID string) bool {
	if _, err := h.Store.GetContract(r.Context(), contractID); errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "contract not found")
		return false
	} else if err != nil {
		h.Logger.Error("get contract for note", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch contract")
		return false
	}
	return true
}

// ListContractNotes handles GET /api/v1/contracts/{id}/notes.
//
// Returns the contract's notes newest-first. A contract with no notes returns
// an empty list rather than an error.
func (h *Handler) ListContractNotes(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	if !h.contractForNote(w, r, contractID) {
		return
	}
	notes, err := h.Store.ListContractNotes(r.Context(), contractID)
	if err != nil {
		h.Logger.Error("list contract notes", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to list notes")
		return
	}
	out := make([]contractNoteResponse, 0, len(notes))
	for _, n := range notes {
		out = append(out, noteResponse(n))
	}
	writeJSON(w, http.StatusOK, contractNotesResponse{ContractID: contractID, Notes: out})
}

// CreateContractNote handles POST /api/v1/contracts/{id}/notes.
//
// Body: {"body": "markdown"}. The stored author is the caller identity, which
// is what later authorizes editing and deleting the note.
func (h *Handler) CreateContractNote(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	author := getUserID(r)
	if author == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required: present a session token or user-bound API key")
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if !validNoteBody(req.Body) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, invalidNoteBodyMessage)
		return
	}
	if !h.contractForNote(w, r, contractID) {
		return
	}
	note, err := h.Store.AddContractNote(r.Context(), contractID, author, req.Body)
	if err != nil {
		h.Logger.Error("add contract note", "err", err, "contract_id", contractID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to add note")
		return
	}
	writeJSON(w, http.StatusCreated, noteResponse(note))
}

// UpdateContractNote handles PATCH /api/v1/contracts/{id}/notes/{noteID}.
//
// Body: {"body": "markdown"}. Only the note's author may rewrite it; any other
// contributor gets 403. The author field itself is never reassigned.
func (h *Handler) UpdateContractNote(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	noteID := chi.URLParam(r, "noteID")
	caller := getUserID(r)
	if caller == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required: present a session token or user-bound API key")
		return
	}
	if !validNoteID(noteID) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, invalidNoteIDMessage)
		return
	}
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, "invalid JSON body")
		return
	}
	if !validNoteBody(req.Body) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, invalidNoteBodyMessage)
		return
	}
	existing, err := h.Store.GetContractNote(r.Context(), contractID, noteID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "note not found")
		return
	} else if err != nil {
		h.Logger.Error("get contract note", "err", err, "contract_id", contractID, "note_id", noteID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch note")
		return
	}
	if existing.Author != caller {
		writeError(w, r, http.StatusForbidden, CodeForbidden, noteAuthorMismatchMessage)
		return
	}
	note, err := h.Store.UpdateContractNote(r.Context(), contractID, noteID, req.Body)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, CodeNotFound, "note not found")
		return
	} else if err != nil {
		h.Logger.Error("update contract note", "err", err, "contract_id", contractID, "note_id", noteID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to update note")
		return
	}
	writeJSON(w, http.StatusOK, noteResponse(note))
}

// DeleteContractNote handles DELETE /api/v1/contracts/{id}/notes/{noteID}.
//
// Only the note's author may delete it. Deleting a note that is already gone
// is a no-op: the caller's desired end state is reached either way, so a
// retried delete does not fail with a confusing 404.
func (h *Handler) DeleteContractNote(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "id")
	noteID := chi.URLParam(r, "noteID")
	caller := getUserID(r)
	if caller == "" {
		writeError(w, r, http.StatusUnauthorized, CodeUnauthorized, "authentication required: present a session token or user-bound API key")
		return
	}
	if !validNoteID(noteID) {
		writeError(w, r, http.StatusUnprocessableEntity, CodeInvalidInput, invalidNoteIDMessage)
		return
	}
	existing, err := h.Store.GetContractNote(r.Context(), contractID, noteID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		h.Logger.Error("get contract note", "err", err, "contract_id", contractID, "note_id", noteID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to fetch note")
		return
	}
	if err == nil && existing.Author != caller {
		writeError(w, r, http.StatusForbidden, CodeForbidden, noteAuthorMismatchMessage)
		return
	}
	if err := h.Store.DeleteContractNote(r.Context(), contractID, noteID); err != nil {
		h.Logger.Error("delete contract note", "err", err, "contract_id", contractID, "note_id", noteID)
		writeError(w, r, http.StatusInternalServerError, CodeInternal, "failed to delete note")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
