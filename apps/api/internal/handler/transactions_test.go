package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

var ctx = context.Background()

const testTxHash = "aabbccdd" + "00112233" + "44556677" + "8899aabb" + "ccddeeff" + "00112233" + "44556677" + "8899aabb"

func TestGetTransactionNotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+testTxHash, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body)
	}
}

func TestGetTransactionInvalidHash(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/not-a-hash", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body)
	}
}

func TestGetTransactionReturnsInvocationAndEvents(t *testing.T) {
	ms := store.NewMockStore()
	now := time.Now().UTC()
	inv := store.Invocation{
		TxHash:         testTxHash,
		ContractID:     v2Addr("I"),
		Network:        "testnet",
		Ledger:         42,
		LedgerClosedAt: now,
		Status:         "SUCCESS",
		FunctionName:   "transfer",
	}
	_ = ms.BatchInsertInvocations(ctx, []store.Invocation{inv})
	_ = ms.BatchInsertEvents(ctx, []store.Event{{
		ID:             "evt-1",
		ContractID:     v2Addr("I"),
		Network:        "testnet",
		Ledger:         42,
		LedgerClosedAt: now,
		TxHash:         testTxHash,
		Type:           "contract",
	}})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/transactions/"+testTxHash, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["tx_hash"] != testTxHash {
		t.Errorf("tx_hash: %v", body["tx_hash"])
	}
	events, ok := body["events"].([]any)
	if !ok || len(events) != 1 {
		t.Errorf("events: %v", body["events"])
	}
	if body["invocation"] == nil {
		t.Error("invocation must be set")
	}
}

func TestGetLedgerNotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledgers/9999?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body)
	}
}

func TestGetLedgerInvalidSequence(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledgers/abc?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body)
	}
}

func TestGetLedgerReturnsSummary(t *testing.T) {
	ms := store.NewMockStore()
	now := time.Now().UTC()
	_ = ms.BatchInsertInvocations(ctx, []store.Invocation{{
		TxHash:         testTxHash,
		ContractID:     v2Addr("L"),
		Network:        "testnet",
		Ledger:         100,
		LedgerClosedAt: now,
		Status:         "SUCCESS",
	}})
	_ = ms.BatchInsertEvents(ctx, []store.Event{{
		ID:             "evt-led",
		ContractID:     v2Addr("L"),
		Network:        "testnet",
		Ledger:         100,
		LedgerClosedAt: now,
		TxHash:         testTxHash,
	}})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ledgers/100?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["sequence"] != float64(100) {
		t.Errorf("sequence: %v", body["sequence"])
	}
	if body["tx_count"] != float64(1) {
		t.Errorf("tx_count: %v", body["tx_count"])
	}
	if body["event_count"] != float64(1) {
		t.Errorf("event_count: %v", body["event_count"])
	}
}
