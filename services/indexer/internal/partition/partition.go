// Package partition keeps the monthly events partitions provisioned and
// bounded: it pre-creates the months ahead the indexer will write to, and
// (when retention is configured) detaches months that have aged out into the
// events_archive schema instead of letting the live table grow unbounded.
package partition

import (
	"context"
	"fmt"
	"time"
)

// Store is the subset of the data store the partition maintenance needs.
type Store interface {
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error
	// ArchiveOldPartitions detaches monthly partitions entirely older than
	// retainMonths and moves them to events_archive, returning the names
	// moved. Backed by the archive_old_partitions() SQL function (000038).
	ArchiveOldPartitions(ctx context.Context, retainMonths int) ([]string, error)
}

// EnsureCoverage creates the partitions for the current month plus the next
// monthsAhead months. The old job provisioned exactly one month ahead, so an
// indexer stopped across a month boundary came back to a gap (the events
// default partition catches the writes, but they would pile up there until
// someone created the real partition). Re-running is cheap: the backing SQL
// is create-if-not-exists.
//
// now is passed in rather than read from the clock so the month arithmetic is
// testable; callers pass time.Now().
func EnsureCoverage(ctx context.Context, store Store, now time.Time, monthsAhead int) error {
	if monthsAhead < 1 {
		monthsAhead = 1
	}
	for i := 0; i <= monthsAhead; i++ {
		// Anchor on the first of the month: now.AddDate(0, i, 0) on a day
		// past the target month's length normalizes into the *next* month
		// (Jan 31 +1 month = Mar 3), silently skipping February.
		t := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		if err := store.CreateMonthlyPartitionIfNotExists(ctx, t.Year(), int(t.Month())); err != nil {
			return fmt.Errorf("ensure partition %04d-%02d: %w", t.Year(), int(t.Month()), err)
		}
	}
	return nil
}

// Retain detaches and archives partitions older than the retention window.
// A non-positive retainMonths disables the job (the default), so archiving
// is strictly opt-in.
func Retain(ctx context.Context, store Store, retainMonths int) ([]string, error) {
	if retainMonths <= 0 {
		return nil, nil
	}
	return store.ArchiveOldPartitions(ctx, retainMonths)
}
