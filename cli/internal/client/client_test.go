package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
)

func serve(t *testing.T, pattern string, status int, body any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
	return httptest.NewServer(mux)
}

func TestGetContractHappyPath(t *testing.T) {
	want := client.Contract{
		ID:      "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Network: "testnet",
		Label:   "test-label",
		Status:  "active",
		AddedAt: time.Now().UTC().Truncate(time.Second),
	}
	srv := serve(t, "/api/v1/contracts/"+want.ID, http.StatusOK, want)
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	got, err := c.GetContract(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ID, want.ID)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
}

func TestGetContract404ReturnsSorolensError(t *testing.T) {
	id := "CNONEXISTENT"
	srv := serve(t, "/api/v1/contracts/"+id, http.StatusNotFound, map[string]any{
		"error": map[string]string{
			"code":    "NOT_FOUND",
			"message": "contract not found",
		},
	})
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	_, err := c.GetContract(context.Background(), id)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	se, ok := err.(*client.SorolensError)
	if !ok {
		t.Fatalf("expected *SorolensError, got %T", err)
	}
	if se.Status != http.StatusNotFound {
		t.Errorf("Status: got %d, want 404", se.Status)
	}
	if se.Code != "NOT_FOUND" {
		t.Errorf("Code: got %q, want NOT_FOUND", se.Code)
	}
}

func TestTrackContract422ReturnsError(t *testing.T) {
	srv := serve(t, "/api/v1/contracts", http.StatusUnprocessableEntity, map[string]any{
		"error": map[string]string{
			"code":    "INVALID_INPUT",
			"message": "id must be a 56-character string starting with 'C'",
		},
	})
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	_, err := c.TrackContract(context.Background(), "short-id", "alias", "testnet")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	se, ok := err.(*client.SorolensError)
	if !ok {
		t.Fatalf("expected *SorolensError, got %T", err)
	}
	if se.Status != http.StatusUnprocessableEntity {
		t.Errorf("Status: got %d, want 422", se.Status)
	}
	if se.Code != "INVALID_INPUT" {
		t.Errorf("Code: got %q, want INVALID_INPUT", se.Code)
	}
}

func TestListEventsWithTypeFilter(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	wantType := "contract"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/contracts/"+contractID+"/events" {
			http.NotFound(w, r)
			return
		}
		gotType := r.URL.Query().Get("type")
		if gotType != wantType {
			t.Errorf("type query param: got %q, want %q", gotType, wantType)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.EventsResponse{
			Events: []client.Event{{
				ID:   "evt-1",
				Type: wantType,
			}},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	resp, err := c.ListEvents(context.Background(), contractID, client.ListEventsOpts{Type: wantType})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Events) != 1 {
		t.Errorf("events count: got %d, want 1", len(resp.Events))
	}
	if resp.Events[0].Type != wantType {
		t.Errorf("event type: got %q, want %q", resp.Events[0].Type, wantType)
	}
}
