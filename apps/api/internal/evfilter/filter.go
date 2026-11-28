// Package evfilter implements a small expression language for filtering events
// by their decoded topic array and decoded value fields (issue #787).
//
// Grammar (EBNF):
//
//	expr        = clause { " AND " clause } .
//	clause      = topicClause | valueClause .
//	topicClause = "topic[" digit+ "]" op scalar .
//	valueClause = "value." path op scalar .
//	path        = ident { "." ident } .
//	op          = "=" | "==" | "!=" | "<" | "<=" | ">" | ">=" .
//	scalar      = string | number .
//
// A compiled Expr translates to parameterised Postgres SQL fragments appended
// to an events query with AND. Every value is a bind parameter, so the
// expression language cannot inject SQL.
package evfilter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Expr is a parsed and compiled event filter expression.
type Expr struct {
	clauses []clause
}

type clauseKind int

const (
	kindTopic clauseKind = iota
	kindValue
)

type clause struct {
	kind  clauseKind
	index int    // topic index (kindTopic)
	path  string // dotted path (kindValue)
	op    string // comparison operator
	value any    // string or float64
}

// Parse parses an expression string and returns an Expr. It returns an error
// if the string is not syntactically valid. An empty string is valid and
// produces an Expr with no clauses.
func Parse(s string) (*Expr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &Expr{}, nil
	}
	parts := splitAnd(s)
	expr := &Expr{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		c, err := parseClause(part)
		if err != nil {
			return nil, err
		}
		expr.clauses = append(expr.clauses, c)
	}
	return expr, nil
}

// Empty reports whether the expression has no clauses.
func (e *Expr) Empty() bool {
	return e == nil || len(e.clauses) == 0
}

// AppendSQL appends one SQL AND predicate per clause to dst. Bind values are
// appended to args. argOffset is ignored; the method uses len(*args)+1 to
// compute parameter positions, so it is safe to call after other parameters
// have already been appended to args.
func (e *Expr) AppendSQL(dst *strings.Builder, args *[]any) {
	if e == nil || len(e.clauses) == 0 {
		return
	}
	for _, c := range e.clauses {
		argN := len(*args) + 1
		switch c.kind {
		case kindTopic:
			switch c.op {
			case "=", "==":
				// Use JSONB containment for equality: compact, index-friendly.
				b, _ := json.Marshal([]any{c.value})
				*args = append(*args, string(b))
				fmt.Fprintf(dst, "\n  AND topic_decoded @> $%d::jsonb", argN)
			default:
				*args = append(*args, fmt.Sprintf("%v", c.value))
				fmt.Fprintf(dst, "\n  AND topic_decoded ->> %d %s $%d", c.index, pgOp(c.op), argN)
			}
		case kindValue:
			path := pgJSONPath(c.path)
			switch c.value.(type) {
			case float64:
				*args = append(*args, c.value)
				fmt.Fprintf(dst, "\n  AND (value_decoded %s)::numeric %s $%d", path, pgOp(c.op), argN)
			default:
				*args = append(*args, fmt.Sprintf("%v", c.value))
				fmt.Fprintf(dst, "\n  AND (value_decoded %s) %s $%d", path, pgOp(c.op), argN)
			}
		}
	}
}

// pgOp translates the parsed operator to its Postgres equivalent.
func pgOp(op string) string {
	if op == "==" {
		return "="
	}
	return op
}

// pgJSONPath converts a dotted path like "a.b.c" to a Postgres JSONB
// extraction expression like "#>> '{a,b,c}'" or "->> 'a'" for single keys.
func pgJSONPath(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return fmt.Sprintf("->> '%s'", parts[0])
	}
	return fmt.Sprintf("#>> '{%s}'", strings.Join(parts, ","))
}

// splitAnd splits on literal " AND " while respecting single- and
// double-quoted strings.
func splitAnd(s string) []string {
	var parts []string
	var buf strings.Builder
	inStr := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr != 0 {
			buf.WriteByte(c)
			if c == inStr {
				inStr = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			inStr = c
			buf.WriteByte(c)
			continue
		}
		if i+5 <= len(s) && strings.ToUpper(s[i:i+5]) == " AND " {
			parts = append(parts, buf.String())
			buf.Reset()
			i += 4
			continue
		}
		buf.WriteByte(c)
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

func parseClause(s string) (clause, error) {
	switch {
	case strings.HasPrefix(s, "topic["):
		return parseTopicClause(s)
	case strings.HasPrefix(s, "value."):
		return parseValueClause(s)
	default:
		return clause{}, fmt.Errorf("evfilter: unrecognised clause %q; expected topic[N]=... or value.field>=...", s)
	}
}

func parseTopicClause(s string) (clause, error) {
	rest := s[len("topic["):]
	closeBracket := strings.IndexByte(rest, ']')
	if closeBracket < 0 {
		return clause{}, fmt.Errorf("evfilter: missing ']' in %q", s)
	}
	idxStr := rest[:closeBracket]
	idx, err := strconv.Atoi(idxStr)
	if err != nil || idx < 0 {
		return clause{}, fmt.Errorf("evfilter: invalid topic index %q", idxStr)
	}
	after := strings.TrimSpace(rest[closeBracket+1:])
	op, val, err := parseOpValue(after)
	if err != nil {
		return clause{}, fmt.Errorf("evfilter: topic clause: %w", err)
	}
	return clause{kind: kindTopic, index: idx, op: op, value: val}, nil
}

func parseValueClause(s string) (clause, error) {
	rest := s[len("value."):]
	opPos, op := findOp(rest)
	if opPos < 0 {
		return clause{}, fmt.Errorf("evfilter: missing operator in %q", s)
	}
	path := strings.TrimSpace(rest[:opPos])
	if path == "" || !validPath(path) {
		return clause{}, fmt.Errorf("evfilter: invalid path %q", path)
	}
	val, err := parseScalar(strings.TrimSpace(rest[opPos+len(op):]))
	if err != nil {
		return clause{}, fmt.Errorf("evfilter: value clause: %w", err)
	}
	return clause{kind: kindValue, path: path, op: op, value: val}, nil
}

var opList = []string{">=", "<=", "!=", "==", ">", "<", "="}

func findOp(s string) (int, string) {
	bestPos := -1
	bestOp := ""
	for _, op := range opList {
		pos := strings.Index(s, op)
		if pos >= 0 && (bestPos < 0 || pos < bestPos) {
			bestPos = pos
			bestOp = op
		}
	}
	return bestPos, bestOp
}

func parseOpValue(s string) (string, any, error) {
	opPos, op := findOp(s)
	if opPos != 0 {
		return "", nil, fmt.Errorf("expected operator at start of %q", s)
	}
	rest := strings.TrimSpace(s[len(op):])
	val, err := parseScalar(rest)
	if err != nil {
		return "", nil, err
	}
	return op, val, nil
}

func parseScalar(s string) (any, error) {
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}
	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		if len(s) < 2 || s[len(s)-1] != quote {
			return nil, fmt.Errorf("unterminated string %q", s)
		}
		return s[1 : len(s)-1], nil
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	if validIdent(s) {
		return s, nil
	}
	return nil, fmt.Errorf("invalid scalar %q", s)
}

func validPath(s string) bool {
	for _, part := range strings.Split(s, ".") {
		if !validIdent(part) {
			return false
		}
	}
	return true
}

func validIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if i > 0 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
