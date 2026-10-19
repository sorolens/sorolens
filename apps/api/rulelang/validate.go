package rulelang

import (
	"strings"
	"time"
)

// Limits on the evaluation window. The upper bound keeps the indexer's metric
// query bounded; longer baselines belong in a dedicated trend rule, not here.
const (
	MinWindow = 5 * time.Second
	MaxWindow = 7 * 24 * time.Hour
)

// ValidNetworks is the set of network names a rule may pin itself to.
var ValidNetworks = []string{"testnet", "mainnet", "futurenet", "standalone"}

// Validate parses source and then checks it semantically. It is the entry
// point the API uses, so the returned error is always a *Error with a position
// and (where possible) a hint.
func Validate(source string) (*Rule, error) {
	r, err := Parse(source)
	if err != nil {
		return nil, err
	}
	if verr := ValidateRule(r); verr != nil {
		return nil, verr
	}
	return r, nil
}

// ValidateRule checks an already-parsed rule for unknown metrics, unit
// mismatches, window bounds, and malformed contract/network clauses.
func ValidateRule(r *Rule) error {
	if r.Expr == nil {
		return Errorf(r.Source, 0, "a rule needs a condition, e.g. error_rate > 0.05", "rule has no condition")
	}
	if !hasComparison(r.Expr) {
		return Errorf(r.Source, r.Expr.Position(), "for example: error_rate > 0.05",
			"a rule must contain a comparison")
	}
	if err := validateNode(r.Source, r.Expr, true); err != nil {
		return err
	}

	if r.For != 0 {
		if r.For < MinWindow {
			return Errorf(r.Source, 0, "", "window %s is too short; the minimum is %s", formatDuration(r.For), formatDuration(MinWindow))
		}
		if r.For > MaxWindow {
			return Errorf(r.Source, 0, "", "window %s is too long; the maximum is %s", formatDuration(r.For), formatDuration(MaxWindow))
		}
	}

	if r.Network != "" {
		ok := false
		for _, n := range ValidNetworks {
			if strings.EqualFold(r.Network, n) {
				r.Network = n
				ok = true
				break
			}
		}
		if !ok {
			return Errorf(r.Source, 0, "one of: "+strings.Join(ValidNetworks, ", "),
				"unknown network %q", r.Network)
		}
	}

	if r.ContractID != "" {
		if !ValidContractID(r.ContractID) {
			return Errorf(r.Source, 0, "a Soroban contract id is 56 characters starting with C, e.g. CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
				"invalid contract id %q", r.ContractID)
		}
	}
	return nil
}

// hasComparison reports whether the tree contains at least one comparison.
func hasComparison(n Node) bool {
	switch v := n.(type) {
	case *Comparison:
		return true
	case *Grouping:
		return hasComparison(v.Inner)
	}
	return false
}

// validateNode walks the expression, resolving metric references and checking
// unit compatibility between the two sides of every comparison.
func validateNode(source string, n Node, top bool) error {
	switch v := n.(type) {
	case *MetricRef:
		return checkMetric(source, v.Pos, v.Name)
	case *Aggregation:
		if !IsAggregation(v.Func) {
			return Errorf(source, v.Pos, "one of: "+strings.Join(AggregationFuncs, ", "),
				"unknown aggregation %q", v.Func)
		}
		return checkMetric(source, v.Metric.Pos, v.Metric.Name)
	case *Grouping:
		if top {
			return validateNode(source, v.Inner, top)
		}
		return validateNode(source, v.Inner, false)
	case *Comparison:
		for _, side := range []Node{v.Left, v.Right} {
			if sideIsBoolean(side) {
				return Errorf(source, side.Position(), "combine conditions with separate rules, not nested comparisons",
					"cannot compare against another comparison")
			}
			if err := validateNode(source, side, false); err != nil {
				return err
			}
		}
		return checkUnits(source, v)
	}
	return nil
}

// checkMetric verifies that a metric name exists, suggesting the nearest known
// name when it does not.
func checkMetric(source string, pos int, name string) error {
	if _, ok := LookupMetric(name); ok {
		return nil
	}
	hint := "known metrics include: " + strings.Join(firstN(Metrics(), 4), ", ")
	if s := suggest(name, Metrics()); s != "" {
		hint = "did you mean " + s + "?"
	}
	return Errorf(source, pos, hint, "unknown metric %q", name)
}

// checkUnits rejects comparisons between incompatible dimensions and attaches
// a literal's unit to the metric on the other side.
func checkUnits(source string, c *Comparison) error {
	lUnit, lLit := sideUnit(c.Left)
	rUnit, rLit := sideUnit(c.Right)

	switch {
	case lLit != nil && rUnit != "":
		if !unitCompatible(lLit.Unit, rUnit) {
			return Errorf(source, literalPos(lLit), "metric uses "+rUnit,
				"unit %s does not match the metric on the other side of %s", displayUnit(lLit.Unit), c.Op)
		}
	case rLit != nil && lUnit != "":
		if !unitCompatible(rLit.Unit, lUnit) {
			return Errorf(source, literalPos(rLit), "metric uses "+lUnit,
				"unit %s does not match the metric on the other side of %s", displayUnit(rLit.Unit), c.Op)
		}
	case lUnit != "" && rUnit != "" && lUnit != rUnit:
		return Errorf(source, c.OpPos, "compare metrics of the same kind, or compare a metric to a number",
			"cannot compare a %s metric with a %s metric", lUnit, rUnit)
	}
	return nil
}

// sideUnit returns the unit of one side of a comparison and, when that side is
// a bare numeric literal, the literal itself.
func sideUnit(n Node) (string, *NumberLiteral) {
	switch v := n.(type) {
	case *MetricRef:
		if m, ok := LookupMetric(v.Name); ok {
			return m.Unit, nil
		}
	case *Aggregation:
		if m, ok := LookupMetric(v.Metric.Name); ok {
			return m.Unit, nil
		}
	case *NumberLiteral:
		return v.Unit, v
	case *Grouping:
		return sideUnit(v.Inner)
	}
	return "", nil
}

func literalPos(l *NumberLiteral) int {
	if l.UnitAt >= 0 {
		return l.UnitAt
	}
	return l.Pos
}

func displayUnit(u string) string {
	if u == "" {
		return "a bare number"
	}
	return u
}

// sideIsBoolean reports whether a node evaluates to a boolean rather than a
// number, which catches `a > (b > c)`.
func sideIsBoolean(n Node) bool {
	switch v := n.(type) {
	case *Comparison:
		return true
	case *Grouping:
		return sideIsBoolean(v.Inner)
	}
	return false
}

func firstN(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
