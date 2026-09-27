package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ContractWasm is a cached Soroban contract Wasm binary, keyed by the
// content-addressed wasm hash (issue #162).
type ContractWasm struct {
	WasmHash  string
	Code      []byte
	SizeBytes int
	FetchedAt time.Time
}

// ContractWasmStore is the read/write surface for cached Wasm binaries.
// Kept as its own interface so the API and indexer adapter can be wired
// independently, mirroring ContractUpgradeStore.
type ContractWasmStore interface {
	// UpsertContractWasm stores Wasm bytes under their content hash.
	// Re-inserting the same hash is a no-op (idempotent).
	UpsertContractWasm(ctx context.Context, wasmHash string, code []byte) error

	// HasContractWasm reports whether bytes for wasmHash are already cached.
	HasContractWasm(ctx context.Context, wasmHash string) (bool, error)

	// GetContractWasmByHash returns the cached binary for a wasm hash, or
	// ErrNotFound when it has not been fetched yet.
	GetContractWasmByHash(ctx context.Context, wasmHash string) (ContractWasm, error)

	// GetContractWasm returns the cached binary for a tracked contract's
	// current wasm_hash, or ErrNotFound when the contract is unknown or the
	// binary has not been fetched yet.
	GetContractWasm(ctx context.Context, contractID string) (ContractWasm, error)

	// UpdateContractWasmHash records the now-current on-chain Wasm hash on
	// the contracts row so subsequent polls can diff against it.
	UpdateContractWasmHash(ctx context.Context, contractID, wasmHash string) error
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) UpsertContractWasm(ctx context.Context, wasmHash string, code []byte) error {
	if wasmHash == "" {
		return fmt.Errorf("upsert contract wasm: empty hash")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_wasm (wasm_hash, code, size_bytes, fetched_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (wasm_hash) DO NOTHING`,
		wasmHash, code, len(code),
	)
	if err != nil {
		return fmt.Errorf("upsert contract wasm: %w", err)
	}
	return nil
}

func (s *postgresStore) HasContractWasm(ctx context.Context, wasmHash string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM contract_wasm WHERE wasm_hash = $1)`, wasmHash,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("has contract wasm: %w", err)
	}
	return exists, nil
}

func (s *postgresStore) GetContractWasmByHash(ctx context.Context, wasmHash string) (ContractWasm, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT wasm_hash, code, size_bytes, fetched_at
		FROM contract_wasm WHERE wasm_hash = $1`, wasmHash)
	var w ContractWasm
	err := row.Scan(&w.WasmHash, &w.Code, &w.SizeBytes, &w.FetchedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ContractWasm{}, ErrNotFound
		}
		return ContractWasm{}, fmt.Errorf("get contract wasm by hash: %w", err)
	}
	return w, nil
}

func (s *postgresStore) GetContractWasm(ctx context.Context, contractID string) (ContractWasm, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT w.wasm_hash, w.code, w.size_bytes, w.fetched_at
		FROM contracts c
		JOIN contract_wasm w ON w.wasm_hash = c.wasm_hash
		WHERE c.id = $1`, contractID)
	var w ContractWasm
	err := row.Scan(&w.WasmHash, &w.Code, &w.SizeBytes, &w.FetchedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ContractWasm{}, ErrNotFound
		}
		return ContractWasm{}, fmt.Errorf("get contract wasm: %w", err)
	}
	return w, nil
}

func (s *postgresStore) UpdateContractWasmHash(ctx context.Context, contractID, wasmHash string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE contracts SET wasm_hash = $2 WHERE id = $1`, contractID, wasmHash)
	if err != nil {
		return fmt.Errorf("update contract wasm hash: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
