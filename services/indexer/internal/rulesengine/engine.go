// Package rulesengine evaluates user-defined alert rules (apps/api/rulelang)
// inside the indexer's poll loop and raises an alert when one fires.
//
// The engine is deliberately decoupled from the concrete database: it depends
// on the narrow Store interface below, so it can be unit-tested with an
// in-memory fake and reused by any indexer topology (single process, worker,
// or coordinator).
package rulesengine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	rulelang "github.com/sorolens/sorolens/apps/api/rulelang"
)

// DefaultWindow is the sample window used for a rule with no `for` clause.
const DefaultWindow = time.Hour

// Rule is the evaluator's view of a stored alert rule.
type Rule struct {
	ID         int64
	Name       string
	Source     string
	Severity   string
	ContractID string // "" means every contract
	Network    string // "" means every network
	WindowSecs int64
}

// Contract is the minimal contract identity the engine needs to fan a rule out
// across the fleet.
type Contract struct {
	ID      string
	Network string
}

// Sample is one per-minute metric bucket.
type Sample struct {
	At     time.Time
	Values map[string]float64
}

// Alert is a firing raised by a rule. It mirrors the poller's alert shape so
// the indexer can persist it with the existing InsertAlert path.
type Alert struct {
	ContractID string
	Severity   string
	Message    string
	// Rule is the rule's name, recorded on the alert so the grouping engine
	// keys on it.
	Rule      string
	TxHash    string
	Timestamp time.Time
}

// Store is the persistence the engine needs.
type Store interface {
	// ListEnabledAlertRules returns the active rules to evaluate this pass.
	ListEnabledAlertRules(ctx context.Context) ([]Rule, error)
	// ListContracts returns every tracked contract, for rules that are not
	// scoped to a single contract.
	ListContracts(ctx context.Context) ([]Contract, error)
	// ContractMetricSamples returns per-minute buckets for a contract in
	// [from, to).
	ContractMetricSamples(ctx context.Context, contractID string, from, to time.Time) ([]Sample, error)
	// InsertAlert persists a firing. Implementations de-duplicate on
	// (tx_hash, contract_id).
	InsertAlert(ctx context.Context, a Alert) error
}

// Engine evaluates rules each pass.
type Engine struct {
	store Store
	log   *slog.Logger
	now   func() time.Time
}

// New returns an engine backed by store.
func New(store Store, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.Default()
	}
	return &Engine{store: store, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// EvaluatePass loads the enabled rules, evaluates each against its target
// contracts, and raises an alert for every rule that fires. A failure on one
// rule is logged and skipped so it cannot abort the pass.
func (e *Engine) EvaluatePass(ctx context.Context) error {
	rules, err := e.store.ListEnabledAlertRules(ctx)
	if err != nil {
		return fmt.Errorf("rulesengine: list rules: %w", err)
	}
	if len(rules) == 0 {
		return nil
	}

	// Resolve the fleet lazily: rules scoped to one contract do not need it.
	var contracts []Contract
	contractsLoaded := false
	loadContracts := func() ([]Contract, error) {
		if contractsLoaded {
			return contracts, nil
		}
		contracts, err = e.store.ListContracts(ctx)
		if err != nil {
			return nil, err
		}
		contractsLoaded = true
		return contracts, nil
	}

	for _, r := range rules {
		if ctx.Err() != nil {
			return nil
		}
		if err := e.evaluateRule(ctx, r, loadContracts); err != nil {
			e.log.Error("rule evaluation failed", "rule_id", r.ID, "rule", r.Name, "err", err)
		}
	}
	return nil
}

// evaluateRule parses one rule and evaluates it against every target contract.
func (e *Engine) evaluateRule(ctx context.Context, r Rule, loadContracts func() ([]Contract, error)) error {
	parsed, err := rulelang.Validate(r.Source)
	if err != nil {
		return fmt.Errorf("invalid stored rule: %w", err)
	}
	window := time.Duration(r.WindowSecs) * time.Second
	if window <= 0 {
		window = DefaultWindow
	}
	now := e.now()

	targets, err := e.targets(ctx, r, parsed.ContractID, loadContracts)
	if err != nil {
		return err
	}
	for _, contractID := range targets {
		samples, err := e.store.ContractMetricSamples(ctx, contractID, now.Add(-window), now)
		if err != nil {
			return fmt.Errorf("load samples for %s: %w", contractID, err)
		}
		res, err := rulelang.Evaluate(parsed, toWindow(now, samples))
		if err != nil {
			return err
		}
		if !res.Fired {
			continue
		}
		alert := Alert{
			ContractID: contractID,
			Severity:   severityOrDefault(r.Severity),
			Message:    message(r, res),
			Rule:       r.Name,
			// A deterministic key means a rule that stays fired across passes
			// collapses into one alert row rather than one per pass.
			TxHash:    fmt.Sprintf("rule:%d:%s", r.ID, contractID),
			Timestamp: now,
		}
		if err := e.store.InsertAlert(ctx, alert); err != nil {
			return fmt.Errorf("insert alert for %s: %w", contractID, err)
		}
		e.log.Info("rule fired",
			"rule_id", r.ID, "rule", r.Name, "contract_id", contractID, "reason", res.Reason)
	}
	return nil
}

// targets returns the contracts a rule applies to, honoring both the rule's
// own `on contract` clause and its network scope.
func (e *Engine) targets(ctx context.Context, r Rule, parsedContractID string, loadContracts func() ([]Contract, error)) ([]string, error) {
	// The stored contract_id wins; a rule may also carry `on contract` in its
	// source (they are normally the same value).
	scope := r.ContractID
	if scope == "" {
		scope = parsedContractID
	}
	if scope != "" {
		return []string{scope}, nil
	}
	all, err := loadContracts()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, c := range all {
		if r.Network != "" && c.Network != r.Network {
			continue
		}
		out = append(out, c.ID)
	}
	return out, nil
}

func toWindow(now time.Time, samples []Sample) rulelang.Window {
	w := rulelang.Window{To: now, From: now}
	if len(samples) > 0 {
		w.From = samples[0].At
	}
	for _, s := range samples {
		w.Samples = append(w.Samples, rulelang.Sample{At: s.At, Values: s.Values})
	}
	return w
}

func severityOrDefault(s string) string {
	switch s {
	case "Info", "Warning", "Critical":
		return s
	}
	return "Warning"
}

func message(r Rule, res rulelang.Result) string {
	name := r.Name
	if name == "" {
		name = r.Source
	}
	return fmt.Sprintf("rule %q fired: %s", name, res.Reason)
}
