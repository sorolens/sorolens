package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"log/slog"
	"os"
	"time"
)

type mockRedisClient struct{}

func (m *mockRedisClient) Incr(ctx context.Context, key string) (int64, error) { return 1, nil }
func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return true, nil
}

func newTestHandler(ms *store.MockStore, dbHealthy, redisHealthy bool) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:       ms,
		DB:          &store.MockPinger{Healthy: dbHealthy},
		Redis:       &store.MockPinger{Healthy: redisHealthy},
		RedisClient: &mockRedisClient{},
		Logger:      logger,
	}
	return router.New(h, config.DefaultRequestMaxBodyBytes)
}

// seedRoleStore returns a MockStore with an admin and a contributor user so
// tests that now enforce RBAC on write routes can act as one of them.
func seedRoleStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	adminGH := adminGitHub
	contribGH := contributorGitHub
	ms.AddUser(store.User{ID: adminUser, GitHubID: &adminGH, Role: store.RoleAdmin})
	ms.AddUser(store.User{ID: contributorUser, GitHubID: &contribGH, Role: store.RoleContributor})
	return ms
}

func TestHealth(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %q", body["status"])
	}
}

func TestReadyzHealthy(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestReadyzUnhealthy(t *testing.T) {
	cases := []struct {
		name         string
		dbHealthy    bool
		redisHealthy bool
	}{
		{"db down", false, true},
		{"redis down", true, false},
		{"both down", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestHandler(store.NewMockStore(), tc.dbHealthy, tc.redisHealthy)
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("want 503, got %d", w.Code)
			}
		})
	}
}

func TestRegisterContractInvalidBody(t *testing.T) {
	srv := newTestHandler(seedRoleStore(t), true, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
	var env map[string]any
	_ = json.NewDecoder(w.Body).Decode(&env)
	if _, ok := env["error"]; !ok {
		t.Error("want error envelope")
	}
}

func TestRegisterContractInvalidID(t *testing.T) {
	srv := newTestHandler(seedRoleStore(t), true, true)
	body, _ := json.Marshal(map[string]string{"id": "short", "network": "testnet"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}

func TestRegisterContractSuccess(t *testing.T) {
	srv := newTestHandler(seedRoleStore(t), true, true)
	validID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB2" + "22"
	// Use a well-formed 56-char contract ID starting with C
	validID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	body, _ := json.Marshal(map[string]string{
		"id":      validID,
		"network": "testnet",
		"label":   "my-contract",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != validID {
		t.Errorf("want id=%s, got %v", validID, resp["id"])
	}
	if resp["status"] != "pending" {
		t.Errorf("want status=pending, got %v", resp["status"])
	}
}

func TestGetContractNotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CNONEXISTENT", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	var env map[string]any
	_ = json.NewDecoder(w.Body).Decode(&env)
	errObj, _ := env["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code=NOT_FOUND, got %v", errObj["code"])
	}
}

func TestGlobalStatsShape(t *testing.T) {
	ms := store.NewMockStore()
	ms.SetGlobalStats(store.GlobalStats{
		TrackedContracts:    3,
		TotalEvents:         100,
		TotalInvocations:    50,
		TotalStorageEntries: 25,
	})
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tracked_contracts", "total_events", "total_invocations", "total_storage_entries"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}
	if resp["tracked_contracts"].(float64) != 3 {
		t.Errorf("want tracked_contracts=3, got %v", resp["tracked_contracts"])
	}
}

func seedInvocationStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	t0 := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	err := ms.BatchInsertInvocations(context.Background(), []store.Invocation{
		{TxHash: "aa", ContractID: "CAAA", Network: "testnet", Ledger: 100, LedgerClosedAt: t0, Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 10, MemByte: 1, ResourceFeeCharged: 5},
		{TxHash: "bb", ContractID: "CBBB", Network: "testnet", Ledger: 200, LedgerClosedAt: t0.Add(24 * time.Hour), Status: "FAILED", FunctionName: "mint", CPUInsn: 99, MemByte: 2, ResourceFeeCharged: 7},
		{TxHash: "cc", ContractID: "CAAA", Network: "mainnet", Ledger: 300, LedgerClosedAt: t0.Add(48 * time.Hour), Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 50, MemByte: 3, ResourceFeeCharged: 11},
	})
	if err != nil {
		t.Fatalf("seed invocations: %v", err)
	}
	return ms
}

type invocationsBody struct {
	Invocations []map[string]any `json:"invocations"`
	NextCursor  string           `json:"next_cursor"`
}

func TestListAllInvocations(t *testing.T) {
	srv := newTestHandler(seedInvocationStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body invocationsBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Invocations) != 3 {
		t.Fatalf("want 3 invocations, got %d", len(body.Invocations))
	}
	// Newest ledger first (ledger DESC).
	if got := body.Invocations[0]["tx_hash"]; got != "cc" {
		t.Errorf("first row tx_hash = %v, want cc", got)
	}
	if got := body.Invocations[2]["tx_hash"]; got != "aa" {
		t.Errorf("last row tx_hash = %v, want aa", got)
	}
	// The global response carries the contract id so the table can render it.
	if got := body.Invocations[0]["contract_id"]; got != "CAAA" {
		t.Errorf("contract_id = %v, want CAAA", got)
	}
	if body.NextCursor != "" {
		t.Errorf("next_cursor = %q, want empty on the last page", body.NextCursor)
	}
}

func TestListAllInvocationsFilters(t *testing.T) {
	srv := newTestHandler(seedInvocationStore(t), true, true)

	tests := []struct {
		name    string
		query   string
		want    int
		wantIDs []string
	}{
		{name: "contract filter", query: "contract_id=CAAA", want: 2, wantIDs: []string{"cc", "aa"}},
		{name: "function filter", query: "fn=mint", want: 1, wantIDs: []string{"bb"}},
		{name: "network filter", query: "network=mainnet", want: 1, wantIDs: []string{"cc"}},
		{name: "status filter", query: "status=FAILED", want: 1, wantIDs: []string{"bb"}},
		{name: "ledger range", query: "from=200&to=200", want: 1, wantIDs: []string{"bb"}},
		{name: "date range covers whole days", query: "since=2026-09-02&until=2026-09-02", want: 1, wantIDs: []string{"bb"}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations?"+tc.query, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
			}
			var body invocationsBody
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.Invocations) != tc.want {
				t.Fatalf("want %d invocations, got %d", tc.want, len(body.Invocations))
			}
			for i, wantID := range tc.wantIDs {
				if got := body.Invocations[i]["tx_hash"]; got != wantID {
					t.Errorf("row %d tx_hash = %v, want %s", i, got, wantID)
				}
			}
		})
	}
}

func TestListAllInvocationsPagination(t *testing.T) {
	srv := newTestHandler(seedInvocationStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations?limit=2", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var first invocationsBody
	if err := json.NewDecoder(w.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	if len(first.Invocations) != 2 {
		t.Fatalf("first page: want 2 invocations, got %d", len(first.Invocations))
	}
	if first.NextCursor == "" {
		t.Fatal("first page: want a next_cursor")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/invocations?limit=2&cursor="+first.NextCursor, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var second invocationsBody
	if err := json.NewDecoder(w.Body).Decode(&second); err != nil {
		t.Fatal(err)
	}
	if len(second.Invocations) != 1 {
		t.Fatalf("second page: want 1 invocation, got %d", len(second.Invocations))
	}
	if got := second.Invocations[0]["tx_hash"]; got != "aa" {
		t.Errorf("second page tx_hash = %v, want aa", got)
	}
	if second.NextCursor != "" {
		t.Errorf("second page next_cursor = %q, want empty", second.NextCursor)
	}
}

func TestListAllInvocationsInvalidCursor(t *testing.T) {
	srv := newTestHandler(seedInvocationStore(t), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/invocations?cursor=not-base64!!", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}

func TestContentTypeMiddleware(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	body, _ := json.Marshal(map[string]string{"id": "x", "network": "testnet"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBuffer(body))
	// no Content-Type header
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", w.Code)
	}
}

func TestSearchContracts(t *testing.T) {
	ms := store.NewMockStore()
	_ = ms.UpsertContract(context.Background(), store.Contract{ID: "C12345", Label: "TokenContract", Network: "testnet"})
	_ = ms.UpsertContract(context.Background(), store.Contract{ID: "C67890", Label: "Other", Network: "testnet"})

	srv := newTestHandler(ms, true, true)

	// test hitting the search with a matching query
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=token", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var res map[string][]map[string]any
	if err := json.NewDecoder(w.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	items := res["items"]
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0]["id"] != "C12345" {
		t.Fatalf("want C12345, got %v", items[0]["id"])
	}
	
	// test hitting search with a query matching multiple (or limit)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=C", nil)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, req2)
	var res2 map[string][]map[string]any
	_ = json.NewDecoder(w2.Body).Decode(&res2)
	if len(res2["items"]) != 2 {
		t.Fatalf("want 2 items, got %d", len(res2["items"]))
	}
}
