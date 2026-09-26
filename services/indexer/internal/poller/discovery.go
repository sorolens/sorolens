package poller

import (
	"context"
	"errors"
	"fmt"

	"github.com/sorolens/sorolens/services/indexer/internal/discovery"
)

const (
	// discoveryCursorPrefix namespaces the per-network discovery cursor in the
	// indexer_cursors table, next to the event cursor keyed by the bare
	// network name.
	discoveryCursorPrefix = "discovery:"

	// discoveryInitialLookback is how far back the first discovery scan of a
	// network starts (~1 hour at 5s ledgers).
	discoveryInitialLookback uint32 = 720

	// discoveryPageSize is the getTransactions page size.
	discoveryPageSize = 200

	// discoveryMaxPages caps the transactions scanned per network per pass so
	// a long catch-up cannot starve event indexing. The cursor only advances
	// over fully scanned ledgers, so the next pass resumes where this stopped.
	discoveryMaxPages = 50
)

// TransactionLister is implemented by RPC clients that support
// getTransactions. Discovery is skipped for networks whose client does not.
type TransactionLister interface {
	// GetTransactions returns up to limit transactions starting at
	// startLedger (first page, cursor "") or after cursor (later pages).
	GetTransactions(ctx context.Context, startLedger uint32, cursor string, limit int) (*GetTransactionsResult, error)
}

// DiscoveryStore is implemented by stores that support contract discovery
// (issue #123). Discovery is skipped when the store does not implement it.
type DiscoveryStore interface {
	// ListWatchedAccounts returns every watched account.
	ListWatchedAccounts(ctx context.Context) ([]WatchedAccount, error)
	// RecordDiscoveredContract tracks the contract as active with label
	// "discovered_by:<account>" and bumps the account's discovered_count.
	// It reports false when the contract was already tracked.
	RecordDiscoveredContract(ctx context.Context, d DiscoveredContract) (bool, error)
}

// Unwrapper is implemented by RPC client decorators (e.g. the watchdog
// interceptor in main.go) so optional capabilities of the wrapped client,
// like TransactionLister, stay reachable.
type Unwrapper interface {
	Unwrap() RPCClient
}

// transactionLister finds a TransactionLister on rpc or any client it wraps.
func transactionLister(rpc RPCClient) (TransactionLister, bool) {
	for rpc != nil {
		if tl, ok := rpc.(TransactionLister); ok {
			return tl, true
		}
		u, ok := rpc.(Unwrapper)
		if !ok {
			return nil, false
		}
		rpc = u.Unwrap()
	}
	return nil, false
}

// runDiscovery scans new ledgers on every network for create_contract
// operations sourced from a watched account and tracks the deployed
// contracts. It runs at the start of each pass so a discovered contract is
// indexed in the same pass. Best-effort: errors are logged, never returned.
func (p *Poller) runDiscovery(ctx context.Context) {
	ds, ok := p.store.(DiscoveryStore)
	if !ok {
		return
	}
	accounts, err := ds.ListWatchedAccounts(ctx)
	if err != nil {
		p.log.Error("discovery: list watched accounts", "err", err)
		return
	}
	watched := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		watched[a.AccountID] = true
	}

	for network, rpc := range p.rpcClients {
		if ctx.Err() != nil {
			return
		}
		tl, ok := transactionLister(rpc)
		if !ok {
			continue
		}
		passphrase, ok := discovery.Passphrase(network)
		if !ok {
			p.log.Warn("discovery: unknown network passphrase, skipping", "network", network)
			continue
		}
		found, err := p.discoverNetwork(ctx, ds, rpc, tl, network, passphrase, watched)
		if err != nil {
			p.log.Error("discovery: scan failed", "network", network, "err", err)
		}
		if found > 0 {
			p.log.Info("discovery: tracked new contracts", "network", network, "count", found)
		}
	}
}

// discoverNetwork scans one network from its discovery cursor to the latest
// ledger and returns how many contracts were newly tracked.
func (p *Poller) discoverNetwork(ctx context.Context, ds DiscoveryStore, rpc RPCClient, tl TransactionLister,
	network, passphrase string, watched map[string]bool) (int, error) {
	cursorKey := discoveryCursorPrefix + network

	latest, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return 0, fmt.Errorf("get latest ledger: %w", err)
	}
	last, err := p.store.GetIndexerCursor(ctx, cursorKey)
	if err != nil {
		return 0, fmt.Errorf("get discovery cursor: %w", err)
	}

	// Nothing to look for: keep the cursor at the tip so adding the first
	// watched account does not trigger a scan of stale history.
	if len(watched) == 0 {
		if latest.Sequence > last {
			return 0, p.store.SetIndexerCursor(ctx, cursorKey, latest.Sequence)
		}
		return 0, nil
	}

	start := last + 1
	if last == 0 {
		start = 1
		if latest.Sequence > discoveryInitialLookback {
			start = latest.Sequence - discoveryInitialLookback
		}
	}
	if start > latest.Sequence {
		return 0, nil
	}

	var (
		found   int
		cursor  string
		scanned = start - 1 // highest ledger fully scanned
	)
	for page := 0; page < discoveryMaxPages; page++ {
		res, err := tl.GetTransactions(ctx, start, cursor, discoveryPageSize)
		if err != nil {
			return found, p.saveDiscoveryCursor(ctx, cursorKey, scanned, fmt.Errorf("get transactions: %w", err))
		}

		for _, tx := range res.Transactions {
			// A page may end part-way through a ledger; everything before
			// this transaction's ledger is complete.
			if tx.Ledger > 0 && tx.Ledger-1 > scanned {
				scanned = tx.Ledger - 1
			}
			if tx.Status != "SUCCESS" {
				continue
			}
			deployments, err := discovery.Deployments(tx.EnvelopeXDR, passphrase)
			if err != nil {
				p.log.Warn("discovery: undecodable envelope, skipping",
					"network", network, "ledger", tx.Ledger, "tx_hash", tx.TxHash, "err", err)
				continue
			}
			for _, d := range deployments {
				account := watchedDeployer(d, watched)
				if account == "" {
					continue
				}
				tracked, err := ds.RecordDiscoveredContract(ctx, DiscoveredContract{
					ContractID: d.ContractID,
					Network:    network,
					AccountID:  account,
					Ledger:     int64(tx.Ledger),
				})
				if err != nil {
					// Leave the cursor before this ledger so the deployment is
					// retried next pass.
					return found, p.saveDiscoveryCursor(ctx, cursorKey, scanned,
						fmt.Errorf("record discovered contract %s: %w", d.ContractID, err))
				}
				if tracked {
					found++
					p.log.Info("discovery: tracking contract",
						"contract_id", d.ContractID,
						"network", network,
						"discovered_by", account,
						"ledger", tx.Ledger,
					)
				}
			}
		}

		if len(res.Transactions) < discoveryPageSize || res.Cursor == "" {
			// Reached the RPC tip: every ledger up to latest is scanned.
			tip := res.LatestLedger
			if tip < latest.Sequence {
				tip = latest.Sequence
			}
			return found, p.saveDiscoveryCursor(ctx, cursorKey, tip, nil)
		}
		cursor = res.Cursor
	}
	return found, p.saveDiscoveryCursor(ctx, cursorKey, scanned, nil)
}

// saveDiscoveryCursor persists the highest fully scanned ledger (when it
// moved forward) and returns cause joined with any write error.
func (p *Poller) saveDiscoveryCursor(ctx context.Context, key string, ledger uint32, cause error) error {
	if ledger == 0 {
		return cause
	}
	if err := p.store.SetIndexerCursor(ctx, key, ledger); err != nil {
		return errors.Join(cause, fmt.Errorf("set discovery cursor: %w", err))
	}
	return cause
}

// watchedDeployer attributes a deployment to a watched account: the account
// that submitted the operation, else the from-address deployer.
func watchedDeployer(d discovery.Deployment, watched map[string]bool) string {
	if watched[d.Source] {
		return d.Source
	}
	if watched[d.Deployer] {
		return d.Deployer
	}
	return ""
}
