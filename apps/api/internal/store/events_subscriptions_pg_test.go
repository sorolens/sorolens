package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func pgPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestListAllEventsPostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE events"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)
	for _, id := range []string{"CAAA_EXPLORER", "CBBB%EXPLORER"} {
		if err := s.UpsertContract(ctx, store.Contract{ID: id, Network: "testnet", Status: "active"}); err != nil {
			t.Fatal(err)
		}
	}

	base := time.Now().UTC().Truncate(time.Hour).Add(-10 * time.Hour)
	var events []store.Event
	for i := 0; i < 6; i++ {
		cid, typ := "CAAA_EXPLORER", "contract"
		if i%2 == 1 {
			cid, typ = "CBBB%EXPLORER", "system"
		}
		events = append(events, store.Event{
			ID: fmt.Sprintf("%019d-0000000001", 5000+i), ContractID: cid, Network: "testnet",
			Ledger: uint32(5000 + i), LedgerClosedAt: base.Add(time.Duration(i) * time.Hour),
			TxHash: fmt.Sprintf("tx%d", i), Type: typ, TopicXDR: []string{},
		})
	}
	if err := s.BatchInsertEvents(ctx, events); err != nil {
		t.Fatalf("insert: %v", err)
	}

	page1, next, err := s.ListAllEvents(ctx, "", 4, store.GlobalEventFilters{})
	if err != nil || len(page1) != 4 || next == "" || page1[0].Ledger != 5005 {
		t.Fatalf("page 1: %d rows next=%q err=%v", len(page1), next, err)
	}
	page2, next2, err := s.ListAllEvents(ctx, next, 4, store.GlobalEventFilters{})
	if err != nil || len(page2) != 2 || next2 != "" || page2[1].Ledger != 5000 {
		t.Fatalf("page 2: %d rows next=%q err=%v", len(page2), next2, err)
	}

	// LIKE wildcards in the prefix are matched literally: "CAAA_" must not
	// match "CBBB%...", and "CBBB%" matches only the literal percent sign.
	got, _, err := s.ListAllEvents(ctx, "", 50, store.GlobalEventFilters{ContractID: "CBBB%"})
	if err != nil || len(got) != 3 {
		t.Fatalf("literal %% prefix: %d rows err=%v", len(got), err)
	}
	got, _, err = s.ListAllEvents(ctx, "", 50, store.GlobalEventFilters{ContractID: "C___"})
	if err != nil || len(got) != 0 {
		t.Fatalf("underscore must not be a wildcard: %d rows err=%v", len(got), err)
	}

	got, _, err = s.ListAllEvents(ctx, "", 50, store.GlobalEventFilters{
		Type: "system", Since: base.Add(2 * time.Hour), Until: base.Add(5 * time.Hour),
	})
	if err != nil || len(got) != 2 {
		t.Fatalf("type + range: %d rows err=%v", len(got), err)
	}
}

func TestAlertSubscriptionChannelsPostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE alert_subscriptions, monitored_contracts CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	s := store.NewFullStore(pool)
	if err := s.UpsertMonitoredContract(ctx, store.MonitoredContract{
		ContractID: "CSUBS", Network: "testnet", Name: "svc", Owner: "G", Status: "Healthy", RegisteredAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	for _, sub := range []store.AlertSubscription{
		{ID: "w", ContractID: "CSUBS", WebhookURL: "https://x.io", SeverityFilter: "Critical"},
		{ID: "p", ContractID: "CSUBS", SeverityFilter: "Critical", ChannelType: "pagerduty", RoutingKey: "rk"},
	} {
		sub.CreatedAt, sub.UpdatedAt = now, now
		if err := s.Create(ctx, sub); err != nil {
			t.Fatalf("create %s: %v", sub.ID, err)
		}
	}
	subs, err := s.ListByContract(ctx, "CSUBS")
	if err != nil || len(subs) != 2 {
		t.Fatalf("list: %v %v", subs, err)
	}
	byID := map[string]store.AlertSubscription{}
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	if byID["w"].ChannelType != "webhook" {
		t.Errorf("default channel: %q", byID["w"].ChannelType)
	}
	if byID["p"].ChannelType != "pagerduty" || byID["p"].RoutingKey != "rk" {
		t.Errorf("pagerduty row: %+v", byID["p"])
	}

	// The CHECK constraint rejects unknown channels.
	if err := s.Create(ctx, store.AlertSubscription{
		ID: "bad", ContractID: "CSUBS", WebhookURL: "https://x.io", SeverityFilter: "Critical",
		ChannelType: "sms", CreatedAt: now, UpdatedAt: now,
	}); err == nil {
		t.Error("unknown channel_type accepted by the database")
	}

	if err := s.Delete(ctx, "w"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.Delete(ctx, "w"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: want ErrNotFound, got %v", err)
	}
}
