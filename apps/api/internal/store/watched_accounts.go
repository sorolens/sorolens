package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---- models ----------------------------------------------------------------

// WatchedAccount is a Stellar account whose contract deployments are tracked
// automatically (issue #123).
type WatchedAccount struct {
	AccountID       string
	AddedBy         string
	DiscoveredCount int64
	CreatedAt       time.Time
}

// DiscoveredContract is a contract the indexer found being deployed by a
// watched account.
type DiscoveredContract struct {
	ContractID string
	Network    string
	AccountID  string
	Ledger     int64
}

// DiscoveredByLabel is the label given to an auto-tracked contract so users
// can see which watched account deployed it.
func DiscoveredByLabel(accountID string) string {
	return "discovered_by:" + accountID
}

// ---- interface -------------------------------------------------------------

// WatchedAccountStore is the read/write surface for contract discovery.
type WatchedAccountStore interface {
	// AddWatchedAccount registers an account. It is idempotent: when the
	// account is already watched the existing row is returned with
	// created=false.
	AddWatchedAccount(ctx context.Context, a WatchedAccount) (WatchedAccount, bool, error)
	// ListWatchedAccounts returns every watched account, oldest first.
	ListWatchedAccounts(ctx context.Context) ([]WatchedAccount, error)
	// DeleteWatchedAccount stops watching an account, or returns ErrNotFound.
	// Contracts it already discovered stay tracked.
	DeleteWatchedAccount(ctx context.Context, accountID string) error
	// RecordDiscoveredContract tracks a contract deployed by a watched
	// account as an active contract labelled DiscoveredByLabel(account) and
	// bumps the account's discovered_count. A contract that is already
	// tracked is left untouched (so a manual label is never overwritten) and
	// tracked=false is returned. A new contract's sync state is seeded just
	// before its deploy ledger so indexing starts from its first event.
	RecordDiscoveredContract(ctx context.Context, d DiscoveredContract) (bool, error)
}

// ---- postgres implementation ----------------------------------------------

func (s *postgresStore) AddWatchedAccount(ctx context.Context, a WatchedAccount) (WatchedAccount, bool, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO watched_accounts (account_id, added_by, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (account_id) DO NOTHING
		RETURNING account_id, added_by, discovered_count, created_at`,
		a.AccountID, a.AddedBy)
	var out WatchedAccount
	err := row.Scan(&out.AccountID, &out.AddedBy, &out.DiscoveredCount, &out.CreatedAt)
	if err == nil {
		return out, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return WatchedAccount{}, false, fmt.Errorf("add watched account: %w", err)
	}
	// Conflict: already watched, return the existing row.
	row = s.pool.QueryRow(ctx, `
		SELECT account_id, added_by, discovered_count, created_at
		FROM watched_accounts WHERE account_id = $1`, a.AccountID)
	if err := row.Scan(&out.AccountID, &out.AddedBy, &out.DiscoveredCount, &out.CreatedAt); err != nil {
		return WatchedAccount{}, false, fmt.Errorf("get watched account: %w", err)
	}
	return out, false, nil
}

func (s *postgresStore) ListWatchedAccounts(ctx context.Context) ([]WatchedAccount, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT account_id, added_by, discovered_count, created_at
		FROM watched_accounts
		ORDER BY created_at ASC, account_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list watched accounts: %w", err)
	}
	defer rows.Close()

	var out []WatchedAccount
	for rows.Next() {
		var a WatchedAccount
		if err := rows.Scan(&a.AccountID, &a.AddedBy, &a.DiscoveredCount, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *postgresStore) DeleteWatchedAccount(ctx context.Context, accountID string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM watched_accounts WHERE account_id = $1`, accountID)
	if err != nil {
		return fmt.Errorf("delete watched account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *postgresStore) RecordDiscoveredContract(ctx context.Context, d DiscoveredContract) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `
		INSERT INTO contracts
			(id, network, label, wasm_hash, created_at_ledger, status, added_at)
		VALUES ($1, $2, $3, '', $4, 'active', NOW())
		ON CONFLICT (id) DO NOTHING`,
		d.ContractID, networkOrDefault(d.Network), DiscoveredByLabel(d.AccountID), d.Ledger)
	if err != nil {
		return false, fmt.Errorf("insert discovered contract: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	// Start indexing at the deploy ledger rather than the network cursor so
	// the contract's earliest events (e.g. its constructor) are not skipped.
	if d.Ledger > 1 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO sync_state (contract_id, last_ledger, error_message, updated_at)
			VALUES ($1, $2, '', NOW())
			ON CONFLICT (contract_id) DO NOTHING`,
			d.ContractID, d.Ledger-1); err != nil {
			return false, fmt.Errorf("seed discovered contract sync state: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE watched_accounts SET discovered_count = discovered_count + 1
		WHERE account_id = $1`, d.AccountID); err != nil {
		return false, fmt.Errorf("bump discovered count: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}
