package poller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/partition"
)

const (
	// DefaultBackfillWindow is how many ledgers one backfill step scans when
	// Config.BackfillWindow is unset.
	DefaultBackfillWindow uint32 = 10_000

	// backfillPassBudget caps how long one contract may spend backfilling in a
	// single pass. It stays well inside lockTTL so the advisory lock cannot
	// expire under a long backfill, and the remaining windows resume on the
	// next pass from the saved position.
	backfillPassBudget = lockTTL / 2
)

// BackfillProgress is the saved position of a contract's backfill. The range
// [OldestLedger, NewestLedger] is walked from newest to oldest; NextLedger is
// the newest ledger not scanned yet.
type BackfillProgress struct {
	ContractID   string
	OldestLedger uint32
	NewestLedger uint32
	NextLedger   uint32
}

// Done reports whether every ledger in the range has been scanned.
func (b BackfillProgress) Done() bool { return b.NextLedger < b.OldestLedger }

// BackfillWindow is one scanned window, committed atomically: the rows, the
// new position and, when Complete is set, the switch to the active status.
type BackfillWindow struct {
	Progress    BackfillProgress
	Complete    bool
	Events      []Event
	Invocations []Invocation
}

// RetentionLimited is implemented by RPC errors that report the node no longer
// retains the requested ledgers. The second result is false when the error is
// unrelated to retention.
type RetentionLimited interface {
	OldestRetainedLedger() (uint32, bool)
}

// retentionFloor extracts the oldest ledger a node still retains from err.
func retentionFloor(err error) (uint32, bool) {
	var limited RetentionLimited
	if errors.As(err, &limited) {
		return limited.OldestRetainedLedger()
	}
	return 0, false
}

// needsBackfill reports whether a contract still has to go through the
// pending -> backfilling -> active lifecycle. An active contract without a
// sync state has never been indexed, so it takes the same path.
func needsBackfill(c Contract, s SyncState) bool {
	switch c.Status {
	case "pending", "backfilling":
		return true
	case "active":
		return s.LastLedger == 0
	default:
		return false
	}
}

// beginBackfill moves the contract to backfilling and returns its saved
// position. A contract that already has one keeps it, which is what makes a
// restart resume instead of starting over.
func (p *Poller) beginBackfill(ctx context.Context, contract Contract, latest uint32, s SyncState) (BackfillProgress, error) {
	newest := latest
	if s.LastLedger > 0 && s.LastLedger < latest {
		newest = s.LastLedger
	}
	oldest := uint32(1)
	if newest > newContractBackfillWindow {
		oldest = newest - newContractBackfillWindow
	}
	progress, err := p.store.BeginBackfill(ctx, BackfillProgress{
		ContractID:   contract.ID,
		OldestLedger: oldest,
		NewestLedger: newest,
		NextLedger:   newest,
	})
	if err != nil {
		return BackfillProgress{}, fmt.Errorf("begin backfill: %w", err)
	}
	return progress, nil
}

// backfillWindowSize resolves the configured window, never exceeding the
// per-request LedgerWindow.
func (p *Poller) backfillWindowSize() uint32 {
	size := p.cfg.BackfillWindow
	if size == 0 {
		size = DefaultBackfillWindow
	}
	if p.cfg.LedgerWindow > 0 && p.cfg.LedgerWindow < size {
		size = p.cfg.LedgerWindow
	}
	return size
}

// runBackfill scans bounded windows from newest to oldest until the range is
// finished, stop is cancelled or the pass budget runs out. Each window commits
// its rows and the new position together, so stopping at any point loses
// nothing and repeats nothing.
func (p *Poller) runBackfill(ctx, stop context.Context, rpc RPCClient, contract Contract, progress BackfillProgress) error {
	log := p.log.With("contract_id", contract.ID)
	deadline := time.Now().Add(backfillPassBudget)
	size := p.backfillWindowSize()

	for !progress.Done() {
		if stop.Err() != nil || time.Now().After(deadline) {
			log.Info("backfill paused, resuming on the next pass", "next_ledger", progress.NextLedger)
			return nil
		}

		high := progress.NextLedger
		low := progress.OldestLedger
		if high-low+1 > size {
			low = high - size + 1
		}

		// getEvents treats endLedger as exclusive, so high+1 includes high.
		events, invocations, err := p.fetchWindow(ctx, rpc, contract.ID, contract.Network, low, high+1)
		if err != nil {
			if floor, ok := retentionFloor(err); ok && floor > progress.OldestLedger {
				log.Warn("rpc node retains less history than requested, shortening backfill",
					"requested_oldest_ledger", progress.OldestLedger,
					"retained_oldest_ledger", floor,
				)
				progress.OldestLedger = floor
				continue
			}
			return fmt.Errorf("backfill window [%d,%d]: %w", low, high, err)
		}

		progress.NextLedger = low - 1
		if err := p.commitBackfillWindow(ctx, contract, progress, events, invocations); err != nil {
			return err
		}
		log.Info("backfill window indexed",
			"start_ledger", low,
			"end_ledger", high,
			"events", len(events),
			"invocations", len(invocations),
		)
		if progress.Done() {
			log.Info("backfill complete", "oldest_ledger", progress.OldestLedger, "newest_ledger", progress.NewestLedger)
			return nil
		}
	}

	// The range was already exhausted when this pass started, for example when
	// the retention floor moved past the saved position or a previous process
	// stopped before flipping the status.
	return p.commitBackfillWindow(ctx, contract, progress, nil, nil)
}

// commitBackfillWindow durably records one scanned window.
func (p *Poller) commitBackfillWindow(ctx context.Context, contract Contract, progress BackfillProgress, events []Event, invocations []Invocation) error {
	// Backfilled events can predate the partitions the regular pass creates.
	closedAt := make([]time.Time, 0, len(events))
	for _, e := range events {
		closedAt = append(closedAt, e.LedgerClosedAt)
	}
	if err := partition.EnsureForTimes(ctx, p.store, closedAt); err != nil {
		return fmt.Errorf("ensure backfill partitions: %w", err)
	}

	parked, err := p.store.CommitBackfillWindow(ctx, BackfillWindow{
		Progress:    progress,
		Complete:    progress.Done(),
		Events:      events,
		Invocations: invocations,
	})
	if err != nil {
		return fmt.Errorf("commit backfill window: %w", err)
	}
	for _, fe := range parked {
		p.log.Warn("event parked in DLQ",
			"event_id", fe.EventID,
			"contract_id", fe.ContractID,
			"attempts", fe.Attempts,
			"err", fe.ErrorMessage,
		)
	}
	p.metrics.AddEventsProcessed(contract.Network, len(events))
	return nil
}
