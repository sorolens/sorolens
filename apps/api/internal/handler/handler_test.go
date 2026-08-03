package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/router"
	"github.com/sorolens/sorolens/apps/api/internal/store"
	"log/slog"
	"os"
)

func newTestHandler(ms *store.MockStore, dbHealthy, redisHealthy bool) http.Handler {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	h := &handler.Handler{
		Store:  ms,
		DB:     &store.MockPinger{Healthy: dbHealthy},
		Redis:  &store.MockPinger{Healthy: redisHealthy},
		Logger: logger,
	}
	return router.New(h)
}

func TestHealth(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
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
}

func TestReadyzHealthy(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestReadyzUnhealthy(t *testing.T) {
	cases := []struct {
		name         string
		dbHealthy    bool
		redisHealthy bool
	}{
		{"db down", false, true},
		{"redis down", true, false},
		{"both down", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestHandler(store.NewMockStore(), tc.dbHealthy, tc.redisHealthy)
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("want 503, got %d", w.Code)
			}
		})
	}
}

func TestRegisterContractInvalidBody(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
	var env map[string]any
	_ = json.NewDecoder(w.Body).Decode(&env)
	if _, ok := env["error"]; !ok {
		t.Error("want error envelope")
	}
}

func TestRegisterContractInvalidID(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	body, _ := json.Marshal(map[string]string{"id": "short", "network": "testnet"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}

func TestRegisterContractSuccess(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	validID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB2" + "22"
	// Use a well-formed 56-char contract ID starting with C
	validID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	body, _ := json.Marshal(map[string]string{
		"id":      validID,
		"network": "testnet",
		"label":   "my-contract",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != validID {
		t.Errorf("want id=%s, got %v", validID, resp["id"])
	}
	if resp["status"] != "pending" {
		t.Errorf("want status=pending, got %v", resp["status"])
	}
}

func TestGetContractNotFound(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts/CNONEXISTENT", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	var env map[string]any
	_ = json.NewDecoder(w.Body).Decode(&env)
	errObj, _ := env["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("want code=NOT_FOUND, got %v", errObj["code"])
	}
}

func TestGlobalStatsShape(t *testing.T) {
	ms := store.NewMockStore()
	ms.SetGlobalStats(store.GlobalStats{
		TrackedContracts:    3,
		TotalEvents:         100,
		TotalInvocations:    50,
		TotalStorageEntries: 25,
	})
	srv := newTestHandler(ms, true, true)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/global", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tracked_contracts", "total_events", "total_invocations", "total_storage_entries"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing key %q in response", key)
		}
	}
	if resp["tracked_contracts"].(float64) != 3 {
		t.Errorf("want tracked_contracts=3, got %v", resp["tracked_contracts"])
	}
}

func TestContentTypeMiddleware(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	body, _ := json.Marshal(map[string]string{"id": "x", "network": "testnet"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", bytes.NewBuffer(body))
	// no Content-Type header
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("want 415, got %d", w.Code)
	}
}
