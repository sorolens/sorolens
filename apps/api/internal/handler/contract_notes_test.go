package handler_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	// noteContract is the tracked contract the note tests attach notes to.
	noteContract = "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	// otherContributorUser is a second contributor identity, used to prove that
	// a note can only be changed by the identity that wrote it.
	otherContributorUser = "user-notes-other-contributor"
)

// seedNoteStore returns a store with the RBAC users plus a second contributor
// and one tracked contract to hang notes off.
func seedNoteStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedRBACUsers(t)
	ms.AddUser(store.User{ID: otherContributorUser, Role: store.RoleContributor})
	if err := ms.UpsertContract(nil, store.Contract{
		ID: noteContract, Network: "testnet", Label: "notes", Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

// noteEnvelope mirrors the single-note response body.
type noteEnvelope struct {
	ID         string `json:"id"`
	ContractID string `json:"contract_id"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// notesEnvelope mirrors the list response body.
type notesEnvelope struct {
	ContractID string         `json:"contract_id"`
	Notes      []noteEnvelope `json:"notes"`
}

func decodeNote(t *testing.T, body []byte) noteEnvelope {
	t.Helper()
	var n noteEnvelope
	if err := json.Unmarshal(body, &n); err != nil {
		t.Fatalf("decode note: %v", err)
	}
	return n
}

func decodeNotes(t *testing.T, body []byte) notesEnvelope {
	t.Helper()
	var n notesEnvelope
	if err := json.Unmarshal(body, &n); err != nil {
		t.Fatalf("decode notes: %v", err)
	}
	return n
}

// createNote posts a note as the given user and returns the stored row.
func createNote(t *testing.T, srv http.Handler, user, body string) noteEnvelope {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"body": body})
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/"+noteContract+"/notes", "", user, string(payload))
	if w.Code != http.StatusCreated {
		t.Fatalf("create note: want 201, got %d: %s", w.Code, w.Body.String())
	}
	return decodeNote(t, w.Body.Bytes())
}

func TestCreateContractNote(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	note := createNote(t, srv, contributorUser, "## Migration\n\nMoved from v1 on 2026-08-10.")
	if note.ID == "" || note.ContractID != noteContract {
		t.Fatalf("stored note missing identity: %+v", note)
	}
	if note.Author != contributorUser {
		t.Fatalf("author: want %q, got %q", contributorUser, note.Author)
	}
	if note.Body != "## Migration\n\nMoved from v1 on 2026-08-10." {
		t.Fatalf("body not stored verbatim: %q", note.Body)
	}
	if note.CreatedAt == "" || note.UpdatedAt == "" {
		t.Fatalf("timestamps missing: %+v", note)
	}

	// The note is readable straight back through the list endpoint.
	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+noteContract+"/notes", "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list notes: want 200, got %d", w.Code)
	}
	notes := decodeNotes(t, w.Body.Bytes())
	if len(notes.Notes) != 1 || notes.Notes[0].ID != note.ID {
		t.Fatalf("want the created note, got %+v", notes.Notes)
	}
}

func TestCreateContractNoteRequiresIdentity(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	payload, _ := json.Marshal(map[string]string{"body": "hello"})
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/"+noteContract+"/notes", "", "", string(payload))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous note: want 401, got %d", w.Code)
	}
}

func TestCreateContractNoteRequiresContributor(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	payload, _ := json.Marshal(map[string]string{"body": "hello"})
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/"+noteContract+"/notes", "", viewerUser, string(payload))
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer note: want 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateContractNoteRejectsInvalidBody(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	for _, body := range []string{"", "   ", "\n\t ", strings.Repeat("a", store.MaxContractNoteBodyLen+1)} {
		payload, _ := json.Marshal(map[string]string{"body": body})
		w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/"+noteContract+"/notes", "", contributorUser, string(payload))
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("body of %d chars: want 422, got %d", len(body), w.Code)
		}
	}
}

func TestCreateContractNoteUnknownContract(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	payload, _ := json.Marshal(map[string]string{"body": "hello"})
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/CNOPE/notes", "", contributorUser, string(payload))
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown contract: want 404, got %d", w.Code)
	}
}

func TestListContractNotesNewestFirst(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	first := createNote(t, srv, contributorUser, "first note")
	second := createNote(t, srv, contributorUser, "second note")

	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+noteContract+"/notes", "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	notes := decodeNotes(t, w.Body.Bytes())
	if len(notes.Notes) != 2 {
		t.Fatalf("want 2 notes, got %d", len(notes.Notes))
	}
	if notes.Notes[0].ID != second.ID || notes.Notes[1].ID != first.ID {
		t.Fatalf("want newest first, got %s then %s", notes.Notes[0].ID, notes.Notes[1].ID)
	}
}

func TestListContractNotesEmptyList(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+noteContract+"/notes", "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	// The list must serialise as [], not null, so clients can iterate blindly.
	if !strings.Contains(w.Body.String(), `"notes":[]`) {
		t.Fatalf("want an empty array, got %s", w.Body.String())
	}
}

func TestListContractNotesUnknownContract(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/CNOPE/notes", "", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestUpdateContractNote(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)
	note := createNote(t, srv, contributorUser, "original body")

	payload, _ := json.Marshal(map[string]string{"body": "edited body"})
	w := doRequestAsUser(srv, http.MethodPatch,
		"/api/v1/contracts/"+noteContract+"/notes/"+note.ID, "", contributorUser, string(payload))
	if w.Code != http.StatusOK {
		t.Fatalf("update note: want 200, got %d: %s", w.Code, w.Body.String())
	}
	updated := decodeNote(t, w.Body.Bytes())
	if updated.Body != "edited body" {
		t.Fatalf("body: want %q, got %q", "edited body", updated.Body)
	}
	// Editing never reassigns the note.
	if updated.Author != contributorUser {
		t.Fatalf("author changed: %q", updated.Author)
	}
	if updated.ID != note.ID || updated.CreatedAt != note.CreatedAt {
		t.Fatalf("identity changed: %+v", updated)
	}
}

func TestUpdateContractNoteRejectsNonAuthor(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)
	note := createNote(t, srv, contributorUser, "original body")

	payload, _ := json.Marshal(map[string]string{"body": "hijacked"})
	w := doRequestAsUser(srv, http.MethodPatch,
		"/api/v1/contracts/"+noteContract+"/notes/"+note.ID, "", otherContributorUser, string(payload))
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-author edit: want 403, got %d: %s", w.Code, w.Body.String())
	}

	// The body must be untouched.
	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+noteContract+"/notes", "", "", "")
	notes := decodeNotes(t, w.Body.Bytes())
	if len(notes.Notes) != 1 || notes.Notes[0].Body != "original body" {
		t.Fatalf("note was modified by a non-author: %+v", notes.Notes)
	}
}

func TestUpdateContractNoteRejectsInvalidBody(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)
	note := createNote(t, srv, contributorUser, "original body")

	payload, _ := json.Marshal(map[string]string{"body": "   "})
	w := doRequestAsUser(srv, http.MethodPatch,
		"/api/v1/contracts/"+noteContract+"/notes/"+note.ID, "", contributorUser, string(payload))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}

func TestUpdateContractNoteNotFound(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	payload, _ := json.Marshal(map[string]string{"body": "hello"})
	w := doRequestAsUser(srv, http.MethodPatch,
		"/api/v1/contracts/"+noteContract+"/notes/00000000-0000-4000-8000-000000000000", "", contributorUser, string(payload))
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestNoteEndpointsRejectMalformedID(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)

	payload, _ := json.Marshal(map[string]string{"body": "hello"})
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		w := doRequestAsUser(srv, method,
			"/api/v1/contracts/"+noteContract+"/notes/not-a-uuid", "", contributorUser, string(payload))
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s malformed id: want 422, got %d", method, w.Code)
		}
	}
}

func TestDeleteContractNote(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)
	note := createNote(t, srv, contributorUser, "temporary")

	w := doRequestAsUser(srv, http.MethodDelete,
		"/api/v1/contracts/"+noteContract+"/notes/"+note.ID, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete note: want 204, got %d: %s", w.Code, w.Body.String())
	}

	// Deleting it again is a no-op: the end state is already reached.
	w = doRequestAsUser(srv, http.MethodDelete,
		"/api/v1/contracts/"+noteContract+"/notes/"+note.ID, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("idempotent delete: want 204, got %d", w.Code)
	}

	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+noteContract+"/notes", "", "", "")
	if notes := decodeNotes(t, w.Body.Bytes()); len(notes.Notes) != 0 {
		t.Fatalf("note still present after delete: %+v", notes.Notes)
	}
}

func TestDeleteContractNoteRejectsNonAuthor(t *testing.T) {
	srv := newTestHandler(seedNoteStore(t), true, true)
	note := createNote(t, srv, contributorUser, "keep me")

	w := doRequestAsUser(srv, http.MethodDelete,
		"/api/v1/contracts/"+noteContract+"/notes/"+note.ID, "", otherContributorUser, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-author delete: want 403, got %d", w.Code)
	}

	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts/"+noteContract+"/notes", "", "", "")
	if notes := decodeNotes(t, w.Body.Bytes()); len(notes.Notes) != 1 {
		t.Fatalf("non-author delete removed the note: %+v", notes.Notes)
	}
}
