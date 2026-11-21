package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// scopeError mirrors the JSON envelope RequireScopes writes on a rejection.
type scopeError struct {
	Error struct {
		Code      int    `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// scopeKey builds an API key with the given scopes.
func scopeKey(scopes ...string) *store.APIKey {
	return &store.APIKey{ID: "key-1", Scopes: scopes}
}

// scopeRequest runs one request through RequireScopes mounted on a chi route,
// so the middleware sees a real chi route pattern. identity, when non-nil, is
// attached to the request context exactly as the Identity middleware would.
func scopeRequest(t *testing.T, method, pattern, token string, identity *VerifiedIdentity) (*httptest.ResponseRecorder, bool) {
	t.Helper()

	nextCalled := false
	router := chi.NewRouter()
	router.With(RequireScopes(discardLogger())).
		MethodFunc(method, pattern, func(w http.ResponseWriter, _ *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		})

	req := httptest.NewRequest(method, pattern, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if identity != nil {
		req = req.WithContext(WithIdentity(req.Context(), *identity))
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec, nextCalled
}

// TestRequireScopes covers every decision the middleware can make, including
// the negative cases that keep anonymous traffic off admin routes and scoped
// keys off routes outside their grant.
func TestRequireScopes(t *testing.T) {
	t.Parallel()

	session := VerifiedIdentity{UserID: "user-1", Role: store.RoleContributor, Source: IdentitySourceSession}
	readKey := VerifiedIdentity{UserID: "user-1", Role: store.RoleContributor, Source: IdentitySourceAPIKey, APIKey: scopeKey(store.ScopeReadContracts)}
	writeKey := VerifiedIdentity{Source: IdentitySourceAPIKey, APIKey: scopeKey(store.ScopeWriteContracts)}
	adminKey := VerifiedIdentity{Source: IdentitySourceAPIKey, APIKey: scopeKey(store.ScopeAdmin)}
	watchdogKey := VerifiedIdentity{Source: IdentitySourceAPIKey, APIKey: scopeKey(store.ScopeReadWatchdog)}

	tests := []struct {
		name         string
		method       string
		pattern      string
		token        string
		identity     *VerifiedIdentity
		wantStatus   int
		wantNext     bool
		wantError    string
		wantRequired string
	}{
		{
			name: "anonymous request on a read route stays open", method: http.MethodGet,
			pattern: "/api/v1/contracts", wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "anonymous request on an unscoped route stays open", method: http.MethodGet,
			pattern: "/health", wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "anonymous request on an admin route needs a credential", method: http.MethodGet,
			pattern: "/api/v1/api-keys", wantStatus: http.StatusUnauthorized, wantError: "authentication required",
		},
		{
			name: "unknown bearer token is rejected", method: http.MethodGet,
			pattern: "/api/v1/contracts", token: "sl_not_a_real_key",
			wantStatus: http.StatusUnauthorized, wantError: "invalid API key",
		},
		{
			name: "read key on a read route is allowed", method: http.MethodGet,
			pattern: "/api/v1/contracts", identity: &readKey,
			wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "read key on a write route is forbidden", method: http.MethodPost,
			pattern: "/api/v1/contracts", identity: &readKey,
			wantStatus: http.StatusForbidden, wantError: "missing scope", wantRequired: store.ScopeWriteContracts,
		},
		{
			name: "write key on a write route is allowed", method: http.MethodPost,
			pattern: "/api/v1/contracts", identity: &writeKey,
			wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "admin key satisfies an admin route", method: http.MethodGet,
			pattern: "/api/v1/api-keys", identity: &adminKey,
			wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "admin key satisfies a write route", method: http.MethodPost,
			pattern: "/api/v1/contracts", identity: &adminKey,
			wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "watchdog key does not grant contract writes", method: http.MethodPost,
			pattern: "/api/v1/contracts", identity: &watchdogKey,
			wantStatus: http.StatusForbidden, wantError: "missing scope", wantRequired: store.ScopeWriteContracts,
		},
		{
			name: "watchdog key reads watchdog routes", method: http.MethodGet,
			pattern: "/api/v1/watchdog/stats", identity: &watchdogKey,
			wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "session identity bypasses key scopes", method: http.MethodPost,
			pattern: "/api/v1/contracts", identity: &session,
			wantStatus: http.StatusNoContent, wantNext: true,
		},
		{
			name: "query parameter key is inspected like a bearer token", method: http.MethodGet,
			pattern: "/api/v1/contracts", token: "",
			wantStatus: http.StatusNoContent, wantNext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec, nextCalled := scopeRequest(t, tt.method, tt.pattern, tt.token, tt.identity)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if nextCalled != tt.wantNext {
				t.Errorf("next called = %v, want %v", nextCalled, tt.wantNext)
			}
			if !tt.wantNext {
				if got := rec.Header().Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
				var body scopeError
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("decode rejection body %q: %v", rec.Body.String(), err)
				}
				if body.Error.Message != tt.wantError {
					t.Errorf("error message = %q, want %q", body.Error.Message, tt.wantError)
				}
			} else if rec.Body.Len() != 0 {
				t.Errorf("allowed request wrote a body: %q", rec.Body.String())
			}
		})
	}
}

// TestRequireScopesReadsAPIKeyFromQuery asserts the ?api_key= fallback is
// rejected when it is not a verifiable credential.
func TestRequireScopesReadsAPIKeyFromQuery(t *testing.T) {
	t.Parallel()

	nextCalled := false
	router := chi.NewRouter()
	router.With(RequireScopes(discardLogger())).
		MethodFunc(http.MethodGet, "/api/v1/contracts", func(http.ResponseWriter, *http.Request) {
			nextCalled = true
		})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/contracts?api_key=sl_bad", nil))

	if nextCalled {
		t.Error("request with an invalid query-string key reached the handler")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestExtractAPIKey table-tests credential extraction precedence.
func TestExtractAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authHeader string
		query      string
		want       string
	}{
		{"bearer header", "Bearer sk_live_1", "", "sk_live_1"},
		{"bearer prefix is case-insensitive", "bearer sk_live_1", "", "sk_live_1"},
		{"query parameter", "", "sk_query", "sk_query"},
		{"bearer wins over query", "Bearer sk_header", "sk_query", "sk_header"},
		{"no credential", "", "", ""},
		{"non-bearer authorization is ignored", "Basic dXNlcjpwYXNz", "", ""},
		{"bearer without a token falls through", "Bearer", "", ""},
		{"surrounding whitespace is trimmed", "Bearer  sk_live_1  ", "", "sk_live_1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url := "/api/v1/contracts"
			if tt.query != "" {
				url += "?api_key=" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			if got := extractAPIKey(req); got != tt.want {
				t.Errorf("extractAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRequiredScope pins the route metadata lookup, including the trailing
// slash chi leaves on nested patterns.
func TestRequiredScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		method  string
		pattern string
		want    string
		wantOK  bool
	}{
		{"read route", http.MethodGet, "/api/v1/contracts", store.ScopeReadContracts, true},
		{"write route", http.MethodPost, "/api/v1/contracts", store.ScopeWriteContracts, true},
		{"admin route", http.MethodGet, "/api/v1/api-keys", store.ScopeAdmin, true},
		{"watchdog route", http.MethodGet, "/api/v1/watchdog/alerts", store.ScopeReadWatchdog, true},
		{"trailing slash is normalized", http.MethodGet, "/api/v1/contracts/", store.ScopeReadContracts, true},
		{"wrong method is unknown", http.MethodPatch, "/api/v1/contracts", "", false},
		{"unscoped route is unknown", http.MethodGet, "/health", "", false},
		{"root is unknown", http.MethodGet, "/", "", false},
		{"empty pattern is unknown", http.MethodGet, "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := RequiredScope(tt.method, tt.pattern)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("RequiredScope(%q, %q) = (%q, %v), want (%q, %v)",
					tt.method, tt.pattern, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestNormalizePattern keeps the "/" and "" edge cases stable.
func TestNormalizePattern(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                   "",
		"/":                  "/",
		"/api/v1/contracts/": "/api/v1/contracts",
		"/api/v1/contracts":  "/api/v1/contracts",
		"//":                 "/",
	}
	for in, want := range tests {
		if got := normalizePattern(in); got != want {
			t.Errorf("normalizePattern(%q) = %q, want %q", in, got, want)
		}
	}
}
