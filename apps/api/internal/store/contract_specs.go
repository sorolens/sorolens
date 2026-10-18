package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ContractSpec is the cached SEP-48 interface description for one contract.
// Spec holds the already-projected JSON tree produced by the indexer's
// contractspec package ({"functions":[...]}) and is passed through to API
// clients verbatim.
type ContractSpec struct {
	ContractID string
	Spec       json.RawMessage
	WasmHash   string
	ParsedAt   time.Time
}

// ContractSpecStore is the read/write surface for cached contract interface
// specs (issue #130). Kept as its own interface so the API can be wired
// without pulling spec handlers into tests that do not need them, mirroring
// ContractUpgradeStore.
type ContractSpecStore interface {
	// UpsertContractSpec stores the parsed spec for a contract, replacing any
	// previous version. Re-parsing after a Wasm upgrade is therefore
	// idempotent.
	UpsertContractSpec(ctx context.Context, spec ContractSpec) error

	// GetContractSpec returns the cached spec for a contract, or ErrNotFound
	// when the indexer has not parsed one yet.
	GetContractSpec(ctx context.Context, contractID string) (ContractSpec, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) UpsertContractSpec(ctx context.Context, spec ContractSpec) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_specs (contract_id, spec, wasm_hash, parsed_at)
		VALUES ($1, $2, NULLIF($3,''), NOW())
		ON CONFLICT (contract_id) DO UPDATE
		SET spec      = EXCLUDED.spec,
		    wasm_hash = EXCLUDED.wasm_hash,
		    parsed_at = NOW()`,
		spec.ContractID, []byte(spec.Spec), spec.WasmHash,
	)
	if err != nil {
		return fmt.Errorf("upsert contract spec: %w", err)
	}
	return nil
}

func (s *postgresStore) GetContractSpec(ctx context.Context, contractID string) (ContractSpec, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT contract_id, spec, COALESCE(wasm_hash,''), parsed_at
		FROM contract_specs
		WHERE contract_id = $1`, contractID)

	var out ContractSpec
	var raw []byte
	if err := row.Scan(&out.ContractID, &raw, &out.WasmHash, &out.ParsedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ContractSpec{}, ErrNotFound
		}
		return ContractSpec{}, fmt.Errorf("get contract spec: %w", err)
	}
	out.Spec = raw
	return out, nil
}
