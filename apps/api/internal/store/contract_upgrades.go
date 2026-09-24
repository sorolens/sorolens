package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ContractUpgradeStore is the read/write surface for contract upgrade history
// (Wasm hash change tracking, issue #129). Kept as its own interface so the
// API can be wired without pulling upgrade handlers into tests that don't need
// them, mirroring WatchdogStore.
type ContractUpgradeStore interface {
	// InsertContractUpgrade records a Wasm-hash change. Duplicate upgrades for
	// the same (contract, tx) are ignored so re-running a poll cycle is
	// idempotent. txHash may be empty when the ledger is the only evidence.
	InsertContractUpgrade(ctx context.Context, u ContractUpgrade) error

	// ListContractUpgrades returns the upgrade history for a contract, newest
	// first, up to limit entries. Returns an empty slice when there is none.
	ListContractUpgrades(ctx context.Context, contractID string, limit int) ([]ContractUpgrade, error)

	// LatestContractUpgrade returns the most recent upgrade for a contract, or
	// ErrNotFound when the contract has never been upgraded.
	LatestContractUpgrade(ctx context.Context, contractID string) (ContractUpgrade, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) InsertContractUpgrade(ctx context.Context, u ContractUpgrade) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contract_upgrades (contract_id, from_hash, to_hash, ledger, tx_hash, at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6)
		ON CONFLICT (contract_id, tx_hash) DO NOTHING`,
		u.ContractID, u.FromHash, u.ToHash, u.Ledger, u.TxHash, u.At,
	)
	return err
}

func (s *postgresStore) ListContractUpgrades(ctx context.Context, contractID string, limit int) ([]ContractUpgrade, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, contract_id, from_hash, to_hash, ledger, COALESCE(tx_hash,''), at
		FROM contract_upgrades
		WHERE contract_id = $1
		ORDER BY ledger DESC, id DESC
		LIMIT $2`, contractID, limit)
	if err != nil {
		return nil, fmt.Errorf("list contract upgrades: %w", err)
	}
	defer rows.Close()

	out := make([]ContractUpgrade, 0)
	for rows.Next() {
		var u ContractUpgrade
		if err := rows.Scan(&u.ID, &u.ContractID, &u.FromHash, &u.ToHash,
			&u.Ledger, &u.TxHash, &u.At); err != nil {
			return nil, fmt.Errorf("list contract upgrades: %w", err)
		}
		out = append(out, u)
	}
	if rows.Err() != nil {
		return nil, fmt.Errorf("list contract upgrades: %w", rows.Err())
	}
	return out, nil
}

func (s *postgresStore) LatestContractUpgrade(ctx context.Context, contractID string) (ContractUpgrade, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, contract_id, from_hash, to_hash, ledger, COALESCE(tx_hash,''), at
		FROM contract_upgrades
		WHERE contract_id = $1
		ORDER BY ledger DESC, id DESC
		LIMIT 1`, contractID)
	var u ContractUpgrade
	err := row.Scan(&u.ID, &u.ContractID, &u.FromHash, &u.ToHash,
		&u.Ledger, &u.TxHash, &u.At)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ContractUpgrade{}, ErrNotFound
		}
		return ContractUpgrade{}, fmt.Errorf("latest contract upgrade: %w", err)
	}
	return u, nil
}