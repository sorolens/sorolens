package timerange_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/timerange"
)

func TestParseQuery(t *testing.T) {
	opts := timerange.DefaultOptions()

	t.Run("valid RFC 3339 with Z", func(t *testing.T) {
		q := url.Values{
			"from": []string{"2026-06-01T12:00:00Z"},
			"to":   []string{"2026-06-02T12:00:00Z"},
		}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFrom := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
		if !rng.From.Equal(wantFrom) || rng.From.Location() != time.UTC {
			t.Errorf("got From %v (%v), want %v (UTC)", rng.From, rng.From.Location(), wantFrom)
		}
		if !rng.To.Equal(wantTo) || rng.To.Location() != time.UTC {
			t.Errorf("got To %v (%v), want %v (UTC)", rng.To, rng.To.Location(), wantTo)
		}
	})

	t.Run("offset timestamps normalized to UTC", func(t *testing.T) {
		// +02:00 -> 14:00 is 12:00 UTC
		// -05:00 -> 07:00 is 12:00 UTC
		q := url.Values{
			"from": []string{"2026-06-01T14:00:00+02:00"},
			"to":   []string{"2026-06-02T07:00:00-05:00"},
		}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFrom := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
		if !rng.From.Equal(wantFrom) || rng.From.Location() != time.UTC {
			t.Errorf("got From %v (%v), want %v (UTC)", rng.From, rng.From.Location(), wantFrom)
		}
		if !rng.To.Equal(wantTo) || rng.To.Location() != time.UTC {
			t.Errorf("got To %v (%v), want %v (UTC)", rng.To, rng.To.Location(), wantTo)
		}
	})

	t.Run("boundary values from equal to to", func(t *testing.T) {
		q := url.Values{
			"from": []string{"2026-06-01T12:00:00Z"},
			"to":   []string{"2026-06-01T12:00:00Z"},
		}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		if !rng.From.Equal(want) || !rng.To.Equal(want) {
			t.Errorf("got %v - %v, want both %v", rng.From, rng.To, want)
		}
	})

	t.Run("inverted range rejected", func(t *testing.T) {
		q := url.Values{
			"from": []string{"2026-06-02T12:00:00Z"},
			"to":   []string{"2026-06-01T12:00:00Z"},
		}
		_, err := timerange.ParseQuery(q, opts)
		if err == nil {
			t.Fatal("expected error on inverted range, got nil")
		}
		wantMsg := "to must not be before from"
		if err.Error() != wantMsg {
			t.Errorf("got error message %q, want %q", err.Error(), wantMsg)
		}
	})

	t.Run("inverted range with aliases", func(t *testing.T) {
		q := url.Values{
			"since": []string{"2026-06-02T12:00:00Z"},
			"until": []string{"2026-06-01T12:00:00Z"},
		}
		_, err := timerange.ParseQuery(q, opts)
		if err == nil {
			t.Fatal("expected error on inverted range, got nil")
		}
		wantMsg := "until must not be before since"
		if err.Error() != wantMsg {
			t.Errorf("got error message %q, want %q", err.Error(), wantMsg)
		}
	})

	t.Run("invalid from format", func(t *testing.T) {
		q := url.Values{
			"from": []string{"not-a-timestamp"},
			"to":   []string{"2026-06-02T12:00:00Z"},
		}
		_, err := timerange.ParseQuery(q, opts)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		wantMsg := "from must be an RFC 3339 timestamp"
		if err.Error() != wantMsg {
			t.Errorf("got error %q, want %q", err.Error(), wantMsg)
		}
	})

	t.Run("invalid to format", func(t *testing.T) {
		q := url.Values{
			"from": []string{"2026-06-01T12:00:00Z"},
			"to":   []string{"yesterday"},
		}
		_, err := timerange.ParseQuery(q, opts)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		wantMsg := "to must be an RFC 3339 timestamp"
		if err.Error() != wantMsg {
			t.Errorf("got error %q, want %q", err.Error(), wantMsg)
		}
	})

	t.Run("invalid since alias format", func(t *testing.T) {
		q := url.Values{
			"since": []string{"invalid-date"},
		}
		_, err := timerange.ParseQuery(q, opts)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		wantMsg := "since must be an RFC 3339 timestamp"
		if err.Error() != wantMsg {
			t.Errorf("got error %q, want %q", err.Error(), wantMsg)
		}
	})

	t.Run("invalid until alias format", func(t *testing.T) {
		q := url.Values{
			"until": []string{"invalid-date"},
		}
		_, err := timerange.ParseQuery(q, opts)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		wantMsg := "until must be an RFC 3339 timestamp"
		if err.Error() != wantMsg {
			t.Errorf("got error %q, want %q", err.Error(), wantMsg)
		}
	})

	t.Run("absent bounds are unbounded", func(t *testing.T) {
		q := url.Values{}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rng.From.IsZero() {
			t.Errorf("expected zero From, got %v", rng.From)
		}
		if !rng.To.IsZero() {
			t.Errorf("expected zero To, got %v", rng.To)
		}
		if !rng.IsZero() {
			t.Error("expected IsZero() to be true")
		}
	})

	t.Run("only from provided", func(t *testing.T) {
		q := url.Values{"from": []string{"2026-06-01T12:00:00Z"}}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFrom := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		if !rng.From.Equal(wantFrom) {
			t.Errorf("got From %v, want %v", rng.From, wantFrom)
		}
		if !rng.To.IsZero() {
			t.Errorf("expected zero To, got %v", rng.To)
		}
	})

	t.Run("only to provided", func(t *testing.T) {
		q := url.Values{"to": []string{"2026-06-02T12:00:00Z"}}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rng.From.IsZero() {
			t.Errorf("expected zero From, got %v", rng.From)
		}
		wantTo := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
		if !rng.To.Equal(wantTo) {
			t.Errorf("got To %v, want %v", rng.To, wantTo)
		}
	})

	t.Run("deprecated alias still works", func(t *testing.T) {
		q := url.Values{
			"since": []string{"2026-06-01T12:00:00Z"},
			"until": []string{"2026-06-02T12:00:00Z"},
		}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFrom := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
		if !rng.From.Equal(wantFrom) {
			t.Errorf("got From %v, want %v", rng.From, wantFrom)
		}
		if !rng.To.Equal(wantTo) {
			t.Errorf("got To %v, want %v", rng.To, wantTo)
		}
	})

	t.Run("when both primary and alias are sent, primary wins", func(t *testing.T) {
		q := url.Values{
			"from":  []string{"2026-06-01T10:00:00Z"},
			"since": []string{"2026-01-01T00:00:00Z"},
			"to":    []string{"2026-06-02T10:00:00Z"},
			"until": []string{"2026-12-31T23:59:59Z"},
		}
		rng, err := timerange.ParseQuery(q, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFrom := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
		wantTo := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
		if !rng.From.Equal(wantFrom) {
			t.Errorf("got From %v (since was used?), want %v (from wins)", rng.From, wantFrom)
		}
		if !rng.To.Equal(wantTo) {
			t.Errorf("got To %v (until was used?), want %v (to wins)", rng.To, wantTo)
		}
	})
}

func TestParseRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?from=2026-06-01T12:00:00Z&to=2026-06-02T12:00:00Z", nil)
	rng, err := timerange.ParseRequest(req, timerange.DefaultOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantFrom := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	if !rng.From.Equal(wantFrom) || !rng.To.Equal(wantTo) {
		t.Errorf("got %v - %v, want %v - %v", rng.From, rng.To, wantFrom, wantTo)
	}
}
