package rules

// Sample describes one curated entry in the alert-rule library. Samples are
// shown in the dashboard editor so users can jump-start a rule, and are served
// by GET /api/v1/rules/samples.
type Sample struct {
	// Name is a short human-readable label.
	Name string `json:"name"`
	// Description explains what the sample watches for.
	Description string `json:"description"`
	// Expression is valid DSL that can be piped straight into POST /rules.
	Expression string `json:"expression"`
}

// Samples is the curated alert-rule library. Every expression must parse so
// new samples can be tested by an extra case in the package tests.
var Samples = []Sample{
	{
		Name:        "Fee spike",
		Description: "Alert when the average fee per invocation over the last 5 minutes exceeds half a lumen.",
		Expression:  "avg(fee) > 0.5 XLM for 5m",
	},
	{
		Name:        "Event flood",
		Description: "Alert when a contract emits more than 100 events per second over an hour.",
		Expression:  "rate(events) > 100 for 1h",
	},
	{
		Name:        "CPU exhaustion",
		Description: "Alert when any single invocation burns more than 200 million CPU instructions in the last 24h.",
		Expression:  "max(cpu) > 200000000 for 24h",
	},
	{
		Name:        "Elevated failure rate",
		Description: "Alert when more than 5% of invocations fail over 5 minutes.",
		Expression:  "rate(failures) > 0.05 for 5m",
	},
	{
		Name:        "Heavy storage reader",
		Description: "Alert when the average ledger I/O per invocation exceeds 1 MiB over the last hour.",
		Expression:  "avg(ledger_bytes) > 1048576 for 1h",
	},
	{
		Name:        "Single-contract fee budget",
		Description: "Alert when a specific contract's average fee per invocation passes 0.1 XLM over the last 30 minutes.",
		Expression:  "avg(fee) > 0.1 XLM for 30m on CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	},
}