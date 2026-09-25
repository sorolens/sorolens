package store

import "testing"

// entry is a small helper so the fixture below reads like recorded snapshots.
func entry(key, value, durability string, modified, liveUntil int64) StorageEntry {
	return StorageEntry{
		ContractID:         "CDIFF",
		Network:            "testnet",
		KeyXDR:             key,
		KeyDecoded:         key,
		ValueXDR:           value,
		Durability:         durability,
		LastModifiedLedger: modified,
		LiveUntilLedger:    liveUntil,
		Status:             "live",
	}
}

func changeFor(t *testing.T, diff StorageDiff, key string) StorageChange {
	t.Helper()
	for _, c := range diff.Changes {
		if c.KeyXDR == key {
			return c
		}
	}
	t.Fatalf("no change recorded for key %q (changes: %+v)", key, diff.Changes)
	return StorageChange{}
}

func TestBuildStorageDiffClassifiesChanges(t *testing.T) {
	// Snapshot at ledger 1000.
	before := []StorageEntry{
		entry("keyUpdated", "v1", "persistent", 1000, 0),
		entry("keySame", "same", "persistent", 500, 0),
		entry("keyDeleted", "gone", "instance", 900, 0),
		entry("keyExpired", "ttl", "temporary", 800, 2000),
		entry("keyRenewed", "stable", "temporary", 700, 4000),
	}
	// Snapshot at ledger 3000.
	after := []StorageEntry{
		entry("keyUpdated", "v2", "persistent", 3000, 0),
		entry("keySame", "same", "persistent", 500, 0),
		entry("keyCreated", "new", "instance", 2500, 0),
		entry("keyRenewed", "stable", "temporary", 3000, 8000),
	}

	diff := BuildStorageDiff("CDIFF", 1000, 3000, before, after)
	if diff.From != 1000 || diff.To != 3000 {
		t.Fatalf("range: want 1000..3000, got %d..%d", diff.From, diff.To)
	}
	if len(diff.Changes) != 5 {
		t.Fatalf("want 5 changes, got %d: %+v", len(diff.Changes), diff.Changes)
	}

	updated := changeFor(t, diff, "keyUpdated")
	if updated.Kind != StorageChangeUpdated {
		t.Errorf("keyUpdated kind: want updated, got %q", updated.Kind)
	}
	if updated.Before == nil || updated.Before.ValueXDR != "v1" {
		t.Errorf("keyUpdated before: want v1, got %+v", updated.Before)
	}
	if updated.After == nil || updated.After.ValueXDR != "v2" {
		t.Errorf("keyUpdated after: want v2, got %+v", updated.After)
	}
	if len(updated.ChangedFields) != 1 || updated.ChangedFields[0] != "value" {
		t.Errorf("keyUpdated changed_fields: want [value], got %v", updated.ChangedFields)
	}

	renewed := changeFor(t, diff, "keyRenewed")
	if renewed.Kind != StorageChangeUpdated {
		t.Errorf("keyRenewed kind: want updated, got %q", renewed.Kind)
	}
	if len(renewed.ChangedFields) != 1 || renewed.ChangedFields[0] != "ttl" {
		t.Errorf("keyRenewed changed_fields: want [ttl], got %v", renewed.ChangedFields)
	}

	created := changeFor(t, diff, "keyCreated")
	if created.Kind != StorageChangeCreated || created.Before != nil {
		t.Errorf("keyCreated: want created with no before, got kind=%q before=%+v", created.Kind, created.Before)
	}
	if created.After == nil || created.After.Durability != "instance" {
		t.Errorf("keyCreated after: want instance durability, got %+v", created.After)
	}

	deleted := changeFor(t, diff, "keyDeleted")
	if deleted.Kind != StorageChangeDeleted || deleted.After != nil {
		t.Errorf("keyDeleted: want deleted with no after, got kind=%q after=%+v", deleted.Kind, deleted.After)
	}

	expired := changeFor(t, diff, "keyExpired")
	if expired.Kind != StorageChangeExpired || expired.After != nil {
		t.Errorf("keyExpired: want expired with no after, got kind=%q after=%+v", expired.Kind, expired.After)
	}
	if expired.Before == nil || expired.Before.Durability != "temporary" {
		t.Errorf("keyExpired before: want temporary durability, got %+v", expired.Before)
	}

	want := map[string]int{"created": 1, "updated": 2, "deleted": 1, "expired": 1}
	for kind, n := range want {
		if diff.Counts[kind] != n {
			t.Errorf("counts[%s]: want %d, got %d", kind, n, diff.Counts[kind])
		}
	}
}

func TestBuildStorageDiffOmitsUnchangedKeys(t *testing.T) {
	before := []StorageEntry{entry("a", "1", "persistent", 100, 0), entry("b", "2", "temporary", 100, 9000)}
	after := []StorageEntry{entry("a", "1", "persistent", 100, 0), entry("b", "2", "temporary", 100, 9000)}

	diff := BuildStorageDiff("CDIFF", 100, 200, before, after)
	if len(diff.Changes) != 0 {
		t.Fatalf("want no changes, got %+v", diff.Changes)
	}
	for _, kind := range []StorageChangeKind{StorageChangeCreated, StorageChangeUpdated, StorageChangeDeleted, StorageChangeExpired} {
		if diff.Counts[string(kind)] != 0 {
			t.Errorf("counts[%s]: want 0, got %d", kind, diff.Counts[string(kind)])
		}
	}
}

func TestBuildStorageDiffSortsByKey(t *testing.T) {
	after := []StorageEntry{
		entry("keyZ", "z", "persistent", 100, 0),
		entry("keyA", "a", "persistent", 100, 0),
		entry("keyM", "m", "persistent", 100, 0),
	}
	diff := BuildStorageDiff("CDIFF", 0, 100, nil, after)
	if len(diff.Changes) != 3 {
		t.Fatalf("want 3 changes, got %d", len(diff.Changes))
	}
	if diff.Changes[0].KeyXDR != "keyA" || diff.Changes[1].KeyXDR != "keyM" || diff.Changes[2].KeyXDR != "keyZ" {
		t.Errorf("changes not sorted by key: %q, %q, %q",
			diff.Changes[0].KeyXDR, diff.Changes[1].KeyXDR, diff.Changes[2].KeyXDR)
	}
}
