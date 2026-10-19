package store

import (
	"testing"
	"time"
)

// FuzzAlertsCursor verifies that decodeAlertsCursor never panics on arbitrary input
// and preserves round-trip equality for valid (time, contractID) pairs.
func FuzzAlertsCursor(f *testing.F) {
	seeds := []string{
		"",
		" ",
		"not-base64",
		"AAAA",
		"invalid|format",
		"2026-01-01|id",
		"2026-01-01T00:00:00Z|",
		"|id",
		"|",
		"||",
		encodeAlertsCursor(time.Now().UTC(), "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE1"),
		encodeAlertsCursor(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "C123"),
		encodeAlertsCursor(time.Unix(0, 0).UTC(), "C000"),
		encodeAlertsCursor(time.Date(2000, 12, 31, 23, 59, 59, 999999999, time.UTC), "C999"),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, cursor string) {
		ts, id, err := decodeAlertsCursor(cursor)
		if err != nil {
			return
		}

		if ts.IsZero() || id == "" {
			t.Errorf("decodeAlertsCursor returned success but empty values: ts=%v, id=%q", ts, id)
		}

		// Round-trip verification
		reEncoded := encodeAlertsCursor(ts, id)
		ts2, id2, err2 := decodeAlertsCursor(reEncoded)
		if err2 != nil {
			t.Fatalf("failed to decode re-encoded alerts cursor: %v", err2)
		}
		if !ts.Equal(ts2) || id != id2 {
			t.Errorf("roundtrip alerts cursor mismatch: got (%v, %q), want (%v, %q)", ts2, id2, ts, id)
		}
	})
}

// FuzzGroupCursor verifies that decodeGroupCursor never panics on arbitrary input
// and preserves round-trip equality for valid (time, id) pairs.
func FuzzGroupCursor(f *testing.F) {
	seeds := []string{
		"",
		" ",
		"not-base64",
		"AAA",
		"bad|123",
		"2026-01-01T00:00:00Z|notanint",
		"2026-01-01T00:00:00Z|999999999999999999999999999",
		"|",
		"||",
		encodeGroupCursor(time.Now().UTC(), 1),
		encodeGroupCursor(time.Now().UTC(), 999999999),
		encodeGroupCursor(time.Unix(0, 0).UTC(), 0),
		encodeGroupCursor(time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), 42),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, cursor string) {
		ts, id, err := decodeGroupCursor(cursor)
		if err != nil {
			return
		}

		if ts.IsZero() {
			t.Errorf("decodeGroupCursor returned success but zero timestamp: ts=%v, id=%d", ts, id)
		}

		// Round-trip verification
		reEncoded := encodeGroupCursor(ts, id)
		ts2, id2, err2 := decodeGroupCursor(reEncoded)
		if err2 != nil {
			t.Fatalf("failed to decode re-encoded group cursor: %v", err2)
		}
		if !ts.Equal(ts2) || id != id2 {
			t.Errorf("roundtrip group cursor mismatch: got (%v, %d), want (%v, %d)", ts2, id2, ts, id)
		}
	})
}
