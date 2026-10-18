package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const testSessionSecret = "test-session-secret"

func testLookup() *store.MockStore {
	m := store.NewMockStore()
	m.AddUser(store.User{ID: "user-contributor", Role: store.RoleContributor})
	m.AddUser(store.User{ID: "user-admin", Role: store.RoleAdmin})
	return m
}

func addKey(m *store.MockStore, id string, userID *string, scopes ...string) string {
	const token = "sl_test_token_" + "x"
	m.AddAPIKey(store.APIKey{
		ID:        id,
		Name:      id,
		KeyHash:   store.HashKey(token),
		Scopes:    scopes,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
	return token
}

func serveRole(t *testing.T, lookup IdentityLookup, opts IdentityOptions, role string, req *http.Request) int {
	t.Helper()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Identity(lookup, opts, nil)(RequireRole(nil, role)(ok))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec.Code
}

func newRequest(method, target string) *http.Request {
	return httptest.NewRequest(method, target, nil)
}

func TestSpoofedIdentityHeaderDoesNotGrantRole(t *testing.T) {
	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("X-User-ID", "user-contributor")
	req.Header.Set("X-GitHub-Login", "user-contributor")
	req.Header.Set("X-User-Sub", "user-contributor")
	req.Header.Set("X-GitHub-ID", "user-admin")

	if got := serveRole(t, testLookup(), IdentityOptions{}, RoleContributor, req); got != http.StatusUnauthorized {
		t.Fatalf("spoofed identity headers: got %d, want 401", got)
	}
}

func TestSessionTokenGrantsBoundRole(t *testing.T) {
	signer := NewSessionSigner(testSessionSecret, time.Hour)
	token, _ := signer.Issue("user-contributor")

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("Authorization", "Bearer "+token)

	opts := IdentityOptions{Sessions: signer}
	if got := serveRole(t, testLookup(), opts, RoleContributor, req); got != http.StatusOK {
		t.Fatalf("valid session: got %d, want 200", got)
	}
}

func TestSessionTokenCannotReachAdmin(t *testing.T) {
	signer := NewSessionSigner(testSessionSecret, time.Hour)
	token, _ := signer.Issue("user-contributor")

	req := newRequest(http.MethodPost, "/api/v1/api-keys")
	req.Header.Set("Authorization", "Bearer "+token)

	opts := IdentityOptions{Sessions: signer}
	if got := serveRole(t, testLookup(), opts, RoleAdmin, req); got != http.StatusForbidden {
		t.Fatalf("contributor session on admin route: got %d, want 403", got)
	}
}

func TestTamperedSessionTokenRejected(t *testing.T) {
	signer := NewSessionSigner(testSessionSecret, time.Hour)
	token, _ := signer.Issue("user-contributor")
	tampered := token[:len(token)-1] + "0"

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("Authorization", "Bearer "+tampered)

	if got := serveRole(t, testLookup(), IdentityOptions{Sessions: signer}, RoleContributor, req); got != http.StatusUnauthorized {
		t.Fatalf("tampered session: got %d, want 401", got)
	}
}

func TestExpiredSessionTokenRejected(t *testing.T) {
	signer := NewSessionSigner(testSessionSecret, time.Hour)
	token, _ := signer.Issue("user-contributor")
	signer.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("Authorization", "Bearer "+token)

	if got := serveRole(t, testLookup(), IdentityOptions{Sessions: signer}, RoleContributor, req); got != http.StatusUnauthorized {
		t.Fatalf("expired session: got %d, want 401", got)
	}
}

func TestAPIKeyBoundToUserCarriesIdentity(t *testing.T) {
	m := testLookup()
	userID := "user-contributor"
	token := addKey(m, "key-contributor", &userID, store.ScopeWriteContracts)

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("Authorization", "Bearer "+token)
	// A spoofed header claiming admin must not override the key's binding.
	req.Header.Set("X-User-ID", "user-admin")

	if got := serveRole(t, m, IdentityOptions{}, RoleContributor, req); got != http.StatusOK {
		t.Fatalf("bound contributor key: got %d, want 200", got)
	}
	if got := serveRole(t, m, IdentityOptions{}, RoleAdmin, req); got != http.StatusForbidden {
		t.Fatalf("bound contributor key on admin route: got %d, want 403", got)
	}
}

func TestUnboundAPIKeyCannotUseSpoofedHeader(t *testing.T) {
	m := testLookup()
	token := addKey(m, "key-unbound", nil, store.ScopeWriteContracts)

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-ID", "user-contributor")

	if got := serveRole(t, m, IdentityOptions{}, RoleContributor, req); got != http.StatusForbidden {
		t.Fatalf("unbound key with spoofed header: got %d, want 403", got)
	}
}

func TestAdminScopedKeySatisfiesAdminRole(t *testing.T) {
	m := testLookup()
	token := addKey(m, "key-admin", nil, store.ScopeAdmin)

	req := newRequest(http.MethodPost, "/api/v1/api-keys")
	req.Header.Set("Authorization", "Bearer "+token)

	if got := serveRole(t, m, IdentityOptions{}, RoleAdmin, req); got != http.StatusOK {
		t.Fatalf("admin-scoped key: got %d, want 200", got)
	}
}

func TestTrustedProxyIdentityRequiresValidSignature(t *testing.T) {
	const secret = "proxy-secret"
	m := testLookup()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	userID := "user-contributor"
	mac := hex.EncodeToString(ProxyMAC(secret, http.MethodPost, "/api/v1/contracts", userID, ts))

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set(proxyUserIDHeader, userID)
	req.Header.Set(proxyTimestampHeader, ts)
	req.Header.Set(proxySignatureHeader, mac)

	opts := IdentityOptions{ProxySecret: secret}
	if got := serveRole(t, m, opts, RoleContributor, req); got != http.StatusOK {
		t.Fatalf("signed proxy identity: got %d, want 200", got)
	}

	req.Header.Set(proxySignatureHeader, "deadbeef")
	if got := serveRole(t, m, opts, RoleContributor, req); got != http.StatusUnauthorized {
		t.Fatalf("bad proxy signature: got %d, want 401", got)
	}
}

func TestActorUsesVerifiedIdentityOnly(t *testing.T) {
	m := testLookup()
	signer := NewSessionSigner(testSessionSecret, time.Hour)
	token, _ := signer.Issue("user-contributor")

	req := newRequest(http.MethodPost, "/api/v1/contracts")
	req.Header.Set("X-User-ID", "user-admin")
	if got := Actor(req); got != "anonymous" {
		t.Fatalf("Actor with only a spoofed header = %q, want anonymous", got)
	}

	var actor string
	handler := Identity(m, IdentityOptions{Sessions: signer}, nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		actor = Actor(r)
	}))
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if actor != "user-contributor" {
		t.Fatalf("Actor with a session = %q, want user-contributor", actor)
	}
}
