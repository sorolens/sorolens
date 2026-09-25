package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const storageDiffContract = "CDIFFCONTRACT000000000000000000000000000000000000000000000"

// seedStorageDiffStore records a small history of storage snapshots so the
// endpoint can be exercised against "what storage looked like at ledger N".
//
// Between ledger 1000 and 3000:
//   - keyUpdated changed value (persistent)
//   - keyCreated appeared (instance)
//   - keyDeleted disappeared with TTL remaining (persistent)
//   - keyExpired ran out of TTL (temporary)
//   - keySame never changed (persistent)
func seedStorageDiffStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      storageDiffContract,
		Network: "testnet",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ms.UpsertStorageEntries(nil, []store.StorageEntry{
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keyUpdated", ValueXDR: "v1", Durability: "persistent", LastModifiedLedger: 1000, Status: "live"},
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keyUpdated", ValueXDR: "v2", Durability: "persistent", LastModifiedLedger: 3000, Status: "live"},
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keyCreated", ValueXDR: "new", Durability: "instance", LastModifiedLedger: 2500, Status: "live"},
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keyDeleted", ValueXDR: "gone", Durability: "persistent", LastModifiedLedger: 900, Status: "live"},
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keyDeleted", ValueXDR: "gone", Durability: "persistent", LastModifiedLedger: 3000, Status: "deleted"},
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keyExpired", ValueXDR: "ttl", Durability: "temporary", LiveUntilLedger: 2000, LastModifiedLedger: 800, Status: "live"},
		{ContractID: storageDiffContract, Network: "testnet", KeyXDR: "keySame", ValueXDR: "same", Durability: "persistent", LastModifiedLedger: 500, Status: "live"},
	}); err != nil {
		t.Fatal(err)
	}
	return ms
}

type storageDiffEntryBody struct {
	ValueXDR   string `json:"value_xdr"`
	Durability string `json:"durability"`
}

type storageDiffChangeBody struct {
	KeyXDR        string                `json:"key_xdr"`
	Kind          string                `json:"kind"`
	Durability    string                `json:"durability"`
	ChangedFields []string              `json:"changed_fields"`
	Before        *storageDiffEntryBody `json:"before"`
	After         *storageDiffEntryBody `json:"after"`
}

type storageDiffBody struct {
	ContractID string                  `json:"contract_id"`
	FromLedger uint32                  `json:"from_ledger"`
	ToLedger   uint32                  `json:"to_ledger"`
	Counts     map[string]int          `json:"counts"`
	Changes    []storageDiffChangeBody `json:"changes"`
}

func getStorageDiff(t *testing.T, srv http.Handler, query string) (*httptest.ResponseRecorder, storageDiffBody) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/"+storageDiffContract+"/storage/diff"+query, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var body storageDiffBody
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode storage diff: %v", err)
		}
	}
	return w, body
}

func diffChange(t *testing.T, body storageDiffBody, key string) storageDiffChangeBody {
	t.Helper()
	for _, c := range body.Changes {
		if c.KeyXDR == key {
			return c
		}
	}
	t.Fatalf("no change for key %q (changes: %+v)", key, body.Changes)
	return storageDiffChangeBody{}
}

func TestContractStorageDiff(t *testing.T) {
	srv := newTestHandler(seedStorageDiffStore(t), true, true)

	w, body := getStorageDiff(t, srv, "?from=1000&to=3000")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if body.ContractID != storageDiffContract {
		t.Errorf("contract_id: want %s, got %s", storageDiffContract, body.ContractID)
	}
	if body.FromLedger != 1000 || body.ToLedger != 3000 {
		t.Errorf("range: want 1000..3000, got %d..%d", body.FromLedger, body.ToLedger)
	}
	if len(body.Changes) != 4 {
		t.Fatalf("want 4 changes, got %d: %+v", len(body.Changes), body.Changes)
	}

	updated := diffChange(t, body, "keyUpdated")
	if updated.Kind != "updated" {
		t.Errorf("keyUpdated kind: want updated, got %q", updated.Kind)
	}
	if updated.Before == nil || updated.Before.ValueXDR != "v1" {
		t.Errorf("keyUpdated before: want v1, got %+v", updated.Before)
	}
	if updated.After == nil || updated.After.ValueXDR != "v2" {
		t.Errorf("keyUpdated after: want v2, got %+v", updated.After)
	}

	created := diffChange(t, body, "keyCreated")
	if created.Kind != "created" || created.Before != nil {
		t.Errorf("keyCreated: want created with no before, got %+v", created)
	}
	if created.After == nil || created.After.Durability != "instance" {
		t.Errorf("keyCreated after: want instance durability, got %+v", created.After)
	}

	deleted := diffChange(t, body, "keyDeleted")
	if deleted.Kind != "deleted" {
		t.Errorf("keyDeleted: want deleted, got %+v", deleted)
	}
	if deleted.Before == nil || deleted.Before.ValueXDR != "gone" {
		t.Errorf("keyDeleted before: want the live value, got %+v", deleted.Before)
	}

	expired := diffChange(t, body, "keyExpired")
	if expired.Kind != "expired" || expired.Before == nil || expired.Before.Durability != "temporary" {
		t.Errorf("keyExpired: want expired temporary entry, got %+v", expired)
	}

	for _, c := range body.Changes {
		if c.KeyXDR == "keySame" {
			t.Errorf("keySame is unchanged and must not appear in the diff")
		}
	}
	if body.Counts["created"] != 1 || body.Counts["updated"] != 1 || body.Counts["deleted"] != 1 || body.Counts["expired"] != 1 {
		t.Errorf("unexpected counts: %v", body.Counts)
	}
}

func TestContractStorageDiffRequiresRange(t *testing.T) {
	srv := newTestHandler(seedStorageDiffStore(t), true, true)

	cases := []string{
		"",
		"?from=1000",
		"?to=3000",
		"?from=abc&to=3000",
		"?from=0&to=3000",
		"?from=1000&to=nope",
		"?from=3000&to=1000",
	}
	for _, q := range cases {
		w, _ := getStorageDiff(t, srv, q)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("query %q: want 422, got %d (%s)", q, w.Code, w.Body.String())
		}
	}
}

func TestContractStorageDiffUnknownContract(t *testing.T) {
	srv := newTestHandler(seedStorageDiffStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CNOPE/storage/diff?from=1000&to=3000", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestContractStorageDiffStoreError(t *testing.T) {
	ms := seedStorageDiffStore(t)
	ms.StorageDiffErr = errors.New("boom")
	srv := newTestHandler(ms, true, true)

	w, _ := getStorageDiff(t, srv, "?from=1000&to=3000")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d (%s)", w.Code, w.Body.String())
	}
}
