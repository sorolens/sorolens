package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
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

func TestGetInvocation_Success(t *testing.T) {
	ms := store.NewMockStore()
	tx := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	contractID := "C12345678901234567890123456789012345678901234567890123456"
	now := time.Now().UTC().Truncate(time.Second)

	_ = ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:             tx,
			ContractID:         contractID,
			Network:            "testnet",
			Ledger:             100,
			LedgerClosedAt:     now,
			Status:             "SUCCESS",
			FunctionName:       "transfer",
			ArgsDecoded:        map[string]any{"to": "GABC", "amount": float64(1000)},
			ResultDecoded:      "ok",
			ResultXDR:          "AAAA...",
			ResourceFeeCharged: 1234,
			CPUInsn:            5678,
			MemByte:            9012,
			LedgerReadByte:     345,
			LedgerWriteByte:    678,
			ApplicationOrder:   1,
		},
	})

	_ = ms.BatchInsertEvents(nil, []store.Event{
		{
			ID:               "evt-1",
			ContractID:       contractID,
			Network:          "testnet",
			Ledger:           100,
			LedgerClosedAt:   now,
			TxHash:           tx,
			Type:             "contract",
			TopicXDR:         []string{"AAA="},
			ValueXDR:         "BBB=",
			TopicDecoded:     []any{"transfer"},
			ValueDecoded:     map[string]any{"amount": float64(1000)},
			InSuccessfulCall: true,
		},
		{
			ID:               "evt-2",
			ContractID:       contractID,
			Network:          "testnet",
			Ledger:           100,
			LedgerClosedAt:   now,
			TxHash:           tx,
			Type:             "contract",
			TopicXDR:         []string{"CCC="},
			ValueXDR:         "DDD=",
			TopicDecoded:     []any{"fee"},
			ValueDecoded:     map[string]any{"fee": float64(10)},
			InSuccessfulCall: true,
		},
		{
			ID:               "evt-other-tx",
			ContractID:       contractID,
			Network:          "testnet",
			Ledger:           100,
			LedgerClosedAt:   now,
			TxHash:           "other-tx-hash",
			Type:             "contract",
			TopicXDR:         []string{"EEE="},
			ValueXDR:         "FFF=",
			InSuccessfulCall: true,
		},
	})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/invocations/"+tx, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var res struct {
		TxHash             string           `json:"tx_hash"`
		ContractID         string           `json:"contract_id"`
		Network            string           `json:"network"`
		Ledger             uint32           `json:"ledger"`
		Status             string           `json:"status"`
		FunctionName       string           `json:"function_name"`
		ArgsDecoded        map[string]any   `json:"args_decoded"`
		ResultDecoded      any              `json:"result_decoded"`
		ResultXDR          string           `json:"result_xdr"`
		ResourceFeeCharged int64            `json:"resource_fee_charged"`
		CPUInsn            int64            `json:"cpu_insn"`
		MemByte            int64            `json:"mem_byte"`
		LedgerReadByte     int64            `json:"ledger_read_byte"`
		LedgerWriteByte    int64            `json:"ledger_write_byte"`
		ApplicationOrder   int              `json:"application_order"`
		Events             []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if res.TxHash != tx {
		t.Errorf("want tx_hash=%s, got %s", tx, res.TxHash)
	}
	if res.ContractID != contractID {
		t.Errorf("want contract_id=%s, got %s", contractID, res.ContractID)
	}
	if res.FunctionName != "transfer" {
		t.Errorf("want function_name=transfer, got %s", res.FunctionName)
	}
	if res.Status != "SUCCESS" {
		t.Errorf("want status=SUCCESS, got %s", res.Status)
	}
	if res.ResourceFeeCharged != 1234 {
		t.Errorf("want fee=1234, got %d", res.ResourceFeeCharged)
	}
	if len(res.Events) != 2 {
		t.Fatalf("want 2 events, got %d", len(res.Events))
	}
	if res.Events[0]["id"] != "evt-1" || res.Events[1]["id"] != "evt-2" {
		t.Errorf("unexpected event ordering or IDs: %v", res.Events)
	}
}

func TestGetInvocation_NoEvents(t *testing.T) {
	ms := store.NewMockStore()
	tx := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	contractID := "C12345678901234567890123456789012345678901234567890123456"

	_ = ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:       tx,
			ContractID:   contractID,
			Network:      "testnet",
			Ledger:       101,
			Status:       "SUCCESS",
			FunctionName: "noop",
		},
	})

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/invocations/"+tx, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}

	var res struct {
		TxHash string           `json:"tx_hash"`
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if res.Events == nil {
		t.Fatal("events should not be nil (must be empty slice)")
	}
	if len(res.Events) != 0 {
		t.Fatalf("want 0 events, got %d", len(res.Events))
	}
}

func TestGetInvocation_CaseInsensitive(t *testing.T) {
	ms := store.NewMockStore()
	tx := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	contractID := "C12345678901234567890123456789012345678901234567890123456"

	_ = ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:       tx,
			ContractID:   contractID,
			Network:      "testnet",
			Ledger:       102,
			Status:       "SUCCESS",
			FunctionName: "hello",
		},
	})

	srv := newTestHandler(ms, true, true)
	upperTx := "A1B2C3D4E5F67890123456789012345678901234567890123456789012345678"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/invocations/"+upperTx, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for uppercase tx hash, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetInvocation_NotFound(t *testing.T) {
	ms := store.NewMockStore()
	contractID := "C12345678901234567890123456789012345678901234567890123456"

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/invocations/unknown-hash", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}

	var env map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	errObj, ok := env["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error envelope: %v", env)
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code NOT_FOUND, got %v", errObj["code"])
	}
}

func TestGetInvocation_WrongContract(t *testing.T) {
	ms := store.NewMockStore()
	tx := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	contractA := "C_CONTRACT_A"
	contractB := "C_CONTRACT_B"

	_ = ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash:       tx,
			ContractID:   contractA,
			Network:      "testnet",
			Ledger:       103,
			Status:       "SUCCESS",
			FunctionName: "swap",
		},
	})

	srv := newTestHandler(ms, true, true)
	// Requesting with contractB must return 404
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractB+"/invocations/"+tx, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for wrong contract, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetInvocation_StoreError(t *testing.T) {
	ms := store.NewMockStore()
	ms.GetInvocationWithEventsErr = errors.New("database connection down")
	contractID := "C12345678901234567890123456789012345678901234567890123456"

	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+contractID+"/invocations/any-hash", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetInvocation_Scope(t *testing.T) {
	scope, ok := middleware.RequiredScope(http.MethodGet, "/api/v1/contracts/{id}/invocations/{tx_hash}")
	if !ok {
		t.Fatal("missing required scope mapping for GET /api/v1/contracts/{id}/invocations/{tx_hash}")
	}
	if scope != middleware.ScopeReadContracts {
		t.Errorf("want %q, got %q", middleware.ScopeReadContracts, scope)
	}
}
