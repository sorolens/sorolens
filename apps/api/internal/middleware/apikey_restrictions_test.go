package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const restrictedKeyToken = "sl_restricted_token_for_tests"

func restrictedStore(t *testing.T) *store.MockStore {
	t.Helper()
	return store.NewMockStore()
}

// serveRestricted runs a request through Identity then APIKeyRestrictions,
// which is the order the router uses.
func serveRestricted(t *testing.T, lookup IdentityLookup, trusted []string, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Identity(lookup, IdentityOptions{}, nil)(APIKeyRestrictions(trusted)(ok))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func restrictedRequest(method string) *http.Request {
	return httptest.NewRequest(method, "/api/v1/contracts", nil)
}

func TestAPIKeyRestrictionsEnforced(t *testing.T) {
	tests := []struct {
		name       string
		key        store.APIKey
		remoteAddr string
		forwarded  string
		trusted    []string
		origin     string
		method     string
		wantStatus int
	}{
		{
			name:       "restrictions empty keeps current behaviour",
			key:        store.APIKey{AllowedCIDRs: nil, AllowedOrigins: nil},
			remoteAddr: "203.0.113.9:5000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ipv4 inside the allowlist",
			key:        store.APIKey{AllowedCIDRs: []string{"203.0.113.0/24"}},
			remoteAddr: "203.0.113.9:5000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ipv4 outside the allowlist",
			key:        store.APIKey{AllowedCIDRs: []string{"203.0.113.0/24"}},
			remoteAddr: "198.51.100.7:5000",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "single host entry",
			key:        store.APIKey{AllowedCIDRs: []string{"198.51.100.7/32"}},
			remoteAddr: "198.51.100.7:5000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ipv6 inside the allowlist",
			key:        store.APIKey{AllowedCIDRs: []string{"2001:db8::/32"}},
			remoteAddr: "[2001:db8::5]:5000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "ipv6 outside the allowlist",
			key:        store.APIKey{AllowedCIDRs: []string{"2001:db8::/32"}},
			remoteAddr: "[2001:db9::5]:5000",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "ipv4-mapped ipv6 peer matches an ipv4 range",
			key:        store.APIKey{AllowedCIDRs: []string{"203.0.113.0/24"}},
			remoteAddr: "[::ffff:203.0.113.9]:5000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "forwarded address from a trusted proxy",
			key:        store.APIKey{AllowedCIDRs: []string{"203.0.113.0/24"}},
			remoteAddr: "10.1.2.3:5000",
			forwarded:  "203.0.113.9",
			trusted:    []string{"10.0.0.0/8"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "forwarded address ignored from an untrusted peer",
			key:        store.APIKey{AllowedCIDRs: []string{"203.0.113.0/24"}},
			remoteAddr: "198.51.100.7:5000",
			forwarded:  "203.0.113.9",
			trusted:    []string{"10.0.0.0/8"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "origin in the allowlist",
			key:        store.APIKey{AllowedOrigins: []string{"https://app.example.com"}},
			remoteAddr: "203.0.113.9:5000",
			origin:     "https://app.example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "origin outside the allowlist",
			key:        store.APIKey{AllowedOrigins: []string{"https://app.example.com"}},
			remoteAddr: "203.0.113.9:5000",
			origin:     "https://evil.example",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "no origin header is a non-browser client",
			key:        store.APIKey{AllowedOrigins: []string{"https://app.example.com"}},
			remoteAddr: "203.0.113.9:5000",
			wantStatus: http.StatusOK,
		},
		{
			name:       "null origin is rejected",
			key:        store.APIKey{AllowedOrigins: []string{"https://app.example.com"}},
			remoteAddr: "203.0.113.9:5000",
			origin:     "null",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "network and origin must both pass",
			key:        store.APIKey{AllowedCIDRs: []string{"203.0.113.0/24"}, AllowedOrigins: []string{"https://app.example.com"}},
			remoteAddr: "203.0.113.9:5000",
			origin:     "https://evil.example",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "cors preflight is not blocked",
			key:        store.APIKey{AllowedCIDRs: []string{"10.0.0.0/8"}, AllowedOrigins: []string{"https://app.example.com"}},
			remoteAddr: "203.0.113.9:5000",
			origin:     "https://evil.example",
			method:     http.MethodOptions,
			wantStatus: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := restrictedStore(t)
			key := tc.key
			key.ID = "key-restricted"
			key.Name = "restricted"
			key.KeyHash = store.HashKey(restrictedKeyToken)
			key.Scopes = []string{store.ScopeReadContracts}
			key.CreatedAt = time.Now()
			m.AddAPIKey(key)

			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req := restrictedRequest(method)
			req.RemoteAddr = tc.remoteAddr
			if tc.forwarded != "" {
				req.Header.Set("X-Forwarded-For", tc.forwarded)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			req.Header.Set("Authorization", "Bearer "+restrictedKeyToken)

			rec := serveRestricted(t, m, tc.trusted, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode error envelope: %v", err)
				}
				if body.Error.Code != CodeAPIKeyRestricted {
					t.Fatalf("error code = %q, want %q", body.Error.Code, CodeAPIKeyRestricted)
				}
			}
		})
	}
}

// TestAPIKeyRestrictionsIgnoreUnboundCredentials keeps the middleware scoped to
// API keys: anonymous requests and session identities are never rejected by a
// key's allowlist, because they carry no key.
func TestAPIKeyRestrictionsIgnoreUnboundCredentials(t *testing.T) {
	m := restrictedStore(t)
	signer := NewSessionSigner(testSessionSecret, time.Hour)
	token, _ := signer.Issue("user-contributor")

	handler := Identity(m, IdentityOptions{Sessions: signer}, nil)(APIKeyRestrictions(nil)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.RemoteAddr = "198.51.100.7:5000"
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session request status = %d, want 200", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.RemoteAddr = "198.51.100.7:5000"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("anonymous request status = %d, want 200", rec.Code)
	}
}
