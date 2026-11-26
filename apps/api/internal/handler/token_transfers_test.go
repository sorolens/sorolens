package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestListTokenTransfersEmpty(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+v2Addr("T")+"/transfers?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	transfers, ok := body["transfers"].([]any)
	if !ok || len(transfers) != 0 {
		t.Errorf("transfers: %v", body["transfers"])
	}
}

func TestListTokenTransfersReturnsData(t *testing.T) {
	ms := store.NewMockStore()
	contractID := v2Addr("T")
	now := time.Now().UTC()
	_ = ms.UpsertTokenTransfer(ctx, store.TokenTransfer{
		ContractID:     contractID,
		Network:        "testnet",
		TxHash:         testTxHash,
		Ledger:         10,
		LedgerClosedAt: now,
		EventID:        "evt-tok-1",
		TransferType:   "transfer",
		FromAddress:    "GABC",
		ToAddress:      "GDEF",
		Amount:         "1000000",
	})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/transfers?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	transfers, ok := body["transfers"].([]any)
	if !ok || len(transfers) != 1 {
		t.Fatalf("transfers: %v", body["transfers"])
	}
	tr := transfers[0].(map[string]any)
	if tr["transfer_type"] != "transfer" {
		t.Errorf("transfer_type: %v", tr["transfer_type"])
	}
	if tr["amount"] != "1000000" {
		t.Errorf("amount: %v", tr["amount"])
	}
}

func TestListTokenTransfersInvalidType(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+v2Addr("T")+"/transfers?network=testnet&type=invalid", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", w.Code, w.Body)
	}
}

func TestListTokenHoldersEmpty(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+v2Addr("H")+"/holders?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	holders, ok := body["holders"].([]any)
	if !ok || len(holders) != 0 {
		t.Errorf("holders: %v", body["holders"])
	}
}

func TestListTokenHoldersReturnsSorted(t *testing.T) {
	ms := store.NewMockStore()
	contractID := v2Addr("H")
	for _, h := range []struct{ holder, balance string }{
		{"GLOW", "500"},
		{"GHIGH", "9000"},
		{"GMID", "1000"},
	} {
		_ = ms.UpsertTokenHolderBalance(ctx, contractID, "testnet", h.holder, h.balance)
	}

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/holders?network=testnet", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	holders, ok := body["holders"].([]any)
	if !ok || len(holders) != 3 {
		t.Fatalf("holders: %v", body["holders"])
	}
	// First holder should be GHIGH (highest balance = "9000").
	first := holders[0].(map[string]any)
	if first["holder"] != "GHIGH" {
		t.Errorf("first holder: %v (want GHIGH)", first["holder"])
	}
}
