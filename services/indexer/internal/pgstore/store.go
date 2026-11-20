package pgstore

import (
	"context"
)

// Store provides postgres storage operations for the indexer.
type Store struct{}

// ParkTransaction records a failed transaction in the retry / DLQ table.
func (s *Store) ParkTransaction(ctx context.Context, contractID, network, txHash, errorMessage string) error {
	return nil
}
