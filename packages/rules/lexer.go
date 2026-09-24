package rules

import (
	"fmt"
	"strings"
)

// tokenKind enumerates lexical token types.
type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokOp
	tokLParen
	tokRParen
	tokStar
)

// token is a single lexical unit with its source position for error messages.
type token struct {
	kind tokenKind
	text string
	pos  int // byte offset in the input
}

func (t token) String() string {
	if t.kind == tokEOF {
		return "end of expression"
	}
	return fmt.Sprintf("%q", t.text)
}

// lex converts an expression into a slice of tokens terminated by tokEOF.
// It returns a friendly error with the offending position when the input
// contains a character the grammar does not use.
func lex(input string) ([]token, error) {
	var toks []token
	i := 0
	n := len(input)

	advanceKw := func() token {
		start := i
		for i < n {
			c := input[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '_' {
				i++
				continue
			}
			break
		}
		return token{kind: tokIdent, text: input[start:i], pos: start}
	}

	for i < n {
		c := input[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			toks = append(toks, token{kind: tokLParen, text: "(", pos: i})
			i++
		case c == ')':
			toks = append(toks, token{kind: tokRParen, text: ")", pos: i})
			i++
		case c == '*':
			toks = append(toks, token{kind: tokStar, text: "*", pos: i})
			i++
		case c == '=':
			if i+1 < n && input[i+1] == '=' {
				toks = append(toks, token{kind: tokOp, text: "==", pos: i})
				i += 2
			} else {
				toks = append(toks, token{kind: tokOp, text: "=", pos: i})
				i++
			}
		case c == '>' || c == '<':
			if i+1 < n && input[i+1] == '=' {
				toks = append(toks, token{kind: tokOp, text: string([]rune{rune(c), '='}), pos: i})
				i += 2
			} else {
				toks = append(toks, token{kind: tokOp, text: string(c), pos: i})
				i++
			}
		case c == '!':
			if i+1 < n && input[i+1] == '=' {
				toks = append(toks, token{kind: tokOp, text: "!=", pos: i})
				i += 2
			} else {
				return nil, parseErr(input, i, "unexpected character '!'; did you mean \"!=\"?")
			}
		case c >= '0' && c <= '9':
			// A numeric run may be trailed directly by letters for compact
			// durations (e.g. "5m", "1h30m"). Split it so the parser always
			// sees NUMBER followed by an IDENT unit token.
			start := i
			seenDot := false
			for i < n {
				dc := input[i]
				if dc >= '0' && dc <= '9' {
					i++
					continue
				}
				if dc == '.' && !seenDot {
					seenDot = true
					i++
					continue
				}
				break
			}
			toks = append(toks, token{kind: tokNumber, text: input[start:i], pos: start})
			if i < n && isIdentStart(input[i]) {
				lstart := i
				for i < n {
					c := input[i]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
						i++
						continue
					}
					break
				}
				toks = append(toks, token{kind: tokIdent, text: input[lstart:i], pos: lstart})
			}
		case isIdentStart(c):
			toks = append(toks, advanceKw())
		case c == '.':
			return nil, parseErr(input, i, "unexpected '.'; use a decimal number like \"0.5\" without a leading '.'")
		default:
			return nil, parseErr(input, i, fmt.Sprintf("unexpected character %q", string(c)))
		}
	}
	toks = append(toks, token{kind: tokEOF, pos: n})
	return toks, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

// parseErr builds a human-readable error that includes the source position.
func parseErr(input string, pos int, msg string) error {
	line := 1
	col := 1
	for i := 0; i < pos && i < len(input); i++ {
		if input[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return fmt.Errorf("line %d, column %d: %s", line, col, msg)
}

// joinOptions renders a sorted-ish list of choices for error messages.
func joinOptions(opts []string) string {
	return strings.Join(opts, ", ")
}