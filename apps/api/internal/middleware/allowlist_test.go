package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestAPIKeyAllowlistEnforcement is table-driven over IPv4, IPv6, proxy and
// Origin cases (issue #793). Each case seeds one key with the given allowlists
// and asserts the Identity middleware's decision for a request made from the
// described address.
func TestAPIKeyAllowlistEnforcement(t *testing.T) {
	const token = "sl_allowlist_test_token"

	tests := []struct {
		name       string
		cidrs      []string
		origins    []string
		remoteAddr string
		forwarded  string
		trusted    []string
		origin     string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "empty allowlists keep current behaviour",
			remoteAddr: "198.51.100.7:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IPv4 inside range",
			cidrs:      []string{"203.0.113.0/24"},
			remoteAddr: "203.0.113.42:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IPv4 outside range",
			cidrs:      []string{"203.0.113.0/24"},
			remoteAddr: "198.51.100.7:1234",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyIPNotAllowed,
		},
		{
			name:       "IPv4 in a second listed range",
			cidrs:      []string{"203.0.113.0/24", "192.0.2.0/24"},
			remoteAddr: "192.0.2.9:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IPv6 inside range",
			cidrs:      []string{"2001:db8::/32"},
			remoteAddr: "[2001:db8::1]:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "IPv6 outside range",
			cidrs:      []string{"2001:db8::/32"},
			remoteAddr: "[2001:db9::1]:1234",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyIPNotAllowed,
		},
		{
			name:       "trusted proxy forwards an allowed client",
			cidrs:      []string{"203.0.113.0/24"},
			remoteAddr: "10.1.2.3:5555",
			forwarded:  "203.0.113.9",
			trusted:    []string{"10.0.0.0/8"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "trusted proxy forwards a denied client",
			cidrs:      []string{"203.0.113.0/24"},
			remoteAddr: "10.1.2.3:5555",
			forwarded:  "198.51.100.7",
			trusted:    []string{"10.0.0.0/8"},
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyIPNotAllowed,
		},
		{
			name:       "trusted proxy skips its own hop in a forwarded chain",
			cidrs:      []string{"203.0.113.0/24"},
			remoteAddr: "10.1.2.3:5555",
			forwarded:  "203.0.113.9, 10.1.2.3",
			trusted:    []string{"10.0.0.0/8"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "untrusted peer cannot spoof X-Forwarded-For",
			cidrs:      []string{"203.0.113.0/24"},
			remoteAddr: "198.51.100.7:1234",
			forwarded:  "203.0.113.9",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyIPNotAllowed,
		},
		{
			name:       "matching origin",
			origins:    []string{"https://app.example.com"},
			remoteAddr: "198.51.100.7:1234",
			origin:     "https://app.example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "origin match is case-insensitive",
			origins:    []string{"https://app.example.com"},
			remoteAddr: "198.51.100.7:1234",
			origin:     "https://APP.Example.COM",
			wantStatus: http.StatusOK,
		},
		{
			name:       "different origin",
			origins:    []string{"https://app.example.com"},
			remoteAddr: "198.51.100.7:1234",
			origin:     "https://evil.example.com",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyOriginNotAllowed,
		},
		{
			name:       "missing origin on an origin-restricted key",
			origins:    []string{"https://app.example.com"},
			remoteAddr: "198.51.100.7:1234",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyOriginNotAllowed,
		},
		{
			name:       "address allowed but origin denied",
			cidrs:      []string{"203.0.113.0/24"},
			origins:    []string{"https://app.example.com"},
			remoteAddr: "203.0.113.42:1234",
			origin:     "https://evil.example.com",
			wantStatus: http.StatusForbidden,
			wantCode:   CodeAPIKeyOriginNotAllowed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := testLookup()
			m.AddAPIKey(store.APIKey{
				ID:             "key-allowlist",
				Name:           "allowlist",
				KeyHash:        store.HashKey(token),
				Scopes:         []string{store.ScopeReadContracts},
				CreatedAt:      time.Now(),
				AllowedCIDRs:   tc.cidrs,
				AllowedOrigins: tc.origins,
			})

			resolver, err := NewClientIPResolver(tc.trusted)
			if err != nil {
				t.Fatalf("NewClientIPResolver: %v", err)
			}

			req := newRequest(http.MethodGet, "/api/v1/contracts")
			req.RemoteAddr = tc.remoteAddr
			req.Header.Set("Authorization", "Bearer "+token)
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}

			handler := Identity(m, IdentityOptions{ClientIPs: resolver}, nil)(
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantCode == "" {
				return
			}
			var body struct {
				Error struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					RequestID string `json:"request_id"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error envelope %q: %v", rec.Body.String(), err)
			}
			if body.Error.Code != tc.wantCode {
				t.Fatalf("error code = %q, want %q", body.Error.Code, tc.wantCode)
			}
			if body.Error.Message == "" {
				t.Error("error envelope has no message")
			}
		})
	}
}

// TestClientIPResolverNeverTrustsForwardedHeadersByDefault pins the rule that
// without configured proxies the transport peer is authoritative.
func TestClientIPResolverNeverTrustsForwardedHeadersByDefault(t *testing.T) {
	resolver, err := NewClientIPResolver(nil)
	if err != nil {
		t.Fatalf("NewClientIPResolver: %v", err)
	}
	req := newRequest(http.MethodGet, "/")
	req.RemoteAddr = "198.51.100.7:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	if got := resolver.ClientIP(req).String(); got != "198.51.100.7" {
		t.Fatalf("ClientIP = %s, want the transport peer 198.51.100.7", got)
	}
}

func TestNewClientIPResolverRejectsInvalidCIDR(t *testing.T) {
	if _, err := NewClientIPResolver([]string{"10.0.0.0/8", "not-a-cidr"}); err == nil {
		t.Fatal("NewClientIPResolver accepted an invalid CIDR")
	}
}
