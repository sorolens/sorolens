package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestAlertGroupStorePostgres exercises the deduplicating upsert, the
// contract/severity/network filters and the keyset pagination of the grouped
// alert feed.
func TestAlertGroupStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE monitored_contracts CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)

	registered := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for id, network := range map[string]string{"CAG-A": "testnet", "CAG-B": "testnet", "CAG-C": "mainnet"} {
		if err := s.UpsertMonitoredContract(ctx, store.MonitoredContract{
			ContractID: id, Network: network, Name: id, Owner: "GOWNER",
			Status: "Healthy", RegisteredAt: registered,
		}); err != nil {
			t.Fatalf("UpsertMonitoredContract(%s): %v", id, err)
		}
	}

	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	seed := []store.AlertGroup{
		{GroupKey: "CAG-A|Critical|latency", ContractID: "CAG-A", Severity: "Critical", Rule: "latency",
			Count: 1, DedupeWindowSecs: 300, FirstSeen: base, LastSeen: base, LastMessage: "first"},
		{GroupKey: "CAG-A|Warning|errors", ContractID: "CAG-A", Severity: "Warning", Rule: "errors",
			Count: 1, DedupeWindowSecs: 300, FirstSeen: base, LastSeen: base.Add(2 * time.Hour), LastMessage: "warn"},
		{GroupKey: "CAG-B|Info|lag", ContractID: "CAG-B", Severity: "Info", Rule: "lag",
			Count: 1, DedupeWindowSecs: 60, FirstSeen: base, LastSeen: base.Add(3 * time.Hour), LastMessage: "info"},
		{GroupKey: "CAG-C|Critical|latency", ContractID: "CAG-C", Severity: "Critical", Rule: "latency",
			Count: 1, DedupeWindowSecs: 300, FirstSeen: base, LastSeen: base.Add(time.Hour), LastMessage: "mainnet"},
	}
	for _, g := range seed {
		created, err := s.UpsertAlertGroup(ctx, g)
		if err != nil {
			t.Fatalf("UpsertAlertGroup(%s): %v", g.GroupKey, err)
		}
		if created.ID == 0 || created.Count != 1 || created.LastMessage != g.LastMessage {
			t.Errorf("UpsertAlertGroup(%s) = %+v", g.GroupKey, created)
		}
	}

	// Merging into an existing group accumulates count and refreshes
	// last_seen/last_message, but never moves first_seen.
	mergedAt := base.Add(5 * time.Hour)
	merged, err := s.UpsertAlertGroup(ctx, store.AlertGroup{
		GroupKey: "CAG-A|Critical|latency", ContractID: "CAG-A", Severity: "Critical", Rule: "latency",
		Count: 2, DedupeWindowSecs: 300, FirstSeen: base.Add(4 * time.Hour), LastSeen: mergedAt, LastMessage: "third",
	})
	if err != nil {
		t.Fatalf("UpsertAlertGroup merge: %v", err)
	}
	if merged.Count != 3 || merged.LastMessage != "third" || !merged.LastSeen.Equal(mergedAt) {
		t.Errorf("merged group = %+v, want count 3 and the newest message", merged)
	}
	if !merged.FirstSeen.Equal(base) {
		t.Errorf("first_seen moved to %s, want %s", merged.FirstSeen, base)
	}

	page1, cursor, err := s.ListAlertGroups(ctx, "", 2, store.AlertGroupFilters{})
	if err != nil {
		t.Fatalf("ListAlertGroups page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].GroupKey != "CAG-A|Critical|latency" ||
		page1[1].GroupKey != "CAG-B|Info|lag" || cursor == "" {
		t.Fatalf("page 1 = %v, cursor %q", groupKeys(page1), cursor)
	}
	page2, cursor2, err := s.ListAlertGroups(ctx, cursor, 2, store.AlertGroupFilters{})
	if err != nil {
		t.Fatalf("ListAlertGroups page 2: %v", err)
	}
	if len(page2) != 2 || page2[0].GroupKey != "CAG-A|Warning|errors" ||
		page2[1].GroupKey != "CAG-C|Critical|latency" || cursor2 != "" {
		t.Fatalf("page 2 = %v, cursor %q", groupKeys(page2), cursor2)
	}
	if _, _, err := s.ListAlertGroups(ctx, "not-a-cursor", 2, store.AlertGroupFilters{}); !errors.Is(err, store.ErrInvalidCursor) {
		t.Errorf("ListAlertGroups(invalid cursor): want ErrInvalidCursor, got %v", err)
	}

	byContract, _, err := s.ListAlertGroups(ctx, "", 50, store.AlertGroupFilters{ContractID: "CAG-A"})
	if err != nil {
		t.Fatalf("ListAlertGroups contract: %v", err)
	}
	if len(byContract) != 2 || byContract[0].ContractID != "CAG-A" || byContract[1].ContractID != "CAG-A" {
		t.Errorf("contract filter = %v", groupKeys(byContract))
	}
	critical, _, err := s.ListAlertGroups(ctx, "", 50, store.AlertGroupFilters{Severity: "Critical"})
	if err != nil {
		t.Fatalf("ListAlertGroups severity: %v", err)
	}
	if len(critical) != 2 || critical[0].Severity != "Critical" || critical[1].Severity != "Critical" {
		t.Errorf("severity filter = %v", groupKeys(critical))
	}
	mainnet, _, err := s.ListAlertGroups(ctx, "", 50, store.AlertGroupFilters{Network: "mainnet"})
	if err != nil {
		t.Fatalf("ListAlertGroups network: %v", err)
	}
	if len(mainnet) != 1 || mainnet[0].ContractID != "CAG-C" {
		t.Errorf("network filter = %v", groupKeys(mainnet))
	}
	combined, _, err := s.ListAlertGroups(ctx, "", 50, store.AlertGroupFilters{
		Network: "testnet", Severity: "Warning",
	})
	if err != nil {
		t.Fatalf("ListAlertGroups combined: %v", err)
	}
	if len(combined) != 1 || combined[0].GroupKey != "CAG-A|Warning|errors" {
		t.Errorf("combined filter = %v", groupKeys(combined))
	}
}

func groupKeys(groups []store.AlertGroup) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.GroupKey
	}
	return out
}
