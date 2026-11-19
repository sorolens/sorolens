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

// EnsureForTimes creates the monthly partition for every month the given
// timestamps fall in. Historical rows can predate the partitions the regular
// pass maintains. Zero times are ignored.
func EnsureForTimes(ctx context.Context, store Store, times []time.Time) error {
	seen := make(map[[2]int]struct{})
	for _, t := range times {
		if t.IsZero() {
			continue
		}
		utc := t.UTC()
		key := [2]int{utc.Year(), int(utc.Month())}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := store.CreateMonthlyPartitionIfNotExists(ctx, key[0], key[1]); err != nil {
			return err
		}
	}
	return nil
}

// Store is the subset of the data store the partition checker needs.
type Store interface {
	CreateMonthlyPartitionIfNotExists(ctx context.Context, year int, month int) error
	CreateNextMonthPartition(ctx context.Context) error
}
