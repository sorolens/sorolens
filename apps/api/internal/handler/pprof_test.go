package handler_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestPprofEndpointsRequireAdminAuth covers issue #157: net/http/pprof is
// mounted on the API router at /debug/pprof/* but is gated by the admin role.
// Every caller that is not an admin — anonymous callers included — must get
// 403, while an admin reaches the real pprof handlers.
func TestPprofEndpointsRequireAdminAuth(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)

	cases := []struct {
		name   string
		path   string
		userID string
		want   int
	}{
		{"anonymous denied index", "/debug/pprof/", "", http.StatusForbidden},
		{"viewer denied index", "/debug/pprof/", viewerUser, http.StatusForbidden},
		{"contributor denied index", "/debug/pprof/", contributorUser, http.StatusForbidden},
		{"admin allowed index", "/debug/pprof/", adminUser, http.StatusOK},

		{"anonymous denied named profile", "/debug/pprof/goroutine", "", http.StatusForbidden},
		{"contributor denied named profile", "/debug/pprof/goroutine", contributorUser, http.StatusForbidden},
		{"admin allowed named profile", "/debug/pprof/goroutine", adminUser, http.StatusOK},

		{"anonymous denied cmdline", "/debug/pprof/cmdline", "", http.StatusForbidden},
		{"admin allowed cmdline", "/debug/pprof/cmdline", adminUser, http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodGet, tc.path, "", tc.userID, "")
			if w.Code != tc.want {
				t.Fatalf("GET %s as %q: want %d, got %d (%s)",
					tc.path, tc.userID, tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// TestPprofIndexBodyForAdmin asserts the admin response is the actual pprof
// index page rather than an empty 200, so the gate does not swallow the
// profiling surface.
func TestPprofIndexBodyForAdmin(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)

	w := doRequestAsUser(srv, http.MethodGet, "/debug/pprof/", "", adminUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("want pprof index HTML, got Content-Type %q", ct)
	}
	if body := w.Body.String(); !strings.Contains(body, "Types of profiles available") {
		t.Errorf("response does not look like the pprof index: %q", body)
	}
}
