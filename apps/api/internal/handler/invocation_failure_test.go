package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// seedFailureStore seeds one successful invocation and one failed invocation per
// failure category, so the filter can be exercised on both list endpoints.
func seedFailureStore(t *testing.T) *store.MockStore {
	t.Helper()

	now := time.Now().UTC()
	ms := store.NewMockStore()
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:         "tx-success",
			ContractID:     invocationContractID,
			Network:        "mainnet",
			Ledger:         1,
			LedgerClosedAt: now,
			Status:         "SUCCESS",
		},
		{
			TxHash:          "tx-budget",
			ContractID:      invocationContractID,
			Network:         "mainnet",
			Ledger:          2,
			LedgerClosedAt:  now,
			Status:          "FAILED",
			FailureCategory: "budget",
			FailureCode:     "sce_budget:5",
		},
		{
			TxHash:          "tx-contract",
			ContractID:      invocationContractID,
			Network:         "mainnet",
			Ledger:          3,
			LedgerClosedAt:  now,
			Status:          "FAILED",
			FailureCategory: "contract",
			FailureCode:     "sce_contract:5",
		},
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

func TestListInvocationsFiltersByFailureCategory(t *testing.T) {
	srv := newTestHandler(seedFailureStore(t), true, true)

	paths := []string{
		"/api/v1/contracts/" + invocationContractID + "/invocations?failure_category=budget",
		"/api/v1/invocations?failure_category=budget",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
			}

			var parsed struct {
				Invocations []struct {
					TxHash          string `json:"tx_hash"`
					FailureCategory string `json:"failure_category"`
					FailureCode     string `json:"failure_code"`
				} `json:"invocations"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
				t.Fatal(err)
			}
			if len(parsed.Invocations) != 1 {
				t.Fatalf("want 1 budget invocation, got %d", len(parsed.Invocations))
			}
			got := parsed.Invocations[0]
			if got.TxHash != "tx-budget" || got.FailureCategory != "budget" || got.FailureCode != "sce_budget:5" {
				t.Fatalf("unexpected invocation %+v", got)
			}
		})
	}
}

// TestListInvocationsUnknownFailureCategoryReturnsEmpty checks that an
// unrecognized category is a filter that matches nothing rather than an error.
func TestListInvocationsUnknownFailureCategoryReturnsEmpty(t *testing.T) {
	srv := newTestHandler(seedFailureStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/"+invocationContractID+"/invocations?failure_category=not-a-category", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Invocations []any `json:"invocations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Invocations) != 0 {
		t.Fatalf("want no invocations, got %d", len(parsed.Invocations))
	}
}

// TestGetInvocationExposesFailureCategory checks the detail endpoint returns the
// classification alongside the raw code.
func TestGetInvocationExposesFailureCategory(t *testing.T) {
	srv := newTestHandler(seedFailureStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/"+invocationContractID+"/invocations/tx-contract", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		FailureCategory string `json:"failure_category"`
		FailureCode     string `json:"failure_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.FailureCategory != "contract" || body.FailureCode != "sce_contract:5" {
		t.Fatalf("failure fields = %+v, want contract/sce_contract:5", body)
	}
}
