package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func seededUpgradeStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: "CONTRACT_A", Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []store.ContractUpgrade{
		{ContractID: "CONTRACT_A", FromHash: "aaa", ToHash: "bbb", Ledger: 300, TxHash: "tx_older", At: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ContractID: "CONTRACT_A", FromHash: "bbb", ToHash: "ccc", Ledger: 500, TxHash: "tx_newer", At: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)},
	} {
		if err := ms.InsertContractUpgrade(nil, u); err != nil {
			t.Fatal(err)
		}
	}
	return ms
}

func TestListContractUpgrades(t *testing.T) {
	srv := newTestHandler(seededUpgradeStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CONTRACT_A/upgrades", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Upgrades []struct {
			ContractID string    `json:"contract_id"`
			FromHash   string    `json:"from_hash"`
			ToHash     string    `json:"to_hash"`
			Ledger     int64     `json:"ledger"`
			TxHash     string    `json:"tx_hash"`
			At         time.Time `json:"at"`
		} `json:"upgrades"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Upgrades) != 2 {
		t.Fatalf("want 2 upgrades, got %d", len(body.Upgrades))
	}
	// Reverse-chronological order: newest ledger first.
	if body.Upgrades[0].ToHash != "ccc" {
		t.Errorf("want newest first (ccc), got %s", body.Upgrades[0].ToHash)
	}
	if body.Upgrades[1].ToHash != "bbb" {
		t.Errorf("want second newest (bbb), got %s", body.Upgrades[1].ToHash)
	}
}

func TestListContractUpgrades_NotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CNOTRACKED/upgrades", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
	var env map[string]any
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope: %v", env)
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code NOT_FOUND, got %v", errObj["code"])
	}
}

func TestListContractUpgrades_EmptyHistory(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: "CONTRACT_A", Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CONTRACT_A/upgrades", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Upgrades []any `json:"upgrades"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Upgrades) != 0 {
		t.Fatalf("want empty list, got %v", body.Upgrades)
	}
}
