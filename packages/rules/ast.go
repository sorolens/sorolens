// Package rules implements the Sorolens alert-rule DSL: a small,
// PromQL-inspired expression language for describing when a contract should
// raise an alert. It is dependency-free so both the API (rule CRUD +
// validation) and the indexer (live evaluation inside the notifier loop) can
// import it without pulling in each other's internals.
//
// Grammar (keywords are case-insensitive):
//
//	expr     := [aggregation] metric comparison number [unit] [window] [scope]
//	aggregation := "avg" | "max" | "rate"
//	metric   := "fee" | "cpu" | "mem" | "ledger_bytes"
//	           | "events" | "invocations" | "failures"
//	comparison := ">" | ">=" | "<" | "<=" | "==" | "!="
//	unit     := "XLM" | "stroops" | "insn" | "bytes"
//	window   := "for" duration          // e.g. "for 5m", "for 1h30m"
//	scope    := "on" contract           // e.g. "on C..."; "*" or "all" = every contract
//
// A metric may be written with or without an aggregation:
//
//	avg(fee) > 0.5 XLM for 5m on CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC
//	rate(events) > 2 for 24h
//	fee > 0.5 XLM for 5m                 // bare fee == avg(fee)
//
// Bare count metrics default to rate (events per second) because a total
// count has no per-invocation average:
//
//	events > 10 for 5m                  // == rate(events) > 10
//
// The window defaults to 5m and "on" defaults to every tracked contract.
package rules

import "time"

// Aggregation is how samples in the window are combined.
type Aggregation string

const (
	// AggAvg is the mean of per-invocation values (or a ratio for failures).
	AggAvg Aggregation = "avg"
	// AggMax is the maximum per-invocation value (1.0 for failures if any).
	AggMax Aggregation = "max"
	// AggRate is the value per second (counts) or a ratio (failures).
	AggRate Aggregation = "rate"
)

// Metric names the quantity a rule measures.
type Metric string

const (
	// MetricFee is the total resource fee in stroops.
	MetricFee Metric = "fee"
	// MetricCPU is the total CPU instructions across invocations.
	MetricCPU Metric = "cpu"
	// MetricMem is the total memory bytes across invocations.
	MetricMem Metric = "mem"
	// MetricLedgerBytes is the total ledger read+write bytes across invocations.
	MetricLedgerBytes Metric = "ledger_bytes"
	// MetricEvents is the count of contract events in the window.
	MetricEvents Metric = "events"
	// MetricInvocations is the count of invocations in the window.
	MetricInvocations Metric = "invocations"
	// MetricFailures is the count of failed invocations in the window.
	MetricFailures Metric = "failures"
)

// Comparison is a threshold operator.
type Comparison string

const (
	CmpGt  Comparison = ">"
	CmpGte Comparison = ">="
	CmpLt  Comparison = "<"
	CmpLte Comparison = "<="
	CmpEq  Comparison = "=="
	CmpNeq Comparison = "!="
)

// Unit scales a threshold and the evaluated value. Only fee is convertible
// (stroops <-> XLM); the rest are unit-less counts/measurements.
type Unit string

const (
	// UnitXLM is 1 XLM = 10,000,000 stroops. Only valid for MetricFee.
	UnitXLM Unit = "XLM"
	// UnitStroops is raw stroops; only valid for MetricFee.
	UnitStroops Unit = "stroops"
	// UnitInsn is CPU instructions; only valid for MetricCPU.
	UnitInsn Unit = "insn"
	// UnitBytes is raw bytes; only valid for MetricMem / MetricLedgerBytes.
	UnitBytes Unit = "bytes"
)

// StroopsPerXLM is the fixed on-chain conversion: 1 XLM = 10^7 stroops.
const StroopsPerXLM = int64(10_000_000)

// DefaultWindow is the aggregation window when a rule omits the "for" clause.
const DefaultWindow = 5 * time.Minute

// Rule is a parsed alert-rule expression ready for evaluation.
type Rule struct {
	Aggregation Aggregation
	Metric      Metric
	Comparison  Comparison
	Threshold   float64
	Unit        Unit
	// Window is the aggregation lookback duration.
	Window time.Duration
	// Contract is the target contract ID, or "" to apply to every contract.
	Contract string
}