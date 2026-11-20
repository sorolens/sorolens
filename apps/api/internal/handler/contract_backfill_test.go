package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func getContractJSON(t *testing.T, ms *store.MockStore, path string) map[string]any {
	t.Helper()
	srv := newTestHandler(ms, true, true)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s", path, w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body
}

func TestGetContract_ReportsBackfillProgress(t *testing.T) {
	started := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID: "CBACKFILLING", Network: "testnet", Status: "backfilling",
		Backfill: &store.BackfillProgress{
			OldestLedger: 1_001,
			NewestLedger: 2_000,
			NextLedger:   1_750,
			StartedAt:    started,
			UpdatedAt:    started.Add(time.Minute),
		},
	}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/api/v1/contracts/CBACKFILLING", "/api/v2/contracts/CBACKFILLING"} {
		t.Run(path, func(t *testing.T) {
			body := getContractJSON(t, ms, path)
			if body["status"] != "backfilling" {
				t.Errorf("status: got %v", body["status"])
			}
			backfill, ok := body["backfill"].(map[string]any)
			if !ok {
				t.Fatalf("backfill missing: %v", body)
			}
			want := map[string]float64{
				"oldest_ledger": 1_001,
				"newest_ledger": 2_000,
				"next_ledger":   1_750,
				"ledgers_total": 1_000,
				"ledgers_done":  250,
				"percent":       25,
			}
			for key, value := range want {
				if backfill[key] != value {
					t.Errorf("%s: want %v, got %v", key, value, backfill[key])
				}
			}
			if backfill["completed_at"] != nil {
				t.Errorf("completed_at: want null, got %v", backfill["completed_at"])
			}
		})
	}
}

func TestGetContract_OmitsBackfillBeforeItStarts(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: "CPENDING", Network: "testnet", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	if _, present := getContractJSON(t, ms, "/api/v1/contracts/CPENDING")["backfill"]; present {
		t.Error("backfill should be absent for a contract the indexer has not started")
	}
}

func TestBackfillProgress_Percent(t *testing.T) {
	cases := []struct {
		name string
		in   store.BackfillProgress
		want float64
	}{
		{"not started", store.BackfillProgress{OldestLedger: 1, NewestLedger: 100, NextLedger: 100}, 0},
		{"rounds down near the end", store.BackfillProgress{OldestLedger: 1, NewestLedger: 100_001, NextLedger: 1}, 99.99},
		{"finished", store.BackfillProgress{OldestLedger: 1, NewestLedger: 100, NextLedger: 0}, 100},
		{"empty range", store.BackfillProgress{OldestLedger: 10, NewestLedger: 5, NextLedger: 5}, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Percent(); got != tc.want {
				t.Errorf("Percent: want %v, got %v", tc.want, got)
			}
		})
	}
}
