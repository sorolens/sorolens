// Package forecast fits a simple additive forecast model (linear trend +
// day-of-week seasonality) to daily time series and produces forward
// predictions with a confidence band. It is intentionally dependency-free and
// statistical rather than Machine-Learning based, matching issue #135.
package forecast

import (
	"math"
	"time"
)

// DayValue is one daily observation.
type DayValue struct {
	Date  time.Time
	Value float64
}

// Prediction is one forecast day.
type Prediction struct {
	Date  time.Time
	Value float64
	// Lower and Upper bound the value at roughly 95% confidence.
	Lower float64
	Upper float64
}

// Fit the additive model (linear trend + 7-bin weekday seasonality) to the
// history and forecast horizon days forward from the last observed day.
//
// Graceful degradation:
//   - < 2 observations: forecast is a flat line anchored to the last value
//     with zero-width band (no residual information).
//   - < 7 observations or a single unique weekday: weekdays collapse to a
//     plain linear fit (seasonality is dropped) and the band widens slightly.
func Fit(history []DayValue, horizon int) []Prediction {
	if horizon <= 0 {
		return nil
	}
	if len(history) == 0 {
		return nil
	}
	if len(history) == 1 {
		v := history[0].Value
		out := make([]Prediction, 0, horizon)
		for i := 1; i <= horizon; i++ {
			out = append(out, Prediction{
				Date:  history[0].Date.AddDate(0, 0, i),
				Value: v, Lower: v, Upper: v,
			})
		}
		return out
	}

	n := len(history)
	// Least squares fit of y = a + b*x over the observation index.
	meanX := float64(n-1) / 2
	var meanY float64
	for _, h := range history {
		meanY += h.Value
	}
	meanY /= float64(n)

	var ssxx, ssxy float64
	for i, h := range history {
		dx := float64(i) - meanX
		ssxx += dx * dx
		ssxy += dx * (h.Value - meanY)
	}
	// With fewer than 2 distinct x positions ssxx could be 0; fall back to a
	// flat model in that impossible case.
	slope := 0.0
	if ssxx > 0 {
		slope = ssxy / ssxx
	}
	intercept := meanY - slope*meanX

	trendAt := func(x float64) float64 { return intercept + slope*x }

	// Weekday seasonal offsets: residual of (obs - trend), averaged per weekday.
	var wdSum [7]float64
	var wdCount [7]int
	var residualSumSq float64
	for i, h := range history {
		r := h.Value - trendAt(float64(i))
		wd := int(h.Date.Weekday())
		if wd < 0 || wd > 6 {
			wd = 0
		}
		wdSum[wd] += r
		wdCount[wd]++
	}
	var seasonal [7]float64
	distinctWeekdays := 0
	for wd := 0; wd < 7; wd++ {
		if wdCount[wd] > 0 {
			seasonal[wd] = wdSum[wd] / float64(wdCount[wd])
			distinctWeekdays++
		}
	}
	useSeasonality := distinctWeekdays >= 7

	predict := func(i int, date time.Time) float64 {
		if useSeasonality {
			return trendAt(float64(i)) + seasonal[int(date.Weekday())]
		}
		return trendAt(float64(i))
	}

	// Standard deviation of forecast residuals for the confidence band.
	for i, h := range history {
		r := h.Value - predict(i, h.Date)
		residualSumSq += r * r
	}
	std := math.Sqrt(residualSumSq / float64(n))
	z := 1.96 // 95% band
	if !useSeasonality {
		// Less data to characterise scatter: widen the band a touch.
		z = 2.2
	}

	out := make([]Prediction, 0, horizon)
	lastDate := history[n-1].Date
	for i := 1; i <= horizon; i++ {
		date := lastDate.AddDate(0, 0, i)
		v := predict(n-1+i, date)
		if v < 0 {
			v = 0
		}
		lower := v - z*std
		upper := v + z*std
		if lower < 0 {
			lower = 0
		}
		out = append(out, Prediction{Date: date, Value: v, Lower: lower, Upper: upper})
	}
	return out
}
