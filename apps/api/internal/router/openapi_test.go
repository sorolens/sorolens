package router_test

import (
	"bufio"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sorolens/sorolens/apps/api/internal/config"
	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// specPath is docs/openapi.yaml relative to this package.
var specPath = filepath.Join("..", "..", "..", "..", "docs", "openapi.yaml")

var (
	specPathLine   = regexp.MustCompile(`^  (/\S*):\s*$`)
	specMethodLine = regexp.MustCompile(`^    (get|put|post|delete|patch|head|options):\s*$`)
)

// specOperations returns "METHOD /path" for every operation in the spec.
func specOperations(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open(specPath)
	if err != nil {
		t.Fatalf("open spec: %v", err)
	}
	defer func() { _ = f.Close() }()

	ops := map[string]bool{}
	inPaths := false
	current := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "paths:" {
			inPaths = true
			continue
		}
		if inPaths && line != "" && !strings.HasPrefix(line, " ") {
			break // next top-level key
		}
		if !inPaths {
			continue
		}
		if m := specPathLine.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if m := specMethodLine.FindStringSubmatch(line); m != nil && current != "" {
			ops[strings.ToUpper(m[1])+" "+current] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return ops
}

// TestOpenAPICoversEveryRoute keeps docs/openapi.yaml in lockstep with the
// router (issue #104): every registered route must be documented, and the
// spec must not document routes that no longer exist.
func TestOpenAPICoversEveryRoute(t *testing.T) {
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

	live := map[string]bool{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// pprof debug routes are internal endpoints not part of the public API.
		if strings.HasPrefix(route, "/debug/") {
			return nil
		}
		route = strings.ReplaceAll(route, "/*/", "/")
		if len(route) > 1 {
			route = strings.TrimSuffix(route, "/")
		}
		live[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	spec := specOperations(t)
	var missing, stale []string
	for op := range live {
		if !spec[op] {
			missing = append(missing, op)
		}
	}
	for op := range spec {
		if !live[op] {
			stale = append(stale, op)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("routes missing from docs/openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("docs/openapi.yaml documents routes the router does not serve:\n  %s", strings.Join(stale, "\n  "))
	}
	if len(live) < 30 {
		t.Fatalf("walked only %d routes; the walker is broken", len(live))
	}
}
