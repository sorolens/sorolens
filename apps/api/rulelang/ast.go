package rulelang

import "time"

// Node is one expression in a rule. Every node carries the byte offset at
// which it starts so the editor can point at the offending text.
type Node interface {
	Position() int
}

// NumberLiteral is a numeric constant, optionally followed by a unit word
// such as `0.5 XLM` or `5%`.
type NumberLiteral struct {
	Pos    int
	Value  float64
	Unit   string // canonical unit, "" when none was written
	UnitAt int    // byte offset of the unit word (for diagnostics)
}

func (n *NumberLiteral) Position() int { return n.Pos }

// MetricRef is a bare metric name, e.g. `error_rate`.
type MetricRef struct {
	Pos  int
	Name string
}

func (n *MetricRef) Position() int { return n.Pos }

// Aggregation reduces a metric over the evaluation window, e.g.
// `avg(cpu_insn_per_invocation)` or `rate(events)`.
type Aggregation struct {
	Pos    int
	Func   string
	Metric MetricRef
}

func (n *Aggregation) Position() int { return n.Pos }

// Comparison is `left op right` where op is one of >, >=, <, <=, ==, !=.
type Comparison struct {
	Pos   int
	Op    string
	OpPos int
	Left  Node
	Right Node
}

func (n *Comparison) Position() int { return n.Pos }

// Grouping is a parenthesised expression.
type Grouping struct {
	Pos   int
	Inner Node
}

func (n *Grouping) Position() int { return n.Pos }

// Rule is a parsed alert rule: an expression plus optional clauses.
//
//	error_rate > 0.05 for 15m on contract CDLZ... on network testnet
type Rule struct {
	Source string

	// Expr is the comparison that decides whether the rule fires.
	Expr Node
	// For is how long the condition must hold. Zero means "as soon as it is
	// observed in the most recent sample".
	For time.Duration
	// ContractID narrows evaluation to one contract. Empty means every
	// contract the rule is scoped to (the rule's own contract, when stored).
	ContractID string
	// Network narrows evaluation to one network. Empty means all.
	Network string
}

// Metrics returns every metric referenced by the rule, in source order.
func (r *Rule) Metrics() []string {
	var out []string
	var walk func(Node)
	walk = func(n Node) {
		switch v := n.(type) {
		case *MetricRef:
			out = append(out, v.Name)
		case *Aggregation:
			out = append(out, v.Metric.Name)
		case *Comparison:
			walk(v.Left)
			walk(v.Right)
		case *Grouping:
			walk(v.Inner)
		}
	}
	if r.Expr != nil {
		walk(r.Expr)
	}
	return out
}

// Format renders the rule back to source form. It is stable, so a rule that
// is parsed and formatted twice is byte-identical the second time.
func (r *Rule) Format() string {
	s := formatNode(r.Expr)
	if r.For > 0 {
		s += " for " + formatDuration(r.For)
	}
	if r.ContractID != "" {
		s += " on contract " + r.ContractID
	}
	if r.Network != "" {
		s += " on network " + r.Network
	}
	return s
}

func formatNode(n Node) string {
	switch v := n.(type) {
	case nil:
		return ""
	case *NumberLiteral:
		if v.Unit == UnitRatio {
			return trimFloat(v.Value*100) + "%"
		}
		s := trimFloat(v.Value)
		if v.Unit != "" {
			s += " " + v.Unit
		}
		return s
	case *MetricRef:
		return v.Name
	case *Aggregation:
		return v.Func + "(" + v.Metric.Name + ")"
	case *Comparison:
		return formatNode(v.Left) + " " + v.Op + " " + formatNode(v.Right)
	case *Grouping:
		return "(" + formatNode(v.Inner) + ")"
	}
	return ""
}
