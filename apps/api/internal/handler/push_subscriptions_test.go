package handler_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

func pushTestSubscription(endpoint string) map[string]any {
	auth := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 16))
	p256 := append([]byte{4}, bytes.Repeat([]byte{2}, 64)...)
	return map[string]any{
		"endpoint": endpoint,
		"keys": map[string]string{
			"auth":   auth,
			"p256dh": base64.RawURLEncoding.EncodeToString(p256),
		},
		"filters": map[string]string{"severity": "Critical"},
	}
}

func doPushRequest(srv http.Handler, method, path, userID, body string, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	req = withTestIdentity(req, userID)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func marshalBody(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// TestPushSubscriptionLifecycle covers issue #592: a session registers a
// subscription, sees only its own, and can unsubscribe idempotently.
func TestPushSubscriptionLifecycle(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)
	endpoint := "https://fcm.googleapis.com/fcm/send/abc123"
	body := marshalBody(t, pushTestSubscription(endpoint))

	w := doPushRequest(srv, http.MethodPost, "/api/v1/push/subscriptions", contributorUser, body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}

	var created struct {
		ID       string `json:"id"`
		Endpoint string `json:"endpoint"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Endpoint != endpoint {
		t.Fatalf("unexpected create response: %s", w.Body.String())
	}

	// The owner sees the subscription, without key material.
	w = doPushRequest(srv, http.MethodGet, "/api/v1/push/subscriptions", contributorUser, "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(endpoint)) {
		t.Fatalf("list should contain the subscription: %s", w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte(`"p256dh"`)) {
		t.Error("list response must not include push key material")
	}

	// Another user neither sees nor removes it.
	w = doPushRequest(srv, http.MethodGet, "/api/v1/push/subscriptions", adminUser, "", nil)
	if w.Code != http.StatusOK || bytes.Contains(w.Body.Bytes(), []byte(endpoint)) {
		t.Fatalf("other user should have an empty list: %d %s", w.Code, w.Body.String())
	}
	w = doPushRequest(srv, http.MethodDelete, "/api/v1/push/subscriptions", adminUser,
		marshalBody(t, map[string]string{"endpoint": endpoint}), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("cross-user delete should be an idempotent 204, got %d", w.Code)
	}
	if subs, _ := ms.ListPushSubscriptions(t.Context(), contributorUser); len(subs) != 1 {
		t.Fatalf("cross-user delete removed the row: %+v", subs)
	}

	// The owner can unsubscribe, and unsubscribing twice still succeeds.
	for i := 0; i < 2; i++ {
		w = doPushRequest(srv, http.MethodDelete, "/api/v1/push/subscriptions", contributorUser,
			marshalBody(t, map[string]string{"endpoint": endpoint}), nil)
		if w.Code != http.StatusNoContent {
			t.Fatalf("unsubscribe #%d: want 204, got %d", i+1, w.Code)
		}
	}
	if subs, _ := ms.ListPushSubscriptions(t.Context(), contributorUser); len(subs) != 0 {
		t.Fatalf("unsubscribe left rows behind: %+v", subs)
	}
}

func TestPushSubscriptionRequiresSession(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	endpoint := "https://fcm.googleapis.com/fcm/send/abc123"

	for _, tc := range []struct {
		method string
		body   string
	}{
		{http.MethodGet, ""},
		{http.MethodPost, marshalBody(t, pushTestSubscription(endpoint))},
		{http.MethodDelete, marshalBody(t, map[string]string{"endpoint": endpoint})},
	} {
		w := doPushRequest(srv, tc.method, "/api/v1/push/subscriptions", "", tc.body, nil)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s without a session: want 401, got %d", tc.method, w.Code)
		}
	}
}

func TestPushSubscriptionValidatesInput(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	endpoint := "https://fcm.googleapis.com/fcm/send/abc123"

	cases := map[string]map[string]any{
		"unknown host": {"endpoint": "https://example.com/push", "keys": pushTestSubscription(endpoint)["keys"]},
		"http":         {"endpoint": "http://fcm.googleapis.com/push", "keys": pushTestSubscription(endpoint)["keys"]},
		"short auth":   {"endpoint": endpoint, "keys": map[string]string{"auth": "short", "p256dh": pushTestSubscription(endpoint)["keys"].(map[string]string)["p256dh"]}},
		"bad filter":   {"endpoint": endpoint, "keys": pushTestSubscription(endpoint)["keys"], "filters": map[string]string{"unknown": "x"}},
		"bad severity": {"endpoint": endpoint, "keys": pushTestSubscription(endpoint)["keys"], "filters": map[string]string{"severity": "Loud"}},
	}
	for name, payload := range cases {
		w := doPushRequest(srv, http.MethodPost, "/api/v1/push/subscriptions", contributorUser, marshalBody(t, payload), nil)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d (%s)", name, w.Code, w.Body.String())
		}
	}
}

// TestPushDispatchRequiresSecret checks that the internal dispatch route is
// not reachable by a user session or with a wrong secret.
func TestPushDispatchRequiresSecret(t *testing.T) {
	ms := seedScopedKeyStore(t)
	ms.AddPushSubscription(store.PushSubscription{
		ID:        "sub-1",
		Endpoint:  "https://fcm.googleapis.com/fcm/send/abc123",
		AuthKey:   "auth",
		P256DHKey: "p256dh",
		UserID:    contributorUser,
	})
	srv := newTestHandler(ms, true, true)

	w := doPushRequest(srv, http.MethodGet, "/api/v1/push/subscriptions/dispatch", contributorUser, "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("dispatch without secret: want 401, got %d", w.Code)
	}

	t.Setenv("PUSH_INTERNAL_SECRET", "dispatch-secret")
	w = doPushRequest(srv, http.MethodGet, "/api/v1/push/subscriptions/dispatch", "",
		"", map[string]string{"X-Push-Secret": "wrong"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("dispatch with wrong secret: want 401, got %d", w.Code)
	}

	w = doPushRequest(srv, http.MethodGet, "/api/v1/push/subscriptions/dispatch", "",
		"", map[string]string{"X-Push-Secret": "dispatch-secret"})
	if w.Code != http.StatusOK {
		t.Fatalf("dispatch with secret: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"p256dh":"p256dh"`)) {
		t.Fatalf("dispatch must include key material: %s", w.Body.String())
	}

	w = doPushRequest(srv, http.MethodDelete, "/api/v1/push/subscriptions/dispatch", "",
		marshalBody(t, map[string]string{"endpoint": "https://fcm.googleapis.com/fcm/send/abc123"}),
		map[string]string{"X-Push-Secret": "dispatch-secret"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("dispatch delete: want 204, got %d", w.Code)
	}
	if subs, _ := ms.ListPushSubscriptions(t.Context(), ""); len(subs) != 0 {
		t.Fatalf("dispatch delete left rows behind: %+v", subs)
	}
}

// TestPushSubscriptionCannotBeHijacked: re-registering an endpoint that another
// user owns must be rejected and must not change its owner or keys.
func TestPushSubscriptionCannotBeHijacked(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)
	endpoint := "https://fcm.googleapis.com/fcm/send/victim"
	body := marshalBody(t, pushTestSubscription(endpoint))

	w := doPushRequest(srv, http.MethodPost, "/api/v1/push/subscriptions", contributorUser, body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("owner create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	before, _ := ms.ListPushSubscriptions(t.Context(), contributorUser)
	if len(before) != 1 {
		t.Fatalf("want 1 owned subscription, got %+v", before)
	}

	attacker := pushTestSubscription(endpoint)
	attacker["keys"] = map[string]string{
		"auth":   base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{9}, 16)),
		"p256dh": base64.RawURLEncoding.EncodeToString(append([]byte{4}, bytes.Repeat([]byte{8}, 64)...)),
	}
	w = doPushRequest(srv, http.MethodPost, "/api/v1/push/subscriptions", adminUser, marshalBody(t, attacker), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("takeover attempt: want 409, got %d (%s)", w.Code, w.Body.String())
	}

	after, _ := ms.ListPushSubscriptions(t.Context(), "")
	if len(after) != 1 || after[0].UserID != contributorUser ||
		after[0].AuthKey != before[0].AuthKey || after[0].P256DHKey != before[0].P256DHKey {
		t.Fatalf("subscription was modified by another user: %+v", after)
	}

	// The owner can still refresh their own subscription.
	w = doPushRequest(srv, http.MethodPost, "/api/v1/push/subscriptions", contributorUser, body, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("owner refresh: want 201, got %d", w.Code)
	}
}
