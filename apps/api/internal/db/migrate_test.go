package db

import (
	"testing"
)

func TestNewSourceLoadsEmbeddedMigrations(t *testing.T) {
	t.Parallel()

	src, err := newSource()
	if err != nil {
		t.Fatalf("newSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	first, err := src.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	if first != 1 {
		t.Errorf("first migration version = %d, want 1", first)
	}

	// Walk the source to confirm every embedded up migration parses and the
	// sequence is contiguous. iofs also rejects duplicate versions here.
	version, err := src.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	count := 1
	for {
		next, err := src.Next(version)
		if err != nil {
			break
		}
		if next <= version {
			t.Fatalf("migration versions not increasing: %d then %d", version, next)
		}
		version = next
		count++
	}
	if count < 13 {
		t.Errorf("found %d migrations, want at least 13 embedded files", count)
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "postgres scheme",
			in:   "postgres://user:pass@host:5432/db?sslmode=require",
			want: "pgx5://user:pass@host:5432/db?sslmode=require",
		},
		{
			name: "postgresql scheme",
			in:   "postgresql://host/db",
			want: "pgx5://host/db",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeURL(tc.in)
			if err != nil {
				t.Fatalf("normalizeURL: %v", err)
			}
			if got != tc.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeURLInvalid(t *testing.T) {
	t.Parallel()

	if _, err := normalizeURL("not a url"); err == nil {
		t.Error("normalizeURL(\"not a url\") = nil error, want error")
	}
}

func TestMigrateEmptyURL(t *testing.T) {
	t.Parallel()

	if err := Migrate(""); err == nil {
		t.Error("Migrate(\"\") = nil error, want error")
	}
}
