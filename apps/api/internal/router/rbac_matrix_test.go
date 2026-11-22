package router_test

// TestRBACAuthorizationMatrix verifies the authorization behaviour declared in
// rbac_policy.yaml for each route × role combination (issue #795).
//
// Identity is injected directly into the request context via
// middleware.WithIdentity so the test bypasses the credential-resolution
// stage and exercises only the role-enforcement middleware.  The Identity
// middleware itself short-circuits when an identity is already present in the
// context (see middleware/identity.go), so a pre-injected identity is never
// overwritten by the middleware chain.
//
// Expected status codes from the YAML are interpreted as:
//   - The exact code when the authorization layer itself rejects the request
//     (401, 403).
//   - Any 2xx code when access is granted ("200" means the middleware passed;
//     the handler may return a different 2xx or even a 404/422 on missing data).

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// rbacPolicyFile is rbac_policy.yaml relative to this package.
var rbacPolicyFile = filepath.Join(".", "rbac_policy.yaml")

type rbacRouteEntry struct {
	Method      string `yaml:"method"`
	Pattern     string `yaml:"pattern"`
	Anonymous   int    `yaml:"anonymous"`
	Viewer      int    `yaml:"viewer"`
	Contributor int    `yaml:"contributor"`
	Admin       int    `yaml:"admin"`
}

type rbacPolicy struct {
	Routes []rbacRouteEntry `yaml:"routes"`
}

func loadRBACPolicy(t *testing.T) rbacPolicy {
	t.Helper()
	f, err := os.Open(rbacPolicyFile)
	if err != nil {
		t.Fatalf("open rbac_policy.yaml: %v", err)
	}
	defer func() { _ = f.Close() }()
	var p rbacPolicy
	if err := yaml.NewDecoder(f).Decode(&p); err != nil {
		t.Fatalf("decode rbac_policy.yaml: %v", err)
	}
	return p
}

// newRBACTestRouter builds a router backed by the MockStore for RBAC tests.
func newRBACTestRouter() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &handler.Handler{
		Store:  store.NewMockStore(),
		DB:     &store.MockPinger{Healthy: true},
		Redis:  &store.MockPinger{Healthy: true},
		Logger: logger,
	}
	return router.New(h, config.DefaultRequestMaxBodyBytes)
}

// identityForRole returns the VerifiedIdentity to inject for the given role
// string.  Empty string means anonymous (no identity injected).
func identityForRole(role string) (middleware.VerifiedIdentity, bool) {
	switch role {
	case "viewer":
		return middleware.VerifiedIdentity{UserID: "u-viewer", Role: store.RoleViewer, Source: middleware.IdentitySourceSession}, true
	case "contributor":
		return middleware.VerifiedIdentity{UserID: "u-contrib", Role: store.RoleContributor, Source: middleware.IdentitySourceSession}, true
	case "admin":
		return middleware.VerifiedIdentity{UserID: "u-admin", Role: store.RoleAdmin, Source: middleware.IdentitySourceSession}, true
	default:
		return middleware.VerifiedIdentity{}, false
	}
}

// expandPattern replaces chi path parameters with a placeholder value so the
// request path can be used in an HTTP call.
func expandPattern(p string) string {
	for _, seg := range []string{"{id}", "{contractId}", "{tx_hash}", "{noteID}", "{tag}", "{contract_id}", "{name}"} {
		p = strings.ReplaceAll(p, seg, "placeholder")
	}
	return p
}

func TestRBACAuthorizationMatrix(t *testing.T) {
	policy := loadRBACPolicy(t)
	srv := newRBACTestRouter()

	type roleCase struct {
		name     string
		wantCode int
	}

	for _, entry := range policy.Routes {
		entry := entry // capture
		roles := []roleCase{
			{"anonymous", entry.Anonymous},
			{"viewer", entry.Viewer},
			{"contributor", entry.Contributor},
			{"admin", entry.Admin},
		}
		for _, rc := range roles {
			rc := rc
			t.Run(entry.Method+"_"+entry.Pattern+"_"+rc.name, func(t *testing.T) {
				t.Parallel()
				path := "/api/v1" + expandPattern(entry.Pattern)
				req := httptest.NewRequest(entry.Method, path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")

				if id, ok := identityForRole(rc.name); ok {
					req = req.WithContext(middleware.WithIdentity(req.Context(), id))
				}

				rr := httptest.NewRecorder()
				srv.ServeHTTP(rr, req)
				got := rr.Code

				// Authorization rejection must be exact.
				if rc.wantCode == http.StatusUnauthorized || rc.wantCode == http.StatusForbidden {
					if got != rc.wantCode {
						t.Errorf("%s %s as %s: got %d, want %d", entry.Method, path, rc.name, got, rc.wantCode)
					}
					return
				}
				// Access granted: the middleware passed; the handler may return
				// any non-auth status (200, 204, 404, 422, 500 are all fine).
				if got == http.StatusUnauthorized || got == http.StatusForbidden {
					t.Errorf("%s %s as %s: got %d (auth rejection), expected access to be granted", entry.Method, path, rc.name, got)
				}
			})
		}
	}
}
