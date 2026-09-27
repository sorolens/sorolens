package coldstorage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func sampleEvent(id, contractID string, ledger uint32, closedAt time.Time) store.Event {
	return store.Event{
		ID:               id,
		ContractID:       contractID,
		Network:          "testnet",
		Ledger:           ledger,
		LedgerClosedAt:   closedAt,
		TxHash:           "tx-" + id,
		Type:             "contract",
		TopicXDR:         []string{"AAAA", "BBBB"},
		ValueXDR:         "CCCC",
		TopicDecoded:     []any{"transfer", "GABC"},
		ValueDecoded:     map[string]any{"amount": "1000"},
		InSuccessfulCall: true,
		InsertedAt:       closedAt.Add(time.Second),
	}
}

// TestParquetRoundTripPreservesEvents is the core guarantee of the tier: what
// goes into object storage comes back byte-for-byte equivalent at the field
// level, including the decoded JSON payloads.
func TestParquetRoundTripPreservesEvents(t *testing.T) {
	closedAt := time.Date(2025, 6, 30, 7, 27, 13, 0, time.UTC)
	in := []store.Event{sampleEvent("e1", "CTEST", 200010, closedAt)}

	data, err := EncodeEvents(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("encode produced an empty payload")
	}

	out, err := DecodeEvents(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("want 1 event, got %d", len(out))
	}

	got := out[0]
	if got.ID != "e1" || got.ContractID != "CTEST" || got.Ledger != 200010 {
		t.Errorf("scalar fields did not survive the round trip: %+v", got)
	}
	if !got.LedgerClosedAt.Equal(closedAt) {
		t.Errorf("want ledger_closed_at %s, got %s", closedAt, got.LedgerClosedAt)
	}
	if !got.InSuccessfulCall {
		t.Error("in_successful_call did not survive the round trip")
	}
	if len(got.TopicXDR) != 2 || got.TopicXDR[0] != "AAAA" {
		t.Errorf("topic_xdr did not survive: %v", got.TopicXDR)
	}
	if len(got.TopicDecoded) != 2 {
		t.Errorf("topic_decoded did not survive: %v", got.TopicDecoded)
	}
	decoded, ok := got.ValueDecoded.(map[string]any)
	if !ok || decoded["amount"] != "1000" {
		t.Errorf("value_decoded did not survive: %#v", got.ValueDecoded)
	}
}

// TestMergeEventsDeduplicatesByID proves the archiver is idempotent: running it
// twice over the same month must not duplicate rows.
func TestMergeEventsDeduplicatesByID(t *testing.T) {
	closedAt := time.Now().UTC()
	existing := []store.Event{sampleEvent("e1", "CTEST", 1, closedAt)}
	// e1 again (an overlapping page) plus a genuinely new event.
	incoming := []store.Event{
		sampleEvent("e1", "CTEST", 1, closedAt),
		sampleEvent("e2", "CTEST", 2, closedAt),
	}

	merged := MergeEvents(existing, incoming)
	if len(merged) != 2 {
		t.Fatalf("want 2 unique events, got %d", len(merged))
	}
	// Deterministic ledger ordering.
	if merged[0].ID != "e1" || merged[1].ID != "e2" {
		t.Errorf("want ledger-ordered output, got %s then %s", merged[0].ID, merged[1].ID)
	}
}

// TestArchiverExportsOldEventsAndDeletesThem covers the archive path end to
// end: old rows land in Parquet and are removed from Postgres, while recent
// rows stay put.
func TestArchiverExportsOldEventsAndDeletesThem(t *testing.T) {
	ms := store.NewMockStore()
	old := time.Now().UTC().Add(-100 * 24 * time.Hour)
	recent := time.Now().UTC().Add(-2 * 24 * time.Hour)

	ms.BatchInsertEvents(context.Background(), []store.Event{
		sampleEvent("old1", "CTEST", 100, old),
		sampleEvent("old2", "CTEST", 101, old.Add(time.Minute)),
		sampleEvent("fresh", "CTEST", 200, recent),
	})

	bucket := NewMemoryStore()
	archiver := NewArchiver(ms, bucket, 90*24*time.Hour, nil)

	stats, err := archiver.Run(context.Background())
	if err != nil {
		t.Fatalf("archive run: %v", err)
	}
	if stats.Events != 2 || stats.Deleted != 2 {
		t.Errorf("want 2 events archived and deleted, got %d / %d", stats.Events, stats.Deleted)
	}
	if stats.Objects != 1 {
		t.Errorf("want 1 object written, got %d", stats.Objects)
	}

	// The recent event must still be in Postgres.
	remaining, err := ms.RecentEventsAll(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].ID != "fresh" {
		t.Fatalf("want only the recent event left in Postgres, got %+v", remaining)
	}

	// Both old events must be readable back from object storage.
	reader := NewReader(bucket)
	archived, err := reader.Events(context.Background(), "CTEST", 0, 0, 0)
	if err != nil {
		t.Fatalf("cold read: %v", err)
	}
	if len(archived) != 2 {
		t.Fatalf("want 2 archived events, got %d", len(archived))
	}
	if archived[0].ID != "old1" || archived[1].ID != "old2" {
		t.Errorf("want ledger-ordered archive, got %s then %s", archived[0].ID, archived[1].ID)
	}

	// Re-running must be a no-op rather than duplicating the archive.
	stats, err = archiver.Run(context.Background())
	if err != nil {
		t.Fatalf("second archive run: %v", err)
	}
	if stats.Events != 0 || stats.Objects != 0 {
		t.Errorf("want a no-op second run, got %+v", stats)
	}
	archived, _ = reader.Events(context.Background(), "CTEST", 0, 0, 0)
	if len(archived) != 2 {
		t.Errorf("want the archive to stay at 2 events, got %d", len(archived))
	}
}

// TestReaderFiltersByLedgerRange covers the query the API makes after asking
// the hot store first: only the requested ledger window comes back.
func TestReaderFiltersByLedgerRange(t *testing.T) {
	bucket := NewMemoryStore()
	closedAt := time.Now().UTC()
	events := []store.Event{
		sampleEvent("a", "CTEST", 10, closedAt),
		sampleEvent("b", "CTEST", 20, closedAt),
		sampleEvent("c", "CTEST", 30, closedAt),
	}
	data, err := EncodeEvents(events)
	if err != nil {
		t.Fatal(err)
	}
	if err := bucket.Put(context.Background(), objectKey("CTEST", closedAt), data); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(bucket)

	got, err := reader.Events(context.Background(), "CTEST", 15, 25, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("want only ledger 20 in [15,25], got %+v", got)
	}

	// A contract with nothing archived yields an empty slice, not an error.
	got, err = reader.Events(context.Background(), "COTHER", 0, 0, 0)
	if err != nil {
		t.Fatalf("want no error for an unarchived contract, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want no events for an unarchived contract, got %d", len(got))
	}

	// limit caps the page.
	got, err = reader.Events(context.Background(), "CTEST", 0, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("want limit to cap the result at 2, got %d", len(got))
	}
}

// TestArchiverMonthsAreSeparateObjects documents the object layout: events from
// different calendar months are written to different Parquet files.
func TestArchiverMonthsAreSeparateObjects(t *testing.T) {
	ms := store.NewMockStore()
	jan := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)
	ms.BatchInsertEvents(context.Background(), []store.Event{
		sampleEvent("jan", "CTEST", 1, jan),
		sampleEvent("feb", "CTEST", 2, feb),
	})

	bucket := NewMemoryStore()
	archiver := NewArchiver(ms, bucket, time.Hour, nil)
	stats, err := archiver.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Objects != 2 {
		t.Fatalf("want 2 monthly objects, got %d", stats.Objects)
	}
	keys, err := bucket.List(context.Background(), "events/CTEST/")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("want 2 keys, got %v", keys)
	}
	want := []string{"events/CTEST/2025-01.parquet", "events/CTEST/2025-02.parquet"}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("want key %s, got %s", k, keys[i])
		}
	}
}

// TestMemoryStoreNotFoundIsTyped keeps the ErrObjectNotFound contract that the
// archiver relies on to distinguish "no object yet" from "storage is broken".
func TestMemoryStoreNotFoundIsTyped(t *testing.T) {
	bucket := NewMemoryStore()
	_, err := bucket.Get(context.Background(), "missing")
	if err == nil {
		t.Fatal("want an error for a missing key")
	}
	if !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("want ErrObjectNotFound, got %v", err)
	}
}
