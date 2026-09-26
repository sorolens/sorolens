package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const invocationContractID = "C_INVOCATION_FILTER"

func seedInvocationFilterStore(t *testing.T) *store.MockStore {
	t.Helper()

	ms := store.NewMockStore()
	ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:       "tx-transfer-1",
			ContractID:   invocationContractID,
			Network:      "mainnet",
			Ledger:       1,
			Status:       "SUCCESS",
			FunctionName: "transfer",
		},
		{
			TxHash:       "tx-mint-1",
			ContractID:   invocationContractID,
			Network:      "mainnet",
			Ledger:       2,
			Status:       "SUCCESS",
			FunctionName: "mint",
		},
		{
			TxHash:       "tx-transfer-2",
			ContractID:   invocationContractID,
			Network:      "mainnet",
			Ledger:       3,
			Status:       "SUCCESS",
			FunctionName: "transfer",
		},
	})

	return ms
}

func TestListInvocationsFiltersByFunctionName(t *testing.T) {
	srv := newTestHandler(seedInvocationFilterStore(t), true, true)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/contracts/"+invocationContractID+"/invocations?function_name=transfer",
		nil,
	)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var parsed struct {
		Invocations []struct {
			FunctionName string `json:"function_name"`
		} `json:"invocations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}

	if len(parsed.Invocations) != 2 {
		t.Fatalf("want 2 transfer invocations, got %d", len(parsed.Invocations))
	}

	for _, inv := range parsed.Invocations {
		if inv.FunctionName != "transfer" {
			t.Errorf("want function_name=transfer, got %q", inv.FunctionName)
		}
	}
}

func TestListInvocationsMissingFunctionNameReturnsEmpty(t *testing.T) {
	srv := newTestHandler(seedInvocationFilterStore(t), true, true)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/contracts/"+invocationContractID+"/invocations?function_name=does_not_exist",
		nil,
	)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var parsed struct {
		Invocations []map[string]any `json:"invocations"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}

	if len(parsed.Invocations) != 0 {
		t.Fatalf("want 0 invocations, got %d", len(parsed.Invocations))
	}
}
