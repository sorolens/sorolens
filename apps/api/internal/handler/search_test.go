package handler_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type searchResult struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	Label        string `json:"label"`
	Network      string `json:"network"`
	ContractID   string `json:"contract_id"`
	TxHash       string `json:"tx_hash"`
	FunctionName string `json:"function_name"`
}

func TestSearch(t *testing.T) {
	t.Run("returns matching contract results as an array", func(t *testing.T) {
		ms := store.NewMockStore()
		if err := ms.UpsertContract(t.Context(), store.Contract{
			ID:      "C12345",
			Label:   "Token contract",
			Network: "testnet",
		}); err != nil {
			t.Fatalf("UpsertContract: %v", err)
		}

		results := searchRequest(t, ms, "token")
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		got := results[0]
		if got.Type != "contract" || got.ID != "C12345" || got.Label != "Token contract" || got.Network != "testnet" {
			t.Errorf("unexpected result: %+v", got)
		}
	})

	t.Run("returns matching event transaction hashes and network", func(t *testing.T) {
		ms := store.NewMockStore()
		err := ms.BatchInsertEvents(t.Context(), []store.Event{{
			ID: "event-1", ContractID: "C12345", Network: "mainnet", TxHash: "d9e771ac",
		}})
		if err != nil {
			t.Fatalf("BatchInsertEvents: %v", err)
		}

		results := searchRequest(t, ms, "e771")
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		got := results[0]
		if got.Type != "event" || got.TxHash != "d9e771ac" || got.Network != "mainnet" || got.ContractID != "C12345" {
			t.Errorf("unexpected result: %+v", got)
		}
	})

	t.Run("returns matching invocation function names and network", func(t *testing.T) {
		ms := store.NewMockStore()
		err := ms.BatchInsertInvocations(t.Context(), []store.Invocation{{
			TxHash: "tx-1", ContractID: "C12345", Network: "futurenet", FunctionName: "transfer",
		}})
		if err != nil {
			t.Fatalf("BatchInsertInvocations: %v", err)
		}

		results := searchRequest(t, ms, "transfer")
		if len(results) != 1 {
			t.Fatalf("got %d results, want 1", len(results))
		}
		got := results[0]
		if got.Type != "function" || got.FunctionName != "transfer" || got.Network != "futurenet" || got.ContractID != "C12345" {
			t.Errorf("unexpected result: %+v", got)
		}
	})

	t.Run("limits results to 10 for each result type", func(t *testing.T) {
		ms := store.NewMockStore()
		events := make([]store.Event, 0, 15)
		invocations := make([]store.Invocation, 0, 15)
		for i := 0; i < 15; i++ {
			contractID := fmt.Sprintf("C%02d", i)
			if err := ms.UpsertContract(t.Context(), store.Contract{
				ID: contractID, Label: "needle contract", Network: "testnet",
			}); err != nil {
				t.Fatalf("UpsertContract %s: %v", contractID, err)
			}
			events = append(events, store.Event{
				ID: fmt.Sprintf("event-%02d", i), ContractID: contractID, Network: "mainnet",
				TxHash: fmt.Sprintf("needle-tx-%02d", i),
			})
			invocations = append(invocations, store.Invocation{
				TxHash: fmt.Sprintf("invocation-%02d", i), ContractID: contractID,
				Network: "futurenet", FunctionName: fmt.Sprintf("needle-function-%02d", i),
			})
		}
		if err := ms.BatchInsertEvents(t.Context(), events); err != nil {
			t.Fatalf("BatchInsertEvents: %v", err)
		}
		if err := ms.BatchInsertInvocations(t.Context(), invocations); err != nil {
			t.Fatalf("BatchInsertInvocations: %v", err)
		}

		results := searchRequest(t, ms, "needle")
		counts := map[string]int{}
		for _, result := range results {
			counts[result.Type]++
		}
		for _, resultType := range []string{"contract", "event", "function"} {
			if counts[resultType] != 10 {
				t.Errorf("got %d %s results, want 10", counts[resultType], resultType)
			}
		}
	})

	t.Run("rejects an empty query", func(t *testing.T) {
		srv := newTestHandler(store.NewMockStore(), true, true)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=%20%20", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status: got %d, want %d; body: %s", w.Code, http.StatusUnprocessableEntity, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "q is required") {
			t.Errorf("response body %q does not contain the validation message", w.Body.String())
		}
	})

	t.Run("returns an error when the store fails", func(t *testing.T) {
		ms := store.NewMockStore()
		ms.SearchErr = errors.New("search unavailable")
		srv := newTestHandler(ms, true, true)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=token", nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status: got %d, want %d; body: %s", w.Code, http.StatusInternalServerError, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "failed to search") {
			t.Errorf("response body %q does not contain the error message", w.Body.String())
		}
	})
}

func searchRequest(t *testing.T, ms *store.MockStore, query string) []searchResult {
	t.Helper()
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+query, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var results []searchResult
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response as JSON array: %v", err)
	}
	return results
}
