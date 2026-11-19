package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// seededHealthScoreStore returns a mock with a contract plus a cached health
// score, as if the indexer had already done one poll pass for it.
func seededHealthScoreStore(t *testing.T, contractID string) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: contractID, Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	err := ms.UpsertContractHealthScore(nil, store.ContractHealthScore{
		ContractID:           contractID,
		Score:                73,
		ComponentUptime:      80,
		ComponentErrorRate:   90,
		ComponentPerformance: 60,
		ComponentStorageTTL:  100,
		ComputedAt:           time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func TestGetContractHealthScore_OK(t *testing.T) {
	contractID := "CONTRACT_A"
	srv := newTestHandler(seededHealthScoreStore(t, contractID), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/health-score", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp struct {
		ContractID string `json:"contract_id"`
		Score      int32  `json:"score"`
		Components struct {
			Uptime      int32 `json:"uptime"`
			ErrorRate   int32 `json:"error_rate"`
			Performance int32 `json:"performance"`
			StorageTTL  int32 `json:"storage_ttl"`
		} `json:"components"`
		ComputedAt string `json:"computed_at"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ContractID != contractID {
		t.Errorf("contract_id = %q, want %q", resp.ContractID, contractID)
	}
	if resp.Score != 73 {
		t.Errorf("score = %d, want 73", resp.Score)
	}
	if resp.Components.Uptime != 80 {
		t.Errorf("uptime = %d, want 80", resp.Components.Uptime)
	}
	if resp.Components.StorageTTL != 100 {
		t.Errorf("storage_ttl = %d, want 100", resp.Components.StorageTTL)
	}
	if resp.ComputedAt != "2025-02-03T04:05:06Z" {
		t.Errorf("computed_at = %q, want RFC3339 UTC", resp.ComputedAt)
	}
}

func TestGetContractHealthScore_UnknownContract(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CNOPE/health-score", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for unknown contract, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetContractHealthScore_NotYetComputed(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{ID: "CONTRACT_B", Network: "testnet", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CONTRACT_B/health-score", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 when score not yet computed, got %d (%s)", w.Code, w.Body.String())
	}
}
func TestGetContractHealthScore_History(t *testing.T) {
	const contractID = "CONTRACT_A"
	ms := seededHealthScoreStore(t, contractID)
	latest := time.Date(2025, 2, 3, 5, 5, 6, 0, time.UTC)
	if err := ms.UpsertContractHealthScore(nil, store.ContractHealthScore{
		ContractID: contractID,
		Score:      81,
		ComputedAt: latest,
	}); err != nil {
		t.Fatal(err)
	}

	srv := newTestHandler(ms, true, true)
	path := "/api/v1/contracts/" + contractID + "/health-score"

	for _, tc := range []struct {
		query  string
		status int
	}{
		{"?history_limit=1", http.StatusOK},
		{"?history_limit=721", http.StatusUnprocessableEntity},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path+tc.query, nil))
			if w.Code != tc.status {
				t.Fatalf("want status %d, got %d (%s)", tc.status, w.Code, w.Body.String())
			}
			if tc.status != http.StatusOK {
				return
			}

			var resp struct {
				History []store.HealthScoreSample `json:"history"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatal(err)
			}
			if len(resp.History) != 1 || resp.History[0].Score != 81 ||
				!resp.History[0].ComputedAt.Equal(latest) {
				t.Fatalf("want only the latest snapshot, got %+v", resp.History)
			}
		})
	}
}
