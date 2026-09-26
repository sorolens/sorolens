package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const summaryContract = "CSUMMARYCONTRACT0000000000000000000000000000000000000000"

// seedSummaryStore returns a mock with one contract that has two events, two
// invocations, one storage entry, and a cached health score.
func seedSummaryStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      summaryContract,
		Network: "testnet",
		Label:   "summary contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := ms.BatchInsertEvents(nil, []store.Event{
		{ID: "evt-old", ContractID: summaryContract, Network: "testnet", Ledger: 100, LedgerClosedAt: now, TxHash: "tx-old", Type: "contract"},
		{ID: "evt-new", ContractID: summaryContract, Network: "testnet", Ledger: 200, LedgerClosedAt: now, TxHash: "tx-new", Type: "contract"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{
		{TxHash: "inv-old", ContractID: summaryContract, Network: "testnet", Ledger: 100, LedgerClosedAt: now, Status: "SUCCESS", FunctionName: "ping"},
		{TxHash: "inv-new", ContractID: summaryContract, Network: "testnet", Ledger: 200, LedgerClosedAt: now, Status: "FAILED", FunctionName: "pong"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertStorageEntries(nil, []store.StorageEntry{
		{ContractID: summaryContract, Network: "testnet", KeyXDR: "key-1", Status: "live", Durability: "persistent"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertContractHealthScore(nil, store.ContractHealthScore{
		ContractID:           summaryContract,
		Score:                88,
		ComponentUptime:      90,
		ComponentErrorRate:   80,
		ComponentPerformance: 85,
		ComponentStorageTTL:  95,
		ComputedAt:           time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

// newSummaryTestHandler mirrors newTestHandler but takes any handler.APIStore,
// so tests can wrap MockStore to observe how many queries a request makes.
func newSummaryTestHandler(s handler.APIStore) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:       s,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &mockRedisClient{},
		Logger:      logger,
	}
	return router.New(h, config.DefaultRequestMaxBodyBytes)
}

func doSummaryRequest(srv http.Handler, contractID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/summary", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

type summaryResponseBody struct {
	ContractID  string `json:"contract_id"`
	Network     string `json:"network"`
	Status      string `json:"status"`
	GeneratedAt string `json:"generated_at"`
	Stats       struct {
		EventCount            int64  `json:"event_count"`
		InvocationCount       int64  `json:"invocation_count"`
		StorageCount          int64  `json:"storage_count"`
		WindowEventCount      int64  `json:"window_event_count"`
		WindowInvocationCount int64  `json:"window_invocation_count"`
		WindowDuration        string `json:"window_duration"`
	} `json:"stats"`
	LatestEvent *struct {
		ID     string `json:"id"`
		TxHash string `json:"tx_hash"`
		Ledger uint32 `json:"ledger"`
	} `json:"latest_event"`
	LatestInvocation *struct {
		TxHash       string `json:"tx_hash"`
		FunctionName string `json:"function_name"`
		Status       string `json:"status"`
	} `json:"latest_invocation"`
	HealthScore *struct {
		Score      int32 `json:"score"`
		Components struct {
			Uptime      int32 `json:"uptime"`
			ErrorRate   int32 `json:"error_rate"`
			Performance int32 `json:"performance"`
			StorageTTL  int32 `json:"storage_ttl"`
		} `json:"components"`
		ComputedAt string `json:"computed_at"`
	} `json:"health_score"`
}

func TestContractSummary_OK(t *testing.T) {
	srv := newSummaryTestHandler(seedSummaryStore(t))

	w := doSummaryRequest(srv, summaryContract)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp summaryResponseBody
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ContractID != summaryContract {
		t.Errorf("contract_id = %q, want %q", resp.ContractID, summaryContract)
	}
	if resp.Network != "testnet" || resp.Status != "active" {
		t.Errorf("network/status = %q/%q, want testnet/active", resp.Network, resp.Status)
	}
	if resp.GeneratedAt == "" {
		t.Error("generated_at should be set")
	}
	if resp.Stats.EventCount != 2 || resp.Stats.InvocationCount != 2 || resp.Stats.StorageCount != 1 {
		t.Errorf("stats counts = %+v, want 2 events, 2 invocations, 1 storage", resp.Stats)
	}
	if resp.Stats.WindowDuration != "24h" {
		t.Errorf("window_duration = %q, want 24h", resp.Stats.WindowDuration)
	}
	if resp.LatestEvent == nil || resp.LatestEvent.TxHash != "tx-new" || resp.LatestEvent.Ledger != 200 {
		t.Errorf("latest_event = %+v, want the ledger-200 event", resp.LatestEvent)
	}
	if resp.LatestInvocation == nil || resp.LatestInvocation.TxHash != "inv-new" || resp.LatestInvocation.Status != "FAILED" {
		t.Errorf("latest_invocation = %+v, want the ledger-200 invocation", resp.LatestInvocation)
	}
	if resp.HealthScore == nil || resp.HealthScore.Score != 88 || resp.HealthScore.Components.StorageTTL != 95 {
		t.Errorf("health_score = %+v, want score 88 with storage_ttl 95", resp.HealthScore)
	}
}

func TestContractSummary_UnknownContract(t *testing.T) {
	srv := newSummaryTestHandler(store.NewMockStore())

	w := doSummaryRequest(srv, "CNOSUCHCONTRACT")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
	var env map[string]any
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code=NOT_FOUND, got %v", errObj["code"])
	}
}

// A contract the indexer has not produced data for yet must still return the
// full schema, with the optional sub-documents explicitly null.
func TestContractSummary_NoIndexedData(t *testing.T) {
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID: summaryContract, Network: "testnet", Label: "idle", Status: "pending",
	}); err != nil {
		t.Fatal(err)
	}
	srv := newSummaryTestHandler(ms)

	w := doSummaryRequest(srv, summaryContract)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var raw map[string]json.RawMessage
	body := w.Body.Bytes()
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"contract_id", "network", "label", "status", "generated_at", "stats", "latest_event", "latest_invocation", "health_score"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}
	for _, key := range []string{"latest_event", "latest_invocation", "health_score"} {
		if string(raw[key]) != "null" {
			t.Errorf("%s = %s, want null", key, raw[key])
		}
	}

	var resp summaryResponseBody
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Stats.EventCount != 0 || resp.Stats.InvocationCount != 0 {
		t.Errorf("stats = %+v, want zero counts", resp.Stats)
	}
}

// countingSummaryStore wraps MockStore to count the composed lookups.
type countingSummaryStore struct {
	*store.MockStore
	statsCalls  int
	invokeCalls int
}

func (c *countingSummaryStore) GetContractStats(ctx context.Context, contractID, window string) (store.ContractStats, error) {
	c.statsCalls++
	return c.MockStore.GetContractStats(ctx, contractID, window)
}

func (c *countingSummaryStore) RecentInvocations(ctx context.Context, contractID string, limit int) ([]store.Invocation, error) {
	c.invokeCalls++
	return c.MockStore.RecentInvocations(ctx, contractID, limit)
}

func TestContractSummary_CacheHitSkipsQueries(t *testing.T) {
	ms := &countingSummaryStore{MockStore: seedSummaryStore(t)}
	srv := newSummaryTestHandler(ms)

	first := doSummaryRequest(srv, summaryContract)
	if first.Code != http.StatusOK {
		t.Fatalf("first request: want 200, got %d (%s)", first.Code, first.Body.String())
	}
	if ms.statsCalls != 1 || ms.invokeCalls != 1 {
		t.Fatalf("first request calls: stats=%d invocations=%d, want 1/1", ms.statsCalls, ms.invokeCalls)
	}

	second := doSummaryRequest(srv, summaryContract)
	if second.Code != http.StatusOK {
		t.Fatalf("second request: want 200, got %d (%s)", second.Code, second.Body.String())
	}
	if ms.statsCalls != 1 || ms.invokeCalls != 1 {
		t.Fatalf("second request within the 5s TTL should hit the cache: stats=%d invocations=%d", ms.statsCalls, ms.invokeCalls)
	}
	if first.Body.String() != second.Body.String() {
		t.Error("cached response body differs from the first response")
	}
}

func TestSummaryCache_ExpiresAfterTTL(t *testing.T) {
	cache := handler.NewSummaryCache(10 * time.Millisecond)
	cache.Set("k", 1)
	if _, ok := cache.Get("k"); !ok {
		t.Fatal("value should be present immediately after Set")
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := cache.Get("k"); ok {
		t.Fatal("value should expire once its TTL has passed")
	}
}

// TestContractSummary_P95Under200ms is the issue's latency acceptance check.
// Each iteration requests a distinct contract, so every request misses the
// memo cache and exercises the full composed query path. It measures the
// handler against the in-memory store, not PostgreSQL round-trip time.
func TestContractSummary_P95Under200ms(t *testing.T) {
	const iterations = 30
	ms := store.NewMockStore()
	now := time.Now().UTC()
	ids := make([]string, 0, iterations)
	for i := 0; i < iterations; i++ {
		id := fmt.Sprintf("CP95CONTRACT%043d", i)
		ids = append(ids, id)
		if err := ms.UpsertContract(nil, store.Contract{ID: id, Network: "testnet", Status: "active"}); err != nil {
			t.Fatal(err)
		}
		if err := ms.BatchInsertEvents(nil, []store.Event{
			{ID: "evt-" + id, ContractID: id, Network: "testnet", Ledger: 100, LedgerClosedAt: now, TxHash: "tx-" + id, Type: "contract"},
		}); err != nil {
			t.Fatal(err)
		}
		if err := ms.BatchInsertInvocations(nil, []store.Invocation{
			{TxHash: "inv-" + id, ContractID: id, Network: "testnet", Ledger: 100, LedgerClosedAt: now, Status: "SUCCESS", FunctionName: "ping"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	srv := newSummaryTestHandler(ms)

	durations := make([]time.Duration, 0, iterations)
	for _, id := range ids {
		start := time.Now()
		w := doSummaryRequest(srv, id)
		durations = append(durations, time.Since(start))
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s summary: want 200, got %d (%s)", id, w.Code, w.Body.String())
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[(iterations*95)/100]
	if p95 > 200*time.Millisecond {
		t.Fatalf("p95 latency = %v, want < 200ms", p95)
	}
}

// The summary route is a scoped read: a key without read:contracts is denied,
// while anonymous reads stay open like the rest of the public v0.1 surface.
func TestContractSummary_ScopeEnforced(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	if w := doRequest(srv, http.MethodGet, "/api/v1/contracts/"+netContractA+"/summary", readWatchdogKey, ""); w.Code != http.StatusForbidden {
		t.Fatalf("watchdog-scoped key: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doRequest(srv, http.MethodGet, "/api/v1/contracts/"+netContractA+"/summary", readContractsKey, ""); w.Code != http.StatusOK {
		t.Fatalf("read:contracts key: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doRequest(srv, http.MethodGet, "/api/v1/contracts/"+netContractA+"/summary", "", ""); w.Code != http.StatusOK {
		t.Fatalf("anonymous read: want 200, got %d (%s)", w.Code, w.Body.String())
	}
}
