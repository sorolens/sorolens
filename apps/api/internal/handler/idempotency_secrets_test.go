package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// postIdempotent sends a POST authenticated with the admin API key (so the
// Idempotency middleware sees an API key) acting as userID.
func postIdempotent(srv http.Handler, path, idemKey, userID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idemKey)
	// A preset identity short-circuits the Identity middleware, so attach the
	// API key explicitly: idempotency only applies to API-key callers.
	req = req.WithContext(middleware.WithIdentity(req.Context(), middleware.VerifiedIdentity{
		UserID: userID,
		Role:   testRole(userID),
		Source: "test",
		APIKey: &store.APIKey{ID: "key-admin", Scopes: []string{store.ScopeAdmin}},
	}))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// TestIdempotencyNeverStoresOrReplaysSecrets covers routes that return a
// plaintext secret exactly once: the response body must not be persisted in
// idempotency_keys and a replay must not return it.
func TestIdempotencyNeverStoresOrReplaysSecrets(t *testing.T) {
	cases := []struct {
		name, path, body, secretField string
	}{
		{"me api key v1", "/api/v1/me/api-keys", `{"name":"ci","scopes":["read:watchdog"]}`, "key"},
		{"me api key v2", "/api/v2/me/api-keys", `{"name":"ci2","scopes":["read:watchdog"]}`, "key"},
		{"subscription signing secret", "/api/v1/watchdog/subscriptions", `{"contract_id":"C1","webhook_url":"https://example.com/hook"}`, "signing_secret"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ms := seedRBACUsers(t)
			seedMonitored(t, ms)
			srv := newTestHandler(ms, true, true)
			idemKey := "idem-secret-" + string(rune('a'+i))

			first := postIdempotent(srv, tc.path, idemKey, contributorUser, tc.body)
			if first.Code != http.StatusCreated {
				t.Fatalf("first: want 201, got %d (%s)", first.Code, first.Body.String())
			}
			var parsed map[string]any
			if err := json.Unmarshal(first.Body.Bytes(), &parsed); err != nil {
				t.Fatal(err)
			}
			secret, _ := parsed[tc.secretField].(string)
			if secret == "" {
				t.Fatalf("expected %q in first response: %s", tc.secretField, first.Body.String())
			}

			// Nothing secret is persisted.
			rec, err := ms.GetIdempotencyRecord(context.Background(), "key-admin", idemKey)
			if err != nil {
				t.Fatalf("idempotency record missing: %v", err)
			}
			if rec.InFlight {
				t.Fatal("record should be completed, not in flight")
			}
			if strings.Contains(rec.ResponseBody, secret) {
				t.Fatalf("secret persisted in idempotency_keys: %s", rec.ResponseBody)
			}
			for _, v := range rec.ResponseHeaders {
				if strings.Contains(v, secret) {
					t.Fatalf("secret persisted in stored headers")
				}
			}

			// A replay with the same key and body is refused, never the original.
			replay := postIdempotent(srv, tc.path, idemKey, contributorUser, tc.body)
			if replay.Code != http.StatusConflict {
				t.Fatalf("replay: want 409, got %d (%s)", replay.Code, replay.Body.String())
			}
			if strings.Contains(replay.Body.String(), secret) {
				t.Fatalf("replay leaked the secret: %s", replay.Body.String())
			}
			if !strings.Contains(replay.Body.String(), "IDEMPOTENT_REPLAY_UNAVAILABLE") {
				t.Fatalf("replay should explain why: %s", replay.Body.String())
			}
		})
	}
}

// Responses without secrets are still stored and replayed (issue #784).
func TestIdempotencyStillReplaysNonSecretResponses(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)
	body := `{"email":"a@example.com","frequency":"daily"}`
	first := postIdempotent(srv, "/api/v1/reports/subscriptions", "idem-plain", contributorUser, body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first: want 201, got %d (%s)", first.Code, first.Body.String())
	}
	replay := postIdempotent(srv, "/api/v1/reports/subscriptions", "idem-plain", contributorUser, body)
	if replay.Code != http.StatusCreated || replay.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay: want 201 replayed, got %d %v", replay.Code, replay.Header())
	}
	if replay.Body.String() != first.Body.String() {
		t.Fatalf("replay body differs")
	}
}
