// Package poller implements the Sorolens indexer worker.
// It reads all tracked contracts from the store, fetches new events and
// invocations from the Soroban RPC, and persists them to Postgres.
//
// The poller never imports apps/api directly. It depends only on the
// RPCClient, Store, and RedisClient interfaces defined in interfaces.go so
// that tests can substitute fakes without touching the network or database.
package poller

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	// lockTTL is the Redis advisory lock lifetime per contract.
	// Set to twice the expected maximum per-contract processing time.
	lockTTL = 60 * time.Second

	// lockKeyPrefix is the Redis key prefix for per-contract indexer locks.
	lockKeyPrefix = "sorolens:lock:indexer:"

	// newContractBackfillWindow is how many ledgers back to start a backfill
	// for a contract with no prior sync state. At ~5s per ledger this is
	// approximately 6 days, safely within the 7-day RPC retention window.
	newContractBackfillWindow uint32 = 100_000
)

// Config holds runtime parameters for the Poller.
type Config struct {
	// LedgerWindow is the maximum number of ledgers to request per getEvents
	// call. Matches INDEXER_LEDGER_WINDOW from the API config.
	LedgerWindow uint32
	// PollInterval is the sleep duration between full passes in continuous mode.
	PollInterval time.Duration
	// MaxDuration is the wall-clock budget for a single once-mode pass.
	// If a pass exceeds this, the poller logs a warning and exits cleanly.
	MaxDuration time.Duration
}

// Poller fetches and persists events and invocations for all tracked contracts.
type Poller struct {
	rpcClients map[string]RPCClient
	store      Store
	redis      RedisClient
	cfg        Config
	log        *slog.Logger
}

// New returns a Poller wired with the given dependencies.
// It preserves the existing single-client behavior by using the provided RPC
// client for all contracts when no network-specific map is needed.
func New(rpc RPCClient, store Store, redis RedisClient, cfg Config, log *slog.Logger) *Poller {
	return NewWithRPCClients(map[string]RPCClient{"": rpc}, store, redis, cfg, log)
}

// NewWithRPCClients returns a Poller that routes each contract to the RPC
// client matching its network. Contracts with an unconfigured network are
// skipped with a warning.
func NewWithRPCClients(rpcClients map[string]RPCClient, store Store, redis RedisClient, cfg Config, log *slog.Logger) *Poller {
	return &Poller{rpcClients: rpcClients, store: store, redis: redis, cfg: cfg, log: log}
}

// Run starts the poller in the given mode.
// mode must be "once" or "continuous".
// The context controls graceful shutdown: when ctx is cancelled the poller
// finishes the current contract then returns.
func (p *Poller) Run(ctx context.Context, mode string) error {
	switch mode {
	case "once":
		return p.runOnce(ctx)
	case "continuous":
		return p.runContinuous(ctx)
	default:
		return fmt.Errorf("poller: unknown mode %q (want once|continuous)", mode)
	}
}

// runOnce executes one full pass and exits.
// If the pass takes longer than cfg.MaxDuration, it logs a warning and
// returns nil (clean exit for GitHub Actions cron runners).
func (p *Poller) runOnce(ctx context.Context) error {
	start := time.Now()
	if p.cfg.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.cfg.MaxDuration)
		defer cancel()
	}

	err := p.processAll(ctx)
	elapsed := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		p.log.Warn("indexer run exceeded max-duration, exiting cleanly",
			"elapsed", elapsed,
			"max_duration", p.cfg.MaxDuration,
		)
		return nil
	}
	return err
}

// runContinuous loops until ctx is cancelled, sleeping PollInterval between passes.
func (p *Poller) runContinuous(ctx context.Context) error {
	for {
		if err := p.processAll(ctx); err != nil {
			p.log.Error("indexer pass error", "err", err)
		}
		select {
		case <-ctx.Done():
			p.log.Info("indexer shutting down")
			return nil
		case <-time.After(p.cfg.PollInterval):
		}
	}
}

// processAll fetches and indexes events for every active contract.
func (p *Poller) processAll(ctx context.Context) error {
	var cursor string
	for {
		// Check for shutdown between contract batches.
		if ctx.Err() != nil {
			return nil
		}

		contracts, next, err := p.store.ListContracts(ctx, cursor, 50)
		if err != nil {
			return fmt.Errorf("list contracts: %w", err)
		}

		for _, c := range contracts {
			if ctx.Err() != nil {
				return nil
			}
			if c.Status != "active" && c.Status != "backfilling" {
				continue
			}
			if err := p.processContract(ctx, c.ID, c.Network); err != nil {
				// Log and continue; one failing contract must not block others.
				p.log.Error("failed to index contract",
					"contract_id", c.ID,
					"err", err,
				)
			}
		}

		if next == "" {
			break
		}
		cursor = next
	}
	return nil
}

// processContract indexes all new events for one contract.
func (p *Poller) processContract(ctx context.Context, contractID, network string) error {
	rpc, ok := p.selectRPCClient(network)
	if !ok {
		p.log.Warn("skipping contract with unconfigured network",
			"contract_id", contractID,
			"network", network,
		)
		return nil
	}

	// Acquire per-contract advisory lock to prevent concurrent runs.
	lockKey := lockKeyPrefix + contractID
	acquired, err := p.redis.SetNX(ctx, lockKey, "1", lockTTL)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	if !acquired {
		p.log.Info("contract locked by another runner, skipping",
			"contract_id", contractID)
		return nil
	}
	defer p.redis.Del(ctx, lockKey) //nolint:errcheck

	latest, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return fmt.Errorf("get latest ledger: %w", err)
	}

	syncState, err := p.store.GetSyncState(ctx, contractID)
	if err != nil {
		return fmt.Errorf("get sync state: %w", err)
	}

	var startLedger uint32
	if syncState.LastLedger == 0 {
		// New contract: best-effort backfill from within the retention window.
		if latest.Sequence > newContractBackfillWindow {
			startLedger = latest.Sequence - newContractBackfillWindow
		} else {
			startLedger = 1
		}
		p.log.Warn("new contract, starting partial backfill; events before this ledger are unavailable",
			"contract_id", contractID,
			"start_ledger", startLedger,
			"retention_window_ledgers", newContractBackfillWindow,
		)
	} else {
		startLedger = syncState.LastLedger + 1
	}

	if startLedger > latest.Sequence {
		p.log.Info("contract is up to date",
			"contract_id", contractID,
			"last_ledger", syncState.LastLedger,
		)
		return nil
	}

	// Fetch events in windows to respect RPC page limits.
	endLedger := min32(startLedger+p.cfg.LedgerWindow-1, latest.Sequence)

	log := p.log.With(
		"contract_id", contractID,
		"start_ledger", startLedger,
		"end_ledger", endLedger,
	)
	log.Info("indexing contract")

	runStart := time.Now()
	events, invocations, err := p.fetchWindow(ctx, rpc, contractID, startLedger, endLedger)
	if err != nil {
		return err
	}

	if len(events) > 0 {
		if err := p.store.BatchInsertEvents(ctx, events); err != nil {
			return fmt.Errorf("batch insert events: %w", err)
		}
	}
	if len(invocations) > 0 {
		if err := p.store.BatchInsertInvocations(ctx, invocations); err != nil {
			return fmt.Errorf("batch insert invocations: %w", err)
		}
	}

	newState := SyncState{ContractID: contractID, LastLedger: endLedger}
	if err := p.store.UpsertSyncState(ctx, newState); err != nil {
		return fmt.Errorf("upsert sync state: %w", err)
	}

	log.Info("contract indexed",
		"events", len(events),
		"invocations", len(invocations),
		"duration", time.Since(runStart),
	)
	return nil
}

// fetchWindow calls getEvents for [startLedger, endLedger] and fetches the
// corresponding transactions for each unique tx hash.
func (p *Poller) fetchWindow(ctx context.Context, rpc RPCClient, contractID string, startLedger, endLedger uint32) ([]Event, []Invocation, error) {
	filters := []EventFilter{{
		Type:        "contract",
		ContractIDs: []string{contractID},
	}}

	result, err := rpc.GetEvents(ctx, startLedger, endLedger, filters)
	if err != nil {
		return nil, nil, fmt.Errorf("get events [%d,%d]: %w", startLedger, endLedger, err)
	}

	var events []Event
	seenTx := make(map[string]struct{})

	for _, re := range result.Events {
		closedAt, _ := time.Parse(time.RFC3339, re.LedgerClosedAt)
		events = append(events, Event{
			ID:               re.ID,
			ContractID:       re.ContractID,
			Ledger:           re.Ledger,
			LedgerClosedAt:   closedAt,
			TxHash:           re.TxHash,
			Type:             re.Type,
			TopicXDR:         re.Topic,
			ValueXDR:         re.Value,
			InSuccessfulCall: re.InSuccessfulContractCall,
		})
		seenTx[re.TxHash] = struct{}{}
	}

	// Fetch one transaction record per unique tx hash.
	var invocations []Invocation
	for txHash := range seenTx {
		tx, err := rpc.GetTransaction(ctx, txHash)
		if err != nil {
			p.log.Warn("failed to fetch transaction, skipping",
				"tx_hash", txHash,
				"err", err,
			)
			continue
		}
		invocations = append(invocations, Invocation{
			TxHash:           txHash,
			ContractID:       contractID,
			Ledger:           tx.Ledger,
			LedgerClosedAt:   tx.LedgerClosedAt,
			Status:           tx.Status,
			ResultXDR:        tx.ResultXDR,
			ApplicationOrder: tx.ApplicationOrder,
		})
	}

	return events, invocations, nil
}

func (p *Poller) selectRPCClient(network string) (RPCClient, bool) {
	if p.rpcClients == nil {
		return nil, false
	}
	if network == "" {
		if rpc, ok := p.rpcClients[""]; ok {
			return rpc, true
		}
		if len(p.rpcClients) == 1 {
			for _, rpc := range p.rpcClients {
				return rpc, true
			}
		}
		return nil, false
	}
	rpc, ok := p.rpcClients[network]
	return rpc, ok
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
