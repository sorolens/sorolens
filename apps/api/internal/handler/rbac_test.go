package handler_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestRBACRoleEndpointMatrix drives the role-by-role, endpoint-by-endpoint
// matrix from issue #133:
//
//	viewer      - read-only: cannot POST /contracts, denied /api/v1/api-keys
//	contributor - can POST /contracts, still denied /api/v1/api-keys
//	admin       - everything, including /api/v1/api-keys
//	anonymous   - denied every protected (write/admin) endpoint
func TestRBACRoleEndpointMatrix(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)

	cases := []struct {
		name   string
		method string
		path   string
		userID string
		token  string
		body   string
		want   int
	}{
		// ---- POST /contracts (contributor+) ---------------------------------
		{"viewer denied POST /contracts", http.MethodPost, "/api/v1/contracts", viewerUser, "", validContractBody, http.StatusForbidden},
		{"contributor POST /contracts", http.MethodPost, "/api/v1/contracts", contributorUser, "", validContractBody, http.StatusCreated},
		{"admin POST /contracts", http.MethodPost, "/api/v1/contracts", adminUser, "", validContractBody, http.StatusCreated},
		{"anonymous denied POST /contracts", http.MethodPost, "/api/v1/contracts", "", "", validContractBody, http.StatusUnauthorized},

		// ---- /api/v1/api-keys (admin scope + admin role) ----------------------
		// An admin-scoped credential passes the scope layer; the role layer
		// then decides. Roles are what matter here.
		{"viewer denied api-keys", http.MethodGet, "/api/v1/api-keys", viewerUser, adminToken, "", http.StatusForbidden},
		{"contributor denied api-keys", http.MethodGet, "/api/v1/api-keys", contributorUser, adminToken, "", http.StatusForbidden},
		{"admin grants api-keys", http.MethodGet, "/api/v1/api-keys", adminUser, adminToken, "", http.StatusOK},
		{"anonymous denied api-keys", http.MethodGet, "/api/v1/api-keys", "", adminToken, "", http.StatusUnauthorized},

		// ---- /api/v1/admin/* (admin role; no API key needed to reach the
		// role gate) ---------------------------------------------------------
		{"viewer denied admin keys", http.MethodGet, "/api/v1/admin/keys", viewerUser, "", "", http.StatusForbidden},
		{"contributor denied admin keys", http.MethodGet, "/api/v1/admin/keys", contributorUser, "", "", http.StatusForbidden},
		{"admin grants admin keys", http.MethodGet, "/api/v1/admin/keys", adminUser, "", "", http.StatusOK},
		{"anonymous denied admin keys", http.MethodGet, "/api/v1/admin/keys", "", "", "", http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, tc.method, tc.path, tc.token, tc.userID, tc.body)
			if w.Code != tc.want {
				t.Fatalf("want %d, got %d (%s)", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// TestRBACAdminWritesOnlyForAdmin verifies admin-only writes (create, revoke)
// are denied to contributors and viewers even when the caller holds an
// admin-scoped API key. The role check gates these routes independently of
// the API key scope.
func TestRBACAdminWritesOnlyForAdmin(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)

	// Contributor armed with an admin-scoped key must still be denied the
	// admin surface: roles gate these routes regardless of API key scope.
	adminKeyStore := adminScopedStore(t)
	srv = newTestHandler(adminKeyStore, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/admin/keys", adminToken, contributorUser,
		`{"name":"x","scopes":["read:watchdog"]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("contributor with admin-scoped key create admin key: want 403, got %d (%s)", w.Code, w.Body.String())
	}

	// Viewer with a write-scoped key still cannot register contracts.
	w = doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts", adminToken, viewerUser, validContractBody)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer with admin-scoped key POST /contracts: want 403, got %d (%s)", w.Code, w.Body.String())
	}
}

// seedRBACUsers seeds a viewer + contributor + admin (mirrors the production
// INITIAL_ADMIN_GITHUB_ID seed) plus an admin-scoped API key so the scope and
// role layers can both be exercised, and a read-only key for write-denial
// cases.
func seedRBACUsers(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	ghAdmin := adminGitHub
	ghContributor := contributorGitHub
	ms.AddUser(store.User{ID: viewerUser, GitHubID: nil, Role: store.RoleViewer})
	ms.AddUser(store.User{ID: contributorUser, GitHubID: &ghContributor, Role: store.RoleContributor})
	ms.AddUser(store.User{ID: adminUser, GitHubID: &ghAdmin, Role: store.RoleAdmin})
	ms.AddAPIKey(store.APIKey{
		ID: "key-admin", Name: "admin", KeyPrefix: "sl_admin",
		KeyHash:   store.HashKey(adminToken),
		Scopes:    []string{store.ScopeAdmin},
		CreatedAt: time.Now().UTC(),
	})
	ms.AddAPIKey(store.APIKey{
		ID: "key-read-contracts", Name: "reader", KeyPrefix: "sl_readc",
		KeyHash:   store.HashKey(readContractsKey),
		Scopes:    []string{store.ScopeReadContracts},
		CreatedAt: time.Now().UTC(),
	})
	return ms
}

// adminScopedStore returns a MockStore identical to seedRBACUsers; retained
// as a named dependency for tests that involve the admin-scoped credential.
func adminScopedStore(t *testing.T) *store.MockStore {
	t.Helper()
	return seedRBACUsers(t)
}
