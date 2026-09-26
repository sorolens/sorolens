package graph_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/graph"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	contractA = "CAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQDZ7H"
	contractB = "CABAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEJXA"
	contractC = "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3"
)

// countingStore wraps the mock store and counts the relationship lookups the
// dataloaders are responsible for batching.
type countingStore struct {
	*store.MockStore
	getContract  atomic.Int32
	getMonitored atomic.Int32
	listAlerts   atomic.Int32

	// monitoredBarrier, when set, blocks every GetMonitoredContract call
	// until that many calls are in flight at once.
	monitoredBarrier *barrier
}

func (s *countingStore) GetContract(ctx context.Context, id string) (store.Contract, error) {
	s.getContract.Add(1)
	return s.MockStore.GetContract(ctx, id)
}

func (s *countingStore) GetMonitoredContract(ctx context.Context, id string) (store.MonitoredContract, error) {
	s.getMonitored.Add(1)
	if s.monitoredBarrier != nil {
		if err := s.monitoredBarrier.wait(); err != nil {
			return store.MonitoredContract{}, err
		}
	}
	return s.MockStore.GetMonitoredContract(ctx, id)
}

func (s *countingStore) ListAlerts(ctx context.Context, contractID, severity, network string, limit int) ([]store.ContractAlert, error) {
	s.listAlerts.Add(1)
	return s.MockStore.ListAlerts(ctx, contractID, severity, network, limit)
}

type barrier struct {
	n       int
	mu      sync.Mutex
	arrived int
	release chan struct{}
}

func newBarrier(n int) *barrier { return &barrier{n: n, release: make(chan struct{})} }

func (b *barrier) wait() error {
	b.mu.Lock()
	b.arrived++
	if b.arrived == b.n {
		close(b.release)
	}
	b.mu.Unlock()
	select {
	case <-b.release:
		return nil
	case <-time.After(2 * time.Second):
		return context.DeadlineExceeded
	}
}

func seedStore(t *testing.T) *countingStore {
	t.Helper()
	ctx := context.Background()
	ms := store.NewMockStore()
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	for i, id := range []string{contractA, contractB, contractC} {
		if err := ms.UpsertContract(ctx, store.Contract{
			ID: id, Network: "testnet", Label: "c" + string(rune('A'+i)), Status: "active", AddedAt: base,
		}); err != nil {
			t.Fatal(err)
		}
		if err := ms.UpsertMonitoredContract(ctx, store.MonitoredContract{
			ContractID: id, Network: "testnet", Name: "svc" + string(rune('A'+i)), Status: "Healthy",
			RegisteredAt: base,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// Three alerts per contract for A and B.
	for i := 0; i < 6; i++ {
		id := contractA
		if i%2 == 1 {
			id = contractB
		}
		sev := "Warning"
		if i < 2 {
			sev = "Critical"
		}
		if err := ms.InsertContractAlert(ctx, store.ContractAlert{
			ContractID: id, Severity: sev, Message: "alert", Ledger: int64(100 + i),
			TxHash: "tx" + string(rune('0'+i)), Timestamp: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ms.BatchInsertEvents(ctx, []store.Event{
		{ID: "e1", ContractID: contractA, Network: "testnet", Ledger: 10, Type: "contract", LedgerClosedAt: base},
		{ID: "e2", ContractID: contractA, Network: "testnet", Ledger: 11, Type: "contract", LedgerClosedAt: base},
		{ID: "e3", ContractID: contractB, Network: "testnet", Ledger: 12, Type: "contract", LedgerClosedAt: base},
	}); err != nil {
		t.Fatal(err)
	}
	return &countingStore{MockStore: ms}
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func post(t *testing.T, h http.Handler, body map[string]any) gqlResponse {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var resp gqlResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response (status %d): %v", w.Code, err)
	}
	return resp
}

func persisted(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("persisted/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newHandler(t *testing.T, s graph.Store, opts graph.Options) http.Handler {
	t.Helper()
	h, err := graph.NewHandler(s, opts)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func mustNoErrors(t *testing.T, resp gqlResponse) {
	t.Helper()
	if len(resp.Errors) > 0 {
		t.Fatalf("unexpected errors: %+v", resp.Errors)
	}
}

// ---- sample queries ---------------------------------------------------------

func TestQueryContractOverview(t *testing.T) {
	s := seedStore(t)
	h := newHandler(t, s, graph.Options{})

	resp := post(t, h, map[string]any{
		"query":     persisted(t, "contract_overview.graphql"),
		"variables": map[string]any{"id": contractA},
	})
	mustNoErrors(t, resp)

	var data struct {
		Contract struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Events []struct {
				Type   string `json:"type"`
				Ledger int    `json:"ledger"`
			} `json:"events"`
			Stats struct {
				EventCount int `json:"eventCount"`
			} `json:"stats"`
		} `json:"contract"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	c := data.Contract
	if c.ID != contractA || c.Label != "cA" {
		t.Fatalf("unexpected contract: %+v", c)
	}
	if len(c.Events) != 2 || c.Events[0].Type != "contract" {
		t.Fatalf("want 2 events, got %+v", c.Events)
	}
	if c.Stats.EventCount != 2 {
		t.Fatalf("want eventCount 2, got %d", c.Stats.EventCount)
	}

	// Unknown contract resolves to null, not an error.
	resp = post(t, h, map[string]any{
		"query":     persisted(t, "contract_overview.graphql"),
		"variables": map[string]any{"id": "CUNKNOWN"},
	})
	mustNoErrors(t, resp)
	if !strings.Contains(string(resp.Data), `"contract":null`) {
		t.Fatalf("want null contract, got %s", resp.Data)
	}
}

func TestQueryContractsPage(t *testing.T) {
	s := seedStore(t)
	h := newHandler(t, s, graph.Options{})

	resp := post(t, h, map[string]any{
		"query":     persisted(t, "contracts_page.graphql"),
		"variables": map[string]any{"network": "testnet", "first": 2},
	})
	mustNoErrors(t, resp)

	var data struct {
		Contracts struct {
			Nodes []struct {
				ID      string `json:"id"`
				Monitor struct {
					Status string `json:"status"`
				} `json:"monitor"`
				Alerts []struct {
					Severity string `json:"severity"`
				} `json:"alerts"`
			} `json:"nodes"`
			PageInfo struct {
				NextCursor *string `json:"nextCursor"`
			} `json:"pageInfo"`
		} `json:"contracts"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	nodes := data.Contracts.Nodes
	if len(nodes) != 2 || nodes[0].ID != contractA || nodes[1].ID != contractB {
		t.Fatalf("unexpected page: %+v", nodes)
	}
	if nodes[0].Monitor.Status != "Healthy" || len(nodes[0].Alerts) != 3 {
		t.Fatalf("unexpected nested data: %+v", nodes[0])
	}
	if data.Contracts.PageInfo.NextCursor == nil {
		t.Fatal("want a next cursor")
	}

	// Following the cursor returns the last contract.
	resp = post(t, h, map[string]any{
		"query":     persisted(t, "contracts_page.graphql"),
		"variables": map[string]any{"first": 2, "after": *data.Contracts.PageInfo.NextCursor},
	})
	mustNoErrors(t, resp)
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Contracts.Nodes) != 1 || data.Contracts.Nodes[0].ID != contractC || data.Contracts.PageInfo.NextCursor != nil {
		t.Fatalf("unexpected second page: %+v", data.Contracts)
	}
}

func TestQueryWatchdogAlerts(t *testing.T) {
	s := seedStore(t)
	h := newHandler(t, s, graph.Options{})

	resp := post(t, h, map[string]any{
		"query": persisted(t, "watchdog_alerts.graphql"),
	})
	mustNoErrors(t, resp)

	var data struct {
		Watchdog struct {
			Stats struct {
				TotalMonitored int `json:"totalMonitored"`
				CriticalAlerts int `json:"criticalAlerts"`
			} `json:"stats"`
			Alerts []struct {
				Severity string `json:"severity"`
				Contract struct {
					ID string `json:"id"`
				} `json:"contract"`
				Monitor struct {
					Name string `json:"name"`
				} `json:"monitor"`
			} `json:"alerts"`
		} `json:"watchdog"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatal(err)
	}
	wd := data.Watchdog
	if wd.Stats.TotalMonitored != 3 || wd.Stats.CriticalAlerts != 2 {
		t.Fatalf("unexpected stats: %+v", wd.Stats)
	}
	if len(wd.Alerts) != 6 {
		t.Fatalf("want 6 alerts, got %d", len(wd.Alerts))
	}
	for _, a := range wd.Alerts {
		if a.Contract.ID == "" || a.Monitor.Name == "" {
			t.Fatalf("relationship not resolved: %+v", a)
		}
	}

	// N+1: six alerts reference two contracts. Each relationship is loaded
	// once per distinct contract, not once per alert.
	if got := s.getContract.Load(); got != 2 {
		t.Errorf("GetContract calls: want 2 (distinct contracts), got %d", got)
	}
	if got := s.getMonitored.Load(); got != 2 {
		t.Errorf("GetMonitoredContract calls: want 2 (distinct contracts), got %d", got)
	}
}

// ---- dataloader batching ----------------------------------------------------

func TestDataloaderBatchesSiblingLookups(t *testing.T) {
	s := seedStore(t)
	// Every monitor lookup blocks until all three are in flight together.
	// Resolving the three contracts' monitors one by one (N+1) would never
	// fill the barrier and the lookups would time out.
	s.monitoredBarrier = newBarrier(3)
	h := newHandler(t, s, graph.Options{})

	resp := post(t, h, map[string]any{
		"query": `{ contracts { nodes { id monitor { name } } } }`,
	})
	mustNoErrors(t, resp)
	if got := s.getMonitored.Load(); got != 3 {
		t.Fatalf("want 3 monitor lookups in one batch, got %d", got)
	}
	// Query.contracts primes the contract loader, so no per-contract fetch.
	if got := s.getContract.Load(); got != 0 {
		t.Fatalf("want contract lookups served from the primed cache, got %d", got)
	}
}

func TestLoadersAreScopedPerRequest(t *testing.T) {
	s := seedStore(t)
	h := newHandler(t, s, graph.Options{})
	q := map[string]any{"query": `{ contract(id: "` + contractA + `") { id } }`}

	mustNoErrors(t, post(t, h, q))
	mustNoErrors(t, post(t, h, q))
	if got := s.getContract.Load(); got != 2 {
		t.Fatalf("each request must fetch fresh data, got %d fetches", got)
	}
}

// ---- safety -----------------------------------------------------------------

func TestComplexityLimitRejectsHeavyQuery(t *testing.T) {
	s := seedStore(t)
	h := newHandler(t, s, graph.Options{})

	resp := post(t, h, map[string]any{
		"query": `{ contracts(first: 100) { nodes { events(first: 100) { contract {
			events(first: 100) { id } } } } } }`,
	})
	if len(resp.Errors) == 0 || !strings.Contains(resp.Errors[0].Message, "complexity") {
		t.Fatalf("want complexity error, got %+v", resp.Errors)
	}
	if s.getContract.Load() != 0 || s.listAlerts.Load() != 0 {
		t.Fatal("rejected query must not touch the store")
	}
}

func TestListArgumentsAreCapped(t *testing.T) {
	s := seedStore(t)
	h := newHandler(t, s, graph.Options{ComplexityLimit: 1_000_000})

	resp := post(t, h, map[string]any{"query": `{ watchdog { alerts(first: 100000) { txHash } } }`})
	mustNoErrors(t, resp)
	if strings.Count(string(resp.Data), "txHash") != 6 {
		t.Fatalf("unexpected alerts: %s", resp.Data)
	}
}

func TestInvalidNetworkIsAnError(t *testing.T) {
	h := newHandler(t, seedStore(t), graph.Options{})
	resp := post(t, h, map[string]any{"query": `{ contracts(network: "moon") { nodes { id } } }`})
	if len(resp.Errors) == 0 || !strings.Contains(resp.Errors[0].Message, "network must be one of") {
		t.Fatalf("want network error, got %+v", resp.Errors)
	}
}

func apqRequest(hash string) map[string]any {
	return map[string]any{
		"variables": map[string]any{"id": contractA},
		"extensions": map[string]any{
			"persistedQuery": map[string]any{"version": 1, "sha256Hash": hash},
		},
	}
}

func hashOf(t *testing.T, name string) string {
	t.Helper()
	all, err := graph.PersistedQueries()
	if err != nil {
		t.Fatal(err)
	}
	want := persisted(t, name)
	for hash, q := range all {
		if q == want {
			return hash
		}
	}
	t.Fatalf("%s is not in the persisted allowlist", name)
	return ""
}

func TestPersistedQueryByHash(t *testing.T) {
	h := newHandler(t, seedStore(t), graph.Options{})

	resp := post(t, h, apqRequest(hashOf(t, "contract_overview.graphql")))
	mustNoErrors(t, resp)
	if !strings.Contains(string(resp.Data), contractA) {
		t.Fatalf("persisted query did not run: %s", resp.Data)
	}

	resp = post(t, h, apqRequest(strings.Repeat("0", 64)))
	if len(resp.Errors) == 0 || resp.Errors[0].Message != "PersistedQueryNotFound" {
		t.Fatalf("want PersistedQueryNotFound, got %+v", resp.Errors)
	}
}

func TestPersistedOnlyMode(t *testing.T) {
	h := newHandler(t, seedStore(t), graph.Options{PersistedOnly: true})

	// Allowlisted query by hash runs.
	resp := post(t, h, apqRequest(hashOf(t, "contract_overview.graphql")))
	mustNoErrors(t, resp)

	// Allowlisted query sent as text also runs.
	resp = post(t, h, map[string]any{
		"query":     persisted(t, "contract_overview.graphql"),
		"variables": map[string]any{"id": contractA},
	})
	mustNoErrors(t, resp)

	// Ad-hoc query is refused.
	resp = post(t, h, map[string]any{"query": `{ contracts { nodes { id } } }`})
	if len(resp.Errors) == 0 || resp.Errors[0].Message != "only persisted queries are allowed" {
		t.Fatalf("want persisted-only rejection, got %+v", resp.Errors)
	}

	// Introspection is disabled.
	resp = post(t, h, map[string]any{"query": `{ __schema { queryType { name } } }`})
	if len(resp.Errors) == 0 {
		t.Fatal("introspection must be refused in persisted-only mode")
	}
}
