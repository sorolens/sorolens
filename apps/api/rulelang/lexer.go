package rulelang

import (
	"strings"
	"unicode"
)

// tokenKind enumerates the lexical classes of the rule language.
type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber // numeric literal (the unit, if any, is a separate ident)
	tokDuration
	tokString
	tokLParen
	tokRParen
	tokGT
	tokGE
	tokLT
	tokLE
	tokEQ
	tokNE
)

// token is one lexeme with its byte offset into the source.
type token struct {
	kind tokenKind
	text string
	pos  int
}

func (k tokenKind) String() string {
	switch k {
	case tokEOF:
		return "end of rule"
	case tokIdent:
		return "identifier"
	case tokNumber:
		return "number"
	case tokDuration:
		return "duration"
	case tokString:
		return "quoted string"
	case tokLParen:
		return "("
	case tokRParen:
		return ")"
	case tokGT:
		return ">"
	case tokGE:
		return ">="
	case tokLT:
		return "<"
	case tokLE:
		return "<="
	case tokEQ:
		return "=="
	case tokNE:
		return "!="
	}
	return "token"
}

// lexer turns a rule source string into a token slice. It is deliberately
// small: the language has no comments, and whitespace only matters for
// separating a number from a unit word.
type lexer struct {
	src  string
	pos  int
	toks []token
}

// lex tokenizes src, returning a friendly Error on an unexpected character.
func lex(src string) ([]token, *Error) {
	l := &lexer{src: src}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case isSpace(c):
			l.pos++
		case c == '(':
			l.emit(tokLParen, "(", 1)
		case c == ')':
			l.emit(tokRParen, ")", 1)
		case c == '>':
			if l.peekAt(1) == '=' {
				l.emit(tokGE, ">=", 2)
			} else {
				l.emit(tokGT, ">", 1)
			}
		case c == '<':
			if l.peekAt(1) == '=' {
				l.emit(tokLE, "<=", 2)
			} else {
				l.emit(tokLT, "<", 1)
			}
		case c == '=':
			if l.peekAt(1) == '=' {
				l.emit(tokEQ, "==", 2)
			} else {
				// A single '=' is a common mistake; accept it as equality but
				// the parser records it so the hint can suggest '=='.
				l.emit(tokEQ, "=", 1)
			}
		case c == '!':
			if l.peekAt(1) == '=' {
				l.emit(tokNE, "!=", 2)
			} else {
				return nil, Errorf(src, l.pos, "use != for not-equal, or == for equal", "unexpected character %q", string(c))
			}
		case c == '"' || c == '\'':
			t, err := l.lexString(c)
			if err != nil {
				return nil, err
			}
			l.toks = append(l.toks, t)
		case c >= '0' && c <= '9' || c == '.':
			t, err := l.lexNumber()
			if err != nil {
				return nil, err
			}
			l.toks = append(l.toks, t)
		case isIdentStart(rune(c)):
			l.lexIdent()
		default:
			return nil, Errorf(src, l.pos, "", "unexpected character %q", string(c))
		}
	}
	l.toks = append(l.toks, token{kind: tokEOF, pos: len(src)})
	return l.toks, nil
}

func (l *lexer) emit(kind tokenKind, text string, width int) {
	l.toks = append(l.toks, token{kind: kind, text: text, pos: l.pos})
	l.pos += width
}

func (l *lexer) peekAt(n int) byte {
	if l.pos+n < len(l.src) {
		return l.src[l.pos+n]
	}
	return 0
}

// lexString reads a quoted string. Both single and double quotes are accepted
// so contract IDs can be written bare or quoted.
func (l *lexer) lexString(quote byte) (token, *Error) {
	start := l.pos
	l.pos++ // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == quote {
			l.pos++
			return token{kind: tokString, text: b.String(), pos: start}, nil
		}
		b.WriteByte(c)
		l.pos++
	}
	return token{}, Errorf(l.src, start, "close the string with "+string(quote), "unterminated string")
}

// lexNumber reads an integer, decimal, or scientific-notation number. A unit
// suffix written with no space (`5m`, `30s`, `1h`, `2d`) becomes a duration
// token; a following word separated by whitespace (`0.5 XLM`) stays an ident.
func (l *lexer) lexNumber() (token, *Error) {
	start := l.pos
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '.') {
		l.pos++
	}
	// Scientific notation: 5e6, 5E6, 1e-3.
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		next := l.peekAt(1)
		if isDigit(next) || ((next == '+' || next == '-') && isDigit(l.peekAt(2))) {
			l.pos++
			if l.src[l.pos] == '+' || l.src[l.pos] == '-' {
				l.pos++
			}
			for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
				l.pos++
			}
		}
	}
	num := l.src[start:l.pos]
	// A percent sign immediately after the number is part of the literal.
	if l.pos < len(l.src) && l.src[l.pos] == '%' {
		l.pos++
		return token{kind: tokNumber, text: num + "%", pos: start}, nil
	}
	// Adjacent letters make a duration, e.g. 5m. Anything else stays numeric.
	if l.pos < len(l.src) && isLetter(l.src[l.pos]) {
		sufStart := l.pos
		for l.pos < len(l.src) && isLetter(l.src[l.pos]) {
			l.pos++
		}
		suffix := l.src[sufStart:l.pos]
		if isDurationUnit(suffix) {
			return token{kind: tokDuration, text: num + suffix, pos: start}, nil
		}
		return token{}, Errorf(l.src, start, "durations are written like 30s, 5m, 1h or 2d",
			"unknown duration unit %q", suffix)
	}
	return token{kind: tokNumber, text: num, pos: start}, nil
}

// lexIdent reads an identifier, which covers metric names, aggregation
// functions and keywords.
func (l *lexer) lexIdent() {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(rune(l.src[l.pos])) {
		l.pos++
	}
	l.toks = append(l.toks, token{kind: tokIdent, text: l.src[start:l.pos], pos: start})
}

func isSpace(c byte) bool      { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func isLetter(c byte) bool     { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isIdentPart(r rune) bool  { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
func isDurationUnit(s string) bool {
	switch strings.ToLower(s) {
	case "s", "m", "h", "d", "w":
		return true
	}
	return false
}
