package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// testLogger returns a logger that discards output so failing assertions are
// not buried in request logs.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// v2Addr builds a syntactically valid 56-character C-address from a repeated
// character, so seeded test contracts pass validateContractID.
func v2Addr(ch string) string { return "C" + strings.Repeat(ch, 55) }

func seedLiveStore(t *testing.T) (*store.MockStore, string, string, string) {
	t.Helper()
	ms := store.NewMockStore()
	a, b, c := v2Addr("A"), v2Addr("B"), v2Addr("C")
	now := time.Now().UTC()
	for _, ct := range []store.Contract{
		{ID: a, Network: "testnet", Label: "Alpha", Status: "active", AddedAt: now},
		{ID: b, Network: "testnet", Label: "Beta", Status: "active", AddedAt: now},
		{ID: c, Network: "mainnet", Status: "active", AddedAt: now},
	} {
		if err := ms.UpsertContract(context.Background(), ct); err != nil {
			t.Fatal(err)
		}
	}
	return ms, a, b, c
}

func getJSON(t *testing.T, srv http.Handler, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("GET %s: decode body: %v", path, err)
	}
	return w.Code, body
}

// TestV2ListPaginationEnvelope pins the v2 list contract: a `data` array plus a
// `pagination` object with an opaque cursor, on every list endpoint.
func TestV2ListPaginationEnvelope(t *testing.T) {
	ms, _, _, _ := seedLiveStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v2/contracts?limit=2")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}

	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("want data array, got %T", body["data"])
	}
	if len(data) != 2 {
		t.Fatalf("want 2 items, got %d", len(data))
	}

	pg, ok := body["pagination"].(map[string]any)
	if !ok {
		t.Fatalf("want pagination object, got %T", body["pagination"])
	}
	if pg["has_more"] != true {
		t.Errorf("want has_more=true on a truncated page, got %v", pg["has_more"])
	}
	cursor, ok := pg["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("want non-empty next_cursor, got %v", pg["next_cursor"])
	}

	// The cursor must actually advance: the next page returns the remaining
	// contract and reports that it is the last page.
	code, body = getJSON(t, srv, "/api/v2/contracts?limit=2&cursor="+cursor)
	if code != http.StatusOK {
		t.Fatalf("want 200 on page 2, got %d", code)
	}
	data, _ = body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("want 1 item on page 2, got %d", len(data))
	}
	pg, _ = body["pagination"].(map[string]any)
	if pg["has_more"] != false {
		t.Errorf("want has_more=false on last page, got %v", pg["has_more"])
	}
	if pg["next_cursor"] != nil {
		t.Errorf("want next_cursor=null on last page, got %v", pg["next_cursor"])
	}
}

// TestV2TimestampsAreRFC3339 enforces convention 3: v2 never returns a unix
// epoch or a non-RFC3339 time.
func TestV2TimestampsAreRFC3339(t *testing.T) {
	ms, a, _, _ := seedLiveStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v2/contracts/"+a)
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}

	addedAt, ok := body["added_at"].(string)
	if !ok {
		t.Fatalf("want added_at string, got %T", body["added_at"])
	}
	if _, err := time.Parse(time.RFC3339, addedAt); err != nil {
		t.Errorf("added_at %q is not RFC3339: %v", addedAt, err)
	}

	// Nullable timestamps stay present as explicit null (convention 4).
	if v, ok := body["backfill_complete_at"]; !ok {
		t.Error("backfill_complete_at key must always be present")
	} else if v != nil {
		t.Errorf("want null backfill_complete_at, got %v", v)
	}
	if body["label"] != "Alpha" {
		t.Errorf("want label Alpha, got %v", body["label"])
	}
}

// TestV2InvocationFieldRenames checks the documented v1 -> v2 renames, and that
// the ambiguous v1 names are gone.
func TestV2InvocationFieldRenames(t *testing.T) {
	ms, a, _, _ := seedLiveStore(t)
	ms.BatchInsertInvocations(context.Background(), []store.Invocation{{
		TxHash:             "tx1",
		ContractID:         a,
		Network:            "testnet",
		Ledger:             10,
		LedgerClosedAt:     time.Now().UTC(),
		Status:             "SUCCESS",
		FunctionName:       "transfer",
		ResourceFeeCharged: 1234,
		CPUInsn:            99,
		MemByte:            42,
		LedgerReadByte:     7,
		LedgerWriteByte:    8,
	}})
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v2/contracts/"+a+"/invocations")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	rows, _ := body["data"].([]any)
	if len(rows) != 1 {
		t.Fatalf("want 1 invocation, got %d", len(rows))
	}
	inv, _ := rows[0].(map[string]any)

	for _, want := range []string{
		"resource_fee_charged_stroops",
		"cpu_instructions",
		"memory_bytes",
		"ledger_read_bytes",
		"ledger_write_bytes",
	} {
		if _, ok := inv[want]; !ok {
			t.Errorf("missing renamed v2 field %q", want)
		}
	}
	for _, gone := range []string{"resource_fee_charged", "cpu_insn", "mem_byte", "ledger_read_byte", "ledger_write_byte"} {
		if _, ok := inv[gone]; ok {
			t.Errorf("v1-only field %q must not appear in a v2 response", gone)
		}
	}
	if inv["ledger_closed_at"] == nil {
		t.Error("ledger_closed_at must be present")
	}
	if _, err := time.Parse(time.RFC3339, inv["ledger_closed_at"].(string)); err != nil {
		t.Errorf("ledger_closed_at is not RFC3339: %v", err)
	}
}

// TestV2EmptyListIsArrayNotNull covers convention 1 for the empty case: clients
// can always iterate `data` without a null check.
func TestV2EmptyListIsArrayNotNull(t *testing.T) {
	ms, a, _, _ := seedLiveStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v2/contracts/"+a+"/events")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	data, ok := body["data"].([]any)
	if !ok {
		t.Fatalf("want data to be a JSON array, got %T (%v)", body["data"], body["data"])
	}
	if len(data) != 0 {
		t.Fatalf("want empty data, got %d items", len(data))
	}
}

// TestV2ErrorsMatchV1Envelope keeps one error shape across both namespaces.
func TestV2ErrorsMatchV1Envelope(t *testing.T) {
	ms, _, _, _ := seedLiveStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := getJSON(t, srv, "/api/v2/contracts/"+v2Addr("Z"))
	if code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", code)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("want error object, got %T", body["error"])
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code NOT_FOUND, got %v", errObj["code"])
	}
	if _, ok := errObj["request_id"]; !ok {
		t.Error("error envelope must carry request_id")
	}
}

// TestV2CoversEveryV1Route walks the live chi route table and asserts that every
// v1 route has a v2 counterpart. This is the acceptance criterion "all v1
// endpoints have a v2 equivalent", enforced mechanically rather than by hand.
func TestV2CoversEveryV1Route(t *testing.T) {
	routes, ok := newTestHandler(store.NewMockStore(), true, true).(chi.Routes)
	if !ok {
		t.Fatal("router does not implement chi.Routes")
	}

	seen := map[string]bool{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		seen[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	var missing []string
	v1Count := 0
	for key := range seen {
		if !strings.Contains(key, "/api/v1/") {
			continue
		}
		v1Count++
		v2Key := strings.Replace(key, "/api/v1/", "/api/v2/", 1)
		if !seen[v2Key] {
			missing = append(missing, key)
		}
	}

	// Guard against the walk silently finding nothing, which would make this
	// test pass without checking anything. v1 currently exposes 33 routes.
	const minV1Routes = 25
	if v1Count < minV1Routes {
		t.Fatalf("route walk found only %d v1 routes (want at least %d); the walk is not exercising the router", v1Count, minV1Routes)
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("v1 routes with no v2 equivalent:\n  %s", strings.Join(missing, "\n  "))
	}
	t.Logf("checked %d v1 routes for v2 coverage", v1Count)
}

// stubColdReader serves a fixed archived event set.
type stubColdReader struct {
	events []store.Event
	calls  int
}

func (s *stubColdReader) Events(_ context.Context, _ string, from, to uint32, limit int) ([]store.Event, error) {
	s.calls++
	var out []store.Event
	for _, e := range s.events {
		if from != 0 && e.Ledger < from {
			continue
		}
		if to != 0 && e.Ledger > to {
			continue
		}
		out = append(out, e)
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// TestV2EventsFallBackToColdStorage covers issue #146 through the v2 namespace:
// when Postgres has nothing for an old ledger range, the archived copy is
// served instead of an empty page.
func TestV2EventsFallBackToColdStorage(t *testing.T) {
	ms, a, _, _ := seedLiveStore(t)
	cold := &stubColdReader{events: []store.Event{{
		ID:             "archived-1",
		ContractID:     a,
		Network:        "testnet",
		Ledger:         100,
		LedgerClosedAt: time.Now().UTC().Add(-100 * 24 * time.Hour),
		TxHash:         "tx-old",
		Type:           "contract",
	}}}
	srv := router.New(&handler.Handler{
		Store:       ms,
		DB:          &store.MockPinger{Healthy: true},
		Redis:       &store.MockPinger{Healthy: true},
		RedisClient: &mockRedisClient{},
		Logger:      testLogger(),
		Cold:        cold,
	})

	code, body := getJSON(t, srv, "/api/v2/contracts/"+a+"/events?from=100&to=200")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("want the archived event, got %d rows", len(data))
	}
	if cold.calls != 1 {
		t.Errorf("want 1 cold lookup, got %d", cold.calls)
	}

	// Without a `from` bound there is no archive lookup; the hot store answer
	// (empty) stands.
	cold.calls = 0
	if _, body = getJSON(t, srv, "/api/v2/contracts/"+a+"/events"); len(body["data"].([]any)) != 0 {
		t.Error("want empty hot result without a from bound")
	}
	if cold.calls != 0 {
		t.Errorf("want no cold lookup without a from bound, got %d", cold.calls)
	}
}
