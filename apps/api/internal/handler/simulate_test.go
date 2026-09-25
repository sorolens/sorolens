package handler_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/simulator"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ---- helpers ----------------------------------------------------------------

type simulateTestResponse struct {
	Status    string   `json:"status"`
	Network   string   `json:"network"`
	Ledger    uint32   `json:"ledger"`
	Contracts []string `json:"contracts"`
	Cached    bool     `json:"cached"`
	ReturnXDR string   `json:"return_xdr"`
	Events    []struct {
		ValueXDR string `json:"value_xdr"`
		TxHash   string `json:"tx_hash"`
	} `json:"events"`
	Metering struct {
		CPUInsn            int64 `json:"cpu_insn"`
		MemByte            int64 `json:"mem_byte"`
		ResourceFeeCharged int64 `json:"resource_fee_charged"`
	} `json:"metering"`
	Diagnostic *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"diagnostic"`
}

type simulateTestError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// simulationIdentity derives a unique contract payload from the test name so
// each test isolates its cache key from the others.
func simulationIdentity(t *testing.T) (string, []byte) {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	payload := make([]byte, simulator.ContractIDPayloadLen)
	copy(payload, sum[:])
	id, err := simulator.EncodeContractID(payload)
	if err != nil {
		t.Fatal(err)
	}
	return id, payload
}

// simulationEnvelope wraps a contract payload in bytes that look like a
// Soroban InvokeHostFunction operation: the host function type (invoke
// contract = 0) followed by an SCAddress CONTRACT (type 1) and the hash.
func simulationEnvelope(payload []byte) string {
	raw := make([]byte, 0, 8+len(payload))
	raw = append(raw, 0, 0, 0, 0, 0, 0, 0, 1)
	raw = append(raw, payload...)
	return base64.StdEncoding.EncodeToString(raw)
}

func postSimulate(t *testing.T, srv http.Handler, body map[string]any) (int, []byte) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/simulate", bytes.NewBuffer(encoded))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func seedSimulationStore(t *testing.T) (*store.MockStore, string, string) {
	t.Helper()
	contractID, payload := simulationIdentity(t)
	ms := store.NewMockStore()
	must(t, ms.UpsertContract(nil, store.Contract{ID: contractID, Network: "testnet", Status: "active"}))
	must(t, ms.UpsertSyncState(nil, store.SyncState{ContractID: contractID, LastLedger: 3000}))
	must(t, ms.UpsertStorageEntries(nil, []store.StorageEntry{
		{
			ContractID: contractID, Network: "testnet", KeyXDR: "key1", ValueXDR: "v1",
			Durability: "persistent", LastModifiedLedger: 2000, Status: "live",
		},
	}))
	must(t, ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash: "tx-old", ContractID: contractID, Network: "testnet", Ledger: 1000,
			Status: "SUCCESS", FunctionName: "increment",
			ResultDecoded: map[string]any{"count": 1}, ResultXDR: "OLD",
			CPUInsn: 100, MemByte: 200, LedgerReadByte: 300, LedgerWriteByte: 400, ResourceFeeCharged: 500,
		},
		{
			TxHash: "tx-new", ContractID: contractID, Network: "testnet", Ledger: 2500,
			Status: "SUCCESS", FunctionName: "increment",
			ResultDecoded: map[string]any{"count": 7}, ResultXDR: "NEW",
			CPUInsn: 1234, MemByte: 2048, LedgerReadByte: 512, LedgerWriteByte: 256, ResourceFeeCharged: 999,
		},
	}))
	must(t, ms.BatchInsertEvents(nil, []store.Event{
		{ID: "e-old", ContractID: contractID, Network: "testnet", Ledger: 1000, TxHash: "tx-old", Type: "contract", ValueXDR: "old"},
		{ID: "e-new", ContractID: contractID, Network: "testnet", Ledger: 2500, TxHash: "tx-new", Type: "contract", TopicXDR: []string{"t0"}, ValueXDR: "val"},
	}))
	return ms, contractID, simulationEnvelope(payload)
}

// ---- tests ------------------------------------------------------------------

func TestSimulateSuccess(t *testing.T) {
	ms, contractID, xdr := seedSimulationStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": xdr})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", code, body)
	}
	var resp simulateTestResponse
	must(t, json.Unmarshal(body, &resp))

	if resp.Status != simulator.StatusSuccess {
		t.Fatalf("status: want %q, got %q (%s)", simulator.StatusSuccess, resp.Status, body)
	}
	if resp.Network != "testnet" {
		t.Errorf("network: want testnet, got %q", resp.Network)
	}
	if resp.Ledger != 3000 {
		t.Errorf("ledger: want the indexer's last synced ledger 3000, got %d", resp.Ledger)
	}
	if len(resp.Contracts) != 1 || resp.Contracts[0] != contractID {
		t.Errorf("contracts: want [%s], got %v", contractID, resp.Contracts)
	}
	if resp.ReturnXDR != "NEW" {
		t.Errorf("return_xdr: want the most recent indexed result NEW, got %q", resp.ReturnXDR)
	}
	if resp.Metering.CPUInsn != 1234 || resp.Metering.MemByte != 2048 || resp.Metering.ResourceFeeCharged != 999 {
		t.Errorf("metering: want {1234 2048 999}, got %+v", resp.Metering)
	}
	if len(resp.Events) != 1 || resp.Events[0].ValueXDR != "val" {
		t.Errorf("events: want only tx-new's event, got %+v", resp.Events)
	}
	if resp.Cached {
		t.Error("the first simulation of a transaction should not be cached")
	}
	if resp.Diagnostic != nil {
		t.Errorf("diagnostic: want none, got %+v", resp.Diagnostic)
	}
}

func TestSimulateCachesIdenticalRequests(t *testing.T) {
	ms, _, xdr := seedSimulationStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": xdr})
	if code != http.StatusOK {
		t.Fatalf("first: want 200, got %d (%s)", code, body)
	}
	var first simulateTestResponse
	must(t, json.Unmarshal(body, &first))
	if first.Cached {
		t.Fatal("first request should not be marked cached")
	}

	code, body = postSimulate(t, srv, map[string]any{"xdr": xdr})
	if code != http.StatusOK {
		t.Fatalf("second: want 200, got %d (%s)", code, body)
	}
	var second simulateTestResponse
	must(t, json.Unmarshal(body, &second))
	if !second.Cached {
		t.Error("an identical simulation within the TTL should be served from cache")
	}
	if second.ReturnXDR != first.ReturnXDR {
		t.Errorf("cached result changed: first %q, second %q", first.ReturnXDR, second.ReturnXDR)
	}
}

func TestSimulateRejectsInvalidXDR(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": "not-base64!!"})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", code, body)
	}
	var errResp simulateTestError
	must(t, json.Unmarshal(body, &errResp))
	if errResp.Error.Code != simulator.CodeInvalidXDR {
		t.Errorf("code: want %s, got %q", simulator.CodeInvalidXDR, errResp.Error.Code)
	}
}

func TestSimulateRejectsMissingXDR(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)

	code, body := postSimulate(t, srv, map[string]any{})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", code, body)
	}
	var errResp simulateTestError
	must(t, json.Unmarshal(body, &errResp))
	if errResp.Error.Code != simulator.CodeInvalidXDR {
		t.Errorf("code: want %s, got %q", simulator.CodeInvalidXDR, errResp.Error.Code)
	}
}

func TestSimulateRejectsUntrackedContract(t *testing.T) {
	_, _, xdr := seedSimulationStore(t)
	srv := newTestHandler(store.NewMockStore(), true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": xdr})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", code, body)
	}
	var errResp simulateTestError
	must(t, json.Unmarshal(body, &errResp))
	if errResp.Error.Code != simulator.CodeNoIndexedContract {
		t.Errorf("code: want %s, got %q", simulator.CodeNoIndexedContract, errResp.Error.Code)
	}
}

func TestSimulateWithoutIndexedInvocations(t *testing.T) {
	contractID, payload := simulationIdentity(t)
	ms := store.NewMockStore()
	must(t, ms.UpsertContract(nil, store.Contract{ID: contractID, Network: "testnet", Status: "active"}))
	srv := newTestHandler(ms, true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": simulationEnvelope(payload)})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", code, body)
	}
	var resp simulateTestResponse
	must(t, json.Unmarshal(body, &resp))
	if resp.Status != simulator.StatusFailure {
		t.Errorf("status: want failure, got %q", resp.Status)
	}
	if resp.Diagnostic == nil || resp.Diagnostic.Code != simulator.CodeNoIndexedState {
		t.Errorf("diagnostic: want %s, got %+v", simulator.CodeNoIndexedState, resp.Diagnostic)
	}
}

func TestSimulateOutOfBudget(t *testing.T) {
	ms, _, xdr := seedSimulationStore(t)
	srv := newTestHandler(ms, true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": xdr, "budget_cpu_insn": 1000})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", code, body)
	}
	var resp simulateTestResponse
	must(t, json.Unmarshal(body, &resp))
	if resp.Status != simulator.StatusFailure {
		t.Fatalf("status: want failure, got %q (%s)", resp.Status, body)
	}
	if resp.Diagnostic == nil || resp.Diagnostic.Code != simulator.CodeOutOfBudget {
		t.Errorf("diagnostic: want %s, got %+v", simulator.CodeOutOfBudget, resp.Diagnostic)
	}
	if resp.Metering.CPUInsn != 1234 {
		t.Errorf("metering should still report the consumed CPU, got %d", resp.Metering.CPUInsn)
	}
}

func TestSimulateRecordsFailedExecution(t *testing.T) {
	contractID, payload := simulationIdentity(t)
	ms := store.NewMockStore()
	must(t, ms.UpsertContract(nil, store.Contract{ID: contractID, Network: "testnet", Status: "active"}))
	must(t, ms.BatchInsertInvocations(nil, []store.Invocation{
		{
			TxHash: "tx-failed", ContractID: contractID, Network: "testnet", Ledger: 1200,
			Status: "FAILED", FunctionName: "transfer", CPUInsn: 42,
		},
	}))
	srv := newTestHandler(ms, true, true)

	code, body := postSimulate(t, srv, map[string]any{"xdr": simulationEnvelope(payload)})
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", code, body)
	}
	var resp simulateTestResponse
	must(t, json.Unmarshal(body, &resp))
	if resp.Status != simulator.StatusFailure {
		t.Fatalf("status: want failure, got %q", resp.Status)
	}
	if resp.Diagnostic == nil || resp.Diagnostic.Code != simulator.CodeSimulationFailed {
		t.Errorf("diagnostic: want %s, got %+v", simulator.CodeSimulationFailed, resp.Diagnostic)
	}
	if resp.Diagnostic != nil && resp.Diagnostic.Message == "" {
		t.Error("a failed simulation should explain itself")
	}
}

func TestSimulateRequiresJSONContentType(t *testing.T) {
	ms, _, xdr := seedSimulationStore(t)
	srv := newTestHandler(ms, true, true)

	body, err := json.Marshal(map[string]any{"xdr": xdr})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/simulate", bytes.NewBuffer(body))
	// deliberately no Content-Type
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", w.Code)
	}
}
