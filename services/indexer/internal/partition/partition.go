package partition

import (
	"context"
	"time"
)

// MaintainPartitions pre-creates partitions for the current month and up to
// monthsAhead future months if they do not exist.
func MaintainPartitions(ctx context.Context, store Store, monthsAhead int) error {
	if monthsAhead < 1 {
		monthsAhead = 6
	}
	now := time.Now()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	for i := 0; i <= monthsAhead; i++ {
		t := firstOfMonth.AddDate(0, i, 0)
		if err := store.CreateMonthlyPartitionIfNotExists(ctx, t.Year(), int(t.Month())); err != nil {
			return err
		}
	}
	return nil
}

// EnsureNextMonthPartition ensures future monthly partitions exist.
// This is called on each indexer cycle.
func EnsureNextMonthPartition(ctx context.Context, store Store) error {
	return MaintainPartitions(ctx, store, 6)
}

// Store is the subset of the data store the partition checker needs.
type Store interface {
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error
	CreateNextMonthPartition(ctx context.Context) error
}
