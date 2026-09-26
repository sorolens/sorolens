package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/middleware"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

type subscriptionBody struct {
	ID             string `json:"id"`
	ContractID     string `json:"contract_id"`
	WebhookURL     string `json:"webhook_url"`
	SeverityFilter string `json:"severity_filter"`
	SigningSecret  string `json:"signing_secret"`
}

type signingSecretBody struct {
	ID            string     `json:"id"`
	SigningSecret string     `json:"signing_secret"`
	CreatedAt     time.Time  `json:"created_at"`
	RotatedAt     *time.Time `json:"rotated_at"`
}

// createSubscription posts a subscription as an admin and returns the decoded
// response.
func createSubscription(t *testing.T, srv http.Handler, contractID string) subscriptionBody {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"contract_id": contractID,
		"webhook_url": "https://example.com/hooks/sorolens",
	})
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watchdog/subscriptions", adminToken, adminUser, string(body))
	if w.Code != http.StatusCreated {
		t.Fatalf("create subscription: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var resp subscriptionBody
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCreateSubscription_ReturnsSigningSecretOnce(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	created := createSubscription(t, srv, "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	if !strings.HasPrefix(created.SigningSecret, "whsec_") {
		t.Fatalf("signing_secret = %q, want a whsec_ secret", created.SigningSecret)
	}
	if created.ID == "" {
		t.Fatal("create response is missing an id")
	}

	// The stored row keeps the key and its SHA-256 digest.
	stored, err := ms.GetSubscription(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if stored.SigningSecret != created.SigningSecret {
		t.Errorf("stored secret %q != returned secret %q", stored.SigningSecret, created.SigningSecret)
	}
	if want := store.HashKey(created.SigningSecret); stored.SigningSecretHash != want {
		t.Errorf("stored hash = %q, want %q", stored.SigningSecretHash, want)
	}
	if stored.SigningSecretCreatedAt.IsZero() {
		t.Error("signing_secret_created_at was not stamped")
	}

	// The list endpoint must not leak the secret.
	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/watchdog/subscriptions", readWatchdogKey, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list subscriptions: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "whsec_") || strings.Contains(w.Body.String(), "signing_secret") {
		t.Errorf("list response leaked the signing secret: %s", w.Body.String())
	}
}

func TestCreateSubscription_RequiresAdmin(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	body, _ := json.Marshal(map[string]string{
		"contract_id": "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"webhook_url": "https://example.com/hook",
	})
	// Admin-scoped credential but only a contributor role: the role gate denies.
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watchdog/subscriptions", adminToken, contributorUser, string(body))
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 for a contributor, got %d (%s)", w.Code, w.Body.String())
	}

	// No credential at all: the admin scope requires authentication.
	w = doRequestAsUser(srv, http.MethodPost, "/api/v1/watchdog/subscriptions", "", "", string(body))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 anonymous, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetSigningSecret_WithinWindow(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)
	created := createSubscription(t, srv, "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/watchdog/subscriptions/"+created.ID+"/signing-secret", adminToken, adminUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body signingSecretBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.SigningSecret != created.SigningSecret {
		t.Errorf("reveal returned %q, want the created secret %q", body.SigningSecret, created.SigningSecret)
	}
}

func TestGetSigningSecret_OutsideWindow(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	// A subscription created long ago: the reveal window has closed.
	stale := store.AlertSubscription{
		ID: "sub_stale", ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		WebhookURL: "https://example.com/hook", SeverityFilter: "Critical",
		SigningSecret: "whsec_stale", SigningSecretHash: store.HashKey("whsec_stale"),
		SigningSecretCreatedAt: time.Now().UTC().Add(-time.Hour),
		CreatedAt:              time.Now().UTC().Add(-time.Hour),
		UpdatedAt:              time.Now().UTC().Add(-time.Hour),
	}
	if err := ms.Create(context.Background(), stale); err != nil {
		t.Fatal(err)
	}

	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/watchdog/subscriptions/sub_stale/signing-secret", adminToken, adminUser, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 outside the window, got %d (%s)", w.Code, w.Body.String())
	}
	var env map[string]any
	_ = json.NewDecoder(w.Body).Decode(&env)
	errObj, _ := env["error"].(map[string]any)
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("want code FORBIDDEN, got %v", errObj["code"])
	}

	// A recent rotation re-opens the window even though creation is old.
	rotated := time.Now().UTC().Add(-time.Minute)
	stale.SigningSecretRotatedAt = &rotated
	if err := ms.RotateSigningSecret(context.Background(), stale.ID, "whsec_new", store.HashKey("whsec_new"), rotated); err != nil {
		t.Fatal(err)
	}
	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/watchdog/subscriptions/sub_stale/signing-secret", adminToken, adminUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 after rotation, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGetSigningSecret_NotFound(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/watchdog/subscriptions/sub_missing/signing-secret", adminToken, adminUser, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRotateSigningSecret(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)
	created := createSubscription(t, srv, "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watchdog/subscriptions/"+created.ID+"/rotate", adminToken, adminUser, "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body signingSecretBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(body.SigningSecret, "whsec_") {
		t.Fatalf("rotated secret = %q, want a whsec_ secret", body.SigningSecret)
	}
	if body.SigningSecret == created.SigningSecret {
		t.Error("rotation must return a new secret")
	}
	if body.RotatedAt == nil {
		t.Error("rotation must stamp rotated_at")
	}

	// The rotated key is what is stored and what the reveal endpoint returns.
	stored, err := ms.GetSubscription(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SigningSecret != body.SigningSecret {
		t.Errorf("stored secret %q != rotated secret %q", stored.SigningSecret, body.SigningSecret)
	}
	if stored.SigningSecretHash != store.HashKey(body.SigningSecret) {
		t.Error("rotated secret hash does not match the stored digest")
	}
}

func TestRotateSigningSecret_NotFound(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watchdog/subscriptions/sub_missing/rotate", adminToken, adminUser, "{}")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestDeleteSubscription_NotFound(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/watchdog/subscriptions/sub_missing", adminToken, adminUser, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestSubscriptionRoutes_Scopes(t *testing.T) {
	cases := []struct {
		method  string
		pattern string
		want    string
	}{
		{http.MethodGet, "/api/v1/watchdog/subscriptions", middleware.ScopeReadWatchdog},
		{http.MethodPost, "/api/v1/watchdog/subscriptions", middleware.ScopeAdmin},
		{http.MethodDelete, "/api/v1/watchdog/subscriptions/{id}", middleware.ScopeAdmin},
		{http.MethodGet, "/api/v1/watchdog/subscriptions/{id}/signing-secret", middleware.ScopeAdmin},
		{http.MethodPost, "/api/v1/watchdog/subscriptions/{id}/rotate", middleware.ScopeAdmin},
	}
	for _, tc := range cases {
		scope, ok := middleware.RequiredScope(tc.method, tc.pattern)
		if !ok {
			t.Errorf("%s %s is missing from the scope table", tc.method, tc.pattern)
			continue
		}
		if scope != tc.want {
			t.Errorf("%s %s scope = %q, want %q", tc.method, tc.pattern, scope, tc.want)
		}
	}
}
