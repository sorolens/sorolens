package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const labelAccount = "GD2E2SJSD2C5SQGMH2YPIXI6F7NS2S6MZ2G4T3VDWXQH2FQT4M6URPLT"

func TestWorkspaceLabelResolutionIsPrivate(t *testing.T) {
	ms := seedRoleStore(t)
	srv := newTestHandler(ms, true, true)
	body, _ := json.Marshal(map[string]string{"label": "treasury", "value": labelAccount, "scope": "workspace"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/labels", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", contributorUser)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create label: want 201, got %d: %s", w.Code, w.Body.String())
	}

	for _, tc := range []struct{ user, want string }{{contributorUser, labelAccount}, {adminUser, ""}} {
		req = httptest.NewRequest(http.MethodGet, "/api/v1/resolve?query=treasury", nil)
		req.Header.Set("X-User-ID", tc.user)
		w = httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if tc.want == "" {
			if w.Code != http.StatusNotFound {
				t.Errorf("other workspace: want 404, got %d", w.Code)
			}
			continue
		}
		var response map[string]string
		if w.Code != http.StatusOK || json.NewDecoder(w.Body).Decode(&response) != nil || response["value"] != tc.want {
			t.Errorf("owner resolution failed: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPublicLabelResolutionAndRawPassthrough(t *testing.T) {
	ms := seedRoleStore(t)
	ms.UpsertLabel(t.Context(), store.Label{Label: "treasury", Value: labelAccount, Public: true})
	srv := newTestHandler(ms, true, true)
	for _, path := range []string{"/api/v1/resolve?query=treasury", "/api/v1/resolve?query=" + labelAccount} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: want 200, got %d", path, w.Code)
		}
	}
}
