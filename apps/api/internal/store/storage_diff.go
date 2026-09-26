package store

import (
	"context"
	"fmt"
	"sort"
)

// StorageChangeKind classifies how a storage entry changed between two ledgers.
type StorageChangeKind string

const (
	// StorageChangeCreated means the key has no live entry at the "from" ledger
	// but does at the "to" ledger.
	StorageChangeCreated StorageChangeKind = "created"
	// StorageChangeUpdated means the key is live at both ledgers but its value,
	// TTL, or durability differs.
	StorageChangeUpdated StorageChangeKind = "updated"
	// StorageChangeDeleted means the key was live at "from" and was removed
	// before "to" while its TTL had not yet elapsed.
	StorageChangeDeleted StorageChangeKind = "deleted"
	// StorageChangeExpired means the key was live at "from" and its TTL elapsed
	// at or before "to".
	StorageChangeExpired StorageChangeKind = "expired"
)

// StorageChange is a single per-key difference between two storage snapshots.
// Exactly one of Before/After is nil for created/deleted/expired changes; both
// are set for updates. Durations and values keep their raw XDR so the diff can
// be rendered without another round-trip to the ledger.
type StorageChange struct {
	KeyXDR             string
	KeyDecoded         any
	Kind               StorageChangeKind
	Durability         string
	ChangedFields      []string
	Before             *StorageEntry
	After              *StorageEntry
	LastModifiedLedger int64
}

// StorageDiff is the full set of storage changes between two ledgers for one
// contract, plus per-kind counts for a quick summary.
type StorageDiff struct {
	ContractID string
	From       uint32
	To         uint32
	Changes    []StorageChange
	Counts     map[string]int
}

// GetStorageDiff returns the per-key storage changes between two ledgers by
// diffing the historical snapshot at each ledger. Because the comparison runs
// over full snapshots, temporary, persistent, and instance storage are all
// handled by the same code path.
func (s *postgresStore) GetStorageDiff(ctx context.Context, contractID string, from, to uint32) (StorageDiff, error) {
	before, err := s.GetStorageSnapshot(ctx, contractID, from)
	if err != nil {
		return StorageDiff{}, fmt.Errorf("storage diff from snapshot: %w", err)
	}
	after, err := s.GetStorageSnapshot(ctx, contractID, to)
	if err != nil {
		return StorageDiff{}, fmt.Errorf("storage diff to snapshot: %w", err)
	}
	return BuildStorageDiff(contractID, from, to, before, after), nil
}

// BuildStorageDiff compares two per-key storage snapshots and returns the
// changes between them. It is pure so the Postgres and in-memory backends
// share one definition of "created", "updated", "deleted", and "expired".
//
// before is the snapshot taken at the `from` ledger and after the snapshot at
// the `to` ledger. Keys that are live and identical at both ledgers are
// omitted.
func BuildStorageDiff(contractID string, from, to uint32, before, after []StorageEntry) StorageDiff {
	beforeByKey := make(map[string]StorageEntry, len(before))
	for _, e := range before {
		beforeByKey[e.KeyXDR] = e
	}

	diff := StorageDiff{
		ContractID: contractID,
		From:       from,
		To:         to,
		Changes:    make([]StorageChange, 0),
		Counts:     map[string]int{},
	}

	seenAfter := make(map[string]bool, len(after))
	for _, a := range after {
		seenAfter[a.KeyXDR] = true
		prev, existed := beforeByKey[a.KeyXDR]

		// A version whose status is "deleted" is a tombstone: the indexer saw
		// the key removed. Report it as a deletion regardless of what the
		// "from" snapshot held.
		if a.Status == "deleted" {
			afterEntry := a
			change := StorageChange{
				KeyXDR:             a.KeyXDR,
				KeyDecoded:         a.KeyDecoded,
				Kind:               StorageChangeDeleted,
				Durability:         a.Durability,
				LastModifiedLedger: a.LastModifiedLedger,
				After:              &afterEntry,
			}
			if existed {
				beforeEntry := prev
				change.Before = &beforeEntry
			}
			diff.Changes = append(diff.Changes, change)
			diff.Counts[string(StorageChangeDeleted)]++
			continue
		}

		if !existed {
			afterEntry := a
			diff.Changes = append(diff.Changes, StorageChange{
				KeyXDR:             a.KeyXDR,
				KeyDecoded:         a.KeyDecoded,
				Kind:               StorageChangeCreated,
				Durability:         a.Durability,
				LastModifiedLedger: a.LastModifiedLedger,
				After:              &afterEntry,
			})
			diff.Counts[string(StorageChangeCreated)]++
			continue
		}
		if fields := changedStorageFields(prev, a); len(fields) > 0 {
			beforeEntry, afterEntry := prev, a
			diff.Changes = append(diff.Changes, StorageChange{
				KeyXDR:             a.KeyXDR,
				KeyDecoded:         a.KeyDecoded,
				Kind:               StorageChangeUpdated,
				Durability:         a.Durability,
				ChangedFields:      fields,
				LastModifiedLedger: a.LastModifiedLedger,
				Before:             &beforeEntry,
				After:              &afterEntry,
			})
			diff.Counts[string(StorageChangeUpdated)]++
		}
	}

	for _, b := range before {
		if seenAfter[b.KeyXDR] {
			continue
		}
		kind := StorageChangeDeleted
		if b.LiveUntilLedger > 0 && b.LiveUntilLedger < int64(to) {
			kind = StorageChangeExpired
		}
		beforeEntry := b
		diff.Changes = append(diff.Changes, StorageChange{
			KeyXDR:             b.KeyXDR,
			KeyDecoded:         b.KeyDecoded,
			Kind:               kind,
			Durability:         b.Durability,
			LastModifiedLedger: b.LastModifiedLedger,
			Before:             &beforeEntry,
		})
		diff.Counts[string(kind)]++
	}

	sort.SliceStable(diff.Changes, func(i, j int) bool {
		return diff.Changes[i].KeyXDR < diff.Changes[j].KeyXDR
	})
	return diff
}

// changedStorageFields reports which aspects of a key differ between two live
// versions. It returns nil when nothing changed. Value comes first, then TTL
// and durability, so the UI can explain why a row is in the diff.
func changedStorageFields(before, after StorageEntry) []string {
	var fields []string
	if before.ValueXDR != after.ValueXDR {
		fields = append(fields, "value")
	}
	if before.LiveUntilLedger != after.LiveUntilLedger {
		fields = append(fields, "ttl")
	}
	if before.Durability != after.Durability {
		fields = append(fields, "durability")
	}
	if before.Status != after.Status {
		fields = append(fields, "status")
	}
	return fields
}
