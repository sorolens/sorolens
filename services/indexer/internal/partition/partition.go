package partition

import (
	"context"
	"time"
)

// EnsureNextMonthPartition checks if the next month's partition exists
// and creates it if needed. This should be called on each indexer cycle.
func EnsureNextMonthPartition(ctx context.Context, store Store) error {
	now := time.Now()
	year := int(now.AddDate(0, 1, 0).Year())
	month := int(now.AddDate(0, 1, 0).Month())
	return store.CreateMonthlyPartitionIfNotExists(ctx, year, month)
}

// Store is the subset of the data store the partition checker needs.
type Store interface {
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error
	CreateNextMonthPartition(ctx context.Context) error
}
