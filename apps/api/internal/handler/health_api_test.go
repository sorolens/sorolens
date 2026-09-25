package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func TestAPIHealthHealthy(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %q", body["status"])
	}
	if body["db"] != "connected" {
		t.Errorf("want db=connected, got %q", body["db"])
	}
	if body["redis"] != "connected" {
		t.Errorf("want redis=connected, got %q", body["redis"])
	}
	if _, err := time.Parse(time.RFC3339, body["timestamp"]); err != nil {
		t.Errorf("timestamp %q is not RFC3339: %v", body["timestamp"], err)
	}
}

func TestAPIHealthUnhealthy(t *testing.T) {
	cases := []struct {
		name         string
		dbHealthy    bool
		redisHealthy bool
		wantDB       string
		wantRedis    string
	}{
		{"db down", false, true, "unreachable", "connected"},
		{"redis down", true, false, "connected", "unreachable"},
		{"both down", false, false, "unreachable", "unreachable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestHandler(store.NewMockStore(), tc.dbHealthy, tc.redisHealthy)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			// The service itself is up even when a dependency is not, so the
			// probe must still report 200 and surface the state in the body.
			if w.Code != http.StatusOK {
				t.Fatalf("want 200 even when unhealthy, got %d", w.Code)
			}
			var body map[string]string
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["db"] != tc.wantDB {
				t.Errorf("want db=%s, got %q", tc.wantDB, body["db"])
			}
			if body["redis"] != tc.wantRedis {
				t.Errorf("want redis=%s, got %q", tc.wantRedis, body["redis"])
			}
			if body["status"] != "degraded" {
				t.Errorf("want status=degraded, got %q", body["status"])
			}
		})
	}
}
