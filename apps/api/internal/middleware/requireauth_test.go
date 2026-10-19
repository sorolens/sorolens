package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serveRequireAuth(t *testing.T, require bool, path string, authed bool) int {
	t.Helper()
	var id *VerifiedIdentity
	if authed {
		id = &VerifiedIdentity{UserID: "user-1"}
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireAuth(require, "/health", "/readyz", "/api/version", "/metrics")(next)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if id != nil {
		req = req.WithContext(WithIdentity(req.Context(), *id))
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

func TestRequireAuthOffAllowsUnauthenticated(t *testing.T) {
	if got := serveRequireAuth(t, false, "/api/v1/contracts", false); got != http.StatusOK {
		t.Fatalf("auth off: got %d, want 200", got)
	}
}

func TestRequireAuthOnRejectsAnonymous(t *testing.T) {
	if got := serveRequireAuth(t, true, "/api/v1/contracts", false); got != http.StatusUnauthorized {
		t.Fatalf("auth on anonymous: got %d, want 401", got)
	}
}

func TestRequireAuthOnAllowsAuthenticated(t *testing.T) {
	if got := serveRequireAuth(t, true, "/api/v1/contracts", true); got != http.StatusOK {
		t.Fatalf("auth on authenticated: got %d, want 200", got)
	}
}

func TestRequireAuthOnExemptsHealthEndpoints(t *testing.T) {
	for _, path := range []string{"/health", "/readyz", "/api/version", "/metrics"} {
		if got := serveRequireAuth(t, true, path, false); got != http.StatusOK {
			t.Fatalf("auth on exempt %s: got %d, want 200", path, got)
		}
	}
}

func TestRequireAuthOnExemptsNonExemptPathsOnly(t *testing.T) {
	// /api/v1/health is not in the exemption list, so it still requires auth.
	if got := serveRequireAuth(t, true, "/api/v1/health", false); got != http.StatusUnauthorized {
		t.Fatalf("auth on non-exempt /api/v1/health: got %d, want 401", got)
	}
}
