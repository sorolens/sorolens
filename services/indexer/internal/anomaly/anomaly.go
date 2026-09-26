// Package anomaly implements rolling per-contract anomaly detection for the
// indexer (issue #136). It is pure and dependency-free: it computes baseline
// statistics over a recent window of hourly samples and flags the trailing
// window when any metric (event count, invocation count, CPU, fees) exceeds
// its mean by sigma standard deviations.
package anomaly

import (
	"fmt"
	"math"
	"time"
)

// Sample is one hourly bucket of contract activity.
type Sample struct {
	At          time.Time
	Events      float64
	Invocations float64
	CPU         float64 // sum of cpu_insn across invocations
	Fees        float64 // sum of resource fees, stroops
}

// DefaultLookbackHours is the default rolling baseline window (7 days).
const DefaultLookbackHours = 7 * 24

// Config controls the detection thresholds.
type Config struct {
	// Sigma is the number of standard deviations above the mean that counts
	// as an anomaly. Defaults to 3.
	Sigma float64
	// MinStd is the floor applied to the standard deviation so perfectly
	// steady series do not produce infinite sensitivity. Defaults to 1.
	MinStd float64
	// MinLift is the multiple of the baseline mean a value must exceed in
	// addition to the sigma rule (guards against false positives on small
	// absolute numbers). Defaults to 1.5.
	MinLift float64
	// MinHistory is the minimum number of samples needed to establish a
	// baseline. Fewer samples yield no alerts. Defaults to 12.
	MinHistory int
}

// defaults returns c with unset fields filled in.
func (c Config) defaults() Config {
	if c.Sigma <= 0 {
		c.Sigma = 3
	}
	if c.MinStd <= 0 {
		c.MinStd = 1
	}
	if c.MinLift <= 0 {
		c.MinLift = 1.5
	}
	if c.MinHistory <= 0 {
		c.MinHistory = 12
	}
	return c
}

// Anomaly describes one flagged metric.
type Anomaly struct {
	Metric  string
	Message string
	At      time.Time
	// Observed is the trailing value that triggered the alert.
	Observed float64
	// Expected is the sigma-adjusted threshold (mean + sigma*std).
	Expected float64
}

// metricSpec wires one Sample field to its metric name.
type metricSpec struct {
	name string
	val  func(*Sample) float64
}

var metrics = []metricSpec{
	{"events", func(s *Sample) float64 { return s.Events }},
	{"invocations", func(s *Sample) float64 { return s.Invocations }},
	{"cpu", func(s *Sample) float64 { return s.CPU }},
	{"fees", func(s *Sample) float64 { return s.Fees }},
}

// Detect compares the trailing sample against the baseline statistics of the
// earlier samples (rolling window). Every metric that violates the threshold
// is reported; a single tick may yield multiple anomalies.
func Detect(samples []Sample, cfg Config) []Anomaly {
	cfg = cfg.defaults()
	if len(samples) < cfg.MinHistory+1 {
		return nil
	}

	history := samples[:len(samples)-1]
	trailing := samples[len(samples)-1]

	var out []Anomaly
	for _, m := range metrics {
		mean, std := rollingStats(history, m.val)
		threshold := mean + cfg.Sigma*math.Max(std, cfg.MinStd)
		observed := m.val(&trailing)
		liftFloor := mean * cfg.MinLift
		if observed > threshold && observed > liftFloor {
			out = append(out, Anomaly{
				Metric:   m.name,
				At:       trailing.At,
				Observed: observed,
				Expected: threshold,
				Message:  fmt.Sprintf("%s spike: trailing %s window value %.2f exceeds baseline mean %.2f + %.1fσ (%.2f)", m.name, trailing.At.Format("15:04"), observed, mean, cfg.Sigma, threshold),
			})
		}
	}
	return out
}

// rollingStats returns the mean and population standard deviation of a metric
// across history. Constant series yield std 0.
func rollingStats(history []Sample, pick func(*Sample) float64) (mean, std float64) {
	n := float64(len(history))
	if n == 0 {
		return 0, 0
	}
	var sum float64
	for i := range history {
		sum += pick(&history[i])
	}
	mean = sum / n
	var sq float64
	for i := range history {
		d := pick(&history[i]) - mean
		sq += d * d
	}
	std = math.Sqrt(sq / n)
	return mean, std
}
