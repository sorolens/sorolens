package store

// StorageFilter defines filters for storage queries.
type StorageFilter struct {
	KeyPrefix  string
	Durability string
}
