package format

import (
	"fmt"
	"strconv"
	"strings"
)

// Query is a parsed jq-style path expression. It is a small, pure-Go subset
// of jq: `.`, `.key`, `."quoted key"`, `[N]` (negative counts from the end),
// `[]` (iterate) and `|` to chain paths, e.g. `.items[] | .name`. It never
// runs external programs.
type Query struct {
	pipes [][]step
}

type stepKind int

const (
	stepKey stepKind = iota
	stepIndex
	stepIter
)

type step struct {
	kind stepKind
	key  string
	idx  int
	pos  int // 1-based offset of the step in the expression
}

// QueryError reports a syntax or runtime error with a 1-based position in the
// original expression.
type QueryError struct {
	Pos int
	Msg string
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query error at position %d: %s", e.Pos, e.Msg)
}

// ParseQuery parses expr.
func ParseQuery(expr string) (*Query, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, &QueryError{Pos: 1, Msg: "empty query"}
	}
	q := &Query{}
	var cur []step
	pos := 0
	n := len(expr)
	flush := func() { q.pipes = append(q.pipes, cur); cur = nil }
	expectPath := true
	for pos < n {
		c := expr[pos]
		switch {
		case c == ' ' || c == '\t':
			pos++
		case c == '|':
			if expectPath {
				return nil, &QueryError{Pos: pos + 1, Msg: "unexpected '|'"}
			}
			flush()
			expectPath = true
			pos++
		case c == '.':
			expectPath = false
			pos++
			if pos < n && expr[pos] == '"' {
				key, next, err := readQuoted(expr, pos)
				if err != nil {
					return nil, err
				}
				cur = append(cur, step{kind: stepKey, key: key, pos: pos + 1})
				pos = next
			} else if pos < n && isIdentChar(expr[pos]) {
				start := pos
				for pos < n && isIdentChar(expr[pos]) {
					pos++
				}
				cur = append(cur, step{kind: stepKey, key: expr[start:pos], pos: start})
			}
		case c == '[':
			if expectPath {
				return nil, &QueryError{Pos: pos + 1, Msg: "path must start with '.'"}
			}
			end := strings.IndexByte(expr[pos:], ']')
			if end < 0 {
				return nil, &QueryError{Pos: pos + 1, Msg: "unterminated '['"}
			}
			inner := strings.TrimSpace(expr[pos+1 : pos+end])
			switch {
			case inner == "":
				cur = append(cur, step{kind: stepIter, pos: pos + 1})
			case inner[0] == '"':
				key, _, err := readQuoted(inner, 0)
				if err != nil {
					return nil, &QueryError{Pos: pos + 2, Msg: "invalid quoted key"}
				}
				cur = append(cur, step{kind: stepKey, key: key})
			default:
				i, err := strconv.Atoi(inner)
				if err != nil {
					return nil, &QueryError{Pos: pos + 2, Msg: fmt.Sprintf("invalid index %q", inner)}
				}
				cur = append(cur, step{kind: stepIndex, idx: i, pos: pos + 1})
			}
			pos += end + 1
		default:
			return nil, &QueryError{Pos: pos + 1, Msg: fmt.Sprintf("unexpected character %q", string(c))}
		}
	}
	if expectPath {
		return nil, &QueryError{Pos: n, Msg: "expression ends after '|'"}
	}
	flush()
	return q, nil
}

func isIdentChar(c byte) bool {
	return c == '_' || c == '-' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func readQuoted(s string, pos int) (string, int, error) {
	end := pos + 1
	for end < len(s) && s[end] != '"' {
		end++
	}
	if end >= len(s) {
		return "", 0, &QueryError{Pos: pos + 1, Msg: "unterminated string"}
	}
	return s[pos+1 : end], end + 1, nil
}

// Run applies the query to generic JSON data and returns every result.
func (q *Query) Run(data any) ([]any, error) {
	values := []any{data}
	for _, steps := range q.pipes {
		var next []any
		for _, v := range values {
			out, err := applySteps(steps, v)
			if err != nil {
				return nil, err
			}
			next = append(next, out...)
		}
		values = next
	}
	return values, nil
}

func applySteps(steps []step, v any) ([]any, error) {
	cur := []any{v}
	for _, s := range steps {
		var next []any
		for _, c := range cur {
			switch s.kind {
			case stepKey:
				m, ok := c.(map[string]any)
				if !ok {
					if c == nil {
						next = append(next, nil)
						continue
					}
					return nil, &QueryError{Pos: s.pos, Msg: fmt.Sprintf("cannot index %s with key %q", typeName(c), s.key)}
				}
				next = append(next, m[s.key])
			case stepIndex:
				a, ok := c.([]any)
				if !ok {
					return nil, &QueryError{Pos: s.pos, Msg: fmt.Sprintf("cannot index %s with number", typeName(c))}
				}
				i := s.idx
				if i < 0 {
					i += len(a)
				}
				if i < 0 || i >= len(a) {
					next = append(next, nil)
				} else {
					next = append(next, a[i])
				}
			case stepIter:
				switch t := c.(type) {
				case []any:
					next = append(next, t...)
				default:
					return nil, &QueryError{Pos: s.pos, Msg: fmt.Sprintf("cannot iterate over %s", typeName(c))}
				}
			}
		}
		cur = next
	}
	return cur, nil
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	default:
		return "number"
	}
}
