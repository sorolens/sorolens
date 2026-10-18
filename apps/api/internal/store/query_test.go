package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestListEventsTopicFilter exercises the ?topic= JSONB containment filter
// against Postgres, covering the GIN index added in migration 000009.
func TestListEventsTopicFilter(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	s := store.NewFullStore(pool)

	const contractID = "C_TEST_TOPIC_FILTER"
	if err := s.UpsertContract(ctx, store.Contract{
		ID:      contractID,
		Network: "testnet",
		Status:  "active",
	}); err != nil {
		t.Fatalf("UpsertContract: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM events WHERE contract_id = $1", contractID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM contracts WHERE id = $1", contractID)
	})

	now := time.Now().UTC()
	insertEvent := func(id string, topics []any, success bool) {
		t.Helper()
		topicJSON, err := json.Marshal(topics)
		if err != nil {
			t.Fatalf("marshal topics for %s: %v", id, err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO events
				(id, contract_id, network, ledger, ledger_closed_at, tx_hash, type,
				 topic_xdr, value_xdr, topic_decoded, value_decoded,
				 in_successful_call, inserted_at)
			VALUES ($1, $2, 'testnet', 1, $3, $4, 'contract',
			        '[]'::jsonb, '', $5::jsonb, '{}'::jsonb, $6, NOW())`,
			id, contractID, now, "tx_"+id, string(topicJSON), success,
		)
		if err != nil {
			t.Fatalf("insert event %s: %v", id, err)
		}
	}

	insertEvent("evt_transfer_1", []any{"transfer", "1"}, true)
	insertEvent("evt_approve_1", []any{"approve"}, true)
	insertEvent("evt_transfer_2", []any{"transfer", "2"}, false)
	insertEvent("evt_numeric_1", []any{float64(7)}, true)

	t.Run("matches string topic element", func(t *testing.T) {
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{Topic: "transfer"})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("topic=transfer: got %d events, want 2", len(got))
		}
	})

	t.Run("quoted JSON string matches the same rows", func(t *testing.T) {
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{Topic: `"transfer"`})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("topic=\"transfer\": got %d events, want 2", len(got))
		}
	})

	t.Run("numeric topic element", func(t *testing.T) {
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{Topic: "7"})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 1 || got[0].ID != "evt_numeric_1" {
			t.Fatalf("topic=7: got %#v, want only evt_numeric_1", got)
		}
	})

	t.Run("no match returns nothing", func(t *testing.T) {
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{Topic: "does-not-exist"})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("topic=does-not-exist: got %d events, want 0", len(got))
		}
	})

	t.Run("empty filter returns all events", func(t *testing.T) {
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 4 {
			t.Fatalf("no topic filter: got %d events, want 4", len(got))
		}
	})

	t.Run("in_successful_call = true", func(t *testing.T) {
		v := true
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{InSuccessfulCall: &v})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("in_successful_call=true: got %d events, want 3", len(got))
		}
	})

	t.Run("in_successful_call = false", func(t *testing.T) {
		v := false
		got, _, err := s.ListEvents(ctx, contractID, "", 50, store.EventFilters{InSuccessfulCall: &v})
		if err != nil {
			t.Fatalf("ListEvents: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("in_successful_call=false: got %d events, want 1", len(got))
		}
		if got[0].ID != "evt_transfer_2" {
			t.Fatalf("want evt_transfer_2, got %s", got[0].ID)
		}
	})
}
