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
	"sort"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/anomaly"
	"github.com/sorolens/sorolens/services/indexer/internal/healthscore"
	"github.com/sorolens/sorolens/services/indexer/internal/metrics"
	"github.com/sorolens/sorolens/services/indexer/internal/partition"
	"github.com/sorolens/sorolens/services/indexer/internal/wasm"
)

const (
	// lockTTL is the Redis advisory lock lifetime per contract.
	// Set to twice the expected maximum per-contract processing time.
	lockTTL = 60 * time.Second

	// lockKeyPrefix is the Redis key prefix for per-contract indexer locks.
	lockKeyPrefix = "sorolens:lock:indexer:"

	// defaultNetworkLabel is the value used for the "network" metric label
	// when the poller is wired with the single unnamed RPC client (no
	// per-network SOROBAN_RPC_URL_* variables configured), issue #198.
	defaultNetworkLabel = "default"

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

	// AnomalyEnabled turns the per-pass anomaly detection job on (default off;
	// issue #136). Indexer main.go wires it via env.
	AnomalyEnabled bool
	// AnomalyLookbackHours is the rolling baseline window, default 168 (7 days).
	AnomalyLookbackHours int
	// AnomalySigma is the standard-deviation threshold, default 3.
	AnomalySigma float64
	// AnomalyMinHistory is the minimum samples before detection starts.
	AnomalyMinHistory int
}

// Poller fetches and persists events and invocations for all tracked contracts.
type Poller struct {
	rpcClients map[string]RPCClient
	store      Store
	redis      RedisClient
	cfg        Config
	log        *slog.Logger
	// metrics records the per-network lag gauges on every pass (issue #198).
	// It is nil unless SetMetrics is called; nil disables metric recording.
	metrics *metrics.Recorder
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

// SetMetrics attaches the Prometheus recorder the poller updates on every
// pass (issue #198). It must be called before Run; when it is never called
// metric recording is skipped, so existing callers are unaffected.
func (p *Poller) SetMetrics(r *metrics.Recorder) {
	p.metrics = r
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
	// Ensure the next month's partition exists before processing.
	if err := partition.EnsureNextMonthPartition(ctx, p.store); err != nil {
		p.log.Warn("failed to ensure next month partition", "err", err)
	}

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
			// Once a contract's batch starts, let it run to completion and
			// commit its cursor even if ctx is cancelled mid-flight (SIGTERM,
			// or the once-mode max-duration timeout): only the decision to
			// start the *next* contract's batch respects cancellation, via
			// the ctx.Err() check above. Without this, a shutdown signal
			// arriving mid-fetch would abort the in-flight RPC/store calls
			// and lose that contract's progress for the pass instead of
			// finishing it cleanly.
			if err := p.processContract(context.WithoutCancel(ctx), c); err != nil {
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

	// Record per-network lag after the contract batches have committed their
	// cursors so the gauge reflects the tip reached by this pass (issue #198).
	p.observeNetworkLag(ctx)

	if p.cfg.AnomalyEnabled {
		p.runAnomalyDetection(ctx)
	}
	p.runHealthScores(ctx)
	return nil
}

// observeNetworkLag records the per-network indexer lag (issue #198), defined
// as the network head ledger minus the last ledger committed for that network
// (its indexer cursor). It runs once per pass for every configured network, so
// both the currently active and merely cached networks report a value rather
// than a single hard-coded one.
//
// Best-effort by design: a transient RPC or store error for one network is
// logged and skipped without failing the indexing pass.
func (p *Poller) observeNetworkLag(ctx context.Context) {
	if p.metrics == nil {
		return
	}

	networks := make([]string, 0, len(p.rpcClients))
	for network := range p.rpcClients {
		networks = append(networks, network)
	}
	sort.Strings(networks)

	for _, network := range networks {
		if ctx.Err() != nil {
			return
		}
		rpc := p.rpcClients[network]
		if rpc == nil {
			continue
		}

		label := network
		if label == "" {
			label = defaultNetworkLabel
		}

		head, err := rpc.GetLatestLedger(ctx)
		if err != nil {
			p.log.Warn("metrics: latest ledger unavailable",
				"network", label,
				"err", err,
			)
			continue
		}
		if head == nil {
			continue
		}

		cursor, err := p.store.GetIndexerCursor(ctx, network)
		if err != nil {
			p.log.Warn("metrics: indexer cursor unavailable",
				"network", label,
				"err", err,
			)
			continue
		}

		p.metrics.ObserveNetwork(label, head.Sequence, cursor)
	}
}

// alertTxKey builds the deterministic de-duplication key for an anomaly alert.
// The contract_alerts table de-duplicates on (tx_hash, contract_id), so this
// synthetic key prevents re-inserting the same alert on every 5-minute pass.
func alertTxKey(metric string, ref time.Time) string {
	return fmt.Sprintf("anomaly:%s:%s", metric, ref.UTC().Format("2006-01-02T15"))
}

// runAnomalyDetection is the periodic anomaly-detection job (issue #136).
// For every active contract it builds a rolling baseline of per-hour activity
// and inserts a Warning contract alert for each metric that spikes more than
// AnomalySigma standard deviations above its mean.
//
// The job is best-effort: store or detection errors are logged and never
// block the indexing pass. It respects context cancellation so the 5-minute
// cadence in continuous mode and once-mode shutdown both behave cleanly.
func (p *Poller) runAnomalyDetection(ctx context.Context) {
	start := time.Now()
	samplesByContract := make(map[string][]anomaly.Sample)

	var cursor string
	for {
		if ctx.Err() != nil {
			return
		}
		contracts, next, err := p.store.ListContracts(ctx, cursor, 50)
		if err != nil {
			p.log.Error("anomaly: list contracts", "err", err)
			return
		}
		for _, c := range contracts {
			if ctx.Err() != nil {
				return
			}
			if c.Status != "active" && c.Status != "backfilling" {
				continue
			}
			samples, err := p.hourlyActivity(ctx, c.ID)
			if err != nil {
				p.log.Warn("anomaly: fetch hourly activity",
					"contract_id", c.ID,
					"err", err,
				)
				continue
			}
			samplesByContract[c.ID] = samples
		}
		if next == "" {
			break
		}
		cursor = next
	}

	var detected int
	for contractID, samples := range samplesByContract {
		cfg := anomaly.Config{
			Sigma:      p.cfg.AnomalySigma,
			MinHistory: p.cfg.AnomalyMinHistory,
		}
		for _, a := range anomaly.Detect(samples, cfg) {
			alert := Alert{
				ContractID: contractID,
				Severity:   "Warning",
				Message:    a.Message,
				Ledger:     0,
				TxHash:     alertTxKey(a.Metric, a.At),
				Timestamp:  time.Now().UTC(),
			}
			if err := p.store.InsertAlert(ctx, alert); err != nil {
				p.log.Warn("anomaly: insert alert",
					"contract_id", contractID,
					"metric", a.Metric,
					"err", err,
				)
				continue
			}
			detected++
			p.log.Warn("anomaly detected",
				"contract_id", contractID,
				"metric", a.Metric,
				"observed", a.Observed,
				"threshold", a.Expected,
			)
		}
	}

	p.log.Info("anomaly detection pass complete",
		"contracts", len(samplesByContract),
		"alerts", detected,
		"duration", time.Since(start),
	)
}

// hourlyActivity fetches the rolling window of per-hour activity for a
// contract and converts it to the detector's sample format (oldest first).
func (p *Poller) hourlyActivity(ctx context.Context, contractID string) ([]anomaly.Sample, error) {
	lookback := p.cfg.AnomalyLookbackHours
	if lookback <= 0 {
		lookback = anomaly.DefaultLookbackHours
	}
	buckets, err := p.store.RecentHourlyActivity(ctx, contractID, lookback)
	if err != nil {
		return nil, err
	}
	samples := make([]anomaly.Sample, 0, len(buckets))
	for _, b := range buckets {
		samples = append(samples, anomaly.Sample{
			At:          b.Hour,
			Events:      float64(b.EventCount),
			Invocations: float64(b.InvokeCount),
			CPU:         float64(b.CPU),
			Fees:        float64(b.Fees),
		})
	}
	return samples, nil
}

// runHealthScores refreshes the cached composite health score (issue #137) for
// every active contract. The job is best-effort like the anomaly pass: store
// errors are logged and never block the indexing pass, and the score for a
// contract with no data yet still gets computed from zero-inputs so the cache
// table receives a row on the first poll.
func (p *Poller) runHealthScores(ctx context.Context) {
	start := time.Now()
	var scored int

	var cursor string
	for {
		if ctx.Err() != nil {
			return
		}
		contracts, next, err := p.store.ListContracts(ctx, cursor, 50)
		if err != nil {
			p.log.Error("health score: list contracts", "err", err)
			return
		}
		for _, c := range contracts {
			if ctx.Err() != nil {
				return
			}
			if c.Status != "active" && c.Status != "backfilling" {
				continue
			}
			inputs, err := p.store.ContractHealthInputs(ctx, c.ID)
			if err != nil {
				p.log.Warn("health score: fetch inputs",
					"contract_id", c.ID,
					"err", err,
				)
				continue
			}
			score := healthscore.Compute(healthInputsToScoreInputs(inputs))
			if err := p.store.UpsertContractHealthScore(ctx, ContractHealthScore{
				ContractID:           c.ID,
				Score:                score.Overall,
				ComponentUptime:      score.Uptime,
				ComponentErrorRate:   score.ErrorRate,
				ComponentPerformance: score.Performance,
				ComponentStorageTTL:  score.StorageTTL,
				ComputedAt:           time.Now().UTC(),
			}); err != nil {
				p.log.Warn("health score: upsert",
					"contract_id", c.ID,
					"err", err,
				)
				continue
			}
			scored++
			p.log.Debug("health score updated",
				"contract_id", c.ID,
				"score", score.Overall,
			)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	p.log.Info("health score pass complete",
		"contracts_scored", scored,
		"duration", time.Since(start),
	)
}

// healthInputsToScoreInputs converts the poller's mirror HealthInputs into the
// pure healthscore package's Inputs type.
func healthInputsToScoreInputs(in HealthInputs) healthscore.Inputs {
	activity := make([]healthscore.Activity, 0, len(in.Activity))
	for _, a := range in.Activity {
		activity = append(activity, healthscore.Activity{
			Invocations: a.InvokeCount,
			CPU:         a.CPU,
			Fees:        a.Fees,
		})
	}
	return healthscore.Inputs{
		HealthyChecks:     in.HealthyChecks,
		TotalChecks:       in.TotalChecks,
		WatchdogStatus:    in.WatchdogStatus,
		TotalInvocations:  in.TotalInvocations,
		FailedInvocations: in.FailedInvocations,
		Activity:          activity,
		TotalStorage:      in.TotalStorage,
		ExpiringStorage:   in.ExpiringStorage,
	}
}

// processContract indexes all new events for one contract. It also checks the
// contract's on-chain Wasm hash for upgrades before scanning events so a code
// upgrade with no indexable events still gets recorded.
func (p *Poller) processContract(ctx context.Context, contract Contract) error {
	contractID := contract.ID
	network := contract.Network
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

	// Detect Wasm upgrades (contract code changes) before scanning events so
	// an upgrade with no indexable events still gets recorded. Any error here
	// must not block event indexing, so we log and continue.
	if err := p.checkWasmHash(ctx, rpc, contract); err != nil {
		p.log.Warn("wasm upgrade check failed (continuing)",
			"contract_id", contractID,
			"err", err,
		)
	}

	latest, err := rpc.GetLatestLedger(ctx)
	if err != nil {
		return fmt.Errorf("get latest ledger: %w", err)
	}

	syncState, err := p.store.GetSyncState(ctx, contractID)
	if err != nil {
		return fmt.Errorf("get sync state: %w", err)
	}

	networkCursor, err := p.store.GetIndexerCursor(ctx, network)
	if err != nil {
		p.log.Warn("failed to fetch indexer cursor for network (continuing)",
			"network", network,
			"err", err,
		)
	}

	var startLedger uint32
	if syncState.LastLedger == 0 {
		if networkCursor > 0 {
			// Resume after the last successfully committed network batch
			startLedger = networkCursor + 1
		} else if latest.Sequence > newContractBackfillWindow {
			// New contract: best-effort backfill from within the retention window.
			startLedger = latest.Sequence - newContractBackfillWindow
		} else {
			startLedger = 1
		}
		p.log.Warn("starting sync for contract",
			"contract_id", contractID,
			"start_ledger", startLedger,
			"network_cursor", networkCursor,
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
	events, invocations, err := p.fetchWindow(ctx, rpc, contractID, network, startLedger, endLedger)
	if err != nil {
		return err
	}

	newState := SyncState{ContractID: contractID, LastLedger: endLedger}
	if err := p.store.BatchInsertWithCursor(ctx, network, endLedger, events, invocations, newState); err != nil {
		return fmt.Errorf("batch insert with cursor: %w", err)
	}

	log.Info("contract indexed",
		"events", len(events),
		"invocations", len(invocations),
		"duration", time.Since(runStart),
	)
	return nil
}

// checkWasmHash compares the current on-chain Wasm hash of the contract's
// instance entry against the hash observed on the previous poll. A mismatch
// means the contract code was upgraded. The change is recorded as a
// ContractUpgrade row and the stored hash is refreshed so later polls diff
// against the new value.
//
// Best-effort by design: a transient RPC error or an unreadable entry is
// reported to the caller, and processContract logs it without failing the
// event index pass.
func (p *Poller) checkWasmHash(ctx context.Context, rpc RPCClient, contract Contract) error {
	key, err := wasm.ContractInstanceKey(contract.ID)
	if err != nil {
		return fmt.Errorf("build instance key: %w", err)
	}

	res, err := rpc.GetLedgerEntries(ctx, []string{key})
	if err != nil {
		return fmt.Errorf("get instance entry: %w", err)
	}

	var currentHash string
	for _, e := range res.Entries {
		if h, ok := wasm.WasmHashFromInstanceEntry(e.XDR); ok {
			currentHash = h
			break
		}
	}
	if currentHash == "" {
		// Entry not yet readable (e.g. ledger retention); nothing to record.
		return nil
	}

	// First observed hash: baseline it without recording an upgrade.
	if contract.WasmHash == "" {
		if err := p.store.UpdateContractWasmHash(ctx, contract.ID, currentHash); err != nil {
			return fmt.Errorf("baseline contract wasm hash: %w", err)
		}
		return nil
	}

	if currentHash == contract.WasmHash {
		return nil // unchanged
	}

	upgrade := ContractUpgrade{
		ContractID: contract.ID,
		FromHash:   contract.WasmHash,
		ToHash:     currentHash,
		Ledger:     ledgerFromEntry(res),
		At:         time.Now().UTC(),
	}
	if err := p.store.InsertContractUpgrade(ctx, upgrade); err != nil {
		return fmt.Errorf("insert contract upgrade: %w", err)
	}
	if err := p.store.UpdateContractWasmHash(ctx, contract.ID, currentHash); err != nil {
		return fmt.Errorf("update contract wasm hash: %w", err)
	}

	p.log.Info("contract code upgraded",
		"contract_id", contract.ID,
		"from_hash", contract.WasmHash,
		"to_hash", currentHash,
	)
	return nil
}

// ledgerFromEntry returns the modification ledger of the first instance entry
// if present, falling back to the latest ledger reported by the RPC result.
func ledgerFromEntry(res *GetLedgerEntriesResult) uint32 {
	for _, e := range res.Entries {
		if e.LastModifiedLedgerSeq != 0 {
			return e.LastModifiedLedgerSeq
		}
	}
	if res.LatestLedger != 0 {
		return res.LatestLedger
	}
	return 0
}

// fetchWindow calls getEvents for [startLedger, endLedger] and fetches the
// corresponding transactions for each unique tx hash. The contract's network
// is stamped onto every row so multi-network queries can filter on it.
func (p *Poller) fetchWindow(ctx context.Context, rpc RPCClient, contractID, network string, startLedger, endLedger uint32) ([]Event, []Invocation, error) {
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
			Network:          network,
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
			Network:          network,
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
