package handler_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestContractInvocationStats(t *testing.T) {
	now := time.Now().UTC()
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{
		{TxHash: "recent-success", ContractID: invocationContractID, LedgerClosedAt: now.Add(-2 * time.Hour), Status: "SUCCESS", ResourceFeeCharged: 10, CPUInsn: 100},
		{TxHash: "recent-failure", ContractID: invocationContractID, LedgerClosedAt: now.Add(-4 * time.Hour), Status: "FAILED", ResourceFeeCharged: 20, CPUInsn: 200},
		{TxHash: "weekly-failure", ContractID: invocationContractID, LedgerClosedAt: now.Add(-48 * time.Hour), Status: "NOT_FOUND", ResourceFeeCharged: 30, CPUInsn: 300},
		{TxHash: "monthly-success", ContractID: invocationContractID, LedgerClosedAt: now.Add(-11 * 24 * time.Hour), Status: "SUCCESS", ResourceFeeCharged: 40, CPUInsn: 400},
		{TxHash: "too-old", ContractID: invocationContractID, LedgerClosedAt: now.Add(-31 * 24 * time.Hour), Status: "FAILED", ResourceFeeCharged: 100, CPUInsn: 1000},
		{TxHash: "other-contract", ContractID: "C_OTHER", LedgerClosedAt: now.Add(-time.Hour), Status: "FAILED", ResourceFeeCharged: 999, CPUInsn: 9999},
	}); err != nil {
		t.Fatal(err)
	}

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+invocationContractID+"/invocations/stats", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Windows map[string]struct {
			InvocationCount   int64   `json:"invocation_count"`
			FailedInvocations int64   `json:"failed_invocations"`
			FailureRate       float64 `json:"failure_rate"`
			AvgFee            float64 `json:"avg_fee"`
			P95Fee            float64 `json:"p95_fee"`
			AvgCPU            float64 `json:"avg_cpu"`
			P95CPU            float64 `json:"p95_cpu"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}

	wants := map[string]struct {
		count, failures int64
		rate            float64
		avgFee, p95Fee  float64
		avgCPU, p95CPU  float64
	}{
		"24h": {count: 2, failures: 1, rate: 0.5, avgFee: 15, p95Fee: 19.5, avgCPU: 150, p95CPU: 195},
		"7d":  {count: 3, failures: 2, rate: 2.0 / 3.0, avgFee: 20, p95Fee: 29, avgCPU: 200, p95CPU: 290},
		"30d": {count: 4, failures: 2, rate: 0.5, avgFee: 25, p95Fee: 38.5, avgCPU: 250, p95CPU: 385},
	}
	if len(response.Windows) != len(wants) {
		t.Fatalf("windows = %v, want keys 24h, 7d, and 30d", response.Windows)
	}
	for name, want := range wants {
		got, ok := response.Windows[name]
		if !ok {
			t.Errorf("missing %s window", name)
			continue
		}
		if got.InvocationCount != want.count || got.FailedInvocations != want.failures {
			t.Errorf("%s counts = %d/%d, want %d/%d", name, got.InvocationCount, got.FailedInvocations, want.count, want.failures)
		}
		for field, pair := range map[string][2]float64{
			"failure_rate": {got.FailureRate, want.rate},
			"avg_fee":      {got.AvgFee, want.avgFee},
			"p95_fee":      {got.P95Fee, want.p95Fee},
			"avg_cpu":      {got.AvgCPU, want.avgCPU},
			"p95_cpu":      {got.P95CPU, want.p95CPU},
		} {
			if math.Abs(pair[0]-pair[1]) > 1e-9 {
				t.Errorf("%s %s = %v, want %v", name, field, pair[0], pair[1])
			}
		}
	}
}
