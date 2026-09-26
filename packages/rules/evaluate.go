package rules

import (
	"fmt"
	"time"
)

// InvocationSample is one invocation row in the evaluation window. Status is
// the invocation status ("SUCCESS" or anything else = failed). Metrics mirror
// the `invocations` table: fees in stroops, CPU in instructions, memory and
// ledger I/O in bytes.
type InvocationSample struct {
	Status      string
	Timestamp   time.Time
	FeeStroops  int64
	CPUInsn     int64
	MemBytes    int64
	LedgerBytes int64
}

// WindowStats describes contract activity over a rule's time window. Invocations
// carries every invocation row; Events carries the total contract-event count.
type WindowStats struct {
	// Duration is the wall-clock length of the window. It is used to derive
	// rates. When zero, rates fall back to counting-based semantics.
	Duration    time.Duration
	Invocations []InvocationSample
	Events      int64
}

// Result is the outcome of evaluating one rule against one window of data.
type Result struct {
	// Value is the computed aggregate scaled to the rule's unit.
	Value float64
	// Fired is true when the comparison holds AND the window had data.
	Fired bool
	// NoData is true when the window contained no invocations and no events.
	// Rules never fire on empty windows so a dead contract does not trigger
	// every ">= 0" alert.
	NoData bool
	// Samples is the number of invocations in the window.
	Samples int
	// Description is a human-readable rendering of the comparison outcome.
	Description string
}

// Evaluate computes a rule against window data. Empty windows always yield
// fired=false (see Result.NoData).
func Evaluate(r Rule, stats WindowStats) Result {
	n := int64(len(stats.Invocations))
	samples := int(n)

	var feeTotal, cpuTotal, memTotal, lbTotal int64
	var failures int64
	var maxFee, maxCPU, maxMem, maxLB int64
	for _, inv := range stats.Invocations {
		feeTotal += inv.FeeStroops
		cpuTotal += inv.CPUInsn
		memTotal += inv.MemBytes
		lbTotal += inv.LedgerBytes
		if inv.FeeStroops > maxFee {
			maxFee = inv.FeeStroops
		}
		if inv.CPUInsn > maxCPU {
			maxCPU = inv.CPUInsn
		}
		if inv.MemBytes > maxMem {
			maxMem = inv.MemBytes
		}
		if inv.LedgerBytes > maxLB {
			maxLB = inv.LedgerBytes
		}
		if inv.Status != "" && inv.Status != "SUCCESS" && inv.Status != "success" {
			failures++
		}
	}

	noData := n == 0 && stats.Events == 0

	windowSecs := stats.Duration.Seconds()
	if windowSecs <= 0 {
		windowSecs = 1
	}

	var raw float64
	switch r.Metric {
	case MetricFee:
		raw = aggregateValue(r.Aggregation, float64(feeTotal), float64(maxFee), n, windowSecs)
	case MetricCPU:
		raw = aggregateValue(r.Aggregation, float64(cpuTotal), float64(maxCPU), n, windowSecs)
	case MetricMem:
		raw = aggregateValue(r.Aggregation, float64(memTotal), float64(maxMem), n, windowSecs)
	case MetricLedgerBytes:
		raw = aggregateValue(r.Aggregation, float64(lbTotal), float64(maxLB), n, windowSecs)
	case MetricEvents:
		raw = aggregateValue(r.Aggregation, float64(stats.Events), 0, n, windowSecs)
	case MetricInvocations:
		raw = aggregateValue(r.Aggregation, float64(n), 0, n, windowSecs)
	case MetricFailures:
		if n == 0 {
			raw = 0
		} else {
			switch r.Aggregation {
			case AggMax:
				if failures > 0 {
					raw = 1
				} else {
					raw = 0
				}
			default:
				raw = float64(failures) / float64(n)
			}
		}
	}

	value := scaleToUnit(raw, r.Metric, r.Unit)
	threshold := r.Threshold
	if r.Unit == UnitXLM {
		threshold = r.Threshold // threshold already in XLM
	}

	fired := compare(value, threshold, r.Comparison) && !noData

	return Result{
		Value:       value,
		Fired:       fired,
		NoData:      noData,
		Samples:     samples,
		Description: describe(r, value, noData),
	}
}

// aggregateValue applies an aggregation to per-window totals.
func aggregateValue(agg Aggregation, total, maxVal float64, n int64, windowSecs float64) float64 {
	switch agg {
	case AggMax:
		return maxVal
	case AggRate:
		return total / windowSecs
	default: // AggAvg
		if n == 0 {
			return 0
		}
		return total / float64(n)
	}
}

// scaleToUnit converts a raw aggregate into the rule's display unit.
func scaleToUnit(raw float64, m Metric, u Unit) float64 {
	if m == MetricFee && u == UnitXLM {
		return raw / float64(StroopsPerXLM)
	}
	return raw
}

func compare(value, threshold float64, cmp Comparison) bool {
	switch cmp {
	case CmpGt:
		return value > threshold
	case CmpGte:
		return value >= threshold
	case CmpLt:
		return value < threshold
	case CmpLte:
		return value <= threshold
	case CmpEq:
		return value == threshold
	case CmpNeq:
		return value != threshold
	default:
		return false
	}
}

func describe(r Rule, value float64, noData bool) string {
	agg := string(r.Aggregation)
	metric := string(r.Metric)
	unit := r.Unit
	if unit == "" {
		unit = defaultUnit(r.Metric)
	}
	window := formatWindow(r.Window)
	if noData {
		return fmt.Sprintf("no data for %s over %s (0 invocations, 0 events)", metric, window)
	}
	return fmt.Sprintf("%s(%s) = %s %s %s %s over %s", agg, metric,
		formatValue(value, string(unit)), r.Comparison, formatValue(r.Threshold, string(unit)), unit, window)
}

func formatWindow(d time.Duration) string {
	if days := d / (24 * time.Hour); days > 0 && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", days)
	}
	if hours := d / time.Hour; hours > 0 && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if minutes := d / time.Minute; minutes > 0 && d%time.Minute == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if seconds := d / time.Second; seconds > 0 && d%time.Second == 0 {
		return fmt.Sprintf("%ds", seconds)
	}
	return d.String()
}

func formatValue(v float64, unit string) string {
	digits := 6
	if unit == string(UnitStroops) || unit == string(UnitInsn) || unit == string(UnitBytes) {
		digits = 0
	}
	return fmt.Sprintf("%.*f", digits, v)
}

// String renders the canonical, normalized form of a rule. It round-trips:
// ParseExpression(r.String()) equals r.
func (r Rule) String() string {
	var b []byte
	b = append(b, string(r.Aggregation)...)
	b = append(b, '(')
	b = append(b, string(r.Metric)...)
	b = append(b, ')')
	b = append(b, ' ')
	b = append(b, string(r.Comparison)...)
	b = append(b, ' ')
	b = append(b, formatValue(r.Threshold, string(r.Unit))...)
	if r.Unit != "" && (r.Unit == UnitXLM || r.Unit != defaultUnit(r.Metric)) {
		b = append(b, ' ')
		b = append(b, string(r.Unit)...)
	}
	if r.Window != 0 && r.Window != DefaultWindow {
		b = append(b, " for "...)
		b = append(b, formatWindow(r.Window)...)
	}
	if r.Contract != "" {
		b = append(b, " on "...)
		b = append(b, r.Contract...)
	}
	return string(b)
}