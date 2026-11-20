package handler_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type allowlistKeyResponse struct {
	ID             string   `json:"id"`
	Key            string   `json:"key"`
	AllowedCIDRs   []string `json:"allowed_cidrs"`
	AllowedOrigins []string `json:"allowed_origins"`
}

// TestAPIKeyAllowlistCreateUpdateAndEnforcement covers the admin contract
// (issue #793): lists are normalised on create, PATCH replaces or clears them,
// and a restricted key is actually refused from another address through the
// full router.
func TestAPIKeyAllowlistCreateUpdateAndEnforcement(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	w := doRequest(srv, http.MethodPost, "/api/v1/api-keys", adminToken,
		`{"name":"restricted","scopes":["read:contracts"],`+
			`"allowed_cidrs":["203.0.113.7/24","203.0.113.0/24"],`+
			`"allowed_origins":["HTTPS://App.Example.com/"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created allowlistKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if want := []string{"203.0.113.0/24"}; !reflect.DeepEqual(created.AllowedCIDRs, want) {
		t.Fatalf("allowed_cidrs = %v, want %v", created.AllowedCIDRs, want)
	}
	if want := []string{"https://app.example.com"}; !reflect.DeepEqual(created.AllowedOrigins, want) {
		t.Fatalf("allowed_origins = %v, want %v", created.AllowedOrigins, want)
	}

	// httptest peers are 192.0.2.1, outside the allowlist.
	w = doRequest(srv, http.MethodGet, "/api/v1/contracts", created.Key, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("restricted key from another address: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	var denied struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &denied); err != nil {
		t.Fatalf("decode error envelope %q: %v", w.Body.String(), err)
	}
	if denied.Error.Code != middleware.CodeAPIKeyIPNotAllowed {
		t.Fatalf("error code = %q, want %q", denied.Error.Code, middleware.CodeAPIKeyIPNotAllowed)
	}

	// Clearing only the CIDR list leaves the origin list alone and makes the
	// key usable again from this peer.
	w = doRequest(srv, http.MethodPatch, "/api/v1/api-keys/"+created.ID, adminToken,
		`{"allowed_cidrs":[]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var updated allowlistKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.AllowedCIDRs) != 0 {
		t.Fatalf("allowed_cidrs = %v, want empty", updated.AllowedCIDRs)
	}
	if want := []string{"https://app.example.com"}; !reflect.DeepEqual(updated.AllowedOrigins, want) {
		t.Fatalf("allowed_origins = %v, want %v", updated.AllowedOrigins, want)
	}
	// The CIDR restriction is gone, so the address check no longer fires;
	// the retained origin list still rejects a request with no Origin.
	w = doRequest(srv, http.MethodGet, "/api/v1/contracts", created.Key, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("key after clearing the CIDR list: want 403 from the retained origin list, got %d", w.Code)
	}
	if err := json.Unmarshal(w.Body.Bytes(), &denied); err != nil {
		t.Fatalf("decode error envelope %q: %v", w.Body.String(), err)
	}
	if denied.Error.Code != middleware.CodeAPIKeyOriginNotAllowed {
		t.Fatalf("error code = %q, want %q", denied.Error.Code, middleware.CodeAPIKeyOriginNotAllowed)
	}

	if got := doRequest(srv, http.MethodPatch, "/api/v1/api-keys/"+created.ID, adminToken,
		`{"allowed_origins":[]}`).Code; got != http.StatusOK {
		t.Fatalf("clearing the origin list: want 200, got %d", got)
	}
	if got := doRequest(srv, http.MethodGet, "/api/v1/contracts", created.Key, "").Code; got != http.StatusOK {
		t.Fatalf("key after clearing both lists: want 200, got %d", got)
	}

	// Invalid input and empty patches are rejected without touching the key.
	cases := []struct {
		name string
		body string
		want string
	}{
		{"invalid CIDR", `{"allowed_cidrs":["not-a-cidr"]}`, "invalid CIDR range"},
		{"invalid origin", `{"allowed_origins":["https://app.example.com/path"]}`, "must not contain"},
		{"empty patch", `{}`, "at least one"},
	}
	for _, tc := range cases {
		w := doRequest(srv, http.MethodPatch, "/api/v1/api-keys/"+created.ID, adminToken, tc.body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: want 422, got %d (%s)", tc.name, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("%s: body %q does not contain %q", tc.name, w.Body.String(), tc.want)
		}
	}
}

// TestMyAPIKeyAllowlistUpdate covers the self-service contract: an owner can
// change its own key's allowlists, and another user cannot.
func TestMyAPIKeyAllowlistUpdate(t *testing.T) {
	ms := seedScopedKeyStore(t)
	other := "user-other"
	ms.AddUser(store.User{ID: other, Role: store.RoleContributor})
	ms.AddAPIKey(store.APIKey{
		ID: "key-other", Name: "someone else", KeyPrefix: "sl_other",
		KeyHash: store.HashKey("sl_other_token"), Scopes: []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(), UserID: &other,
	})
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/me/api-keys", "", contributorUser,
		`{"name":"mine","scopes":["read:contracts"],"allowed_cidrs":["203.0.113.0/24"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created allowlistKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	w = doRequestAsUser(srv, http.MethodPatch, "/api/v1/me/api-keys/"+created.ID, "", contributorUser,
		`{"allowed_cidrs":["192.0.2.0/24"],"allowed_origins":["https://app.example.com"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("owner patch: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var updated allowlistKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if want := []string{"192.0.2.0/24"}; !reflect.DeepEqual(updated.AllowedCIDRs, want) {
		t.Fatalf("allowed_cidrs = %v, want %v", updated.AllowedCIDRs, want)
	}
	if want := []string{"https://app.example.com"}; !reflect.DeepEqual(updated.AllowedOrigins, want) {
		t.Fatalf("allowed_origins = %v, want %v", updated.AllowedOrigins, want)
	}

	// Another user's key is reported as not found, whether the caller is a
	// different contributor or an admin.
	if got := doRequestAsUser(srv, http.MethodPatch, "/api/v1/me/api-keys/key-other", "", contributorUser,
		`{"allowed_cidrs":[]}`).Code; got != http.StatusNotFound {
		t.Fatalf("cross-user patch: want 404, got %d", got)
	}
	if got := doRequestAsUser(srv, http.MethodPatch, "/api/v1/me/api-keys/"+created.ID, "", adminUser,
		`{"allowed_cidrs":[]}`).Code; got != http.StatusNotFound {
		t.Fatalf("other user patching this key: want 404, got %d", got)
	}
}
