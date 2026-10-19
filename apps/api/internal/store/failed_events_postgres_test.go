package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestFailedEventStorePostgres covers the dead-letter queue SQL end to end:
// defaults, upsert-by-event-id, newest-first keyset pagination and clean
// not-found/delete semantics.
func TestFailedEventStorePostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE TABLE failed_events"); err != nil {
		t.Fatalf("truncate failed_events: %v", err)
	}
	s := store.NewFullStore(pool)

	payload := json.RawMessage(`{"ID":"evt-1","ContractID":"CDLQ"}`)
	created := time.Date(2026, 4, 2, 9, 30, 0, 0, time.UTC)
	first := store.FailedEvent{
		EventID:      "evt-1",
		ContractID:   "CDLQ",
		Network:      "testnet",
		EventPayload: payload,
		ErrorMessage: "first failure",
		CreatedAt:    created,
	}
	if err := s.InsertFailedEvent(ctx, first); err != nil {
		t.Fatalf("InsertFailedEvent: %v", err)
	}

	rows, next, err := s.ListFailedEvents(ctx, "", 10)
	if err != nil {
		t.Fatalf("ListFailedEvents: %v", err)
	}
	if len(rows) != 1 || next != "" {
		t.Fatalf("after first insert: %d rows, next %q", len(rows), next)
	}
	stored := rows[0]
	if stored.ID == 0 {
		t.Fatal("surrogate id was not assigned")
	}
	if stored.Attempts != 3 {
		t.Errorf("attempts = %d, want the 3-attempt default", stored.Attempts)
	}
	if stored.Network != "testnet" {
		t.Errorf("network = %q, want testnet", stored.Network)
	}
	if !stored.CreatedAt.Equal(created) {
		t.Errorf("created_at = %s, want %s", stored.CreatedAt, created)
	}
	var roundTrip store.Event
	if err := json.Unmarshal(stored.EventPayload, &roundTrip); err != nil {
		t.Fatalf("event_payload is not an Event: %v", err)
	}
	if roundTrip.ID != "evt-1" || roundTrip.ContractID != "CDLQ" {
		t.Errorf("payload round trip = %+v", roundTrip)
	}

	// Re-parking the same event updates the existing row instead of adding
	// another one.
	first.ErrorMessage = "second failure"
	first.Attempts = 5
	first.Network = "mainnet"
	first.EventPayload = json.RawMessage(`{"ID":"evt-1","ContractID":"CDLQ","Network":"mainnet"}`)
	if err := s.InsertFailedEvent(ctx, first); err != nil {
		t.Fatalf("InsertFailedEvent upsert: %v", err)
	}
	rows, _, err = s.ListFailedEvents(ctx, "", 10)
	if err != nil {
		t.Fatalf("ListFailedEvents after upsert: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("upsert created %d rows, want 1", len(rows))
	}
	if rows[0].ID != stored.ID {
		t.Errorf("upsert changed the surrogate id: %d -> %d", stored.ID, rows[0].ID)
	}
	if rows[0].ErrorMessage != "second failure" || rows[0].Attempts != 5 || rows[0].Network != "mainnet" {
		t.Errorf("upserted row = %+v", rows[0])
	}

	// Three more rows; pagination is newest-first by surrogate id.
	for i := 2; i <= 4; i++ {
		fe := store.FailedEvent{
			EventID:      fmt.Sprintf("evt-%d", i),
			ContractID:   "CDLQ",
			Network:      "testnet",
			EventPayload: json.RawMessage(`{}`),
			ErrorMessage: fmt.Sprintf("failure %d", i),
			Attempts:     i,
			CreatedAt:    created.Add(time.Duration(i) * time.Minute),
		}
		if err := s.InsertFailedEvent(ctx, fe); err != nil {
			t.Fatalf("InsertFailedEvent(evt-%d): %v", i, err)
		}
	}

	page1, cursor, err := s.ListFailedEvents(ctx, "", 2)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(page1) != 2 || page1[0].EventID != "evt-4" || page1[1].EventID != "evt-3" || cursor == "" {
		t.Fatalf("page 1 = %v, cursor %q, want [evt-4 evt-3] and a cursor", eventIDs(page1), cursor)
	}
	if cursor != fmt.Sprintf("%d", page1[1].ID) {
		t.Errorf("cursor %q is not the last row id %d", cursor, page1[1].ID)
	}
	page2, cursor2, err := s.ListFailedEvents(ctx, cursor, 2)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2) != 2 || page2[0].EventID != "evt-2" || page2[1].EventID != "evt-1" || cursor2 != "" {
		t.Fatalf("page 2 = %v, cursor %q, want [evt-2 evt-1] and no cursor", eventIDs(page2), cursor2)
	}

	if _, err := s.GetFailedEvent(ctx, 9_999_999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetFailedEvent(unknown): want ErrNotFound, got %v", err)
	}
	one, err := s.GetFailedEvent(ctx, stored.ID)
	if err != nil {
		t.Fatalf("GetFailedEvent: %v", err)
	}
	if one.EventID != "evt-1" {
		t.Errorf("GetFailedEvent = %+v", one)
	}
	if err := s.DeleteFailedEvent(ctx, stored.ID); err != nil {
		t.Fatalf("DeleteFailedEvent: %v", err)
	}
	if _, err := s.GetFailedEvent(ctx, stored.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetFailedEvent after delete: want ErrNotFound, got %v", err)
	}
	if err := s.DeleteFailedEvent(ctx, stored.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("second DeleteFailedEvent: want ErrNotFound, got %v", err)
	}
}

func eventIDs(rows []store.FailedEvent) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.EventID
	}
	return out
}
