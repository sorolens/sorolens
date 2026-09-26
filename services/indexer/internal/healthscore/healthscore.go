// Package healthscore computes the composite 0-100 contract health score
// (issue #137).
//
// The score is a weighted linear combination of four inputs, each normalized
// so that 100 is "fully healthy" and 0 is "critically degraded":
//
//	-uptime       (40%) - watchdog uptime, from the healthy/total ratio of
//	  recent health checks.
//	-error rate   (25%) - share of recent invocations that did not succeed.
//	-performance  (20%) - CPU/fee cost per invocation trend: recent cost vs
//	  the trailing baseline.
//	-storage TTL  (15%) - share of live storage entries that still have
//	  healthy TTL headroom.
//
// The package is pure and dependency-free so the scoring rules can be unit
// tested in isolation from the store and network, mirroring internal/anomaly.
package healthscore

import "math"

// Activity is one hourly activity bucket for a contract.
type Activity struct {
	Invocations int64
	CPU         int64 // sum of cpu_insn
	Fees        int64 // sum of resource fees, stroops
}

// Inputs holds the raw signals the score is computed from.
type Inputs struct {
	// Uptime (watchdog).
	HealthyChecks int64 // healthy checks in the recent window
	TotalChecks   int64 // checks considered
	// WatchdogStatus is the contract's live watchdog status, used as a
	// fallback when there is no check history yet: Healthy | Degraded |
	// Unresponsive, or "" when the contract is not registered.
	WatchdogStatus string

	// Error rate over the trailing window.
	TotalInvocations  int64
	FailedInvocations int64

	// Performance trend, chronological buckets oldest first.
	Activity []Activity

	// Storage TTL headroom.
	TotalStorage    int64
	ExpiringStorage int64
}

// Score is the rounded 0-100 composite and its components.
type Score struct {
	Overall     int32
	Uptime      int32
	ErrorRate   int32
	Performance int32
	StorageTTL  int32
}

// Weight constants used by Compute. They must sum to 100.
const (
	WeightUptime     = 40
	WeightErrorRate  = 25
	WeightPerf       = 20
	WeightStorageTTL = 15
)

// Compute combines the four normalized inputs into a weighted 0-100 score.
func Compute(in Inputs) Score {
	up := uptimeScore(in)
	errR := errorRateScore(in)
	perf := performanceScore(in)
	store := storageTTLScore(in)

	overall := math.Round(
		float64(up)*WeightUptime+float64(errR)*WeightErrorRate+
			float64(perf)*WeightPerf+float64(store)*WeightStorageTTL,
	) / 100

	return Score{
		Overall:     clampQ(overall),
		Uptime:      up,
		ErrorRate:   errR,
		Performance: perf,
		StorageTTL:  store,
	}
}

// uptimeScore uses the healthy/total ratio of recent health checks. With no
// check history it falls back to the watchdog's live status. An unregistered
// contract has no evidence of downtime, so it is scored as healthy.
func uptimeScore(in Inputs) int32 {
	if in.TotalChecks > 0 {
		return ratioScore(in.HealthyChecks, in.TotalChecks)
	}
	switch in.WatchdogStatus {
	case "Degraded":
		return 50
	case "Unresponsive":
		return 10
	default:
		return 100
	}
}

// errorRateScore is 100 when nothing failed; otherwise the share that
// succeeded. A contract with no invocations has no failures to score.
func errorRateScore(in Inputs) int32 {
	if in.TotalInvocations == 0 {
		return 100
	}
	return ratioScore(in.TotalInvocations-in.FailedInvocations, in.TotalInvocations)
}

// performanceScore measures the cost per invocation (CPU + fees) of the most
// recent busy hour against the average of the earlier hours. A ratio at or
// under baseline scores 100 and degrades to 0 as the recent cost grows. With
// no comparable history the trend is unknown and scored neutrally.
func performanceScore(in Inputs) int32 {
	recent, baseline := costPerInvocation(in.Activity)
	if recent <= 0 && baseline <= 0 {
		return 100
	}
	if baseline <= 0 {
		// No baseline to compare against: anything non-trivial reads as
		// normal startup, so score neutrally.
		return 100
	}
	ratio := float64(recent) / float64(baseline)
	if ratio <= 1 {
		return 100
	}
	// Linear decay: 2x cost -> ~50, 4x -> ~25, ... floor at 0.
	return clampQ(100 * (2 - ratio))
}

// costPerInvocation returns the CPU+fee cost per invocation of the most recent
// busy hour and the average across all earlier busy hours.
func costPerInvocation(activity []Activity) (recent, baseline float64) {
	if len(activity) == 0 {
		return 0, 0
	}
	var baselineN, baselineCost float64
	for i := 0; i < len(activity)-1; i++ {
		a := activity[i]
		if a.Invocations <= 0 {
			continue
		}
		baselineN++
		baselineCost += float64(a.CPU+a.Fees) / float64(a.Invocations)
	}
	last := activity[len(activity)-1]
	if last.Invocations > 0 {
		recent = float64(last.CPU+last.Fees) / float64(last.Invocations)
	}
	if baselineN > 0 {
		baseline = baselineCost / baselineN
	}
	return recent, baseline
}

// storageTTLScore is the share of live entries with enough TTL headroom. No
// storage means nothing is about to expire.
func storageTTLScore(in Inputs) int32 {
	if in.TotalStorage == 0 {
		return 100
	}
	return ratioScore(in.TotalStorage-in.ExpiringStorage, in.TotalStorage)
}

// ratioScore returns 100*num/den clamped to [0,100].
func ratioScore(num, den int64) int32 {
	if den <= 0 {
		return 100
	}
	return clampQ(100 * float64(num) / float64(den))
}

// clampQ rounds q and clamps it to [0,100].
func clampQ(q float64) int32 {
	if q <= 0 {
		return 0
	}
	if q >= 100 {
		return 100
	}
	return int32(math.Round(q))
}
