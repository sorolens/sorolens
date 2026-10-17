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

func TestResourceTrendBucketsByUTCDate(t *testing.T) {
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
	const contractID = "C_TEST_RESOURCE_TREND"
	if err := s.UpsertContract(ctx, store.Contract{
		ID:      contractID,
		Network: "testnet",
		Status:  "active",
	}); err != nil {
		t.Fatalf("UpsertContract: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM invocations WHERE contract_id = $1", contractID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM contracts WHERE id = $1", contractID)
	})

	today := time.Now().UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	twoDaysAgo := today.AddDate(0, 0, -2)
	yesterday := today.AddDate(0, 0, -1)
	outsideWindow := today.AddDate(0, 0, -7)
	if err := s.BatchInsertInvocations(ctx, []store.Invocation{
		{TxHash: "trend-two-days-1", ContractID: contractID, Ledger: 1, LedgerClosedAt: twoDaysAgo, Status: "SUCCESS", CPUInsn: 1000, MemByte: 500, ResourceFeeCharged: 100},
		{TxHash: "trend-two-days-2", ContractID: contractID, Ledger: 2, LedgerClosedAt: twoDaysAgo, Status: "SUCCESS", CPUInsn: 3000, MemByte: 700, ResourceFeeCharged: 200},
		{TxHash: "trend-yesterday", ContractID: contractID, Ledger: 3, LedgerClosedAt: yesterday, Status: "SUCCESS", CPUInsn: 500, MemByte: 100, ResourceFeeCharged: 50},
		{TxHash: "trend-today", ContractID: contractID, Ledger: 4, LedgerClosedAt: today, Status: "SUCCESS", CPUInsn: 400, MemByte: 400, ResourceFeeCharged: 400},
		{TxHash: "trend-outside-window", ContractID: contractID, Ledger: 5, LedgerClosedAt: outsideWindow, Status: "SUCCESS", CPUInsn: 9000, MemByte: 9000, ResourceFeeCharged: 9000},
	}); err != nil {
		t.Fatalf("BatchInsertInvocations: %v", err)
	}

	got, err := s.ResourceTrend(ctx, contractID, 7)
	if err != nil {
		t.Fatalf("ResourceTrend: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("got %d buckets, want 7", len(got))
	}
	if !got[0].Day.Equal(today.AddDate(0, 0, -6)) || !got[6].Day.Equal(today) {
		t.Fatalf("unexpected UTC date range: %s .. %s", got[0].Day, got[6].Day)
	}
	if got[0].Count != 0 {
		t.Fatalf("oldest bucket count = %d, want 0", got[0].Count)
	}
	if got[4].AvgCPUInsn != 2000 || got[4].AvgMemByte != 600 || got[4].AvgFee != 150 || got[4].Count != 2 {
		t.Fatalf("two-days-ago bucket = %+v, want average cpu=2000 mem=600 fee=150 count=2", got[4])
	}
	if got[5].AvgCPUInsn != 500 || got[5].AvgMemByte != 100 || got[5].AvgFee != 50 || got[5].Count != 1 {
		t.Fatalf("yesterday bucket = %+v, want cpu=500 mem=100 fee=50 count=1", got[5])
	}
	if got[6].AvgCPUInsn != 400 || got[6].AvgMemByte != 400 || got[6].AvgFee != 400 || got[6].Count != 1 {
		t.Fatalf("today bucket = %+v, want cpu=400 mem=400 fee=400 count=1", got[6])
	}
}
