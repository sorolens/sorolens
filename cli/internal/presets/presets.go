package presets

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PresetType identifies a contract category. New types can be added by
// contributors without changing the core logic: the CLI and API only ever
// pass the type name through to the indexer, which matches it against the
// registered preset.
type PresetType string

const (
	// PresetDEX identifies automated market maker / DEX contracts (e.g. Stellar
	// Term realise, Vortex, and other AMM flavours).
	PresetDEX PresetType = "dex"

	// PresetLending identifies lending / borrowing protocol contracts.
	PresetLending PresetType = "lending"

	// PresetStablecoin identifies stablecoin issuer / peg-maintenance contracts.
	PresetStablecoin PresetType = "stablecoin"
)

// ContractPreset is the documented format for a contract-type preset. It is
// serialised to JSON and embedded in the API contract response so that the
// dashboard and CLI can render type-specific filters, function-name chips, and
// labels without hardcoding contract knowledge.
//
// The format is intentionally flat and stable: adding a new preset is a matter
// of registering another entry in BuiltInPresets (or, longer term, a
// user-defined preset store). Contributors add more by appending to that map
// and running the existing test suite.
type ContractPreset struct {
	// Type is the machine-readable preset identifier used in the CLI --preset
	// flag and the API contract.tracking_preset field. It is also the key into
	// BuiltInPresets.
	Type PresetType `json:"type"`

	// Label is a human-readable name for the preset, shown in the dashboard
	// preset selector and in CLI help text.
	Label string `json:"label"`

	// Description is a short paragraph describing what this preset tracks and
	// why a user would pick it. Used in CLI --help and the dashboard tooltip.
	Description string `json:"description"`

	// EventFilters is a list of event-type filters that the indexer should
	// prioritise for this contract class. Each entry is a free-form string that
	// matches the event type column in the events table. An empty list means
	// "track all events"; non-empty lists are used by the dashboard to build
	// the event-type filter chips and by the indexer as a hint for backfill
	// ordering.
	EventFilters []string `json:"event_filters"`

	// FunctionNames is a curated list of function names that this preset cares
	// about. The dashboard uses it to surface function-name chips on the
	// invocations tab and to highlight named-function invocations in the feed.
	// The indexer uses it as a hint for RPC calls that need function-name
	// resolution via diagnostic events.
	FunctionNames []string `json:"function_names"`

	// Labels is a list of suggested contract labels that the user can apply when
	// tracking. The dashboard pre-fills the label dropdown from this list; the
	// CLI surfaces them in --help. These are suggestions only — the user can
	// type any label.
	Labels []string `json:"labels"`
}

// BuiltInPresets is the registry of shipped presets. It is the single source
// of truth for the preset format; the tests in presets_test.go iterate over it
// to validate every entry.
var BuiltInPresets = map[PresetType]ContractPreset{
	PresetDEX: {
		Type:        PresetDEX,
		Label:       "DEX / AMM",
		Description: "Automated market maker or decentralised exchange contract. Tracks swap, liquidity add/remove, fee, and pool-state events.",
		EventFilters: []string{
			"swap",
			"liquidity_added",
			"liquidity_removed",
			"fee_collected",
			"pool_update",
		},
		FunctionNames: []string{
			"swap",
			"add_liquidity",
			"remove_liquidity",
			"set_fee",
			"reebalance",
		},
		Labels: []string{
			"dex",
			"amm",
			"liquidity",
		},
	},
	PresetLending: {
		Type:        PresetLending,
		Label:       "Lending / Borrowing",
		Description: "Lending or borrowing pool contract. Tracks deposit, withdraw, borrow, repay, liquidation, and interest-accumulation events.",
		EventFilters: []string{
			"deposit",
			"withdraw",
			"borrow",
			"repay",
			"liquidation",
			"accrue_interest",
			"liquidation_call",
		},
		FunctionNames: []string{
			"deposit",
			"withdraw",
			"borrow",
			"repay",
			"liquidate",
			"liquidate_account",
			"accrue_interest",
		},
		Labels: []string{
			"lending",
			"borrowing",
			"credit",
		},
	},
	PresetStablecoin: {
		Type:        PresetStablecoin,
		Label:       "Stablecoin",
		Description: "Stablecoin issuer or peg-maintenance contract. Tracks mint, burn, peg-adjust, and reserve-report events.",
		EventFilters: []string{
			"mint",
			"burn",
			"peg_update",
			"reserve_report",
			"redeem",
		},
		FunctionNames: []string{
			"mint",
			"burn",
			"redeem",
			"set_peg",
			"report_reserves",
			"adjust_collateral_ratio",
		},
		Labels: []string{
			"stablecoin",
			"peg",
			"reserve",
		},
	},
}

// Validate checks that a preset is internally consistent: non-empty type and
// label, and no duplicate event filters or function names. It is used by the
// tests to guard the built-in registry but is also available to any future
// user-defined preset store.
func (p ContractPreset) Validate() error {
	if strings.TrimSpace(string(p.Type)) == "" {
		return fmt.Errorf("preset type must be non-empty")
	}
	if strings.TrimSpace(p.Label) == "" {
		return fmt.Errorf("preset label must be non-empty")
	}
	if err := checkNoDuplicates(p.EventFilters, "event_filters"); err != nil {
		return err
	}
	if err := checkNoDuplicates(p.FunctionNames, "function_names"); err != nil {
		return err
	}
	return nil
}

func checkNoDuplicates(list []string, fieldName string) error {
	seen := make(map[string]struct{}, len(list))
	for _, item := range list {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("%s entries must be non-empty", fieldName)
		}
		if _, dup := seen[item]; dup {
			return fmt.Errorf("%s contains duplicate entry %q", fieldName, item)
		}
		seen[item] = struct{}{}
	}
	return nil
}

// MarshalJSON implements json.Marshaler so that the registry serialises as an
// object keyed by preset type, matching the shape the dashboard expects when it
// fetches /api/v1/contracts/{id}/presets.
func (p ContractPreset) MarshalJSON() ([]byte, error) {
	type Alias ContractPreset
	return json.Marshal(struct {
		Alias
		Type string `json:"type"`
	}{
		Alias: Alias(p),
		Type:  string(p.Type),
	})
}

// PresetsByName returns a sorted list of all built-in presets by label, useful
// for the CLI --help output and the dashboard preset selector.
func PresetsByName() []ContractPreset {
	out := make([]ContractPreset, 0, len(BuiltInPresets))
	for _, p := range BuiltInPresets {
		out = append(out, p)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i].Label > out[j].Label {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Lookup returns the built-in preset for a given type, or an error when the
// type is not registered. The API and CLI use this after parsing a user-supplied
// --preset value.
func Lookup(t PresetType) (ContractPreset, error) {
	p, ok := BuiltInPresets[t]
	if !ok {
		return ContractPreset{}, fmt.Errorf("unknown preset type %q", t)
	}
	return p, nil
}
