package store

// PartitionStats holds information about a single partition.
type PartitionStats struct {
	PartitionName string
	Year          int
	Month         int
	TableSize     int64
	RowCount      int64
}
