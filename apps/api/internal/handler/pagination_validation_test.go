package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func seedManyContracts(t *testing.T, count int) (*store.MockStore, string) {
	t.Helper()
	ms := store.NewMockStore()
	var firstID string
	for i := 1; i <= count; i++ {
		id := fmt.Sprintf("C%s%04d", strings.Repeat("A", 51), i)
		if i == 1 {
			firstID = id
		}
		if err := ms.UpsertContract(nil, store.Contract{
			ID:      id,
			Network: "testnet",
			Label:   fmt.Sprintf("contract-%d", i),
			Status:  "active",
			AddedAt: time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return ms, firstID
}

func seedManyEvents(t *testing.T, count int) (*store.MockStore, string) {
	t.Helper()
	ms, contractID := seedManyContracts(t, 1)
	events := make([]store.Event, count)
	for i := 0; i < count; i++ {
		events[i] = store.Event{
			ID:               fmt.Sprintf("evt-%04d", i+1),
			ContractID:       contractID,
			Network:          "testnet",
			Ledger:           uint32(1000 + i),
			LedgerClosedAt:   time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC),
			TxHash:           fmt.Sprintf("txhash%04d", i+1),
			Type:             "contract",
			InSuccessfulCall: true,
		}
	}
	if err := ms.BatchInsertEvents(nil, events); err != nil {
		t.Fatal(err)
	}
	return ms, contractID
}

func seedManyInvocations(t *testing.T, count int) (*store.MockStore, string) {
	t.Helper()
	ms, contractID := seedManyContracts(t, 1)
	invs := make([]store.Invocation, count)
	for i := 0; i < count; i++ {
		invs[i] = store.Invocation{
			TxHash:         fmt.Sprintf("txhash%04d", i+1),
			ContractID:     contractID,
			Network:        "testnet",
			Ledger:         uint32(1000 + i),
			LedgerClosedAt: time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC),
			Status:         "SUCCESS",
			FunctionName:   "transfer",
		}
	}
	if err := ms.BatchInsertInvocations(nil, invs); err != nil {
		t.Fatal(err)
	}
	return ms, contractID
}

func seedManyStorageEntries(t *testing.T, count int) (*store.MockStore, string) {
	t.Helper()
	ms, contractID := seedManyContracts(t, 1)
	entries := make([]store.StorageEntry, count)
	for i := 0; i < count; i++ {
		entries[i] = store.StorageEntry{
			ContractID:         contractID,
			Network:            "testnet",
			KeyXDR:             fmt.Sprintf("key_%04d", i+1),
			ValueXDR:           fmt.Sprintf("val_%04d", i+1),
			Durability:         "persistent",
			LiveUntilLedger:    50000,
			LastModifiedLedger: 1000,
			Status:             "live",
			LastSeenAt:         time.Date(2025, 1, 1, 0, 0, i, 0, time.UTC),
		}
	}
	if err := ms.UpsertStorageEntries(nil, entries); err != nil {
		t.Fatal(err)
	}
	return ms, contractID
}

func TestListContractsLimitClamping(t *testing.T) {
	ms, _ := seedManyContracts(t, 250)
	srv := newTestHandler(ms, true, true)

	tests := []struct {
		name      string
		url       string
		wantCount int
	}{
		{"huge limit clamped to 200", "/api/v1/contracts?limit=10000000", 200},
		{"limit 250 clamped to 200", "/api/v1/contracts?limit=250", 200},
		{"limit 200 allowed", "/api/v1/contracts?limit=200", 200},
		{"limit 50 allowed", "/api/v1/contracts?limit=50", 50},
		{"omitted limit defaults to 50", "/api/v1/contracts", 50},
		{"v2 huge limit clamped to 200", "/api/v2/contracts?limit=10000000", 200},
		{"v2 limit 250 clamped to 200", "/api/v2/contracts?limit=250", 200},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
			}

			if strings.HasPrefix(tc.url, "/api/v2/") {
				var resp struct {
					Data []any `json:"data"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatal(err)
				}
				if len(resp.Data) != tc.wantCount {
					t.Errorf("got %d items, want %d", len(resp.Data), tc.wantCount)
				}
			} else {
				var resp struct {
					Contracts []any `json:"contracts"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatal(err)
				}
				if len(resp.Contracts) != tc.wantCount {
					t.Errorf("got %d contracts, want %d", len(resp.Contracts), tc.wantCount)
				}
			}
		})
	}
}

func TestListEventsLimitClamping(t *testing.T) {
	ms, contractID := seedManyEvents(t, 250)
	srv := newTestHandler(ms, true, true)

	// v1
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s/events?limit=10000000", contractID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var respV1 struct {
		Events []any `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respV1); err != nil {
		t.Fatal(err)
	}
	if len(respV1.Events) != 200 {
		t.Errorf("v1 events: got %d, want 200", len(respV1.Events))
	}

	// v2
	reqV2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/contracts/%s/events?limit=10000000", contractID), nil)
	wV2 := httptest.NewRecorder()
	srv.ServeHTTP(wV2, reqV2)
	if wV2.Code != http.StatusOK {
		t.Fatalf("v2 want 200, got %d: %s", wV2.Code, wV2.Body.String())
	}
	var respV2 struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(wV2.Body.Bytes(), &respV2); err != nil {
		t.Fatal(err)
	}
	if len(respV2.Data) != 200 {
		t.Errorf("v2 events: got %d, want 200", len(respV2.Data))
	}

	// recent events v1 & v2
	wRecent1 := httptest.NewRecorder()
	srv.ServeHTTP(wRecent1, httptest.NewRequest(http.MethodGet, "/api/v1/events/recent?limit=10000000", nil))
	if wRecent1.Code != http.StatusOK {
		t.Fatalf("recent v1 want 200, got %d: %s", wRecent1.Code, wRecent1.Body.String())
	}
	var recentV1 struct {
		Events []any `json:"events"`
	}
	if err := json.Unmarshal(wRecent1.Body.Bytes(), &recentV1); err != nil {
		t.Fatal(err)
	}
	if len(recentV1.Events) != 200 {
		t.Errorf("recent events v1: got %d, want 200", len(recentV1.Events))
	}

	wRecent2 := httptest.NewRecorder()
	srv.ServeHTTP(wRecent2, httptest.NewRequest(http.MethodGet, "/api/v2/events/recent?limit=10000000", nil))
	if wRecent2.Code != http.StatusOK {
		t.Fatalf("recent v2 want 200, got %d: %s", wRecent2.Code, wRecent2.Body.String())
	}
	var recentV2 struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(wRecent2.Body.Bytes(), &recentV2); err != nil {
		t.Fatal(err)
	}
	if len(recentV2.Data) != 200 {
		t.Errorf("recent events v2: got %d, want 200", len(recentV2.Data))
	}
}

func TestListInvocationsLimitClamping(t *testing.T) {
	ms, contractID := seedManyInvocations(t, 250)
	srv := newTestHandler(ms, true, true)

	// v1 contract invocations
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s/invocations?limit=10000000", contractID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var respV1 struct {
		Invocations []any `json:"invocations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respV1); err != nil {
		t.Fatal(err)
	}
	if len(respV1.Invocations) != 200 {
		t.Errorf("v1 invocations: got %d, want 200", len(respV1.Invocations))
	}

	// v2 contract invocations
	reqV2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/contracts/%s/invocations?limit=10000000", contractID), nil)
	wV2 := httptest.NewRecorder()
	srv.ServeHTTP(wV2, reqV2)
	if wV2.Code != http.StatusOK {
		t.Fatalf("v2 want 200, got %d: %s", wV2.Code, wV2.Body.String())
	}
	var respV2 struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(wV2.Body.Bytes(), &respV2); err != nil {
		t.Fatal(err)
	}
	if len(respV2.Data) != 200 {
		t.Errorf("v2 invocations: got %d, want 200", len(respV2.Data))
	}

	// global invocations
	reqGlobal := httptest.NewRequest(http.MethodGet, "/api/v1/invocations?limit=10000000", nil)
	wGlobal := httptest.NewRecorder()
	srv.ServeHTTP(wGlobal, reqGlobal)
	if wGlobal.Code != http.StatusOK {
		t.Fatalf("global invocations want 200, got %d: %s", wGlobal.Code, wGlobal.Body.String())
	}
	var respGlobal struct {
		Invocations []any `json:"invocations"`
	}
	if err := json.Unmarshal(wGlobal.Body.Bytes(), &respGlobal); err != nil {
		t.Fatal(err)
	}
	if len(respGlobal.Invocations) != 200 {
		t.Errorf("global invocations: got %d, want 200", len(respGlobal.Invocations))
	}
}

func TestListStorageEntriesLimitClamping(t *testing.T) {
	ms, contractID := seedManyStorageEntries(t, 250)
	srv := newTestHandler(ms, true, true)

	// v1
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/contracts/%s/storage?limit=10000000", contractID), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var respV1 struct {
		Storage []any `json:"storage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &respV1); err != nil {
		t.Fatal(err)
	}
	if len(respV1.Storage) != 200 {
		t.Errorf("v1 storage: got %d, want 200", len(respV1.Storage))
	}

	// v2
	reqV2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v2/contracts/%s/storage?limit=10000000", contractID), nil)
	wV2 := httptest.NewRecorder()
	srv.ServeHTTP(wV2, reqV2)
	if wV2.Code != http.StatusOK {
		t.Fatalf("v2 want 200, got %d: %s", wV2.Code, wV2.Body.String())
	}
	var respV2 struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(wV2.Body.Bytes(), &respV2); err != nil {
		t.Fatal(err)
	}
	if len(respV2.Data) != 200 {
		t.Errorf("v2 storage: got %d, want 200", len(respV2.Data))
	}
}

func TestQueryParameterValidationErrors(t *testing.T) {
	ms, contractID := seedManyEvents(t, 5)
	srv := newTestHandler(ms, true, true)

	tests := []struct {
		name    string
		url     string
		wantMsg string
	}{
		// Cursor validation
		{"bad cursor v1 contracts", "/api/v1/contracts?cursor=invalid!base64", "invalid cursor"},
		{"bad cursor v2 contracts", "/api/v2/contracts?cursor=invalid!base64", "invalid cursor"},
		{"bad cursor events", fmt.Sprintf("/api/v1/contracts/%s/events?cursor=invalid!base64", contractID), "invalid cursor"},
		{"bad cursor invocations", fmt.Sprintf("/api/v1/contracts/%s/invocations?cursor=invalid!base64", contractID), "invalid cursor"},
		{"bad cursor all invocations", "/api/v1/invocations?cursor=invalid!base64", "invalid cursor"},
		{"bad cursor storage", fmt.Sprintf("/api/v1/contracts/%s/storage?cursor=invalid!base64", contractID), "invalid cursor"},

		// Network validation
		{"bad network v1 contracts", "/api/v1/contracts?network=devnet", "network must be one of"},
		{"bad network v2 contracts", "/api/v2/contracts?network=devnet", "network must be one of"},
		{"bad network events", fmt.Sprintf("/api/v1/contracts/%s/events?network=devnet", contractID), "network must be one of"},
		{"bad network invocations", fmt.Sprintf("/api/v1/contracts/%s/invocations?network=devnet", contractID), "network must be one of"},
		{"bad network all invocations", "/api/v1/invocations?network=devnet", "network must be one of"},
		{"bad network storage", fmt.Sprintf("/api/v1/contracts/%s/storage?network=devnet", contractID), "network must be one of"},

		// Enum: contract status
		{"bad contract status v1", "/api/v1/contracts?status=bogus", "status must be one of: pending, active, backfilling, error, paused"},
		{"bad contract status v2", "/api/v2/contracts?status=bogus", "status must be one of: pending, active, backfilling, error, paused"},

		// Enum: invocation status
		{"bad invocation status v1", fmt.Sprintf("/api/v1/contracts/%s/invocations?status=BOGUS", contractID), "status must be one of: SUCCESS, FAILED, NOT_FOUND"},
		{"bad invocation status v2", fmt.Sprintf("/api/v2/contracts/%s/invocations?status=BOGUS", contractID), "status must be one of: SUCCESS, FAILED, NOT_FOUND"},
		{"bad invocation status global", "/api/v1/invocations?status=BOGUS", "status must be one of: SUCCESS, FAILED, NOT_FOUND"},

		// Enum: event type (on global events feed)
		{"bad event type global", "/api/v1/events?type=bogus", "type must be one of: contract, system, diagnostic"},

		// Enum: storage durability & status
		{"bad storage durability v1", fmt.Sprintf("/api/v1/contracts/%s/storage?durability=bogus", contractID), "durability must be one of: temporary, persistent, instance"},
		{"bad storage durability v2", fmt.Sprintf("/api/v2/contracts/%s/storage?durability=bogus", contractID), "durability must be one of: temporary, persistent, instance"},
		{"bad storage status v1", fmt.Sprintf("/api/v1/contracts/%s/storage?status=bogus", contractID), "status must be one of: live, archived, deleted"},
		{"bad storage status v2", fmt.Sprintf("/api/v2/contracts/%s/storage?status=bogus", contractID), "status must be one of: live, archived, deleted"},

		// Ledger bounds
		{"malformed from ledger", fmt.Sprintf("/api/v1/contracts/%s/events?from=abc", contractID), "from must be a positive integer"},
		{"malformed to ledger", fmt.Sprintf("/api/v1/contracts/%s/events?to=0", contractID), "to must be a positive integer"},
		{"inverted ledger bounds events v1", fmt.Sprintf("/api/v1/contracts/%s/events?from=200&to=100", contractID), "from must not exceed to"},
		{"inverted ledger bounds events v2", fmt.Sprintf("/api/v2/contracts/%s/events?from=200&to=100", contractID), "from must not exceed to"},
		{"inverted ledger bounds invocations v1", fmt.Sprintf("/api/v1/contracts/%s/invocations?from=200&to=100", contractID), "from must not exceed to"},
		{"inverted ledger bounds invocations v2", fmt.Sprintf("/api/v2/contracts/%s/invocations?from=200&to=100", contractID), "from must not exceed to"},
		{"inverted ledger bounds all invocations", "/api/v1/invocations?from=200&to=100", "from must not exceed to"},

		// Date bounds
		{"malformed since date", "/api/v1/invocations?since=not-a-date", "since must be an RFC3339 timestamp or YYYY-MM-DD date"},
		{"malformed until date", "/api/v1/invocations?until=not-a-date", "until must be an RFC3339 timestamp or YYYY-MM-DD date"},
		{"inverted date bounds", "/api/v1/invocations?since=2026-06-01&until=2026-05-01", "until must be after since"},

		// in_successful_call boolean
		{"bad in_successful_call", fmt.Sprintf("/api/v1/contracts/%s/events?in_successful_call=maybe", contractID), "in_successful_call must be true or false"},

		// Watchdog severity
		{"bad severity watchdog alerts v1", "/api/v1/watchdog/alerts?severity=extreme", "severity must be one of: Info, Warning, Critical"},
		{"bad severity watchdog alerts v2", "/api/v2/watchdog/alerts?severity=extreme", "severity must be one of: Info, Warning, Critical"},
		{"bad severity alert groups", "/api/v1/alerts?severity=extreme", "severity must be one of: Info, Warning, Critical"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d: %s", w.Code, w.Body.String())
			}

			var errEnvelope struct {
				Error struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					RequestID string `json:"request_id"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &errEnvelope); err != nil {
				t.Fatalf("failed to parse standard error envelope: %v (body: %s)", err, w.Body.String())
			}
			if errEnvelope.Error.Code != "INVALID_INPUT" {
				t.Errorf("want error code INVALID_INPUT, got %q", errEnvelope.Error.Code)
			}
			if !strings.Contains(errEnvelope.Error.Message, tc.wantMsg) {
				t.Errorf("error message %q does not contain %q", errEnvelope.Error.Message, tc.wantMsg)
			}
		})
	}
}

func TestGlobalStatsCacheAndApproximateFlag(t *testing.T) {
	ms := seedSortableContracts(t)
	ms.SetGlobalStats(store.GlobalStats{
		TrackedContracts:    10,
		TotalEvents:         1000,
		TotalInvocations:    500,
		TotalStorageEntries: 200,
		Approximate:         true,
	})

	srv := newCachedServer(t, ms)

	// v1 /api/v1/stats/global: first call is a cache MISS
	w1 := doRequest(srv, http.MethodGet, "/api/v1/stats/global", "", "")
	if w1.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/stats/global: want 200, got %d: %s", w1.Code, w1.Body.String())
	}
	if got := w1.Header().Get("X-Cache"); got != "MISS" {
		t.Errorf("first request X-Cache: want MISS, got %q", got)
	}

	var statsV1 struct {
		TrackedContracts    int64 `json:"tracked_contracts"`
		TotalEvents         int64 `json:"total_events"`
		TotalInvocations    int64 `json:"total_invocations"`
		TotalStorageEntries int64 `json:"total_storage_entries"`
		Approximate         bool  `json:"approximate"`
	}
	if err := json.Unmarshal(w1.Body.Bytes(), &statsV1); err != nil {
		t.Fatal(err)
	}
	if !statsV1.Approximate {
		t.Errorf("v1 approximate flag: want true, got false")
	}
	if statsV1.TrackedContracts != 10 || statsV1.TotalEvents != 1000 {
		t.Errorf("v1 unexpected counts: %+v", statsV1)
	}

	// second call is a cache HIT
	w2 := doRequest(srv, http.MethodGet, "/api/v1/stats/global", "", "")
	if w2.Code != http.StatusOK {
		t.Fatalf("second GET /api/v1/stats/global: want 200, got %d: %s", w2.Code, w2.Body.String())
	}
	if got := w2.Header().Get("X-Cache"); got != "HIT" {
		t.Errorf("second request X-Cache: want HIT, got %q", got)
	}

	// v2 /api/v2/stats/global: first call is a cache MISS
	wV2_1 := doRequest(srv, http.MethodGet, "/api/v2/stats/global", "", "")
	if wV2_1.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/stats/global: want 200, got %d: %s", wV2_1.Code, wV2_1.Body.String())
	}
	if got := wV2_1.Header().Get("X-Cache"); got != "MISS" {
		t.Errorf("v2 first request X-Cache: want MISS, got %q", got)
	}

	var statsV2 struct {
		TrackedContracts    int64 `json:"tracked_contracts"`
		TotalEvents         int64 `json:"total_events"`
		TotalInvocations    int64 `json:"total_invocations"`
		TotalStorageEntries int64 `json:"total_storage_entries"`
		Approximate         bool  `json:"approximate"`
	}
	if err := json.Unmarshal(wV2_1.Body.Bytes(), &statsV2); err != nil {
		t.Fatal(err)
	}
	if !statsV2.Approximate {
		t.Errorf("v2 approximate flag: want true, got false")
	}

	// second call is a cache HIT
	wV2_2 := doRequest(srv, http.MethodGet, "/api/v2/stats/global", "", "")
	if wV2_2.Code != http.StatusOK {
		t.Fatalf("second GET /api/v2/stats/global: want 200, got %d: %s", wV2_2.Code, wV2_2.Body.String())
	}
	if got := wV2_2.Header().Get("X-Cache"); got != "HIT" {
		t.Errorf("v2 second request X-Cache: want HIT, got %q", got)
	}
}
