package allowlist_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/allowlist"
)

func TestParseCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr error
	}{
		{name: "empty list", input: nil, want: []string{}},
		{
			name:  "ipv4 range keeps network address",
			input: []string{"203.0.113.0/24"},
			want:  []string{"203.0.113.0/24"},
		},
		{
			name:  "host bits are cleared",
			input: []string{"203.0.113.7/24"},
			want:  []string{"203.0.113.0/24"},
		},
		{
			name:  "bare ipv4 becomes a single host range",
			input: []string{"198.51.100.7"},
			want:  []string{"198.51.100.7/32"},
		},
		{
			name:  "ipv6 range",
			input: []string{"2001:db8::/32"},
			want:  []string{"2001:db8::/32"},
		},
		{
			name:  "bare ipv6 becomes a single host range",
			input: []string{"2001:db8::5"},
			want:  []string{"2001:db8::5/128"},
		},
		{
			name:  "duplicates are dropped and order preserved",
			input: []string{"203.0.113.0/24", " 203.0.113.9/24 ", "10.0.0.0/8"},
			want:  []string{"203.0.113.0/24", "10.0.0.0/8"},
		},
		{name: "empty entry", input: []string{"203.0.113.0/24", "  "}, wantErr: allowlist.ErrInvalidCIDR},
		{name: "not an address", input: []string{"example.com"}, wantErr: allowlist.ErrInvalidCIDR},
		{name: "bad mask", input: []string{"203.0.113.0/33"}, wantErr: allowlist.ErrInvalidCIDR},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := allowlist.ParseCIDRs(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseCIDRs(%v) error = %v, want %v", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseCIDRs(%v): %v", tc.input, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ParseCIDRs(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseOrigins(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr error
	}{
		{name: "empty list", input: nil, want: []string{}},
		{
			name:  "scheme and host are lower-cased",
			input: []string{"HTTPS://App.Example.com"},
			want:  []string{"https://app.example.com"},
		},
		{
			name:  "trailing slash is dropped and duplicates collapse",
			input: []string{"https://app.example.com/", "https://app.example.com"},
			want:  []string{"https://app.example.com"},
		},
		{
			name:  "port is kept",
			input: []string{"http://localhost:3000"},
			want:  []string{"http://localhost:3000"},
		},
		{name: "missing scheme", input: []string{"app.example.com"}, wantErr: allowlist.ErrInvalidOrigin},
		{name: "wildcard", input: []string{"https://*.example.com"}, wantErr: allowlist.ErrInvalidOrigin},
		{name: "path", input: []string{"https://app.example.com/keys"}, wantErr: allowlist.ErrInvalidOrigin},
		{name: "query", input: []string{"https://app.example.com?x=1"}, wantErr: allowlist.ErrInvalidOrigin},
		{name: "non-http scheme", input: []string{"file://tmp"}, wantErr: allowlist.ErrInvalidOrigin},
		{name: "empty entry", input: []string{""}, wantErr: allowlist.ErrInvalidOrigin},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := allowlist.ParseOrigins(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseOrigins(%v) error = %v, want %v", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOrigins(%v): %v", tc.input, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ParseOrigins(%v) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestIPAllowed(t *testing.T) {
	tests := []struct {
		name     string
		entries  []string
		clientIP string
		want     bool
	}{
		{name: "no restriction", entries: nil, clientIP: "203.0.113.9", want: true},
		{name: "ipv4 inside range", entries: []string{"203.0.113.0/24"}, clientIP: "203.0.113.9", want: true},
		{name: "ipv4 outside range", entries: []string{"203.0.113.0/24"}, clientIP: "203.0.114.9", want: false},
		{name: "single host match", entries: []string{"198.51.100.7/32"}, clientIP: "198.51.100.7", want: true},
		{name: "single host mismatch", entries: []string{"198.51.100.7/32"}, clientIP: "198.51.100.8", want: false},
		{name: "ipv6 inside range", entries: []string{"2001:db8::/32"}, clientIP: "2001:db8::5", want: true},
		{name: "ipv6 outside range", entries: []string{"2001:db8::/32"}, clientIP: "2001:db9::5", want: false},
		{name: "ipv6 loopback", entries: []string{"::1/128"}, clientIP: "::1", want: true},
		{
			name:     "ipv4-mapped ipv6 client matches ipv4 range",
			entries:  []string{"203.0.113.0/24"},
			clientIP: "::ffff:203.0.113.9",
			want:     true,
		},
		{name: "second entry matches", entries: []string{"10.0.0.0/8", "203.0.113.9/32"}, clientIP: "203.0.113.9", want: true},
		{name: "unparsable client fails closed", entries: []string{"0.0.0.0/0"}, clientIP: "unknown", want: false},
		{name: "unparsable entry is skipped", entries: []string{"nope", "203.0.113.9/32"}, clientIP: "203.0.113.9", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := allowlist.IPAllowed(tc.entries, tc.clientIP); got != tc.want {
				t.Fatalf("IPAllowed(%v, %q) = %v, want %v", tc.entries, tc.clientIP, got, tc.want)
			}
		})
	}
}

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name    string
		entries []string
		origin  string
		want    bool
	}{
		{name: "no restriction", entries: nil, origin: "https://evil.example", want: true},
		{name: "exact match", entries: []string{"https://app.example.com"}, origin: "https://app.example.com", want: true},
		{name: "case-insensitive match", entries: []string{"https://app.example.com"}, origin: "HTTPS://APP.EXAMPLE.COM", want: true},
		{name: "other origin", entries: []string{"https://app.example.com"}, origin: "https://evil.example", want: false},
		{name: "different port", entries: []string{"https://app.example.com"}, origin: "https://app.example.com:8443", want: false},
		{name: "different scheme", entries: []string{"https://app.example.com"}, origin: "http://app.example.com", want: false},
		{
			name:    "subdomain is not covered by a bare host",
			entries: []string{"https://example.com"},
			origin:  "https://app.example.com",
			want:    false,
		},
		{name: "no origin header is not a browser request", entries: []string{"https://app.example.com"}, origin: "", want: true},
		{name: "null origin is rejected", entries: []string{"https://app.example.com"}, origin: "null", want: false},
		{
			name:    "one of several entries matches",
			entries: []string{"https://app.example.com", "http://localhost:3000"},
			origin:  "http://localhost:3000",
			want:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := allowlist.OriginAllowed(tc.entries, tc.origin); got != tc.want {
				t.Fatalf("OriginAllowed(%v, %q) = %v, want %v", tc.entries, tc.origin, got, tc.want)
			}
		})
	}
}
