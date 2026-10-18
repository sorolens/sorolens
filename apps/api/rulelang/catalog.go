package rulelang

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// Canonical units. A metric declares one, and a literal next to a unit word
// must be compatible with it.
const (
	UnitCount        = "count"
	UnitRatio        = "ratio" // 0..1; the `%` suffix is accepted and divided by 100
	UnitXLM          = "XLM"
	UnitStroops      = "stroops"
	UnitInstructions = "instructions"
	UnitBytes        = "bytes"
	UnitLedgers      = "ledgers"
	UnitScore        = "score" // 0..100
)

// Metric describes one signal the evaluator understands.
type Metric struct {
	Name        string
	Unit        string
	Description string
}

// catalog is the closed set of metrics a rule may reference. Keeping it closed
// is what lets validation produce "did you mean" hints and lets the dashboard
// render a picker.
var catalog = []Metric{
	{Name: "invocations", Unit: UnitCount, Description: "Transactions that invoked the contract in the window."},
	{Name: "events", Unit: UnitCount, Description: "Contract events emitted in the window."},
	{Name: "failed_invocations", Unit: UnitCount, Description: "Invocations whose transaction failed."},
	{Name: "error_rate", Unit: UnitRatio, Description: "Failed invocations divided by total invocations."},
	{Name: "uptime", Unit: UnitRatio, Description: "Fraction of health checks reporting Healthy."},
	{Name: "fee_per_invocation", Unit: UnitXLM, Description: "Average fee charged per invocation, in XLM."},
	{Name: "total_fee", Unit: UnitXLM, Description: "Total fee charged across the window, in XLM."},
	{Name: "fee_per_invocation_stroops", Unit: UnitStroops, Description: "Average fee charged per invocation, in stroops."},
	{Name: "cpu_insn_per_invocation", Unit: UnitInstructions, Description: "Average CPU instructions per invocation."},
	{Name: "cpu_insn_total", Unit: UnitInstructions, Description: "Total CPU instructions across the window."},
	{Name: "mem_byte_per_invocation", Unit: UnitBytes, Description: "Average memory bytes per invocation."},
	{Name: "ledger_read_bytes", Unit: UnitBytes, Description: "Ledger bytes read across the window."},
	{Name: "ledger_write_bytes", Unit: UnitBytes, Description: "Ledger bytes written across the window."},
	{Name: "storage_entries", Unit: UnitCount, Description: "Live storage entries for the contract."},
	{Name: "expiring_storage_entries", Unit: UnitCount, Description: "Storage entries expiring within seven days."},
	{Name: "min_storage_ttl_ledgers", Unit: UnitLedgers, Description: "Ledgers until the soonest storage entry expires."},
	{Name: "health_score", Unit: UnitScore, Description: "Composite 0-100 contract health score."},
}

var metricByName = func() map[string]Metric {
	m := make(map[string]Metric, len(catalog))
	for _, metric := range catalog {
		m[metric.Name] = metric
	}
	return m
}()

// Catalog returns every known metric, sorted by name.
func Catalog() []Metric {
	out := make([]Metric, len(catalog))
	copy(out, catalog)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Metrics returns the known metric names, sorted.
func Metrics() []string {
	out := make([]string, 0, len(catalog))
	for _, m := range catalog {
		out = append(out, m.Name)
	}
	sort.Strings(out)
	return out
}

// LookupMetric returns the metric with the given name.
func LookupMetric(name string) (Metric, bool) {
	m, ok := metricByName[strings.ToLower(name)]
	return m, ok
}

// AggregationFuncs is the closed set of aggregation functions.
var AggregationFuncs = []string{"avg", "max", "min", "sum", "rate", "count"}

// IsAggregation reports whether name is a known aggregation function.
func IsAggregation(name string) bool {
	for _, f := range AggregationFuncs {
		if strings.EqualFold(name, f) {
			return true
		}
	}
	return false
}

// unitAliases maps every spelling a user might write to its canonical unit.
var unitAliases = map[string]string{
	"xlm":          UnitXLM,
	"stroop":       UnitStroops,
	"stroops":      UnitStroops,
	"instruction":  UnitInstructions,
	"instructions": UnitInstructions,
	"byte":         UnitBytes,
	"bytes":        UnitBytes,
	"b":            UnitBytes,
	"ledger":       UnitLedgers,
	"ledgers":      UnitLedgers,
	"%":            UnitRatio,
	"percent":      UnitRatio,
}

// canonicalUnit normalizes a unit word. It returns ok=false for a word that is
// not a unit at all (which usually means the user wrote a stray identifier).
func canonicalUnit(word string) (string, bool) {
	u, ok := unitAliases[strings.ToLower(word)]
	return u, ok
}

// unitCompatible reports whether a literal unit may be compared against a
// metric of the given unit. Dimensionless metrics accept any count/score
// literal, and ratios additionally accept percentages.
func unitCompatible(literal, metricUnit string) bool {
	if literal == "" || metricUnit == "" {
		return true
	}
	switch metricUnit {
	case UnitRatio:
		return literal == UnitRatio
	case UnitCount, UnitScore:
		return literal == UnitCount
	default:
		return literal == metricUnit
	}
}

// formatDuration renders a duration in the compact rule syntax.
func formatDuration(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0:
		return strconv.Itoa(int(d/(24*time.Hour))) + "d"
	case d%time.Hour == 0:
		return strconv.Itoa(int(d/time.Hour)) + "h"
	case d%time.Minute == 0:
		return strconv.Itoa(int(d/time.Minute)) + "m"
	default:
		return strconv.Itoa(int(d/time.Second)) + "s"
	}
}

// trimFloat renders a float without trailing zeros.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
