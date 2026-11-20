package store_test

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestNormalizeAllowedCIDRs(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr string
	}{
		{
			name: "IPv4 is reduced to its network",
			in:   []string{"203.0.113.7/24"},
			want: []string{"203.0.113.0/24"},
		},
		{
			name: "IPv6 is reduced to its network",
			in:   []string{"2001:db8:1::5/32"},
			want: []string{"2001:db8::/32"},
		},
		{
			name: "duplicates and blanks are dropped and output is sorted",
			in:   []string{" 10.0.0.0/8 ", "", "10.0.0.1/8", "192.0.2.0/24"},
			want: []string{"10.0.0.0/8", "192.0.2.0/24"},
		},
		{
			name: "empty input means unrestricted",
			in:   []string{"", "  "},
			want: nil,
		},
		{
			name:    "bare address is not a CIDR",
			in:      []string{"10.0.0.1"},
			wantErr: "invalid CIDR range",
		},
		{
			name:    "garbage is rejected",
			in:      []string{"not-a-cidr"},
			wantErr: "invalid CIDR range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.NormalizeAllowedCIDRs(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAllowedCIDRs: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeAllowedCIDRs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeAllowedOrigins(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr string
	}{
		{
			name: "scheme and host are lowercased and a trailing slash is ignored",
			in:   []string{"HTTPS://App.Example.com/"},
			want: []string{"https://app.example.com"},
		},
		{
			name: "explicit port is preserved",
			in:   []string{"http://localhost:3000"},
			want: []string{"http://localhost:3000"},
		},
		{
			name: "IPv6 literal origin is accepted",
			in:   []string{"http://[::1]:8080"},
			want: []string{"http://[::1]:8080"},
		},
		{
			name: "duplicates are dropped and output is sorted",
			in:   []string{"https://b.example.com", "https://a.example.com", "https://B.example.com"},
			want: []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name: "empty input means unrestricted",
			in:   nil,
			want: nil,
		},
		{
			name:    "path is rejected",
			in:      []string{"https://app.example.com/dashboard"},
			wantErr: "must not contain",
		},
		{
			name:    "credentials are rejected",
			in:      []string{"https://user:pass@app.example.com"},
			wantErr: "must not contain",
		},
		{
			name:    "non-http scheme is rejected",
			in:      []string{"ftp://app.example.com"},
			wantErr: "http or https",
		},
		{
			name:    "wildcard is not an origin",
			in:      []string{"*"},
			wantErr: "invalid origin",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.NormalizeAllowedOrigins(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAllowedOrigins: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("NormalizeAllowedOrigins = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAPIKeyAllowedClientIP(t *testing.T) {
	tests := []struct {
		name  string
		cidrs []string
		ip    string
		want  bool
	}{
		{name: "no allowlist permits anything", ip: "203.0.113.9", want: true},
		{name: "no allowlist permits an unknown address", ip: "", want: true},
		{name: "IPv4 inside", cidrs: []string{"203.0.113.0/24"}, ip: "203.0.113.9", want: true},
		{name: "IPv4 outside", cidrs: []string{"203.0.113.0/24"}, ip: "203.0.114.9", want: false},
		{name: "IPv6 inside", cidrs: []string{"2001:db8::/32"}, ip: "2001:db8::1", want: true},
		{name: "IPv6 outside", cidrs: []string{"2001:db8::/32"}, ip: "2001:db9::1", want: false},
		{name: "unknown address fails closed", cidrs: []string{"203.0.113.0/24"}, ip: "", want: false},
		{name: "malformed stored entry is ignored", cidrs: []string{"nonsense"}, ip: "203.0.113.9", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := store.APIKey{AllowedCIDRs: tc.cidrs}
			var ip net.IP
			if tc.ip != "" {
				ip = net.ParseIP(tc.ip)
				if ip == nil {
					t.Fatalf("bad test address %q", tc.ip)
				}
			}
			if got := key.AllowsClientIP(ip); got != tc.want {
				t.Fatalf("AllowsClientIP = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAPIKeyAllowedOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origins []string
		origin  string
		want    bool
	}{
		{name: "no allowlist permits any origin", origin: "https://anything.example", want: true},
		{name: "no allowlist permits no origin", origin: "", want: true},
		{name: "exact match", origins: []string{"https://app.example.com"}, origin: "https://app.example.com", want: true},
		{name: "case-insensitive match", origins: []string{"https://app.example.com"}, origin: "https://APP.example.com", want: true},
		{name: "different host", origins: []string{"https://app.example.com"}, origin: "https://api.example.com", want: false},
		{name: "different port", origins: []string{"https://app.example.com"}, origin: "https://app.example.com:8443", want: false},
		{name: "missing origin fails closed", origins: []string{"https://app.example.com"}, origin: "", want: false},
		{name: "null origin fails closed", origins: []string{"https://app.example.com"}, origin: "null", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := store.APIKey{AllowedOrigins: tc.origins}
			if got := key.AllowsOrigin(tc.origin); got != tc.want {
				t.Fatalf("AllowsOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}
