package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testServer returns an httptest.Server stubbing the Sorolens API surface.
// Requests without a bearer token are rejected so the test proves the client
// attaches its API key.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	requireKey := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if got, want := r.Header.Get("Authorization"), "Bearer sl_test-token"; got != want {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			next(w, r)
		}
	}

	contract := map[string]any{
		"id":                   "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2",
		"network":              "testnet",
		"label":                "counter",
		"wasm_hash":            "0x1",
		"created_at_ledger":    100,
		"backfill_complete_at": time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		"status":               "active",
		"added_at":             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}

	mux.HandleFunc("GET /api/v1/contracts", requireKey(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contracts":   []any{contract},
			"next_cursor": "",
		})
	}))
	mux.HandleFunc("GET /api/v1/contracts/{id}", requireKey(func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != contract["id"] {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "NOT_FOUND", "message": "contract not found"}})
			return
		}
		_ = json.NewEncoder(w).Encode(contract)
	}))
	mux.HandleFunc("GET /api/v1/stats/global", requireKey(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tracked_contracts": 1, "total_events": 100, "total_invocations": 80, "total_storage_entries": 5,
		})
	}))
	mux.HandleFunc("GET /api/v1/contracts/{id}/forecast", requireKey(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"contract_id":   contract["id"],
			"lookback_days": 90,
			"series": []any{map[string]any{
				"metric": "events", "horizon": 30, "daily_count": 30,
				"points": []any{map[string]any{"date": "2026-09-24", "value": 10.1, "lower": 9.0, "upper": 11.2}},
			}},
		})
	}))
	mux.HandleFunc("GET /api/v1/watchdog/alerts", requireKey(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"alerts": []any{map[string]any{
				"contract_id": contract["id"], "severity": "Warning", "message": "spike",
				"ledger": 100, "tx_hash": "0xabc", "timestamp": time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
			}},
		})
	}))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestApiClient_ListContracts(t *testing.T) {
	server := testServer(t)
	c, err := New(server.URL, "sl_test-token", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.ListContracts(context.Background(), &ListContractsParams{})
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if resp.JSON200 == nil {
		t.Fatalf("JSON200 is nil")
	}
	if len(resp.JSON200.Contracts) != 1 {
		t.Fatalf("got %d contracts, want 1", len(resp.JSON200.Contracts))
	}
	got := resp.JSON200.Contracts[0]
	if got.Id != "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2" {
		t.Errorf("contract id = %q", got.Id)
	}
	if got.Status != "active" {
		t.Errorf("contract status = %q, want active", got.Status)
	}
	if got.WasmHash != "0x1" {
		t.Errorf("wasm_hash = %q, want 0x1", got.WasmHash)
	}
	if resp.JSON200.NextCursor != "" {
		t.Errorf("next_cursor = %q, want empty", resp.JSON200.NextCursor)
	}
}

func TestApiClient_GetContract(t *testing.T) {
	server := testServer(t)
	c, err := New(server.URL, "sl_test-token", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const id = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2"
	resp, err := c.GetContract(context.Background(), id)
	if err != nil {
		t.Fatalf("GetContract: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatalf("JSON200 is nil")
	}
	if resp.JSON200.Id != id {
		t.Errorf("id = %q", resp.JSON200.Id)
	}
	if resp.JSON200.BackfillCompleteAt == nil {
		t.Error("backfill_complete_at is nil, want a time")
	}
}

func TestApiClient_AuthInjectedOnEveryRequest(t *testing.T) {
	var sawAuth bool
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/contracts", func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") != ""
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"contracts": []any{}, "next_cursor": ""})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	c, err := New(server.URL, "sl_token", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, _ = c.ListContracts(context.Background(), &ListContractsParams{})
	if !sawAuth {
		t.Error("Authorization header was not set on the outgoing request")
	}
}

func TestApiClient_ErrorMapping(t *testing.T) {
	server := testServer(t)
	c, err := New(server.URL, "sl_test-token", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = c.GetContract(context.Background(), "CWRONGID")
	if err != nil {
		t.Fatalf("GetContract: %v", err)
	}

	// 404 surfaces a typed body on the generated response.
	resp, err := c.GetContract(context.Background(), "CWRONGID")
	if err != nil {
		t.Fatalf("GetContract err: %v", err)
	}
	if resp.StatusCode() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode())
	}
	if resp.JSON404 == nil {
		t.Fatal("JSON404 is nil; 404 error body not decoded")
	}
}

func TestApiClient_GlobalStats(t *testing.T) {
	server := testServer(t)
	c, err := New(server.URL, "sl_test-token", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	resp, err := c.GetGlobalStats(context.Background())
	if err != nil {
		t.Fatalf("GetGlobalStats: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatalf("JSON200 is nil")
	}
	if resp.JSON200.TotalEvents != 100 {
		t.Errorf("total_events = %d, want 100", resp.JSON200.TotalEvents)
	}
}

func TestApiClient_Forecast(t *testing.T) {
	server := testServer(t)
	c, err := New(server.URL, "sl_test-token", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const id = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH2"
	horizon := 30
	resp, err := c.GetContractForecast(context.Background(), id, &GetContractForecastParams{
		Horizon: &horizon,
	})
	if err != nil {
		t.Fatalf("GetContractForecast: %v", err)
	}
	if resp.JSON200 == nil {
		t.Fatalf("JSON200 is nil")
	}
	if len(resp.JSON200.Series) != 1 {
		t.Fatalf("got %d series, want 1", len(resp.JSON200.Series))
	}
	s := resp.JSON200.Series[0]
	if s.Metric != "events" {
		t.Errorf("metric = %q, want events", s.Metric)
	}
	if len(s.Points) != 1 {
		t.Fatalf("got %d points, want 1", len(s.Points))
	}
	if s.Points[0].Value != 10.1 {
		t.Errorf("point value = %v, want 10.1", s.Points[0].Value)
	}
	_ = fmt.Sprintf("%v", s.Points[0].Date)
}

func TestApiClient_SchemaEmbedded(t *testing.T) {
	swagger, err := GetSwagger()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}
	if len(swagger.Paths.Map()) == 0 {
		t.Fatal("embedded schema has no paths")
	}
}
