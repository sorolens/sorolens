package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestMockStoreRetentionAndPartitionMaintenance(t *testing.T) {
	ms := store.NewMockStore()
	ctx := context.Background()

	// 1. Maintain partitions
	if err := ms.MaintainPartitions(ctx, 6); err != nil {
		t.Fatalf("MaintainPartitions: %v", err)
	}

	// 2. Insert recent and old events
	now := time.Now().UTC()
	oldTime := now.Add(-100 * 24 * time.Hour) // 100 days old

	recentEvent := store.Event{
		ID:               "ev-recent",
		ContractID:       "C1",
		Network:          "testnet",
		Ledger:           100,
		LedgerClosedAt:   now,
		TxHash:           "tx1",
		Type:             "contract",
		TopicXDR:         []string{"t1"},
		ValueXDR:         "val1",
		TopicDecoded:     []any{"t1"},
		ValueDecoded:     map[string]any{},
		InSuccessfulCall: true,
	}

	oldEvent := store.Event{
		ID:               "ev-old",
		ContractID:       "C1",
		Network:          "testnet",
		Ledger:           50,
		LedgerClosedAt:   oldTime,
		TxHash:           "tx0",
		Type:             "contract",
		TopicXDR:         []string{"t0"},
		ValueXDR:         "val0",
		TopicDecoded:     []any{"t0"},
		ValueDecoded:     map[string]any{},
		InSuccessfulCall: true,
	}

	if err := ms.BatchInsertEvents(ctx, []store.Event{recentEvent, oldEvent}); err != nil {
		t.Fatalf("BatchInsertEvents: %v", err)
	}

	// 3. Run retention with 90 days threshold
	archived, err := ms.ArchiveOldEvents(ctx, 90)
	if err != nil {
		t.Fatalf("ArchiveOldEvents: %v", err)
	}

	if archived != 1 {
		t.Errorf("expected 1 archived event, got %d", archived)
	}

	// 4. Verify only recent event remains in active table
	active, _, err := ms.ListEvents(ctx, "C1", "", 10, store.EventFilters{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(active) != 1 || active[0].ID != "ev-recent" {
		t.Errorf("expected only recent event in active events, got %+v", active)
	}
}
