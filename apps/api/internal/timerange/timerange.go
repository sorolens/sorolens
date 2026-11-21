// Package timerange provides query parameter parsing and validation for
// RFC 3339 time ranges across Sorolens list endpoints (issue #799).
//
// Inclusivity:
// Both "from" and "to" bounds are INCLUSIVE: a timestamp t falls within
// the range if and only if:
//
//	(from.IsZero() || !t.Before(from)) && (to.IsZero() || !t.After(to))
//
// This follows the dominant behavior across the Sorolens database queries and
// API endpoints (e.g. ListAllEvents, ListAllInvocations, ListAuditEvents,
// and ledger-range queries).
//
// Timezones:
// Timestamps must conform to RFC 3339 (e.g. 2026-11-20T12:00:00Z or
// 2026-11-20T14:00:00+02:00). Offsets are parsed and normalized to UTC.
//
// Absent Bounds:
// An empty or omitted bound is treated as unbounded (zero time.Time).
package timerange

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Range holds parsed and normalized UTC time range bounds.
type Range struct {
	From time.Time
	To   time.Time
}

// IsZero reports whether both bounds are unset.
func (r Range) IsZero() bool {
	return r.From.IsZero() && r.To.IsZero()
}

// ParseError records a time range validation failure with a user-facing message.
type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

// Options configures parameter names and alias resolution.
type Options struct {
	FromKey      string
	ToKey        string
	FromAliasKey string
	ToAliasKey   string
}

// DefaultOptions returns options for standard "from" and "to" parameters with
// deprecated "since" and "until" aliases.
func DefaultOptions() Options {
	return Options{
		FromKey:      "from",
		ToKey:        "to",
		FromAliasKey: "since",
		ToAliasKey:   "until",
	}
}

// ParseRequest extracts and validates the time range from an HTTP request.
// If both the primary parameter and its alias are present, the primary parameter wins.
func ParseRequest(r *http.Request, opts Options) (Range, error) {
	return ParseQuery(r.URL.Query(), opts)
}

// ParseQuery extracts and validates the time range from URL query values.
// If both the primary parameter and its alias are present, the primary parameter wins.
func ParseQuery(q url.Values, opts Options) (Range, error) {
	if opts.FromKey == "" {
		opts.FromKey = "from"
	}
	if opts.ToKey == "" {
		opts.ToKey = "to"
	}

	fromName := opts.FromKey
	fromVal := strings.TrimSpace(q.Get(opts.FromKey))
	if fromVal == "" && opts.FromAliasKey != "" {
		if aliasVal := strings.TrimSpace(q.Get(opts.FromAliasKey)); aliasVal != "" {
			fromName = opts.FromAliasKey
			fromVal = aliasVal
		}
	}

	toName := opts.ToKey
	toVal := strings.TrimSpace(q.Get(opts.ToKey))
	if toVal == "" && opts.ToAliasKey != "" {
		if aliasVal := strings.TrimSpace(q.Get(opts.ToAliasKey)); aliasVal != "" {
			toName = opts.ToAliasKey
			toVal = aliasVal
		}
	}

	var rng Range
	if fromVal != "" {
		t, err := time.Parse(time.RFC3339, fromVal)
		if err != nil {
			return Range{}, &ParseError{Message: fmt.Sprintf("%s must be an RFC 3339 timestamp", fromName)}
		}
		rng.From = t.UTC()
	}

	if toVal != "" {
		t, err := time.Parse(time.RFC3339, toVal)
		if err != nil {
			return Range{}, &ParseError{Message: fmt.Sprintf("%s must be an RFC 3339 timestamp", toName)}
		}
		rng.To = t.UTC()
	}

	if !rng.From.IsZero() && !rng.To.IsZero() && rng.To.Before(rng.From) {
		return Range{}, &ParseError{Message: fmt.Sprintf("%s must not be before %s", toName, fromName)}
	}

	return rng, nil
}
