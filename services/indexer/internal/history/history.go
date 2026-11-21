// Package history imports contract events older than the Soroban RPC
// retention window from an archive, such as SDF's Hubble dataset in
// BigQuery, and writes them through the same store as live indexing.
//
// The RPC keeps only a few days of events, which the backfill walks when a
// contract is added. Anything older has to come from an archive. A Source
// reads one ledger range of one contract's events; the Importer splits the
// requested range into batches, keeps it below the ledgers the RPC backfill
// already covers, and commits each batch with the store's idempotent
// inserts, so a re-run of the same range stores nothing twice.
package history

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// DefaultBatchLedgers is how many ledgers one Source query covers when
// Importer.BatchLedgers is unset: about two days of mainnet history, small
// enough that one batch is one transaction.
const DefaultBatchLedgers uint32 = 30_000

// Query is one ledger range of one contract's events, both ends inclusive.
type Query struct {
	ContractID string
	Network    string
	FromLedger uint32
	ToLedger   uint32
}

// Source reads archived contract events.
//
// Events must return every event the contract emitted in the range, as
// poller.Event values shaped exactly like live indexing's: the same ID
// format, base64 XDR topics and value, and the network label. Their IDs must
// be deterministic, because re-running an import relies on them to skip rows
// already stored.
type Source interface {
	Events(ctx context.Context, q Query) ([]poller.Event, error)
}

// Store is the part of the indexer store an import writes through.
type Store interface {
	// ImportTarget reports the contract's network and the oldest ledger the
	// RPC backfill covers.
	ImportTarget(ctx context.Context, contractID string) (Target, error)
	// CommitHistoryBatch stores rows with the live indexer's idempotent
	// inserts. It moves no cursor.
	CommitHistoryBatch(ctx context.Context, events []poller.Event, invocations []poller.Invocation) ([]poller.FailedEvent, error)
}

// Target describes the contract an import writes to.
type Target struct {
	Network string
	// RPCFloorLedger is the oldest ledger the RPC backfill covers, or 0
	// when the contract has no recorded backfill.
	RPCFloorLedger uint32
}

// Request is one import: a contract and an inclusive ledger range.
type Request struct {
	ContractID string
	FromLedger uint32
	ToLedger   uint32
}

// Result summarises an import.
type Result struct {
	ContractID string `json:"contract_id"`
	Network    string `json:"network"`
	// FromLedger and ToLedger are the range actually imported, after
	// clipping at the RPC floor.
	FromLedger uint32 `json:"from_ledger"`
	ToLedger   uint32 `json:"to_ledger"`
	// Clipped is true when the requested range reached into ledgers the
	// RPC backfill covers, and was shortened to stop below them.
	Clipped bool `json:"clipped"`
	// Batches is how many Source queries ran.
	Batches int `json:"batches"`
	// Events is how many events the source returned. Rows already stored
	// are counted here but skipped by the store.
	Events int `json:"events"`
	// Parked is how many events the store could not insert and moved to
	// the dead-letter queue.
	Parked int `json:"parked"`
	// Skipped is true when nothing was left to import.
	Skipped bool `json:"skipped"`
}

// Importer copies archived events into the store.
type Importer struct {
	Source Source
	Store  Store
	// BatchLedgers is the ledger span of one Source query; 0 means
	// DefaultBatchLedgers.
	BatchLedgers uint32
	Log          *slog.Logger
}

// Import copies the contract's events in the requested range.
//
// The range is clipped to end below the RPC backfill's oldest ledger: those
// ledgers are already indexed from the RPC, whose event IDs an archive
// cannot always reproduce exactly, so importing them could store an event
// twice under two IDs. Batches are committed oldest first; a failure stops
// the import with the batches before it stored, and re-running the same
// request resumes safely because every insert is idempotent.
func (im *Importer) Import(ctx context.Context, req Request) (Result, error) {
	if im.Source == nil || im.Store == nil {
		return Result{}, errors.New("history: importer needs a source and a store")
	}
	if req.ContractID == "" {
		return Result{}, errors.New("history: a contract id is required")
	}
	if req.FromLedger == 0 || req.ToLedger == 0 {
		return Result{}, errors.New("history: from and to ledgers are required")
	}
	if req.FromLedger > req.ToLedger {
		return Result{}, fmt.Errorf("history: from ledger %d is after to ledger %d", req.FromLedger, req.ToLedger)
	}
	log := im.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}

	target, err := im.Store.ImportTarget(ctx, req.ContractID)
	if err != nil {
		return Result{}, err
	}
	res := Result{ContractID: req.ContractID, Network: target.Network, FromLedger: req.FromLedger, ToLedger: req.ToLedger}
	if target.RPCFloorLedger > 0 && res.ToLedger >= target.RPCFloorLedger {
		res.Clipped = true
		if target.RPCFloorLedger <= res.FromLedger {
			res.Skipped = true
			log.Info("history import: range is already covered by the RPC backfill",
				"contract", req.ContractID, "rpc_floor", target.RPCFloorLedger)
			return res, nil
		}
		res.ToLedger = target.RPCFloorLedger - 1
	}

	batch := im.BatchLedgers
	if batch == 0 {
		batch = DefaultBatchLedgers
	}
	for from := res.FromLedger; ; {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		to := res.ToLedger
		if span := to - from; span >= batch {
			to = from + batch - 1
		}
		events, err := im.Source.Events(ctx, Query{
			ContractID: req.ContractID,
			Network:    target.Network,
			FromLedger: from,
			ToLedger:   to,
		})
		if err != nil {
			return res, fmt.Errorf("history: read ledgers %d-%d: %w", from, to, err)
		}
		for i := range events {
			if err := checkEvent(events[i], req.ContractID, from, to); err != nil {
				return res, fmt.Errorf("history: ledgers %d-%d: %w", from, to, err)
			}
			events[i].Network = target.Network
		}
		parked, err := im.Store.CommitHistoryBatch(ctx, events, nil)
		if err != nil {
			return res, fmt.Errorf("history: store ledgers %d-%d: %w", from, to, err)
		}
		res.Batches++
		res.Events += len(events)
		res.Parked += len(parked)
		log.Info("history import: batch stored", "contract", req.ContractID,
			"from", from, "to", to, "events", len(events), "parked", len(parked))

		if to >= res.ToLedger {
			return res, nil
		}
		from = to + 1
	}
}

// checkEvent rejects a row a source should never have returned, so a bad
// query cannot write another contract's events or ledgers outside the batch.
func checkEvent(e poller.Event, contractID string, from, to uint32) error {
	switch {
	case e.ID == "":
		return errors.New("source returned an event without an id")
	case e.ContractID != contractID:
		return fmt.Errorf("source returned event %s of contract %s", e.ID, e.ContractID)
	case e.Ledger < from || e.Ledger > to:
		return fmt.Errorf("source returned event %s from ledger %d, outside the batch", e.ID, e.Ledger)
	case e.LedgerClosedAt.IsZero():
		return fmt.Errorf("source returned event %s without a close time", e.ID)
	}
	return nil
}
