package poller

import (
	"context"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/services/indexer/internal/wasm"
)

// TestSnapshotStorage_SkipsInvalidContractID verifies that snapshotStorage
// returns nil without calling the store when the contract ID is not valid hex
// (issue #503). This is the most common case for tests that use arbitrary IDs.
func TestSnapshotStorage_SkipsInvalidContractID(t *testing.T) {
	t.Parallel()
	fs := newFakeStore(nil)
	rpc := &fakeRPC{}
	p := New(rpc, fs, newFakeRedis(), testConfig(), testLogger())
	contract := Contract{ID: "not-valid-hex", Network: "testnet", Status: "active"}
	if err := p.snapshotStorage(context.Background(), rpc, contract); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.storageEntries) != 0 {
		t.Errorf("want 0 storage entries, got %d", len(fs.storageEntries))
	}
}

// TestSnapshotStorage_UpsertsInstanceEntry verifies that snapshotStorage
// upserts the instance entry and marks it live (issue #503).
func TestSnapshotStorage_UpsertsInstanceEntry(t *testing.T) {
	t.Parallel()
	contractIDHex := strings.Repeat("ab", 32) // 64 hex chars
	instanceKey, err := wasm.ContractInstanceKey(contractIDHex)
	if err != nil {
		t.Fatalf("ContractInstanceKey: %v", err)
	}

	fs := newFakeStore(nil)
	rpc := &fakeRPC{
		ledgerEntries: map[string]LedgerEntry{
			instanceKey: {
				Key:                   instanceKey,
				XDR:                   "aGVsbG8=",
				LastModifiedLedgerSeq: 1000,
				LiveUntilLedgerSeq:    2000,
			},
		},
	}
	p := New(rpc, fs, newFakeRedis(), testConfig(), testLogger())
	contract := Contract{ID: contractIDHex, Network: "testnet", Status: "active"}
	if err := p.snapshotStorage(context.Background(), rpc, contract); err != nil {
		t.Fatalf("snapshotStorage: %v", err)
	}
	if len(fs.storageEntries) != 1 {
		t.Fatalf("want 1 storage entry, got %d", len(fs.storageEntries))
	}
	e := fs.storageEntries[0]
	if e.Durability != "instance" {
		t.Errorf("want durability 'instance', got %q", e.Durability)
	}
	if e.Status != "live" {
		t.Errorf("want status 'live', got %q", e.Status)
	}
	if e.LiveUntilLedger != 2000 {
		t.Errorf("want LiveUntilLedger 2000, got %d", e.LiveUntilLedger)
	}
	if e.Network != "testnet" {
		t.Errorf("want Network 'testnet', got %q", e.Network)
	}
}
