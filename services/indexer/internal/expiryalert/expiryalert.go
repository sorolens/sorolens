// Package expiryalert implements storage expiry alerting (issue #510): after
// each indexer pass it scans the tracked storage entries of every active
// contract for entries within a configurable number of ledgers of expiry,
// raises one summary alert per contract, and hands it to the existing
// watchdog notifier so configured alert subscriptions (webhook/Slack/Discord/
// PagerDuty) receive it.
//
// The job follows the anomaly/rulesengine pattern of this codebase:
//   - It depends only on the narrow Store and Notifier interfaces below, so
//     tests substitute mocks and no topology (single process or worker) needs
//     to know about Postgres.
//   - Each alert carries a deterministic synthetic TxHash derived from the
//     full near-expiry entry set of the contract. The store de-duplicates
//     alerts on (tx_hash, contract_id) and AlertExists lets the runner skip
//     re-dispatch when nothing changed, so an entry crossing the threshold
//     alerts exactly once and repeats only after the near-expiry set changes
//     (a new entry crosses, one is extended, or one is archived).
//   - It is best-effort: a failure on one contract is logged and the pass
//     continues with the rest, mirroring the anomaly and health-score jobs.
package expiryalert

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

const (
	// DefaultThresholdLedgers is the proximity window for "close to expiry":
	// 5000 ledgers is roughly 7 days at the ~5s Stellar ledger cadence and
	// matches the default promised in ROADMAP (issue #510).
	DefaultThresholdLedgers int64 = 5_000

	// DefaultMaxEntries caps how many near-expiry entries one contract's scan
	// reads per pass. It mirrors snapshotStorageLimit's intent in
	// apps/api/internal/handler/snapshot_export.go: bound the scan so a
	// runaway contract cannot turn the alert job into an unbounded query.
	DefaultMaxEntries = 200

	// defaultContractPageSize matches the page size every other pass job in
	// the poller uses when walking the contract set.
	defaultContractPageSize = 50

	// alertSeverity is Critical because the shared watchdog notifier only
	// dispatches Critical alerts to subscriptions (see
	// services/indexer/internal/watchdog/notifier.go), and expiring storage
	// is a headline alert per README.
	alertSeverity = "Critical"

	// txKeyPrefix namespaces synthetic dedup keys, like anomaly's and
	// rulesengine's ("anomaly:<metric>:<hour>", "rule:<id>:<contract>").
	txKeyPrefix = "expiry"
)

// Config holds runtime parameters for the expiry scanner.
type Config struct {
	// ThresholdLedgers is the number of ledgers before expiry that counts as
	// "close to expiring". Zero or negative selects DefaultThresholdLedgers.
	ThresholdLedgers int64
	// MaxEntries caps the entries read per contract per pass. Zero or negative
	// selects DefaultMaxEntries.
	MaxEntries int
	// ContractPageSize controls the ListContracts cursor. Zero or negative
	// selects defaultContractPageSize.
	ContractPageSize int
}

func (c Config) withDefaults() Config {
	if c.ThresholdLedgers <= 0 {
		c.ThresholdLedgers = DefaultThresholdLedgers
	}
	if c.MaxEntries <= 0 {
		c.MaxEntries = DefaultMaxEntries
	}
	if c.ContractPageSize <= 0 {
		c.ContractPageSize = defaultContractPageSize
	}
	return c
}

// Environment keys the indexer reads to configure storage expiry alerting
// (issue #510). They follow the INDEXER_ prefix every other indexer knob uses
// (INDEXER_ANOMALY_ENABLED, INDEXER_METRICS_ADDR, ...).
const (
	// EnvThresholdLedgers is the near-expiry window in ledgers. Unset, blank
	// or invalid keeps DefaultThresholdLedgers (5000, ~7 days).
	EnvThresholdLedgers = "INDEXER_STORAGE_EXPIRY_THRESHOLD_LEDGERS"
	// EnvMaxEntries caps how many near-expiry entries one contract's scan
	// reads per pass. Unset, blank or invalid keeps DefaultMaxEntries.
	EnvMaxEntries = "INDEXER_STORAGE_EXPIRY_MAX_ENTRIES"
	// EnvEnabled turns the storage expiry alert job on or off. Anything
	// strconv.ParseBool understands is accepted; the default is enabled, so an
	// operator opts out with INDEXER_STORAGE_EXPIRY_ALERT_ENABLED=false.
	EnvEnabled = "INDEXER_STORAGE_EXPIRY_ALERT_ENABLED"
)

// ConfigFromEnv builds a Config from the process environment, using getenv for
// lookups (callers pass os.Getenv). A nil getenv yields the defaults. Values
// that are empty, unparseable or non-positive fall back to the documented
// default rather than disabling the safety bounds, so a typo can never turn
// the scan into "everything is expiring" or an unbounded query.
func ConfigFromEnv(getenv func(string) string) Config {
	cfg := Config{}.withDefaults()
	if getenv == nil {
		return cfg
	}
	if v := strings.TrimSpace(getenv(EnvThresholdLedgers)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.ThresholdLedgers = n
		}
	}
	if v := strings.TrimSpace(getenv(EnvMaxEntries)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxEntries = n
		}
	}
	return cfg
}

// Enabled reports whether storage expiry alerting is turned on. The job is on
// by default; only an explicit, parseable false (ENV=0/false/F) disables it, so
// an unrecognised value never silently switches the feature off. A nil getenv
// means "no environment consulted" and reports the default, true.
func Enabled(getenv func(string) string) bool {
	if getenv == nil {
		return true
	}
	v := strings.TrimSpace(getenv(EnvEnabled))
	if v == "" {
		return true
	}
	on, err := strconv.ParseBool(v)
	if err != nil {
		return true
	}
	return on
}

// Store is the persistence surface the scanner needs. pgstore.Store is the
// production implementation; the alert row is de-duplicated on
// (tx_hash, contract_id) exactly like the anomaly job's.
type Store interface {
	// ListContracts returns one page of tracked contracts; cursor is the last
	// id of the previous page ("" for the first) and the returned cursor is
	// "" once the final page has been read.
	ListContracts(ctx context.Context, cursor string, limit int) ([]poller.Contract, string, error)
	// ExpiringStorageEntries returns at most limit live entries of the
	// contract that expire within thresholdLedgers ledgers of the contract's
	// last indexed ledger, ordered by live_until_ledger ascending.
	ExpiringStorageEntries(ctx context.Context, contractID string, thresholdLedgers int64, limit int) ([]poller.ExpiringStorageEntry, error)
	// AlertExists reports whether an alert row with this dedup key was already
	// recorded for the contract, so the pass skips both the insert and the
	// webhook dispatch when the near-expiry set is unchanged.
	AlertExists(ctx context.Context, txHash, contractID string) (bool, error)
	// InsertAlert persists the alert row (de-duplicated on
	// (tx_hash, contract_id) by the store).
	InsertAlert(ctx context.Context, a poller.Alert) error
}

// Notifier delivers one persisted expiry alert to the configured
// subscriptions. The production implementation wraps watchdog.DispatchAlerts.
type Notifier interface {
	Notify(ctx context.Context, alert poller.Alert)
}

// Runner executes the near-expiry scan at the end of every indexer pass.
type Runner struct {
	store  Store
	notify Notifier
	cfg    Config
	log    *slog.Logger
	now    func() time.Time
}

// New returns a Runner. A nil logger falls back to slog.Default.
func New(store Store, notify Notifier, cfg Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	return &Runner{
		store:  store,
		notify: notify,
		cfg:    cfg.withDefaults(),
		log:    log,
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// AlertExpiry walks every active/backfilling contract and raises one summary
// alert per contract whose near-expiry entry set changed since the last
// alert. A per-contract store error is logged and skipped so one bad contract
// cannot abort the pass; only a ListContracts failure ends it. It satisfies
// poller.ExpiryAlerter.
func (r *Runner) AlertExpiry(ctx context.Context) error {
	start := time.Now()
	var alerted, skipped, scanned int

	var cursor string
	for {
		if ctx.Err() != nil {
			return nil
		}
		contracts, next, err := r.store.ListContracts(ctx, cursor, r.cfg.ContractPageSize)
		if err != nil {
			return fmt.Errorf("expiry alert: list contracts: %w", err)
		}
		for _, c := range contracts {
			if ctx.Err() != nil {
				return nil
			}
			if c.Status != "active" && c.Status != "backfilling" {
				continue
			}
			scanned++
			res, err := r.checkContract(ctx, c.ID)
			if err != nil {
				r.log.Warn("expiry alert: contract scan",
					"contract_id", c.ID,
					"err", err,
				)
				continue
			}
			switch res {
			case outcomeAlerted:
				alerted++
			case outcomeDuplicate:
				skipped++
			}
		}
		if next == "" {
			break
		}
		cursor = next
	}

	r.log.Info("storage expiry alert pass complete",
		"contracts", scanned,
		"alerts", alerted,
		"duplicates_skipped", skipped,
		"threshold_ledgers", r.cfg.ThresholdLedgers,
		"duration", time.Since(start),
	)
	return nil
}

// outcome distinguishes what checkContract did for one contract.
type outcome int

const (
	outcomeNothing outcome = iota
	outcomeAlerted
	outcomeDuplicate
)

// checkContract scans one contract's near-expiry entries and, when the set is
// non-empty and not already alerted, inserts and dispatches the summary.
func (r *Runner) checkContract(ctx context.Context, contractID string) (outcome, error) {
	entries, err := r.store.ExpiringStorageEntries(ctx, contractID, r.cfg.ThresholdLedgers, r.cfg.MaxEntries)
	if err != nil {
		return outcomeNothing, fmt.Errorf("expiring entries: %w", err)
	}
	if len(entries) == 0 {
		return outcomeNothing, nil
	}

	txKey := DedupKey(contractID, entries)
	exists, err := r.store.AlertExists(ctx, txKey, contractID)
	if err != nil {
		return outcomeNothing, fmt.Errorf("alert exists: %w", err)
	}
	if exists {
		return outcomeDuplicate, nil
	}

	alert := poller.Alert{
		ContractID: contractID,
		Severity:   alertSeverity,
		Message:    r.summary(entries),
		Ledger:     entries[0].LiveUntilLedger, // earliest expiry (ordered by store)
		TxHash:     txKey,
		Timestamp:  r.now(),
	}
	if err := r.store.InsertAlert(ctx, alert); err != nil {
		return outcomeNothing, fmt.Errorf("insert alert: %w", err)
	}
	// Dispatch only after the dedup row is durable, so a crash between the
	// two never re-notifies the same state on the next pass.
	r.notify.Notify(ctx, alert)

	r.log.Warn("storage entries close to expiry",
		"contract_id", contractID,
		"entries", len(entries),
		"earliest_expiry_ledger", entries[0].LiveUntilLedger,
		"threshold_ledgers", r.cfg.ThresholdLedgers,
	)
	return outcomeAlerted, nil
}

// summary renders the one-per-contract message body. It reports the capped
// scan size honestly when the limit truncated the set.
func (r *Runner) summary(entries []poller.ExpiringStorageEntry) string {
	byDurability := make(map[string]int, 3)
	for _, e := range entries {
		byDurability[e.Durability]++
	}
	var parts []string
	for _, d := range []string{"persistent", "temporary", "instance"} {
		if n := byDurability[d]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, d))
		}
	}
	truncated := ""
	if len(entries) >= r.cfg.MaxEntries {
		truncated = fmt.Sprintf(" (first %d by expiry; more may exist)", r.cfg.MaxEntries)
	}
	return fmt.Sprintf(
		"%d storage entries expire within %d ledgers%s: %s; earliest expiry at ledger %d",
		len(entries), r.cfg.ThresholdLedgers, truncated,
		strings.Join(parts, ", "), entries[0].LiveUntilLedger,
	)
}

// DedupKey builds the deterministic de-duplication key for one contract's
// near-expiry state: a hash over the full (key_xdr, live_until_ledger) set in
// store order. Identical state across passes collapses into the one alert row
// on contract_alerts' (tx_hash, contract_id) unique index; any change to the
// set (an entry crossing, being extended, or archived) yields a new key and
// therefore a new alert.
func DedupKey(contractID string, entries []poller.ExpiringStorageEntry) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s", contractID)
	for _, e := range entries {
		fmt.Fprintf(h, "\x00%s\x00%d", e.KeyXDR, e.LiveUntilLedger)
	}
	return fmt.Sprintf("%s:%s:%s", txKeyPrefix, contractID, hex.EncodeToString(h.Sum(nil))[:16])
}
