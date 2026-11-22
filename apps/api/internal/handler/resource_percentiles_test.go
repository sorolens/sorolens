package handler_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type percentileTriple struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

type resourcePercentilesBody struct {
	ResourcePercentiles *struct {
		CPUInsn percentileTriple `json:"cpu_insn"`
		MemByte percentileTriple `json:"mem_byte"`
		Fee     percentileTriple `json:"fee"`
	} `json:"resource_percentiles"`
}

func getResourcePercentiles(t *testing.T, ms *store.MockStore, query string) resourcePercentilesBody {
	t.Helper()
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+invocationContractID+"/stats"+query, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body resourcePercentilesBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ResourcePercentiles == nil {
		t.Fatalf("resource_percentiles missing from %s", w.Body.String())
	}
	return body
}

func assertTriple(t *testing.T, metric string, got, want percentileTriple) {
	t.Helper()
	const tolerance = 1e-6
	if math.Abs(got.P50-want.P50) > tolerance || math.Abs(got.P95-want.P95) > tolerance || math.Abs(got.P99-want.P99) > tolerance {
		t.Errorf("%s = %+v, want %+v", metric, got, want)
	}
}

func TestContractStatsResourcePercentiles(t *testing.T) {
	now := time.Now().UTC()
	invocations := make([]store.Invocation, 0, 103)
	// 1..101 interpolates to exact integers at p50, p95 and p99.
	for i := int64(1); i <= 101; i++ {
		invocations = append(invocations, store.Invocation{
			TxHash:             fmt.Sprintf("tx-%d", i),
			ContractID:         invocationContractID,
			LedgerClosedAt:     now.Add(-time.Hour),
			Status:             "SUCCESS",
			CPUInsn:            i * 100,
			MemByte:            i * 10,
			ResourceFeeCharged: i,
		})
	}
	invocations = append(invocations,
		store.Invocation{TxHash: "three-days-old", ContractID: invocationContractID, LedgerClosedAt: now.Add(-72 * time.Hour), Status: "SUCCESS", CPUInsn: 5_000_000, MemByte: 5_000_000, ResourceFeeCharged: 5_000_000},
		store.Invocation{TxHash: "other-contract", ContractID: "C_OTHER", LedgerClosedAt: now.Add(-time.Hour), Status: "SUCCESS", CPUInsn: 9_000_000, MemByte: 9_000_000, ResourceFeeCharged: 9_000_000},
	)
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(nil, invocations); err != nil {
		t.Fatal(err)
	}

	got := getResourcePercentiles(t, ms, "?window=24h").ResourcePercentiles
	assertTriple(t, "cpu_insn", got.CPUInsn, percentileTriple{P50: 5100, P95: 9600, P99: 10000})
	assertTriple(t, "mem_byte", got.MemByte, percentileTriple{P50: 510, P95: 960, P99: 1000})
	assertTriple(t, "fee", got.Fee, percentileTriple{P50: 51, P95: 96, P99: 100})

	// The older invocation only counts once the window reaches it.
	for _, window := range []string{"7d", "30d"} {
		wider := getResourcePercentiles(t, ms, "?window="+window).ResourcePercentiles
		if wider.CPUInsn.P99 <= got.CPUInsn.P99 {
			t.Errorf("%s cpu p99 = %v, want it above the 24h p99 %v", window, wider.CPUInsn.P99, got.CPUInsn.P99)
		}
	}
}

func TestContractStatsResourcePercentilesEmptyWindow(t *testing.T) {
	got := getResourcePercentiles(t, store.NewMockStore(), "").ResourcePercentiles
	zero := percentileTriple{}
	assertTriple(t, "cpu_insn", got.CPUInsn, zero)
	assertTriple(t, "mem_byte", got.MemByte, zero)
	assertTriple(t, "fee", got.Fee, zero)
}

func TestContractStatsResourcePercentilesStoreError(t *testing.T) {
	ms := store.NewMockStore()
	ms.GetContractResourcePercentilesErr = errors.New("boom")
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+invocationContractID+"/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}
