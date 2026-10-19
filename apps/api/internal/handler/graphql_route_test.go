package handler_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const watchdogOnlyKey = "sl_watchdog_only_token_for_tests"

func TestGraphQLRoute(t *testing.T) {
	ms := seedRBACUsers(t)
	ms.AddAPIKey(store.APIKey{
		ID: "key-watchdog", Name: "watchdog", KeyPrefix: "sl_watch",
		KeyHash:   store.HashKey(watchdogOnlyKey),
		Scopes:    []string{store.ScopeReadWatchdog},
		CreatedAt: time.Now().UTC(),
	})
	srv := newTestHandler(ms, true, true)
	body := `{"query":"{ contracts { nodes { id } } }"}`

	cases := []struct {
		name   string
		method string
		token  string
		want   int
	}{
		{"anonymous POST", http.MethodPost, "", http.StatusOK},
		{"read:contracts key", http.MethodPost, readContractsKey, http.StatusOK},
		{"key without read:contracts", http.MethodPost, watchdogOnlyKey, http.StatusForbidden},
		{"GET not allowed", http.MethodGet, "", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := body
			if tc.method == http.MethodGet {
				b = ""
			}
			w := doRequest(srv, tc.method, "/graphql", tc.token, b)
			if w.Code != tc.want {
				t.Fatalf("want %d, got %d (%s)", tc.want, w.Code, w.Body.String())
			}
			if tc.want == http.StatusOK && !strings.Contains(w.Body.String(), `"contracts"`) {
				t.Fatalf("unexpected body: %s", w.Body.String())
			}
		})
	}

	// GraphQL is read-only and lives outside /api/v1, so it is not audited.
	time.Sleep(20 * time.Millisecond)
	if n := len(ms.AuditEvents()); n != 0 {
		t.Fatalf("graphql reads must not be audited, got %d rows", n)
	}
}
