package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/failurecategory"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Issue #806: failed invocations carry a failure category and raw code, the
// list endpoints filter by category and the stats endpoint counts them.

const failureContractID = "C_FAILURE_CATEGORY"

func seedFailureCategoryStore(t *testing.T) *store.MockStore {
	t.Helper()
	now := time.Now().UTC()
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(context.Background(), []store.Invocation{
		{TxHash: "tx-ok", ContractID: failureContractID, Network: "testnet", Ledger: 1, LedgerClosedAt: now.Add(-time.Hour), Status: "SUCCESS"},
		{TxHash: "tx-budget", ContractID: failureContractID, Network: "testnet", Ledger: 2, LedgerClosedAt: now.Add(-2 * time.Hour), Status: "FAILED",
			FailureCategory: "budget_exceeded", FailureCode: "Error(Budget, ExceededLimit)"},
		{TxHash: "tx-contract-1", ContractID: failureContractID, Network: "testnet", Ledger: 3, LedgerClosedAt: now.Add(-3 * time.Hour), Status: "FAILED",
			FailureCategory: "contract_error", FailureCode: "Error(Contract, #4)"},
		{TxHash: "tx-contract-2", ContractID: failureContractID, Network: "testnet", Ledger: 4, LedgerClosedAt: now.Add(-72 * time.Hour), Status: "FAILED",
			FailureCategory: "contract_error", FailureCode: "Error(Contract, #1)"},
		{TxHash: "tx-other", ContractID: failureContractID, Network: "testnet", Ledger: 5, LedgerClosedAt: now.Add(-10 * 24 * time.Hour), Status: "FAILED",
			FailureCategory: "other", FailureCode: "Error(#12, #3)"},
		// Indexed before classification existed: counted as failed, not
		// under any category.
		{TxHash: "tx-legacy", ContractID: failureContractID, Network: "testnet", Ledger: 6, LedgerClosedAt: now.Add(-4 * time.Hour), Status: "FAILED"},
		{TxHash: "tx-elsewhere", ContractID: "C_ELSEWHERE", Network: "testnet", Ledger: 7, LedgerClosedAt: now.Add(-time.Hour), Status: "FAILED",
			FailureCategory: "contract_error", FailureCode: "Error(Contract, #2)"},
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

type failureInvocation struct {
	TxHash          string  `json:"tx_hash"`
	Status          string  `json:"status"`
	FailureCategory *string `json:"failure_category"`
	FailureCode     *string `json:"failure_code"`
}

func getInvocations(t *testing.T, srv http.Handler, path string) (int, []failureInvocation, string) {
	t.Helper()
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body struct {
		Invocations []failureInvocation `json:"invocations"`
	}
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
	return w.Code, body.Invocations, w.Body.String()
}

func txHashes(invs []failureInvocation) []string {
	out := make([]string, len(invs))
	for i, inv := range invs {
		out[i] = inv.TxHash
	}
	return out
}

func TestListInvocations_FilterByFailureCategory(t *testing.T) {
	srv := newTestHandler(seedFailureCategoryStore(t), true, true)

	tests := []struct {
		name string
		path string
		want []string
	}{
		{"global contract_error", "/api/v1/invocations?failure_category=contract_error", []string{"tx-elsewhere", "tx-contract-2", "tx-contract-1"}},
		{"global case-insensitive", "/api/v1/invocations?failure_category=Budget_Exceeded", []string{"tx-budget"}},
		{"global other keeps unknown codes", "/api/v1/invocations?failure_category=other", []string{"tx-other"}},
		{"global category with no rows", "/api/v1/invocations?failure_category=auth_failure", []string{}},
		{"global combined with contract_id", "/api/v1/invocations?failure_category=contract_error&contract_id=" + failureContractID, []string{"tx-contract-2", "tx-contract-1"}},
		{"per-contract contract_error", "/api/v1/contracts/" + failureContractID + "/invocations?failure_category=contract_error", []string{"tx-contract-1", "tx-contract-2"}},
		{"per-contract wasm_trap", "/api/v1/contracts/" + failureContractID + "/invocations?failure_category=wasm_trap", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, invs, body := getInvocations(t, srv, tt.path)
			if code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", code, body)
			}
			got := txHashes(invs)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListInvocations_RejectsInvalidFailureCategory(t *testing.T) {
	srv := newTestHandler(seedFailureCategoryStore(t), true, true)
	for _, path := range []string{
		"/api/v1/invocations?failure_category=out_of_gas",
		"/api/v1/contracts/" + failureContractID + "/invocations?failure_category=budget",
	} {
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: want 422, got %d: %s", path, w.Code, w.Body.String())
		}
		var body struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Error.Code != "INVALID_INPUT" {
			t.Errorf("%s: code = %q, want INVALID_INPUT", path, body.Error.Code)
		}
		for _, c := range failurecategory.All() {
			if !strings.Contains(body.Error.Message, string(c)) {
				t.Errorf("%s: message %q does not list %q", path, body.Error.Message, c)
			}
		}
	}
}

func TestListInvocations_ExposesFailureFieldsOnlyOnClassifiedFailures(t *testing.T) {
	srv := newTestHandler(seedFailureCategoryStore(t), true, true)
	code, invs, body := getInvocations(t, srv, "/api/v1/invocations?contract_id="+failureContractID)
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", code, body)
	}
	byHash := map[string]failureInvocation{}
	for _, inv := range invs {
		byHash[inv.TxHash] = inv
	}
	budget := byHash["tx-budget"]
	if budget.FailureCategory == nil || *budget.FailureCategory != "budget_exceeded" ||
		budget.FailureCode == nil || *budget.FailureCode != "Error(Budget, ExceededLimit)" {
		t.Fatalf("tx-budget = %+v", budget)
	}
	for _, hash := range []string{"tx-ok", "tx-legacy"} {
		inv := byHash[hash]
		if inv.FailureCategory != nil || inv.FailureCode != nil {
			t.Fatalf("%s must omit failure fields, got %+v", hash, inv)
		}
	}
}

func TestContractInvocationStats_FailuresByCategory(t *testing.T) {
	srv := newTestHandler(seedFailureCategoryStore(t), true, true)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+failureContractID+"/invocations/stats", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Windows map[string]struct {
			FailedInvocations  int64            `json:"failed_invocations"`
			FailuresByCategory map[string]int64 `json:"failures_by_category"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	wants := map[string]struct {
		failed int64
		counts map[string]int64
	}{
		// tx-legacy is failed but unclassified, so it counts toward
		// failed_invocations and no category.
		"24h": {failed: 3, counts: map[string]int64{"budget_exceeded": 1, "contract_error": 1}},
		"7d":  {failed: 4, counts: map[string]int64{"budget_exceeded": 1, "contract_error": 2}},
		"30d": {failed: 5, counts: map[string]int64{"budget_exceeded": 1, "contract_error": 2, "other": 1}},
	}
	for window, want := range wants {
		got, ok := resp.Windows[window]
		if !ok {
			t.Fatalf("missing window %s", window)
		}
		if got.FailedInvocations != want.failed {
			t.Errorf("%s failed_invocations = %d, want %d", window, got.FailedInvocations, want.failed)
		}
		if len(got.FailuresByCategory) != len(failurecategory.All()) {
			t.Errorf("%s: want every category listed, got %v", window, got.FailuresByCategory)
		}
		for _, c := range failurecategory.All() {
			n, present := got.FailuresByCategory[string(c)]
			if !present {
				t.Errorf("%s: category %q missing (zero counts must be listed)", window, c)
			}
			if n != want.counts[string(c)] {
				t.Errorf("%s: %s = %d, want %d", window, c, n, want.counts[string(c)])
			}
		}
	}
}

func TestContractInvocationStats_EmptyContractListsZeroCounts(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/contracts/C_NONE/invocations/stats", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp struct {
		Windows map[string]struct {
			FailuresByCategory map[string]int64 `json:"failures_by_category"`
		} `json:"windows"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for window, stats := range resp.Windows {
		for _, c := range failurecategory.All() {
			if n, ok := stats.FailuresByCategory[string(c)]; !ok || n != 0 {
				t.Errorf("%s: %s = %d (present %v), want 0", window, c, n, ok)
			}
		}
	}
}
