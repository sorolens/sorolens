package handler_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestMyAPIKeyLifecycle covers the self-service key flow (issue #613): a
// contributor session can mint a key bound to itself, see only its own keys,
// and revoke one. The plaintext token is returned once and then never again.
func TestMyAPIKeyLifecycle(t *testing.T) {
	ms := seedScopedKeyStore(t)
	other := "user-other"
	ms.AddUser(store.User{ID: other, Role: store.RoleContributor})
	ms.AddAPIKey(store.APIKey{
		ID:        "key-other",
		Name:      "someone else",
		KeyPrefix: "sl_other",
		KeyHash:   store.HashKey("sl_other_token"),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(),
		UserID:    &other,
	})
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/me/api-keys", "", contributorUser,
		`{"name":"my script","scopes":["read:watchdog"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		ID     string  `json:"id"`
		Key    string  `json:"key"`
		UserID *string `json:"user_id"`
		Scopes []string `json:"scopes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Key, "sl_") {
		t.Fatalf("key should be an sl_ token, got %q", created.Key)
	}
	if created.UserID == nil || *created.UserID != contributorUser {
		t.Fatalf("key must be bound to the session user, got %v", created.UserID)
	}
	if len(created.Scopes) != 1 || created.Scopes[0] != store.ScopeReadWatchdog {
		t.Fatalf("scopes: %+v", created.Scopes)
	}

	// The minted key authenticates as its bound user.
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", created.Key, "").Code; got != http.StatusOK {
		t.Fatalf("new key watchdog read: want 200, got %d", got)
	}

	// The list contains this user's keys only and never key material.
	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/me/api-keys", "", contributorUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var listed struct {
		APIKeys []struct {
			ID string `json:"id"`
		} `json:"api_keys"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.APIKeys) != 1 || listed.APIKeys[0].ID != created.ID {
		t.Fatalf("list should contain only the caller's key, got %+v", listed.APIKeys)
	}
	if strings.Contains(w.Body.String(), `"key":`) {
		t.Error("list response must not include plaintext key material")
	}

	// Another user's key cannot be revoked through this session.
	if got := doRequestAsUser(srv, http.MethodDelete, "/api/v1/me/api-keys/key-other", "", contributorUser, "").Code; got != http.StatusNotFound {
		t.Fatalf("cross-user revoke: want 404, got %d", got)
	}
	if got := doRequestAsUser(srv, http.MethodDelete, "/api/v1/me/api-keys/"+created.ID, "", contributorUser, "").Code; got != http.StatusNoContent {
		t.Fatalf("revoke: want 204, got %d", got)
	}
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", created.Key, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked key: want 401, got %d", got)
	}
}

func TestMyAPIKeyRequiresSession(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/me/api-keys", ""},
		{http.MethodPost, "/api/v1/me/api-keys", `{"name":"k"}`},
		{http.MethodDelete, "/api/v1/me/api-keys/whatever", ""},
	} {
		if got := doRequest(srv, tc.method, tc.path, "", tc.body).Code; got != http.StatusUnauthorized {
			t.Fatalf("%s %s without a session: want 401, got %d", tc.method, tc.path, got)
		}
	}
}

func TestMyAPIKeyRejectsAdminScope(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/me/api-keys", "", contributorUser,
		`{"name":"escalate","scopes":["admin:*"]}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("admin scope: want 422, got %d (%s)", w.Code, w.Body.String())
	}
}
