package router_test

import (
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestPprofRoutesRegisteredOnce pins the /debug/pprof route set (issue #538):
// the pprof family is mounted by a single admin-gated block, not twice.
// Admin gating itself is covered by handler.TestPprofEndpointsRequireAdminAuth.
func TestPprofRoutesRegisteredOnce(t *testing.T) {
	r := router.New(&handler.Handler{
		Store:  store.NewMockStore(),
		DB:     &store.MockPinger{Healthy: true},
		Redis:  &store.MockPinger{Healthy: true},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, config.DefaultRequestMaxBodyBytes)
	routes, ok := r.(chi.Routes)
	if !ok {
		t.Fatalf("router is %T, want chi.Routes", r)
	}

	seen := map[string]bool{}
	err := chi.Walk(routes, func(_, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(route, "/debug/pprof") {
			seen[route] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(seen))
	for route := range seen {
		got = append(got, route)
	}
	sort.Strings(got)

	want := []string{
		"/debug/pprof",
		"/debug/pprof/cmdline",
		"/debug/pprof/profile",
		"/debug/pprof/symbol",
		"/debug/pprof/trace",
		"/debug/pprof/{name}",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pprof routes:\n got  %v\n want %v", got, want)
	}
}
