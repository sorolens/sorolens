package rules

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// validMetrics lists every metric name in parse order (for error messages).
var validMetrics = []string{
	"fee", "cpu", "mem", "ledger_bytes", "events", "invocations", "failures",
}

// validAggregations lists every aggregation function name.
var validAggregations = []string{"avg", "max", "rate"}

// metricAliases maps accepted spellings to canonical metrics.
var metricAliases = map[string]Metric{
	"fee":          MetricFee,
	"fee_per_invocation": MetricFee,
	"fees":         MetricFee,
	"cpu":          MetricCPU,
	"cpu_insn":     MetricCPU,
	"insn":         MetricCPU,
	"mem":          MetricMem,
	"memory":       MetricMem,
	"ledger_bytes": MetricLedgerBytes,
	"ledger_io":    MetricLedgerBytes,
	"events":       MetricEvents,
	"invoices":     MetricInvocations,
	"invocations":  MetricInvocations,
	"invocations_count": MetricInvocations,
	"failed":       MetricFailures,
	"failures":     MetricFailures,
}

var unitAliases = map[string]Unit{
	"xlm":     UnitXLM,
	"lumens":  UnitXLM,
	"stroops": UnitStroops,
	"insn":    UnitInsn,
	"insns":   UnitInsn,
	"ops":     UnitInsn,
	"bytes":   UnitBytes,
	"byte":    UnitBytes,
}

var durationUnits = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": 24 * time.Hour,
}

// keywords are structural words that can never be a unit token.
func isKeyword(s string) bool {
	switch strings.ToLower(s) {
	case "for", "on", "and", "every", "all":
		return true
	}
	return false
}

// ParseExpression parses and fully validates an alert-rule expression.
// The returned Rule has normalized aggregation, metric, comparison and unit
// values plus a resolved window and scope. Errors are human-readable with a
// source position and a suggestion where one is obviously available.
func ParseExpression(expr string) (Rule, error) {
	toks, err := lex(expr)
	if err != nil {
		return Rule{}, err
	}
	p := &parser{toks: toks, input: expr}
	return p.parse()
}

type parser struct {
	toks  []token
	pos   int
	input string
}

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) next() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

// err reports a parse error at the current token with a friendly message.
func (p *parser) err(tok token, msg string) error {
	if tok.kind == tokEOF {
		return parseErr(p.input, len(p.input), msg)
	}
	return parseErr(p.input, tok.pos, msg)
}

func (p *parser) parse() (Rule, error) {
	r := Rule{}

	// ---- optional aggregation + metric: avg(fee) | rate(events) | fee ...
	first := p.peek()
	if (first.kind == tokIdent && strings.EqualFold(first.text, "avg")) ||
		(first.kind == tokIdent && strings.EqualFold(first.text, "max")) ||
		(first.kind == tokIdent && strings.EqualFold(first.text, "rate")) {
		aggTok := p.next()
		agg, ok := map[string]Aggregation{
			"avg": AggAvg, "max": AggMax, "rate": AggRate,
		}[strings.ToLower(aggTok.text)]
		if !ok {
			return Rule{}, p.err(aggTok, "unknown aggregation")
		}
		r.Aggregation = agg

		open := p.next()
		if open.kind != tokLParen {
			return Rule{}, p.err(open, fmt.Sprintf("expected '(' after %q, got %s", aggTok.text, open))
		}
		metricTok := p.next()
		m, err := p.parseMetric(metricTok)
		if err != nil {
			return Rule{}, err
		}
		r.Metric = m

		close := p.next()
		if close.kind != tokRParen {
			return Rule{}, p.err(close, fmt.Sprintf("expected ')' to close %s(%s), got %s", aggTok.text, metricTok.text, close))
		}
	} else {
		// Bare metric (sugar). Defaults: per-invocation metrics -> avg,
		// count metrics -> rate.
		metricTok := p.next()
		if metricTok.kind != tokIdent {
			return Rule{}, p.err(metricTok, "expected an aggregation like \"avg(fee)\" or a metric name like \"fee\"")
		}
		if p.peek().kind == tokLParen {
			return Rule{}, p.err(metricTok, fmt.Sprintf("unknown aggregation %q; valid aggregations: %s", metricTok.text, joinOptions(validAggregations)))
		}
		m, err := p.parseMetric(metricTok)
		if err != nil {
			return Rule{}, err
		}
		r.Metric = m
		if m == MetricEvents || m == MetricInvocations {
			r.Aggregation = AggRate
		} else {
			r.Aggregation = AggAvg
		}
	}

	// ---- comparison operator
	opTok := p.next()
	cmp, ok := map[string]Comparison{
		">": CmpGt, ">=": CmpGte, "<": CmpLt, "<=": CmpLte,
		"==": CmpEq, "=": CmpEq, "!=": CmpNeq,
	}[opTok.text]
	if !ok {
		return Rule{}, p.err(opTok, fmt.Sprintf("expected a comparison operator (>, >=, <, <=, ==, !=), got %s", opTok))
	}
	r.Comparison = cmp

	// ---- threshold
	numTok := p.next()
	if numTok.kind != tokNumber {
		return Rule{}, p.err(numTok, "expected a numeric threshold after the comparison operator")
	}
	threshold, err := strconv.ParseFloat(numTok.text, 64)
	if err != nil {
		return Rule{}, p.err(numTok, fmt.Sprintf("invalid threshold %q", numTok.text))
	}
	r.Threshold = threshold

	// ---- optional unit
	if t := p.peek(); t.kind == tokIdent && !isKeyword(t.text) {
		u, ok := unitAliases[strings.ToLower(t.text)]
		if !ok {
			return Rule{}, p.err(t, fmt.Sprintf("unknown unit %q; valid units: xlm, stroops, insn, bytes", t.text))
		}
		if err := validateUnit(r.Metric, u); err != nil {
			return Rule{}, p.err(t, err.Error())
		}
		r.Unit = u
		p.next()
	} else {
		if err := validateUnit(r.Metric, ""); err != nil {
			return Rule{}, p.err(numTok, err.Error())
		}
	}
	if r.Unit == "" {
		r.Unit = defaultUnit(r.Metric)
	}

	// ---- window: [for <duration>]
	r.Window = DefaultWindow
	if t := p.peek(); t.kind == tokIdent && strings.EqualFold(t.text, "for") {
		p.next()
		window, err := p.parseDuration()
		if err != nil {
			return Rule{}, err
		}
		if window < time.Second || window > 90*24*time.Hour {
			return Rule{}, parseErr(p.input, t.pos, "window must be between 1s and 90d")
		}
		r.Window = window
	}

	// ---- scope: [on <contract>]
	if t := p.peek(); t.kind == tokIdent && strings.EqualFold(t.text, "on") {
		p.next()
		target := p.next()
		switch {
		case target.kind == tokStar:
			r.Contract = ""
		case target.kind == tokIdent && strings.EqualFold(target.text, "all"):
			r.Contract = ""
		case target.kind == tokIdent && strings.EqualFold(target.text, "every"):
			r.Contract = ""
		case target.kind == tokIdent:
			if !isContractID(target.text) {
				return Rule{}, p.err(target, fmt.Sprintf("%q is not a valid contract ID (expected a 56-character string starting with 'C')", target.text))
			}
			r.Contract = target.text
		default:
			return Rule{}, p.err(target, "expected a contract ID, or \"*\" / \"all\" for every contract, after \"on\"")
		}
	}

	// ---- trailing garbage check
	if t := p.next(); t.kind != tokEOF {
		if t.kind == tokIdent && strings.EqualFold(t.text, "and") {
			return Rule{}, p.err(t, "only one condition is supported per rule; split it into multiple rules")
		}
		return Rule{}, p.err(t, fmt.Sprintf("unexpected trailing %s", t))
	}

	if err := validateCombo(r); err != nil {
		return Rule{}, parseErr(p.input, 0, err.Error())
	}
	return r, nil
}

// parseMetric resolves an identifier to a metric or returns a friendly error.
func (p *parser) parseMetric(tok token) (Metric, error) {
	if tok.kind != tokIdent {
		return "", p.err(tok, fmt.Sprintf("expected a metric name (%s), got %s", joinOptions(validMetrics), tok))
	}
	m, ok := metricAliases[strings.ToLower(tok.text)]
	if !ok {
		return "", p.err(tok, fmt.Sprintf("unknown metric %q; valid metrics: %s", tok.text, joinOptions(validMetrics)))
	}
	return m, nil
}

// parseDuration parses a duration expression like "5m", "1h30m", "90d".
// Duration components are maximal sequences of a number followed by a unit.
func (p *parser) parseDuration() (time.Duration, error) {
	var total time.Duration
	components := 0
	for {
		t := p.peek()
		if t.kind != tokNumber {
			break
		}
		numTok := p.next()
		value, err := strconv.ParseFloat(numTok.text, 64)
		if err != nil {
			return 0, p.err(numTok, fmt.Sprintf("invalid duration number %q", numTok.text))
		}
		unitTok := p.next()
		if unitTok.kind != tokIdent {
			return 0, p.err(unitTok, fmt.Sprintf("expected a duration unit (s, m, h, d) after %q", numTok.text))
		}
		unit, ok := durationUnits[strings.ToLower(unitTok.text)]
		if !ok {
			return 0, p.err(unitTok, fmt.Sprintf("unknown duration unit %q; valid units: s, m, h, d", unitTok.text))
		}
		total += time.Duration(value * float64(unit))
		components++
	}
	if components == 0 {
		t := p.peek()
		return 0, p.err(t, "expected a window like \"5m\", \"1h30m\", or \"24h\" after \"for\"")
	}
	return total, nil
}

// defaultUnit returns the canonical unit for a metric.
func defaultUnit(m Metric) Unit {
	switch m {
	case MetricFee:
		return UnitStroops
	case MetricCPU:
		return UnitInsn
	case MetricMem, MetricLedgerBytes:
		return UnitBytes
	default:
		return ""
	}
}

// validateUnit checks a unit supplied by the user against the metric.
func validateUnit(m Metric, u Unit) error {
	switch m {
	case MetricFee:
		if u != "" && u != UnitXLM && u != UnitStroops {
			return fmt.Errorf("unit %q is not valid for fee; use XLM or stroops", u)
		}
	case MetricCPU:
		if u != "" && u != UnitInsn {
			return fmt.Errorf("unit %q is not valid for cpu; use insn (or omit the unit)", u)
		}
	case MetricMem, MetricLedgerBytes:
		if u != "" && u != UnitBytes {
			return fmt.Errorf("unit %q is not valid for %s; use bytes (or omit the unit)", u, m)
		}
	default:
		if u != "" {
			return fmt.Errorf("unit %q is not valid for %s; omit the unit (it is already a count)", u, m)
		}
	}
	return nil
}

// validateCombo rejects aggregation/metric combinations that have no meaning.
func validateCombo(r Rule) error {
	switch r.Aggregation {
	case AggMax:
		switch r.Metric {
		case MetricEvents, MetricInvocations:
			return fmt.Errorf("%s(%s) is not meaningful; use %s(%s) to measure the rate", r.Aggregation, r.Metric, AggRate, r.Metric)
		}
	case AggAvg:
		switch r.Metric {
		case MetricInvocations:
			return fmt.Errorf("avg(%s) is meaningless over a window; use %s(%s) to measure the rate", r.Metric, AggRate, r.Metric)
		}
	}
	return nil
}

// isContractID reports whether s looks like a Soroban contract address:
// a 56-character base32 (A-Z2-7) string starting with 'C'.
func isContractID(s string) bool {
	if len(s) != 56 || s[0] != 'C' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
			return false
		}
	}
	return true
}