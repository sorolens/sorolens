package coldstorage

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// Integration test for the real S3 client (issue #146).
//
// It is skipped unless INTEGRATION=true, matching the convention in
// CONTRIBUTING.md: `go test ./...` stays hermetic, and the S3 path is exercised
// against a real S3-compatible server when one is available.
//
// Using the MinIO service from docker-compose.yml:
//
//	docker compose up -d minio
//	docker compose exec minio mc mb --ignore-existing local/sorolens-archive
//	INTEGRATION=true \
//	  COLD_STORAGE_ENDPOINT=http://localhost:9000 \
//	  COLD_STORAGE_BUCKET=sorolens-archive \
//	  COLD_STORAGE_ACCESS_KEY_ID=sorolens \
//	  COLD_STORAGE_SECRET_ACCESS_KEY=sorolens123 \
//	  go test ./internal/coldstorage/ -run TestS3 -v
func integrationS3Config(t *testing.T) S3Config {
	t.Helper()
	if os.Getenv("INTEGRATION") != "true" {
		t.Skip("integration test: set INTEGRATION=true and start an S3-compatible service (see docker-compose.yml)")
	}
	cfg := S3Config{
		Bucket:          envOr("COLD_STORAGE_BUCKET", "sorolens-archive"),
		Region:          envOr("COLD_STORAGE_REGION", "us-east-1"),
		Endpoint:        envOr("COLD_STORAGE_ENDPOINT", "http://localhost:9000"),
		AccessKeyID:     envOr("COLD_STORAGE_ACCESS_KEY_ID", "sorolens"),
		SecretAccessKey: envOr("COLD_STORAGE_SECRET_ACCESS_KEY", "sorolens123"),
	}
	return cfg
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestS3StoreRoundTrip puts a real Parquet payload through a real S3 endpoint
// and reads it back, covering Put/Get/List and the not-found mapping.
func TestS3StoreRoundTrip(t *testing.T) {
	cfg := integrationS3Config(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s3, err := NewS3Store(ctx, cfg)
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}

	closedAt := time.Date(2025, 6, 30, 7, 27, 13, 0, time.UTC)
	events := []store.Event{sampleEvent("s3-evt-1", "CS3TEST", 200010, closedAt)}
	data, err := EncodeEvents(events)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	key := objectKey("CS3TEST", closedAt)
	if err := s3.Put(ctx, key, data); err != nil {
		t.Fatalf("put: %v", err)
	}
	t.Cleanup(func() { cleanupObject(context.Background(), t, s3, key) })

	got, err := s3.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	decoded, err := DecodeEvents(got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded) != 1 || decoded[0].ID != "s3-evt-1" {
		t.Fatalf("round trip lost data: %+v", decoded)
	}

	keys, err := s3.List(ctx, eventsPrefix("CS3TEST"))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, k := range keys {
		if k == key {
			found = true
		}
	}
	if !found {
		t.Errorf("listed keys %v do not include %s", keys, key)
	}

	// A missing key must map onto ErrObjectNotFound so the archiver can treat
	// it as "nothing archived yet" rather than a failure.
	if _, err := s3.Get(ctx, eventsPrefix("CS3TEST")+"1999-01.parquet"); !errors.Is(err, ErrObjectNotFound) {
		t.Errorf("want ErrObjectNotFound for a missing key, got %v", err)
	}
}

// TestS3BackedArchiveEndToEnd runs the archiver and the reader against MinIO,
// which is the acceptance scenario for issue #146: after archiving, a query for
// an old ledger range still returns data.
func TestS3BackedArchiveEndToEnd(t *testing.T) {
	cfg := integrationS3Config(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	s3, err := NewS3Store(ctx, cfg)
	if err != nil {
		t.Fatalf("new s3 store: %v", err)
	}

	// Unique contract id per run so parallel or repeated runs cannot collide.
	contractID := "C" + time.Now().UTC().Format("150405.000000")
	old := time.Now().UTC().Add(-400 * 24 * time.Hour)

	ms := store.NewMockStore()
	ms.BatchInsertEvents(ctx, []store.Event{
		sampleEvent("int-old-1", contractID, 1000, old),
		sampleEvent("int-old-2", contractID, 1001, old.Add(time.Minute)),
	})

	archiver := NewArchiver(ms, s3, 90*24*time.Hour, nil)
	stats, err := archiver.Run(ctx)
	if err != nil {
		t.Fatalf("archive run: %v", err)
	}
	if stats.Events != 2 {
		t.Fatalf("want 2 events archived, got %d", stats.Events)
	}
	t.Cleanup(func() {
		for _, k := range objectsUnder(context.Background(), s3, eventsPrefix(contractID)) {
			cleanupObject(context.Background(), t, s3, k)
		}
	})

	// The rows are gone from Postgres...
	remaining, err := ms.RecentEventsAll(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range remaining {
		if e.ContractID == contractID {
			t.Fatalf("event %s was not deleted from the hot store", e.ID)
		}
	}

	// ...and a query for the old ledger range still returns them.
	archived, err := NewReader(s3).Events(ctx, contractID, 1000, 1001, 0)
	if err != nil {
		t.Fatalf("cold read: %v", err)
	}
	if len(archived) != 2 {
		t.Fatalf("want 2 archived events after archive, got %d", len(archived))
	}
	for i, id := range []string{"int-old-1", "int-old-2"} {
		if archived[i].ID != id {
			t.Errorf("want %s at position %d, got %s", id, i, archived[i].ID)
		}
	}
}

func objectsUnder(ctx context.Context, s3 *S3Store, prefix string) []string {
	keys, err := s3.List(ctx, prefix)
	if err != nil {
		return nil
	}
	return keys
}

// cleanupObject removes a test object; failures are reported but not fatal so a
// cleanup error cannot mask the test result.
func cleanupObject(ctx context.Context, t *testing.T, s3 *S3Store, key string) {
	t.Helper()
	if err := s3.Delete(ctx, key); err != nil {
		t.Logf("cleanup %s: %v", key, err)
	}
}
