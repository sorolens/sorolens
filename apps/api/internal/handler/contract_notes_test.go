package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const noteContractID = "CNOTESTCONTRACT000000000000000000000000000000000000000000"

func seededNoteStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: noteContractID, Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func createNote(t *testing.T, srv http.Handler, author, body string) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"author": author, "body": body})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/"+noteContractID+"/notes", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestContractNotes_CRUD(t *testing.T) {
	srv := newTestHandler(seededNoteStore(t), true, true)

	// An empty body is rejected before anything is persisted.
	if w := createNote(t, srv, "alice", "   "); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty body: want 422, got %d (%s)", w.Code, w.Body.String())
	}

	// Create.
	w := createNote(t, srv, "alice", "migrated from v1 on 2026-08-10")
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		Author string `json:"author"`
		Body   string `json:"body"`
	}
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Body == "" || created.Author != "alice" {
		t.Fatalf("unexpected created note: %+v", created)
	}

	// List.
	lreq := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+noteContractID+"/notes", nil)
	lw := httptest.NewRecorder()
	srv.ServeHTTP(lw, lreq)
	if lw.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d (%s)", lw.Code, lw.Body.String())
	}
	var listed struct {
		Notes []struct {
			ID string `json:"id"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(lw.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Notes) != 1 || listed.Notes[0].ID != created.ID {
		t.Fatalf("list: want the created note, got %+v", listed.Notes)
	}

	// Delete.
	dreq := httptest.NewRequest(http.MethodDelete, "/api/v1/contracts/"+noteContractID+"/notes/"+created.ID, nil)
	dw := httptest.NewRecorder()
	srv.ServeHTTP(dw, dreq)
	if dw.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d (%s)", dw.Code, dw.Body.String())
	}

	// Deleting the same note twice is a 404, not a silent success.
	dreq2 := httptest.NewRequest(http.MethodDelete, "/api/v1/contracts/"+noteContractID+"/notes/"+created.ID, nil)
	dw2 := httptest.NewRecorder()
	srv.ServeHTTP(dw2, dreq2)
	if dw2.Code != http.StatusNotFound {
		t.Fatalf("delete twice: want 404, got %d", dw2.Code)
	}
}

func TestContractNotes_UnknownContract(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CNOPE/notes", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}
