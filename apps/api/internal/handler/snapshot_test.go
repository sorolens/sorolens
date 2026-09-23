package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const snapshotContract = "CSNAPSHOTCONTRACT00000000000000000000000000000000000000000"

func seedSnapshotStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      snapshotContract,
		Network: "testnet",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}

	// The contract was first tracked at ledger 1000.
	if err := ms.BatchInsertEvents(nil, []store.Event{
		{
			ID: "e1", ContractID: snapshotContract, Network: "testnet",
			Ledger: 1000, LedgerClosedAt: time.Now().UTC(), TxHash: "tx1", Type: "contract",
			ValueXDR: "VALUE_AT_1000",
		},
		{
			ID: "e2", ContractID: snapshotContract, Network: "testnet",
			Ledger: 3000, LedgerClosedAt: time.Now().UTC(), TxHash: "tx2", Type: "contract",
			ValueXDR: "VALUE_AT_3000",
		},
	}); err != nil {
		t.Fatal(err)
	}

	// key1 was written at 1000 then overwritten at 3000.
	// key2 only appeared at 3000.
	if err := ms.UpsertStorageEntries(nil, []store.StorageEntry{
		{
			ContractID: snapshotContract, Network: "testnet", KeyXDR: "key1",
			ValueXDR: "v1", Durability: "persistent",
			LastModifiedLedger: 1000, Status: "live",
		},
		{
			ContractID: snapshotContract, Network: "testnet", KeyXDR: "key1",
			ValueXDR: "v2", Durability: "persistent",
			LastModifiedLedger: 3000, Status: "live",
		},
		{
			ContractID: snapshotContract, Network: "testnet", KeyXDR: "key2",
			ValueXDR: "w1", Durability: "temporary", LiveUntilLedger: 9000,
			LastModifiedLedger: 3000, Status: "live",
		},
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

type snapshotBody struct {
	ContractID         string `json:"contract_id"`
	Network            string `json:"network"`
	Ledger             uint32 `json:"ledger"`
	FirstTrackedLedger uint32 `json:"first_tracked_ledger"`
	Storage            []struct {
		KeyXDR   string `json:"key_xdr"`
		ValueXDR string `json:"value_xdr"`
	} `json:"storage"`
	LastEvent *struct {
		ID     string `json:"id"`
		Ledger uint32 `json:"ledger"`
	} `json:"last_event"`
}

func getSnapshot(t *testing.T, srv http.Handler, ledger string) (*httptest.ResponseRecorder, snapshotBody) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/"+snapshotContract+"/snapshot?ledger="+ledger, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var body snapshotBody
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode snapshot: %v", err)
		}
	}
	return w, body
}

func storageValue(body snapshotBody, key string) (string, bool) {
	for _, e := range body.Storage {
		if e.KeyXDR == key {
			return e.ValueXDR, true
		}
	}
	return "", false
}

// Case 1: a ledger before the contract was tracked returns 404.
func TestContractSnapshotBeforeTracking(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	w, _ := getSnapshot(t, srv, "500")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error.Code != "NOT_FOUND" {
		t.Errorf("want code=NOT_FOUND, got %q", env.Error.Code)
	}
	if env.Error.Message == "" {
		t.Error("expected a helpful message explaining the snapshot range")
	}
}

// Case 2: a ledger in the middle of history returns the state as of then.
func TestContractSnapshotMidHistory(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	w, body := getSnapshot(t, srv, "1500")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if body.FirstTrackedLedger != 1000 {
		t.Errorf("first_tracked_ledger: want 1000, got %d", body.FirstTrackedLedger)
	}
	if v, ok := storageValue(body, "key1"); !ok || v != "v1" {
		t.Errorf("key1: want v1 at ledger 1500, got %q (present=%v)", v, ok)
	}
	if _, ok := storageValue(body, "key2"); ok {
		t.Error("key2 should not exist at ledger 1500 (written at 3000)")
	}
	if body.LastEvent == nil || body.LastEvent.ID != "e1" {
		t.Fatalf("last_event: want e1, got %+v", body.LastEvent)
	}
}

// Case 3: the current ledger returns the latest state.
func TestContractSnapshotCurrent(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	w, body := getSnapshot(t, srv, "5000")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if body.Ledger != 5000 {
		t.Errorf("ledger: want 5000, got %d", body.Ledger)
	}
	if v, ok := storageValue(body, "key1"); !ok || v != "v2" {
		t.Errorf("key1: want v2 at ledger 5000, got %q (present=%v)", v, ok)
	}
	if v, ok := storageValue(body, "key2"); !ok || v != "w1" {
		t.Errorf("key2: want w1 at ledger 5000, got %q (present=%v)", v, ok)
	}
	if body.LastEvent == nil || body.LastEvent.ID != "e2" {
		t.Fatalf("last_event: want e2, got %+v", body.LastEvent)
	}
	if body.Network != "testnet" {
		t.Errorf("network: want testnet, got %q", body.Network)
	}
}

func TestContractSnapshotRequiresLedger(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/"+snapshotContract+"/snapshot", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/"+snapshotContract+"/snapshot?ledger=notanumber", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422 for bad ledger, got %d", w.Code)
	}
}

func TestContractSnapshotUnknownContract(t *testing.T) {
	srv := newTestHandler(seedSnapshotStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/contracts/CUNKNOWN/snapshot?ledger=1000", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}
