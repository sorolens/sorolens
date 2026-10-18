package rulelang

import (
	"strconv"
	"strings"
	"time"
)

// parser is a small recursive-descent parser. It is intentionally hand-rolled:
// the grammar is tiny, and a hand-rolled parser gives precise byte offsets and
// hints, which is the point of the friendly-error requirement.
type parser struct {
	src  string
	toks []token
	i    int
}

// Parse parses source into a Rule. It returns a *Error (implements error) on
// any lexical or syntactic problem; semantic checks live in Validate.
func Parse(source string) (*Rule, error) {
	toks, lerr := lex(source)
	if lerr != nil {
		return nil, lerr
	}
	p := &parser{src: source, toks: toks}
	return p.parseRule()
}

func (p *parser) cur() token  { return p.toks[p.i] }
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }

func (p *parser) errAt(pos int, hint, format string, args ...any) *Error {
	return Errorf(p.src, pos, hint, format, args...)
}

// isKeyword reports whether the current token is the given bare word.
func (p *parser) isKeyword(kw string) bool {
	t := p.cur()
	return t.kind == tokIdent && strings.EqualFold(t.text, kw)
}

func (p *parser) parseRule() (*Rule, error) {
	r := &Rule{Source: p.src}

	expr, err := p.parseComparison(true)
	if err != nil {
		return nil, err
	}
	r.Expr = expr

	for p.cur().kind != tokEOF {
		switch {
		case p.isKeyword("for"):
			forPos := p.cur().pos
			p.next()
			if p.cur().kind != tokDuration {
				return nil, p.errAt(p.cur().pos, "write it like 30s, 5m, 1h or 2d",
					"expected a duration after `for`, found %s", p.cur().kind)
			}
			d, derr := parseDuration(p.cur().text)
			if derr != nil {
				return nil, p.errAt(forPos, "", "%s", derr)
			}
			r.For = d
			p.next()
		case p.isKeyword("on"):
			p.next()
			switch {
			case p.isKeyword("contract"):
				p.next()
				t := p.cur()
				if t.kind != tokIdent && t.kind != tokString {
					return nil, p.errAt(t.pos, "for example: on contract CDLZ...",
						"expected a contract id after `on contract`, found %s", t.kind)
				}
				r.ContractID = t.text
				p.next()
			case p.isKeyword("network"):
				p.next()
				t := p.cur()
				if t.kind != tokIdent && t.kind != tokString {
					return nil, p.errAt(t.pos, "one of testnet, mainnet, futurenet",
						"expected a network name after `on network`, found %s", t.kind)
				}
				r.Network = t.text
				p.next()
			default:
				return nil, p.errAt(p.cur().pos, "the supported clauses are `on contract <id>` and `on network <name>`",
					"expected `contract` or `network` after `on`, found %s", p.cur().text)
			}
		default:
			return nil, p.errAt(p.cur().pos, "clauses are written after the condition, e.g. `... for 5m on network testnet`",
				"unexpected %s %q", p.cur().kind, p.cur().text)
		}
	}
	return r, nil
}

// parseComparison parses `<term> [op <term>]`. When requireOp is set the
// operator is mandatory, which is what turns a bare metric into a helpful
// "a rule needs a comparison" message at the top level.
func (p *parser) parseComparison(requireOp bool) (Node, error) {
	left, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	opTok := p.cur()
	if !isComparisonOp(opTok.kind) {
		if requireOp {
			return nil, p.errAt(p.cur().pos, "for example: error_rate > 0.05",
				"expected a comparison operator (>, >=, <, <=, ==, !=) after %q", describeNode(left))
		}
		return left, nil
	}
	p.next()
	if opTok.kind == tokEQ && opTok.text == "=" {
		return nil, p.errAt(opTok.pos, "use == for equality, or >= / <= for a bound",
			"`=` is not a comparison operator")
	}
	right, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	return &Comparison{Pos: left.Position(), Op: opTok.text, OpPos: opTok.pos, Left: left, Right: right}, nil
}

// parseTerm parses a primary expression and attaches a trailing unit word to a
// numeric literal.
func (p *parser) parseTerm() (Node, error) {
	term, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	nl, ok := term.(*NumberLiteral)
	if !ok || nl.Unit != "" {
		return term, nil
	}
	if p.cur().kind != tokIdent || p.isClauseKeyword() {
		return term, nil
	}
	word := p.cur()
	if unit, ok := canonicalUnit(word.text); ok {
		nl.Unit = unit
		nl.UnitAt = word.pos
		if unit == UnitRatio {
			nl.Value = nl.Value / 100
		}
		p.next()
		return nl, nil
	}
	// A word after a bare number is almost always a mistaken unit.
	return nil, p.errAt(word.pos, "units are only needed for readability; drop it, or use one of XLM, stroops, %, instructions, bytes, ledgers",
		"unknown unit %q", word.text)
}

func (p *parser) parsePrimary() (Node, error) {
	t := p.cur()
	switch t.kind {
	case tokNumber:
		p.next()
		text := t.text
		unit := ""
		unitAt := -1
		if strings.HasSuffix(text, "%") {
			text = strings.TrimSuffix(text, "%")
			unit = UnitRatio
			unitAt = t.pos + len(t.text) - 1
		}
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, p.errAt(t.pos, "", "invalid number %q", t.text)
		}
		if unit == UnitRatio {
			v = v / 100
		}
		return &NumberLiteral{Pos: t.pos, Value: v, Unit: unit, UnitAt: unitAt}, nil
	case tokIdent:
		if p.isClauseKeyword() {
			return nil, p.errAt(t.pos, "put the condition before the `for`/`on` clauses",
				"expected an expression, found clause keyword %q", t.text)
		}
		if IsAggregation(t.text) {
			// avg(...) / max(...) / ...
			if p.toks[p.i+1].kind != tokLParen {
				return nil, p.errAt(t.pos, "write it as "+strings.ToLower(t.text)+"(metric), e.g. "+strings.ToLower(t.text)+"(error_rate)",
					"aggregation %q needs a metric in parentheses", t.text)
			}
			p.next() // function name
			p.next() // (
			mt := p.cur()
			if mt.kind != tokIdent {
				return nil, p.errAt(mt.pos, "for example: avg(error_rate)",
					"expected a metric name inside %s(...), found %s", t.text, mt.kind)
			}
			p.next()
			if p.cur().kind != tokRParen {
				return nil, p.errAt(p.cur().pos, "aggregations take exactly one metric",
					"expected `)`, found %s", p.cur().kind)
			}
			p.next()
			return &Aggregation{Pos: t.pos, Func: strings.ToLower(t.text), Metric: MetricRef{Pos: mt.pos, Name: mt.text}}, nil
		}
		p.next()
		return &MetricRef{Pos: t.pos, Name: strings.ToLower(t.text)}, nil
	case tokLParen:
		p.next()
		inner, err := p.parseComparison(false)
		if err != nil {
			return nil, err
		}
		if p.cur().kind != tokRParen {
			return nil, p.errAt(p.cur().pos, "", "expected `)`, found %s", p.cur().kind)
		}
		p.next()
		return &Grouping{Pos: t.pos, Inner: inner}, nil
	case tokDuration:
		return nil, p.errAt(t.pos, "durations only appear after `for`",
			"expected an expression, found duration %q", t.text)
	default:
		return nil, p.errAt(t.pos, "an expression is a metric, a number, or an aggregation like avg(error_rate)",
			"expected an expression, found %s", t.kind)
	}
}

func (p *parser) isClauseKeyword() bool {
	return p.isKeyword("for") || p.isKeyword("on")
}

func isComparisonOp(k tokenKind) bool {
	switch k {
	case tokGT, tokGE, tokLT, tokLE, tokEQ, tokNE:
		return true
	}
	return false
}

// describeNode names a node kind for error messages.
func describeNode(n Node) string {
	switch n.(type) {
	case *MetricRef:
		return "metric"
	case *Aggregation:
		return "aggregation"
	case *NumberLiteral:
		return "number"
	case *Grouping:
		return "group"
	}
	return "expression"
}

// parseDuration converts a compact duration token (`30s`, `5m`, `1h`, `2d`,
// `1w`) into a time.Duration.
func parseDuration(text string) (time.Duration, error) {
	if len(text) < 2 {
		return 0, errInvalidDuration(text)
	}
	unit := text[len(text)-1]
	num, err := strconv.ParseFloat(text[:len(text)-1], 64)
	if err != nil || num <= 0 {
		return 0, errInvalidDuration(text)
	}
	var mult time.Duration
	switch unit {
	case 's', 'S':
		mult = time.Second
	case 'm', 'M':
		mult = time.Minute
	case 'h', 'H':
		mult = time.Hour
	case 'd', 'D':
		mult = 24 * time.Hour
	case 'w', 'W':
		mult = 7 * 24 * time.Hour
	default:
		return 0, errInvalidDuration(text)
	}
	return time.Duration(num * float64(mult)), nil
}

type durationError string

func (e durationError) Error() string { return string(e) }

func errInvalidDuration(text string) error {
	return durationError("invalid duration " + strconv.Quote(text) + "; use 30s, 5m, 1h, 2d or 1w")
}
