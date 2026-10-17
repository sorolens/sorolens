package handler_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const resourceTrendContract = "CTRENDCONTRACT00000000000000000000000000000000000000000"

func seedResourceTrendStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      resourceTrendContract,
		Network: "testnet",
		Label:   "trend contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	midnight := func(d time.Time) time.Time {
		y, mo, dd := d.UTC().Date()
		return time.Date(y, mo, dd, 0, 0, 0, 0, time.UTC)
	}
	twoDaysAgo := midnight(now.AddDate(0, 0, -2))
	yesterday := midnight(now.AddDate(0, 0, -1))
	today := midnight(now)
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{
		// two days ago: two invocations
		{TxHash: "inv-1", ContractID: resourceTrendContract, Network: "testnet", Ledger: 100, LedgerClosedAt: twoDaysAgo, Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 1000, MemByte: 500, ResourceFeeCharged: 100},
		{TxHash: "inv-2", ContractID: resourceTrendContract, Network: "testnet", Ledger: 101, LedgerClosedAt: twoDaysAgo, Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 3000, MemByte: 700, ResourceFeeCharged: 200},
		// yesterday: one invocation
		{TxHash: "inv-3", ContractID: resourceTrendContract, Network: "testnet", Ledger: 102, LedgerClosedAt: yesterday, Status: "FAILED", FunctionName: "transfer", CPUInsn: 500, MemByte: 100, ResourceFeeCharged: 50},
		// other contract, must be ignored
		{TxHash: "inv-other", ContractID: "COTHERCONTRACT00000000000000000000000000000000000000000", Network: "testnet", Ledger: 103, LedgerClosedAt: twoDaysAgo, Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 99999, MemByte: 99999, ResourceFeeCharged: 99999},
		// today, must be included (window is inclusive of today)
		{TxHash: "inv-today", ContractID: resourceTrendContract, Network: "testnet", Ledger: 104, LedgerClosedAt: today, Status: "SUCCESS", FunctionName: "transfer", CPUInsn: 400, MemByte: 400, ResourceFeeCharged: 400},
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func newResourceTrendTestHandler(s *store.MockStore) http.Handler {
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

func doResourceTrendRequest(srv http.Handler, contractID, days string) *httptest.ResponseRecorder {
	url := "/api/v1/contracts/" + contractID + "/resource-trend"
	if days != "" {
		url += "?days=" + days
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

type resourceTrendPoint struct {
	Date        string  `json:"date"`
	AvgCPUInsn  float64 `json:"avg_cpu_insn"`
	AvgMemByte  float64 `json:"avg_mem_byte"`
	AvgFee      float64 `json:"avg_fee"`
	Count       int     `json:"count"`
}

func TestResourceTrend_OK(t *testing.T) {
	srv := newResourceTrendTestHandler(seedResourceTrendStore(t))

	w := doResourceTrendRequest(srv, resourceTrendContract, "30")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var pts []resourceTrendPoint
	if err := json.NewDecoder(w.Body).Decode(&pts); err != nil {
		t.Fatal(err)
	}
	if len(pts) != 30 {
		t.Fatalf("want 30 daily buckets, got %d", len(pts))
	}
	day := func(d time.Time) string {
		y, mo, dd := d.UTC().Date()
		return time.Date(y, mo, dd, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	now := time.Now().UTC()
	twoDaysAgo := day(now.AddDate(0, 0, -2))
	yesterday := day(now.AddDate(0, 0, -1))
	today := day(now)

	// Oldest first; window covers the last 30 days including today.
	if pts[0].Date != day(now.AddDate(0, 0, -29)) || pts[len(pts)-1].Date != today {
		t.Errorf("unexpected window: %s .. %s", pts[0].Date, pts[len(pts)-1].Date)
	}
	// two days ago: two invocations -> avg cpu 2000, mem 600, fee 150, count 2.
	day03 := pts[len(pts)-3]
	if day03.Date != twoDaysAgo {
		t.Errorf("two-days-ago bucket = %+v, want date %s", day03, twoDaysAgo)
	}
	if day03.AvgCPUInsn != 2000 || day03.AvgMemByte != 600 || day03.AvgFee != 150 || day03.Count != 2 {
		t.Errorf("two-days-ago = %+v, want cpu 2000 mem 600 fee 150 count 2", day03)
	}
	// yesterday: one invocation -> cpu 500, mem 100, fee 50, count 1.
	day04 := pts[len(pts)-2]
	if day04.Date != yesterday {
		t.Errorf("yesterday bucket = %+v, want date %s", day04, yesterday)
	}
	if day04.AvgCPUInsn != 500 || day04.AvgMemByte != 100 || day04.AvgFee != 50 || day04.Count != 1 {
		t.Errorf("yesterday = %+v, want cpu 500 mem 100 fee 50 count 1", day04)
	}
	// today: one invocation -> cpu 400, mem 400, fee 400, count 1.
	dayToday := pts[len(pts)-1]
	if dayToday.Date != today {
		t.Errorf("today bucket = %+v, want date %s", dayToday, today)
	}
	if dayToday.AvgCPUInsn != 400 || dayToday.AvgMemByte != 400 || dayToday.AvgFee != 400 || dayToday.Count != 1 {
		t.Errorf("today = %+v, want cpu 400 mem 400 fee 400 count 1", dayToday)
	}
}

func TestResourceTrend_EmptyContract(t *testing.T) {
	srv := newResourceTrendTestHandler(store.NewMockStore())

	w := doResourceTrendRequest(srv, resourceTrendContract, "7")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var pts []resourceTrendPoint
	if err := json.NewDecoder(w.Body).Decode(&pts); err != nil {
		t.Fatal(err)
	}
	if len(pts) != 7 {
		t.Fatalf("want 7 zero-filled buckets, got %d", len(pts))
	}
	for _, p := range pts {
		if p.Count != 0 || p.AvgCPUInsn != 0 || p.AvgMemByte != 0 || p.AvgFee != 0 {
			t.Errorf("bucket %s = %+v, want all zeroes", p.Date, p)
		}
	}
}

func TestResourceTrend_DefaultsTo30Days(t *testing.T) {
	ms := seedResourceTrendStore(t)
	srv := newResourceTrendTestHandler(ms)

	w := doResourceTrendRequest(srv, resourceTrendContract, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var pts []resourceTrendPoint
	if err := json.NewDecoder(w.Body).Decode(&pts); err != nil {
		t.Fatal(err)
	}
	if len(pts) != 30 {
		t.Fatalf("want 30 buckets with default window, got %d", len(pts))
	}
}

func TestResourceTrend_StoreError(t *testing.T) {
	ms := store.NewMockStore()
	ms.ResourceTrendErr = store.ErrNotFound // any error triggers the 500 path
	srv := newResourceTrendTestHandler(ms)

	w := doResourceTrendRequest(srv, resourceTrendContract, "30")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestResourceTrend_InvalidDays(t *testing.T) {
	srv := newResourceTrendTestHandler(seedResourceTrendStore(t))

	for _, days := range []string{"0", "-1", "abc"} {
		t.Run(days, func(t *testing.T) {
			w := doResourceTrendRequest(srv, resourceTrendContract, days)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}
