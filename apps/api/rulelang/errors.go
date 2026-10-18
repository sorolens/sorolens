package rulelang

import (
	"fmt"
	"sort"
	"strings"
)

// Error is a validation or parse failure. It always carries the byte offset
// into the rule source and, where useful, a Hint that tells the author how to
// fix the problem. Errors are meant to be shown verbatim in the dashboard
// editor, so the messages read as English rather than Go.
type Error struct {
	// Msg is the human-readable description of the problem.
	Msg string
	// Hint is an optional suggestion (for example a "did you mean" metric).
	Hint string
	// Pos is the byte offset into Source where the problem starts.
	Pos int
	// Source is the original rule text, used to derive line:column.
	Source string
}

// Error implements the error interface. The format is
// "line:col: message" followed by an optional " (hint)" suffix.
func (e *Error) Error() string {
	line, col := e.LineCol()
	msg := fmt.Sprintf("%d:%d: %s", line, col, e.Msg)
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

// LineCol converts the byte offset into a 1-based line and column.
func (e *Error) LineCol() (int, int) {
	line, col := 1, 1
	for i := 0; i < e.Pos && i < len(e.Source); i++ {
		if e.Source[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}

// Errorf builds an Error at pos.
func Errorf(source string, pos int, hint, format string, args ...any) *Error {
	return &Error{
		Msg:    fmt.Sprintf(format, args...),
		Hint:   hint,
		Pos:    pos,
		Source: source,
	}
}

// suggest returns the candidate closest to want (case-insensitive Levenshtein
// distance), or "" when nothing is close enough to be worth suggesting.
func suggest(want string, candidates []string) string {
	best := ""
	bestDist := -1
	lower := strings.ToLower(want)
	for _, c := range candidates {
		d := levenshtein(lower, strings.ToLower(c))
		if bestDist == -1 || d < bestDist {
			bestDist, best = d, c
		}
	}
	// Only suggest when the edit distance is small relative to the word, so
	// a wildly wrong name does not get a misleading suggestion.
	limit := len(want)/2 + 1
	if bestDist >= 0 && bestDist <= limit {
		return best
	}
	return ""
}

// levenshtein returns the edit distance between a and b.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// sortedNames returns the map keys in sorted order, for deterministic output.
func sortedNames(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
