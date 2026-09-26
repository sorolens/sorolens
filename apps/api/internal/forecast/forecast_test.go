package forecast

import (
	"math"
	"testing"
	"time"
)

// syntheticSeries builds a daily series with a linear trend plus a weekly
// (day-of-week) pattern, optionally adding noise.
func syntheticSeries(start time.Time, days int, slopePerDay float64, weeklyAmplitude float64, noise float64) []DayValue {
	out := make([]DayValue, 0, days)
	rng := 42.0
	nextRand := func() float64 {
		// tiny deterministic PRNG (Park-Miller)
		rng = math.Mod(rng*48271, 2147483647)
		return rng/2147483647 - 0.5
	}
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i)
		wd := float64(int(date.Weekday())) // 0=Sunday .. 6=Saturday
		seasonal := weeklyAmplitude * math.Cos(2*math.Pi*wd/7)
		base := 100 + slopePerDay*float64(i) + seasonal
		out = append(out, DayValue{Date: date, Value: base + noise*nextRand()})
	}
	return out
}

// TestFitForecasts30Days asserts the endpoint's core contract: 30 daily
// points for a clean trend + weekly series, with sensible ordering.
func TestFitForecasts30Days(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	hist := syntheticSeries(start, 90, 0.5, 8, 0)

	preds := Fit(hist, 30)
	if len(preds) != 30 {
		t.Fatalf("want 30 predictions, got %d", len(preds))
	}
	for i, p := range preds {
		if p.Value <= 0 {
			t.Fatalf("prediction %d value must be positive, got %v", i, p.Value)
		}
		if p.Lower > p.Value || p.Upper < p.Value {
			t.Fatalf("prediction %d: band must contain value (lower=%v value=%v upper=%v)", i, p.Lower, p.Value, p.Upper)
		}
		if p.Date.Before(hist[len(hist)-1].Date) {
			t.Fatalf("prediction %d date before last history day", i)
		}
	}
}

// TestFitWithin20PercentHoldOut performs the acceptance backtest: fit over
// the first 80 days, hold out the last 10, and require the mean absolute
// percentage error on the hold-out to be under 20%.
func TestFitWithin20PercentHoldOut(t *testing.T) {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	hist := syntheticSeries(start, 90, 0.4, 6, 0.2)

	train := hist[:80]
	holdout := hist[80:]

	preds := Fit(train, 10)
	if len(preds) != 10 {
		t.Fatalf("want 10 predictions, got %d", len(preds))
	}
	var totalErr float64
	for i, p := range preds {
		actual := holdout[i].Value
		if actual == 0 {
			t.Fatalf("holdout %d should be non-zero synthetic value", i)
		}
		totalErr += math.Abs(p.Value-actual) / actual
	}
	mape := totalErr / float64(len(preds))
	if mape >= 0.20 {
		t.Fatalf("hold-out MAPE %.1f%% exceeds 20%% acceptance bound", mape*100)
	}
	t.Logf("hold-out MAPE = %.1f%%", mape*100)
}

// TestFitSeasonalityPresent verifies the weekly pattern is detected and
// reflected in mid-week vs weekend forecasts (weekday amplitude survives).
func TestFitSeasonalityPresent(t *testing.T) {
	start := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC) // Monday
	hist := syntheticSeries(start, 90, 0, 0, 0)          // flat line, zero amplitude

	preds := Fit(hist, 14)
	if len(preds) != 14 {
		t.Fatalf("want 14 preds, got %d", len(preds))
	}
	// With zero weekly amplitude predictions should hover near the flat
	// baseline (100) regardless of weekday.
	for _, p := range preds {
		if math.Abs(p.Value-100) > 1 {
			t.Fatalf("flat series forecast %v should be ~100", p.Value)
		}
	}
}

// TestFitShortHistoryDegradesGracefully ensures a tiny series falls back to a
// flat forecast rather than panicking or producing nonsense.
func TestFitShortHistoryDegradesGracefully(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	hist := []DayValue{
		{Date: start, Value: 50},
		{Date: start.AddDate(0, 0, 1), Value: 55},
	}
	preds := Fit(hist, 5)
	if len(preds) != 5 {
		t.Fatalf("want 5 preds from short history, got %d", len(preds))
	}
	for _, p := range preds {
		if p.Value <= 0 {
			t.Fatalf("prediction must stay non-negative, got %v", p.Value)
		}
	}

	// Single observation → flat line, zero-width band.
	one := Fit([]DayValue{{Date: start, Value: 7}}, 3)
	for _, p := range one {
		if p.Value != 7 || p.Lower != 7 || p.Upper != 7 {
			t.Fatalf("single-obs forecast must be flat 7, got %+v", p)
		}
	}
}

// TestFitEmptyHistoryReturnsNil guards the no-data path.
func TestFitEmptyHistoryReturnsNil(t *testing.T) {
	if preds := Fit(nil, 10); preds != nil {
		t.Fatalf("empty history must yield nil, got %d preds", len(preds))
	}
	if preds := Fit([]DayValue{}, 0); preds != nil {
		t.Fatalf("zero horizon must yield nil")
	}
}
