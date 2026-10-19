package rulelang

import (
	"fmt"
	"math"
	"time"
)

// Sample is one bucket of metrics. The indexer produces one sample per minute
// for the window a rule asks about; a metric that was not reported in that
// bucket is simply absent from Values.
type Sample struct {
	At     time.Time
	Values map[string]float64
}

// Value returns the metric value and whether it was present in the bucket.
func (s Sample) Value(metric string) (float64, bool) {
	v, ok := s.Values[metric]
	return v, ok
}

// Window is the data a rule is evaluated against.
type Window struct {
	// From and To bound the window. To is normally "now".
	From time.Time
	To   time.Time
	// Samples are ordered oldest first.
	Samples []Sample
}

// Point is one sample's contribution to a preview: the value of the rule's
// left-hand side in that bucket, and whether the comparison held.
type Point struct {
	At       time.Time
	Value    float64
	HasValue bool
	Fired    bool
}

// Result is the outcome of evaluating one rule.
type Result struct {
	// Fired is true when the comparison held for the whole evaluation window.
	Fired bool
	// Value is the observed left-hand-side value; HasValue is false when the
	// metric was not reported in the window.
	Value    float64
	HasValue bool
	// Threshold is the observed right-hand-side value.
	Threshold float64
	// Op is the comparison operator that was applied.
	Op string
	// Points is the per-sample series behind the verdict, for the dashboard
	// preview and sparkline. It is empty when the rule has no window.
	Points []Point
	// Reason is a short human-readable explanation of the verdict, suitable
	// for an alert message.
	Reason string
}

// EvaluateSource parses, validates, and evaluates a rule in one call.
func EvaluateSource(source string, w Window) (Result, *Rule, error) {
	r, err := Validate(source)
	if err != nil {
		return Result{}, nil, err
	}
	res, err := Evaluate(r, w)
	return res, r, err
}

// Evaluate runs a validated rule against a window.
func Evaluate(rule *Rule, w Window) (Result, error) {
	cmp, ok := rule.Expr.(*Comparison)
	if !ok {
		if g, isGroup := rule.Expr.(*Grouping); isGroup {
			cmp, ok = g.Inner.(*Comparison)
		}
	}
	if !ok || cmp == nil {
		return Result{}, Errorf(rule.Source, rule.Expr.Position(), "a rule must contain a comparison", "rule has no comparison to evaluate")
	}

	selected := selectSamples(w, rule.For)
	span := windowSpan(selected, rule.For)

	// A bare metric under a `for` window is a sustained condition: every
	// bucket in the window must satisfy the comparison, which is how the
	// acceptance example (`fee_per_invocation > 0.5 XLM for 5m`) reads. An
	// aggregation, by contrast, reduces the whole window to one number and
	// applies a single comparison to it.
	if rule.For > 0 && !containsAggregation(cmp.Left) && !containsAggregation(cmp.Right) {
		return evaluateSustained(cmp, selected, rule.For), nil
	}

	lhs, lOK, lReason := resolve(cmp.Left, selected, span)
	rhs, rOK, _ := resolve(cmp.Right, selected, span)

	res := Result{Op: cmp.Op, Threshold: rhs}
	if !lOK {
		res.Reason = lReason
		return res, nil
	}
	if !rOK {
		res.Reason = "the right-hand side of the comparison has no data in this window"
		return res, nil
	}

	res.Value = lhs
	res.HasValue = true
	res.Fired = Compare(cmp.Op, lhs, rhs)
	res.Points = pointsFor(cmp, selected, span)
	res.Reason = verdictReason(cmp.Op, lhs, rhs, res.Fired, rule.For, len(selected))
	return res, nil
}

// evaluateSustained implements the per-bucket semantics used when a bare
// metric is compared under a `for` window.
func evaluateSustained(cmp *Comparison, samples []Sample, window time.Duration) Result {
	res := Result{Op: cmp.Op}
	present := 0
	allFired := true

	for _, s := range samples {
		lv, lok, _ := resolve(cmp.Left, []Sample{s}, time.Minute)
		rv, rok, _ := resolve(cmp.Right, []Sample{s}, time.Minute)
		p := Point{At: s.At}
		if lok && rok {
			p.Value = lv
			p.HasValue = true
			p.Fired = Compare(cmp.Op, lv, rv)
			present++
			res.Value = lv
			res.Threshold = rv
			res.HasValue = true
			if !p.Fired {
				allFired = false
			}
		} else {
			// A bucket with no data cannot sustain the condition.
			allFired = false
		}
		res.Points = append(res.Points, p)
	}

	if present == 0 {
		res.HasValue = false
		res.Reason = fmt.Sprintf("%s has not been observed in this window", termLabel(cmp.Left))
		return res
	}
	res.Fired = allFired
	res.Reason = verdictReason(cmp.Op, res.Value, res.Threshold, res.Fired, window, len(samples))
	return res
}

// termLabel names a term for messages.
func termLabel(n Node) string {
	switch v := n.(type) {
	case *MetricRef:
		return v.Name
	case *Aggregation:
		return v.Func + "(" + v.Metric.Name + ")"
	case *Grouping:
		return termLabel(v.Inner)
	case *NumberLiteral:
		return "the literal"
	}
	return "the expression"
}

// containsAggregation reports whether the term uses an aggregation function.
func containsAggregation(n Node) bool {
	switch v := n.(type) {
	case *Aggregation:
		return true
	case *Grouping:
		return containsAggregation(v.Inner)
	}
	return false
}

// Compare applies a comparison operator to two values.
func Compare(op string, a, b float64) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	return false
}

// selectSamples returns the buckets that fall inside the rule's window. With
// no window the verdict rests on the most recent bucket alone.
func selectSamples(w Window, window time.Duration) []Sample {
	if len(w.Samples) == 0 {
		return nil
	}
	if window <= 0 {
		return w.Samples[len(w.Samples)-1:]
	}
	cutoff := w.To.Add(-window)
	out := make([]Sample, 0, len(w.Samples))
	for _, s := range w.Samples {
		if !s.At.Before(cutoff) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		// Nothing fell inside the nominal window; fall back to the latest
		// bucket so the caller gets a verdict rather than a silent no-op.
		return w.Samples[len(w.Samples)-1:]
	}
	return out
}

// windowSpan is the duration covered by the selected samples, used by rate().
func windowSpan(samples []Sample, fallback time.Duration) time.Duration {
	if len(samples) >= 2 {
		if d := samples[len(samples)-1].At.Sub(samples[0].At); d > 0 {
			return d
		}
	}
	if fallback > 0 {
		return fallback
	}
	return time.Minute
}

// resolve reduces a term over the selected samples to a scalar.
func resolve(n Node, samples []Sample, span time.Duration) (float64, bool, string) {
	switch v := n.(type) {
	case *NumberLiteral:
		return v.Value, true, ""
	case *MetricRef:
		for i := len(samples) - 1; i >= 0; i-- {
			if val, ok := samples[i].Value(v.Name); ok {
				return val, true, ""
			}
		}
		return 0, false, fmt.Sprintf("%s has not been observed in this window", v.Name)
	case *Aggregation:
		values := make([]float64, 0, len(samples))
		for _, s := range samples {
			if val, ok := s.Value(v.Metric.Name); ok {
				values = append(values, val)
			}
		}
		return aggregate(v.Func, v.Metric.Name, values, span)
	case *Grouping:
		return resolve(v.Inner, samples, span)
	}
	return 0, false, "unsupported expression"
}

// aggregate applies one aggregation function to the observed values.
func aggregate(fn, metric string, values []float64, span time.Duration) (float64, bool, string) {
	switch fn {
	case "count":
		return float64(len(values)), true, ""
	case "sum":
		total := 0.0
		for _, v := range values {
			total += v
		}
		return total, true, ""
	case "rate":
		if len(values) == 0 {
			return 0, true, ""
		}
		total := 0.0
		for _, v := range values {
			total += v
		}
		secs := math.Max(span.Seconds(), 1)
		return total / secs, true, ""
	}

	if len(values) == 0 {
		return 0, false, fmt.Sprintf("%s(%s) has no samples in this window", fn, metric)
	}
	switch fn {
	case "avg":
		total := 0.0
		for _, v := range values {
			total += v
		}
		return total / float64(len(values)), true, ""
	case "max":
		m := values[0]
		for _, v := range values {
			if v > m {
				m = v
			}
		}
		return m, true, ""
	case "min":
		m := values[0]
		for _, v := range values {
			if v < m {
				m = v
			}
		}
		return m, true, ""
	}
	return 0, false, fmt.Sprintf("unknown aggregation %q", fn)
}

// pointsFor builds the per-sample preview series from the comparison's
// left-hand side.
func pointsFor(cmp *Comparison, samples []Sample, span time.Duration) []Point {
	pts := make([]Point, 0, len(samples))
	for _, s := range samples {
		val, ok, _ := resolve(cmp.Left, []Sample{s}, span)
		p := Point{At: s.At, HasValue: ok}
		if ok {
			p.Value = val
			rhs, rOK, _ := resolve(cmp.Right, []Sample{s}, span)
			p.Fired = rOK && Compare(cmp.Op, val, rhs)
		}
		pts = append(pts, p)
	}
	return pts
}

// verdictReason renders a one-line explanation for an alert message.
func verdictReason(op string, lhs, rhs float64, fired bool, window time.Duration, buckets int) string {
	span := ""
	if window > 0 {
		span = fmt.Sprintf(" over the last %s", formatDuration(window))
	}
	if fired {
		return fmt.Sprintf("condition met: %.6g %s %.6g%s (%d samples)", lhs, op, rhs, span, buckets)
	}
	return fmt.Sprintf("condition not met: %.6g is not %s %.6g%s (%d samples)", lhs, op, rhs, span, buckets)
}
