package handler_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	adminToken        = "sl_admin_token_for_tests"
	readContractsKey  = "sl_read_contracts_token_for_tests"
	readWatchdogKey   = "sl_read_watchdog_token_for_tests"
	validContractBody = `{"id":"CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","network":"testnet"}`
)

const (
	adminUser         = "user-admin"
	contributorUser   = "user-contrib"
	viewerUser        = "user-viewer"
	adminGitHub       = "github-admin"
	contributorGitHub = "github-contrib"
)

func seedScopedKeyStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedMultiNetworkStore(t)
	ms.AddAPIKey(store.APIKey{
		ID: "key-admin", Name: "admin", KeyPrefix: "sl_admin",
		KeyHash: store.HashKey(adminToken), Scopes: []string{store.ScopeAdmin},
		CreatedAt: time.Now().UTC(),
	})
	ms.AddAPIKey(store.APIKey{
		ID: "key-read-contracts", Name: "reader", KeyPrefix: "sl_readc",
		KeyHash: store.HashKey(readContractsKey), Scopes: []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(),
	})
	ms.AddAPIKey(store.APIKey{
		ID: "key-read-watchdog", Name: "bot", KeyPrefix: "sl_readw",
		KeyHash: store.HashKey(readWatchdogKey), Scopes: []string{store.ScopeReadWatchdog},
		CreatedAt: time.Now().UTC(),
	})
	adminGH := adminGitHub
	contribGH := contributorGitHub
	ms.AddUser(store.User{ID: adminUser, GitHubID: &adminGH, Role: store.RoleAdmin})
	ms.AddUser(store.User{ID: contributorUser, GitHubID: &contribGH, Role: store.RoleContributor})
	return ms
}

func doRequest(srv http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	return doRequestAsUser(srv, method, path, token, "", body)
}

// doRequestAsUser is doRequest with an optional calling user identity in
// X-User-ID, used to exercise role-based access control.
func doRequestAsUser(srv http.Handler, method, path, token, userID, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestAPIKeyGrantAndDenyPerScope(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	cases := []struct {
		name     string
		method   string
		path     string
		token    string
		userID   string
		body     string
		wantCode int
	}{
		// read:contracts grants reads, denies writes.
		{"read:contracts grants GET /contracts", http.MethodGet, "/api/v1/contracts", readContractsKey, "", "", http.StatusOK},
		{"read:contracts denies POST /contracts", http.MethodPost, "/api/v1/contracts", readContractsKey, "", validContractBody, http.StatusForbidden},
		{"read:contracts denies watchdog", http.MethodGet, "/api/v1/watchdog/stats", readContractsKey, "", "", http.StatusForbidden},
		// read:watchdog grants watchdog reads, denies contracts.
		{"read:watchdog grants watchdog stats", http.MethodGet, "/api/v1/watchdog/stats", readWatchdogKey, "", "", http.StatusOK},
		{"read:watchdog grants watchdog contracts", http.MethodGet, "/api/v1/watchdog/contracts", readWatchdogKey, "", "", http.StatusOK},
		{"read:watchdog denies GET /contracts", http.MethodGet, "/api/v1/contracts", readWatchdogKey, "", "", http.StatusForbidden},
		// admin:* grants everything when the caller is an admin.
		{"admin grants POST /contracts", http.MethodPost, "/api/v1/contracts", adminToken, adminUser, validContractBody, http.StatusCreated},
		{"admin grants GET /contracts", http.MethodGet, "/api/v1/contracts", adminToken, "", "", http.StatusOK},
		{"admin grants watchdog", http.MethodGet, "/api/v1/watchdog/stats", adminToken, "", "", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, tc.method, tc.path, tc.token, tc.userID, tc.body)
			if w.Code != tc.wantCode {
				t.Fatalf("want %d, got %d (%s)", tc.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestAPIKeyMissingScopeBodyShape(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequest(srv, http.MethodPost, "/api/v1/contracts", readContractsKey, validContractBody)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "missing scope" {
		t.Errorf(`error: want "missing scope", got %q`, body["error"])
	}
	if body["required"] != store.ScopeWriteContracts {
		t.Errorf("required: want %q, got %q", store.ScopeWriteContracts, body["required"])
	}
}

func TestAPIKeyInvalidOrRevokedIsUnauthorized(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	w := doRequest(srv, http.MethodGet, "/api/v1/contracts", "sl_not_a_real_key", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unknown key: want 401, got %d", w.Code)
	}

	// Revoke a valid key, then reuse it.
	if err := ms.RevokeAPIKey(nil, "key-read-contracts"); err != nil {
		t.Fatal(err)
	}
	w = doRequest(srv, http.MethodGet, "/api/v1/contracts", readContractsKey, "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key: want 401, got %d", w.Code)
	}
}

func TestAPIKeyViaXAPIKeyHeader(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("X-API-Key", readContractsKey)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200 via X-API-Key, got %d", w.Code)
	}
}

func TestAnonymousReadsRemainAllowed(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequest(srv, http.MethodGet, "/api/v1/contracts", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("anonymous GET /contracts: want 200, got %d", w.Code)
	}

	// API key management always requires a credential.
	w = doRequest(srv, http.MethodPost, "/api/v1/api-keys", "", `{"name":"x","scopes":["read:contracts"]}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous POST /api-keys: want 401, got %d", w.Code)
	}
}

func TestCreateAPIKeyLifecycle(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/api-keys", adminToken, adminUser,
		`{"name":"monitoring bot","scopes":["read:watchdog"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		ID     string   `json:"id"`
		Key    string   `json:"key"`
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Key, "sl_") {
		t.Fatalf("key should be an sl_ token, got %q", created.Key)
	}
	if len(created.Scopes) != 1 || created.Scopes[0] != store.ScopeReadWatchdog {
		t.Fatalf("scopes: %+v", created.Scopes)
	}

	// The new key works for its granted scope...
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", created.Key, "").Code; got != http.StatusOK {
		t.Fatalf("new key watchdog read: want 200, got %d", got)
	}
	// ...and is denied elsewhere.
	if got := doRequest(srv, http.MethodGet, "/api/v1/contracts", created.Key, "").Code; got != http.StatusForbidden {
		t.Fatalf("new key contracts read: want 403, got %d", got)
	}

	// List returns metadata, never key material.
	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/api-keys", adminToken, adminUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list keys: want 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), `"key"`) {
		t.Error("list response must not include plaintext key material")
	}

	// Revoke and confirm the key stops working.
	w = doRequestAsUser(srv, http.MethodDelete, "/api/v1/api-keys/"+created.ID, adminToken, adminUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("revoke: want 204, got %d (%s)", w.Code, w.Body.String())
	}
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", created.Key, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked key: want 401, got %d", got)
	}
}

func TestCreateAPIKeyRejectsUnknownScope(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/api-keys", adminToken, adminUser,
		`{"name":"bad","scopes":["read:everything"]}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRevokeUnknownAPIKey(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/api-keys/does-not-exist", adminToken, adminUser, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}
