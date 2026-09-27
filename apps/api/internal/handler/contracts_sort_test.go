package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func seedSortableContracts(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	added := map[string]time.Time{
		"sortA": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		"sortB": time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		"sortC": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}
	for id, when := range added {
		if err := ms.UpsertContract(nil, store.Contract{
			ID:      id,
			Network: "testnet",
			Label:   "label-" + id,
			Status:  "active",
			AddedAt: when,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return ms
}

func getContractsSortedBy(t *testing.T, url string) []string {
	t.Helper()
	srv := newTestHandler(seedSortableContracts(t), true, true)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body %s)", w.Code, w.Body.String())
	}
	var parsed struct {
		Contracts []struct {
			ID string `json:"id"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(parsed.Contracts))
	for _, c := range parsed.Contracts {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestListContractsSortsByAddedAt(t *testing.T) {
	got := getContractsSortedBy(t, "/api/v1/contracts?sort=added_at&dir=asc")
	want := []string{"sortA", "sortC", "sortB"} // Jan, Feb, Mar
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("added_at asc = %v, want %v", got, want)
		}
	}

	got = getContractsSortedBy(t, "/api/v1/contracts?sort=added_at&dir=desc")
	want = []string{"sortB", "sortC", "sortA"} // Mar, Feb, Jan
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("added_at desc = %v, want %v", got, want)
		}
	}
}

func TestListContractsCombinesSortWithFilters(t *testing.T) {
	ms := seedSortableContracts(t)
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      "sortD",
		Network: "mainnet",
		Label:   "label-sortD",
		Status:  "paused",
		AddedAt: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts?network=testnet&status=active&sort=added_at&dir=asc", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var parsed struct {
		Contracts []struct {
			ID string `json:"id"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Contracts) != 3 {
		t.Fatalf("want 3 testnet/active contracts, got %d", len(parsed.Contracts))
	}
	if parsed.Contracts[0].ID != "sortA" {
		t.Fatalf("first sorted id = %q, want sortA", parsed.Contracts[0].ID)
	}
}

func TestListContractsRejectsUnknownSort(t *testing.T) {
	srv := newTestHandler(seedSortableContracts(t), true, true)
	for _, url := range []string{
		"/api/v1/contracts?sort=bogus",
		"/api/v1/contracts?sort=label&dir=sideways",
	} {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d", url, w.Code)
		}
	}
}

func TestListContractsSortDefaultsToIDAsc(t *testing.T) {
	got := getContractsSortedBy(t, "/api/v1/contracts?dir=desc")
	want := []string{"sortC", "sortB", "sortA"} // still id asc when no sort column
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
}