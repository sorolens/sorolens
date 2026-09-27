package coldstorage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// ArchiveStore is the Postgres subset the archiver needs. It is satisfied by
// store.ArchiveStore and by the in-memory MockStore.
type ArchiveStore interface {
	ContractsWithEventsBefore(ctx context.Context, before time.Time) ([]string, error)
	EventsBefore(ctx context.Context, contractID string, before time.Time, limit int) ([]store.Event, error)
	DeleteEventsByID(ctx context.Context, ids []string) (int64, error)
}

// ArchiveStats summarises a single archive run.
type ArchiveStats struct {
	// Contracts is the number of contracts that had at least one cold event.
	Contracts int
	// Events is the number of events exported.
	Events int
	// Deleted is the number of rows removed from Postgres.
	Deleted int64
	// Objects is the number of Parquet objects written.
	Objects int
}

// DefaultBatch is the page size used when reading cold events from Postgres.
const DefaultBatch = 5_000

// Archiver moves events older than a threshold from Postgres to Parquet
// objects in an ObjectStore.
type Archiver struct {
	store     ArchiveStore
	bucket    ObjectStore
	threshold time.Duration
	batch     int
	logger    *slog.Logger
	now       func() time.Time
}

// NewArchiver builds an Archiver. threshold is how old an event must be before
// it is eligible for archiving (issue #146 defaults this to 90 days via
// COLD_STORAGE_THRESHOLD_DAYS).
func NewArchiver(s ArchiveStore, bucket ObjectStore, threshold time.Duration, logger *slog.Logger) *Archiver {
	if threshold <= 0 {
		threshold = 90 * 24 * time.Hour
	}
	return &Archiver{
		store:     s,
		bucket:    bucket,
		threshold: threshold,
		batch:     DefaultBatch,
		logger:    logger,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// Run exports every event older than the threshold and deletes the exported
// rows from Postgres.
//
// The write to object storage always happens before the delete, and a failed
// upload aborts the run for that contract, so a transient S3 error can never
// drop data from the hot store.
func (a *Archiver) Run(ctx context.Context) (ArchiveStats, error) {
	var stats ArchiveStats
	before := a.now().Add(-a.threshold)

	contractIDs, err := a.store.ContractsWithEventsBefore(ctx, before)
	if err != nil {
		return stats, fmt.Errorf("archive: list contracts: %w", err)
	}
	stats.Contracts = len(contractIDs)

	for _, contractID := range contractIDs {
		for {
			if err := ctx.Err(); err != nil {
				return stats, err
			}

			events, err := a.store.EventsBefore(ctx, contractID, before, a.batch)
			if err != nil {
				return stats, fmt.Errorf("archive: read events for %s: %w", contractID, err)
			}
			if len(events) == 0 {
				break
			}

			written, err := a.exportByMonth(ctx, contractID, events)
			if err != nil {
				return stats, err
			}
			stats.Objects += written

			ids := make([]string, len(events))
			for i, e := range events {
				ids[i] = e.ID
			}
			deleted, err := a.store.DeleteEventsByID(ctx, ids)
			if err != nil {
				return stats, fmt.Errorf("archive: delete events for %s: %w", contractID, err)
			}

			stats.Events += len(events)
			stats.Deleted += deleted

			if a.logger != nil {
				a.logger.Info("archived events",
					"contract_id", contractID,
					"events", len(events),
					"deleted", deleted,
				)
			}

			// A short page means this contract has no more cold rows.
			if len(events) < a.batch {
				break
			}
		}
	}

	return stats, nil
}

// exportByMonth groups events by calendar month, merges each group into the
// corresponding Parquet object, and returns the number of objects written.
func (a *Archiver) exportByMonth(ctx context.Context, contractID string, events []store.Event) (int, error) {
	groups := make(map[string][]store.Event)
	for _, e := range events {
		key := monthKey(e.LedgerClosedAt)
		groups[key] = append(groups[key], e)
	}

	written := 0
	for _, group := range groups {
		// The month is the same for every event in the group, so any member
		// identifies the object.
		key := objectKey(contractID, group[0].LedgerClosedAt)

		var merged []store.Event
		existing, err := a.bucket.Get(ctx, key)
		switch {
		case err == nil:
			existingEvents, decErr := DecodeEvents(existing)
			if decErr != nil {
				return written, fmt.Errorf("archive: decode existing %s: %w", key, decErr)
			}
			merged = MergeEvents(existingEvents, group)
		case errors.Is(err, ErrObjectNotFound):
			merged = MergeEvents(nil, group)
		default:
			return written, fmt.Errorf("archive: read existing %s: %w", key, err)
		}

		data, err := EncodeEvents(merged)
		if err != nil {
			return written, err
		}
		if err := a.bucket.Put(ctx, key, data); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}
