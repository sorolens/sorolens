package rulelang

// LibraryRule is one curated example rule. The library gives new users
// something that works out of the box and doubles as documentation of the
// grammar.
type LibraryRule struct {
	// Name is a short identifier shown as the card title.
	Name string `json:"name"`
	// Description explains what the rule watches for and why it matters.
	Description string `json:"description"`
	// Severity is the alert severity the rule should raise.
	Severity string `json:"severity"`
	// Source is the rule text, valid per Validate.
	Source string `json:"source"`
}

// Library returns the built-in sample rules. Every source in the library is
// guaranteed to parse and validate; a test asserts it.
func Library() []LibraryRule {
	return []LibraryRule{
		{
			Name:        "Failed invocations spike",
			Description: "More than 5% of invocations failed for a sustained 15 minutes.",
			Severity:    "Warning",
			Source:      "error_rate > 5% for 15m",
		},
		{
			Name:        "Expensive invocation",
			Description: "The average invocation cost more than 0.5 XLM over 10 minutes.",
			Severity:    "Warning",
			Source:      "fee_per_invocation > 0.5 XLM for 10m",
		},
		{
			Name:        "Storage about to expire",
			Description: "Some storage entry is within 24 hours (17,280 ledgers) of expiry.",
			Severity:    "Critical",
			Source:      "min_storage_ttl_ledgers < 17280 ledgers for 5m",
		},
		{
			Name:        "CPU regression",
			Description: "Average CPU instructions per invocation crossed 5M for 30 minutes.",
			Severity:    "Warning",
			Source:      "avg(cpu_insn_per_invocation) > 5000000 instructions for 30m",
		},
		{
			Name:        "Contract unhealthy",
			Description: "The composite health score stayed below 80 for an hour.",
			Severity:    "Critical",
			Source:      "health_score < 80 for 1h",
		},
		{
			Name:        "Event throughput surge",
			Description: "The contract emitted more than 10 events per second for 5 minutes.",
			Severity:    "Info",
			Source:      "rate(events) > 10 for 5m",
		},
		{
			Name:        "Uptime dip",
			Description: "Fewer than 99% of watchdog health checks reported Healthy.",
			Severity:    "Critical",
			Source:      "uptime < 0.99 for 30m",
		},
		{
			Name:        "Storage growth",
			Description: "Peak live storage entries exceeded 10,000 over the last hour.",
			Severity:    "Warning",
			Source:      "max(storage_entries) > 10000 for 1h",
		},
		{
			Name:        "Quiet contract",
			Description: "A contract that normally runs every minute went quiet for 15 minutes.",
			Severity:    "Warning",
			Source:      "sum(invocations) == 0 for 15m on network testnet",
		},
	}
}
