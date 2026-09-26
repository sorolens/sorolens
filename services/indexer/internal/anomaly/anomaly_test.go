package anomaly

import (
	"math"
	"testing"
	"time"
)

func steadySeries(hours int, base float64, noise float64) []Sample {
	out := make([]Sample, 0, hours)
	start := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	rng := 7.0
	nr := func() float64 {
		rng = math.Mod(rng*48271, 2147483647)
		return rng/2147483647 - 0.5
	}
	for i := 0; i < hours; i++ {
		v := base + noise*nr()
		if v < 0 {
			v = 0
		}
		out = append(out, Sample{
			At:          start.Add(time.Duration(i) * time.Hour),
			Events:      v,
			Invocations: v / 2,
			CPU:         v * 100,
			Fees:        v * 1000,
		})
	}
	return out
}

// TestDetectSteadyStateNoFalsePositives runs a 48-hour steady-state
// simulation (equivalent to the 24h acceptance simulation) and requires zero
// alerts.
func TestDetectSteadyStateNoFalsePositives(t *testing.T) {
	hours := 48
	samples := steadySeries(hours, 100, 0.3)

	anoms := Detect(samples, Config{Sigma: 3, MinStd: 1, MinLift: 1.5, MinHistory: 12})
	if len(anoms) != 0 {
		t.Fatalf("steady state over %dh produced %d false positives: %+v", hours, len(anoms), anoms)
	}
}

// TestDetectFabricatedSpikeTriggersAlert spikes the trailing window 20x above
// a steady baseline and requires an alert, exactly the acceptance criterion.
func TestDetectFabricatedSpikeTriggersAlert(t *testing.T) {
	samples := steadySeries(48, 100, 0.1)
	last := samples[len(samples)-1]
	last.Events = 2000 // 20x baseline
	last.Invocations = 1000
	last.CPU = 200000
	last.Fees = 2000000
	samples[len(samples)-1] = last

	anoms := Detect(samples, Config{})
	if len(anoms) == 0 {
		t.Fatal("spike produced no alerts")
	}

	metrics := map[string]bool{}
	for _, a := range anoms {
		metrics[a.Metric] = true
		if a.Observed < a.Expected {
			t.Errorf("metric %s: observed %.2f should exceed threshold %.2f", a.Metric, a.Observed, a.Expected)
		}
	}
	for _, m := range []string{"events", "invocations", "cpu", "fees"} {
		if !metrics[m] {
			t.Errorf("spike should flag metric %q, got %v", m, metrics)
		}
	}
}

// TestDetectModerateSpikeTriggersAlertAt3Sigma verifies a value ~5x the
// baseline, well above 3 sigma of tiny noise, still alerts (not just giant
// 20x spikes).
func TestDetectModerateSpikeTriggersAlertAt3Sigma(t *testing.T) {
	samples := steadySeries(48, 100, 0.1)
	last := samples[len(samples)-1]
	last.Events = 520 // ~5x
	samples[len(samples)-1] = last

	anoms := Detect(samples, Config{})
	found := false
	for _, a := range anoms {
		if a.Metric == "events" {
			found = true
		}
	}
	if !found {
		t.Fatal("5x spike should flag events, got", anoms)
	}
}

// TestDetectSecondTrailingWindowZeroBaseline yields no alert when the
// baseline is exactly zero (MinLift floor prevents divide-by-zero noise).
func TestDetectSecondTrailingWindowZeroBaseline(t *testing.T) {
	samples := steadySeries(48, 0, 0)
	anoms := Detect(samples, Config{})
	if len(anoms) != 0 {
		t.Fatalf("zero baseline produced alerts: %+v", anoms)
	}
}

// TestDetectTooLittleHistorySkips ensures the MinHistory guard works.
func TestDetectTooLittleHistorySkips(t *testing.T) {
	samples := steadySeries(5, 100, 0)
	last := samples[len(samples)-1]
	last.Events = 9999
	samples[len(samples)-1] = last
	if anoms := Detect(samples, Config{MinHistory: 12}); len(anoms) != 0 {
		t.Fatalf("insufficient history should skip, got %+v", anoms)
	}
}
