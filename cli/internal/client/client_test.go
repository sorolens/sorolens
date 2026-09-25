package client_test

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestListContractsSendsAPIKeyAndParsesPage(t *testing.T) {
	want := client.Contract{ID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", Status: "active"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/contracts" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("X-API-Key"); got != "secret-key" {
			t.Errorf("X-API-Key: got %q, want secret-key", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Errorf("Authorization: got %q, want Bearer secret-key", got)
		}
		if got := r.URL.Query().Get("status"); got != "active" {
			t.Errorf("status query param: got %q, want active", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.ContractsResponse{
			Contracts:  []client.Contract{want},
			NextCursor: "next",
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second).WithAPIKey("secret-key")
	resp, err := c.ListContracts(context.Background(), client.ListContractsOpts{Status: "active", Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Contracts) != 1 || resp.Contracts[0].ID != want.ID {
		t.Fatalf("contracts = %+v, want one %q", resp.Contracts, want.ID)
	}
	if resp.NextCursor != "next" {
		t.Errorf("next cursor: got %q, want next", resp.NextCursor)
	}
}

func TestListInvocationsPathAndFilters(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/contracts/"+contractID+"/invocations" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("fn"); got != "transfer" {
			t.Errorf("fn query param: got %q, want transfer", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.InvocationsResponse{
			Invocations: []client.Invocation{{TxHash: "tx-1", FunctionName: "transfer"}},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	resp, err := c.ListInvocations(context.Background(), contractID, client.ListInvocationsOpts{Fn: "transfer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Invocations) != 1 || resp.Invocations[0].FunctionName != "transfer" {
		t.Fatalf("invocations = %+v, want transfer invocation", resp.Invocations)
	}
}

func TestListAlertsSeverityFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/watchdog/alerts" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("severity"); got != "Critical" {
			t.Errorf("severity query param: got %q, want Critical", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.AlertsResponse{
			Alerts: []client.ContractAlert{{ContractID: "C1", Severity: "Critical", Message: "ttl low"}},
		})
	}))
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	resp, err := c.ListAlerts(context.Background(), client.ListAlertsOpts{Severity: "Critical"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Alerts) != 1 || resp.Alerts[0].Severity != "Critical" {
		t.Fatalf("alerts = %+v, want one Critical alert", resp.Alerts)
	}
}


func TestGetMonitoredContractHappyPath(t *testing.T) {
	want := client.MonitoredContract{
		ContractID:    "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Network:       "testnet",
		Name:          "MyVault",
		Owner:         "GABCXYZ",
		Status:        "healthy",
		CheckInterval: 60,
		RegisteredAt:  time.Now().UTC().Truncate(time.Second),
		UpdatedAt:     time.Now().UTC().Truncate(time.Second),
	}
	srv := serve(t, "/api/v1/watchdog/contracts/"+want.ContractID, http.StatusOK, want)
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	got, err := c.GetMonitoredContract(context.Background(), want.ContractID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ContractID != want.ContractID {
		t.Errorf("ContractID: got %q, want %q", got.ContractID, want.ContractID)
	}
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
}

func TestGetMonitoredContract404ReturnsSorolensError(t *testing.T) {
	id := "CNONEXISTENT"
	srv := serve(t, "/api/v1/watchdog/contracts/"+id, http.StatusNotFound, map[string]any{
		"error": map[string]string{
			"code":    "NOT_FOUND",
			"message": "monitored contract not found",
		},
	})
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	_, err := c.GetMonitoredContract(context.Background(), id)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	se, ok := err.(*client.SorolensError)
	if !ok {
		t.Fatalf("expected *client.SorolensError, got %T", err)
	}
	if se.Status != http.StatusNotFound {
		t.Errorf("Status: got %d, want %d", se.Status, http.StatusNotFound)
	}
}

func TestStreamEventsParsesSSEFrames(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/stream/events" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("contract_id"); got != contractID {
			t.Errorf("contract_id query param: got %q, want %q", got, contractID)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept header: got %q, want text/event-stream", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"type\":\"connected\",\"message\":\"event stream connected\"}\n\n")
		// Comment frames (heartbeats) must be ignored, not parsed.
		fmt.Fprint(w, ": ping\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"event\",\"contract_id\":%q,\"event\":{\"id\":\"evt-1\",\"contract_id\":%q,\"type\":\"contract\",\"ledger\":42,\"tx_hash\":\"deadbeef\"}}\n\n", contractID, contractID)
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	var got []client.StreamMessage
	err := c.StreamEvents(context.Background(), contractID, func(msg client.StreamMessage) error {
		got = append(got, msg)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("messages count: got %d, want 2 (%+v)", len(got), got)
	}
	if got[0].Type != "connected" {
		t.Errorf("first message type: got %q, want connected", got[0].Type)
	}
	if got[1].Type != "event" {
		t.Fatalf("second message type: got %q, want event", got[1].Type)
	}
	if got[1].Event == nil {
		t.Fatal("second message event: got nil, want populated")
	}
	if got[1].Event.ID != "evt-1" {
		t.Errorf("event ID: got %q, want evt-1", got[1].Event.ID)
	}
	if got[1].Event.Ledger != 42 {
		t.Errorf("event ledger: got %d, want 42", got[1].Event.Ledger)
	}
}

func TestStreamEventsContextCancelReturnsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	c := client.New(srv.URL, 5*time.Second)
	err := c.StreamEvents(ctx, "", func(client.StreamMessage) error { return nil })
	if err != nil {
		t.Fatalf("expected nil error after context cancel, got %v", err)
	}
}

func TestStreamEventsNon2xxReturnsSorolensError(t *testing.T) {
	srv := serve(t, "/api/v1/stream/events", http.StatusServiceUnavailable, map[string]any{
		"error": map[string]string{
			"code":    "UNAVAILABLE",
			"message": "stream unavailable",
		},
	})
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	err := c.StreamEvents(context.Background(), "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", func(client.StreamMessage) error { return nil })
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	se, ok := err.(*client.SorolensError)
	if !ok {
		t.Fatalf("expected *SorolensError, got %T", err)
	}
	if se.Status != http.StatusServiceUnavailable {
		t.Errorf("Status: got %d, want 503", se.Status)
	}
	if se.Code != "UNAVAILABLE" {
		t.Errorf("Code: got %q, want UNAVAILABLE", se.Code)
	}
}
