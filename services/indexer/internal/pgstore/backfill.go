package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// BeginBackfill records the backfill range for a contract and moves it to the
// backfilling status. An existing row wins over p, so a restarted indexer
// continues from the saved position; the row that is in force is returned.
//
// The live cursor is seeded at the head of the range so new events are tailed
// while older ledgers are still being walked.
func (s *Store) BeginBackfill(ctx context.Context, p poller.BackfillProgress) (poller.BackfillProgress, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return poller.BackfillProgress{}, fmt.Errorf("begin backfill: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, `
		INSERT INTO contract_backfills (contract_id, oldest_ledger, newest_ledger, next_ledger)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (contract_id) DO NOTHING`,
		p.ContractID, int64(p.OldestLedger), int64(p.NewestLedger), int64(p.NextLedger),
	); err != nil {
		return poller.BackfillProgress{}, fmt.Errorf("begin backfill: insert: %w", err)
	}

	var oldest, newest, next int64
	if err := tx.QueryRow(ctx, `
		SELECT oldest_ledger, newest_ledger, next_ledger
		FROM contract_backfills WHERE contract_id = $1`, p.ContractID,
	).Scan(&oldest, &newest, &next); err != nil {
		return poller.BackfillProgress{}, fmt.Errorf("begin backfill: read position: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE contracts SET status = 'backfilling'
		WHERE id = $1 AND status IN ('pending', 'active')`, p.ContractID,
	); err != nil {
		return poller.BackfillProgress{}, fmt.Errorf("begin backfill: set status: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sync_state (contract_id, last_ledger, last_run_at, error_message, updated_at)
		VALUES ($1, $2, NOW(), NULL, NOW())
		ON CONFLICT (contract_id) DO UPDATE SET
			last_ledger = EXCLUDED.last_ledger,
			updated_at  = EXCLUDED.updated_at
		WHERE sync_state.last_ledger = 0`, p.ContractID, newest,
	); err != nil {
		return poller.BackfillProgress{}, fmt.Errorf("begin backfill: seed sync state: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return poller.BackfillProgress{}, fmt.Errorf("begin backfill: commit: %w", err)
	}
	return poller.BackfillProgress{
		ContractID:   p.ContractID,
		OldestLedger: uint32(oldest),
		NewestLedger: uint32(newest),
		NextLedger:   uint32(next),
	}, nil
}

// CommitBackfillWindow stores one scanned window and its new position in a
// single transaction, so a crash can never record progress for rows that were
// not written. The final window also activates the contract. A contract paused
// in the meantime keeps its status.
func (s *Store) CommitBackfillWindow(ctx context.Context, w poller.BackfillWindow) ([]poller.FailedEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("commit backfill window: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := time.Now()
	parked, err := insertWindowRows(ctx, tx, w.Events, w.Invocations, now)
	if err != nil {
		return nil, err
	}

	var completedAt *time.Time
	if w.Complete {
		completedAt = &now
	}
	tag, err := tx.Exec(ctx, `
		UPDATE contract_backfills
		SET oldest_ledger = $2, next_ledger = $3, updated_at = $4, completed_at = $5
		WHERE contract_id = $1`,
		w.Progress.ContractID, int64(w.Progress.OldestLedger), int64(w.Progress.NextLedger), now, completedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("commit backfill window: save position: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("commit backfill window: no backfill recorded for contract %s", w.Progress.ContractID)
	}

	if w.Complete {
		if _, err := tx.Exec(ctx, `
			UPDATE contracts SET status = 'active', backfill_complete_at = $2
			WHERE id = $1 AND status = 'backfilling'`, w.Progress.ContractID, now,
		); err != nil {
			return nil, fmt.Errorf("commit backfill window: activate contract: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit backfill window: commit: %w", err)
	}
	return parked, nil
}

// CommitHistoryBatch stores rows imported from a history source. It writes
// through the same idempotent inserts as live indexing and moves no cursor:
// historical rows say nothing about where the live indexer has reached.
func (s *Store) CommitHistoryBatch(ctx context.Context, events []poller.Event, invocations []poller.Invocation) ([]poller.FailedEvent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("commit history batch: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	parked, err := insertWindowRows(ctx, tx, events, invocations, time.Now())
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit history batch: commit: %w", err)
	}
	return parked, nil
}

// ImportTarget describes a contract a history import writes to.
type ImportTarget struct {
	Network string
	// RPCFloorLedger is the oldest ledger the RPC backfill covers, or 0 when
	// the contract has no recorded backfill.
	RPCFloorLedger uint32
}

// ErrContractNotTracked is returned for an import into an unknown contract.
var ErrContractNotTracked = errors.New("contract is not tracked")

// ImportTarget loads what an import needs to know about a tracked contract.
func (s *Store) ImportTarget(ctx context.Context, contractID string) (ImportTarget, error) {
	var network string
	var floor *int64
	err := s.pool.QueryRow(ctx, `
		SELECT c.network, b.oldest_ledger
		FROM contracts c
		LEFT JOIN contract_backfills b ON b.contract_id = c.id
		WHERE c.id = $1`, contractID,
	).Scan(&network, &floor)
	if errors.Is(err, pgx.ErrNoRows) {
		return ImportTarget{}, fmt.Errorf("%w: %s", ErrContractNotTracked, contractID)
	}
	if err != nil {
		return ImportTarget{}, fmt.Errorf("import target: %w", err)
	}
	target := ImportTarget{Network: network}
	if floor != nil {
		target.RPCFloorLedger = uint32(*floor)
	}
	return target, nil
}
