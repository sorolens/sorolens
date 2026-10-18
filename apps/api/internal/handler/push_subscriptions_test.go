package handler_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// pushP256dh is a realistic 65-byte uncompressed P-256 point (0x04 prefix)
// in base64url — exactly what PushSubscription.toJSON() produces. The string
// is the sample VAPID public key from the web-side push route tests.
const pushP256dh = "BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U"

// pushAuth is base64url of a 16-byte auth secret (22 unpadded chars).
const pushAuth = "AAAAAAAAAAAAAAAAAAAAAA"

func pushBody(endpoint, auth, p256, extra string) string {
	if extra != "" && !strings.HasPrefix(extra, ",") {
		extra = "," + extra
	}
	return fmt.Sprintf(`{"endpoint":%q,"keys":{"auth":%q,"p256dh":%q}%s}`,
		endpoint, auth, p256, extra)
}

const pushSubPath = "/api/v1/push/subscriptions"

func TestPushSubscriptionLifecycle(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	// Subscribe (self-service, same anonymous posture as report subs).
	w := doRequest(srv, http.MethodPost, pushSubPath, "",
		pushBody("https://push.example.com/abc123", pushAuth, pushP256dh, `"user_id":"u-1"`))
	if w.Code != http.StatusCreated {
		t.Fatalf("subscribe: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		Endpoint string `json:"endpoint"`
		UserID   string `json:"user_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.UserID != "u-1" {
		t.Fatalf("unexpected create response: %s", w.Body.String())
	}
	// The response must never echo the key material back.
	if strings.Contains(w.Body.String(), pushAuth) || strings.Contains(w.Body.String(), pushP256dh) {
		t.Errorf("create response leaked key material: %s", w.Body.String())
	}

	// Anonymous listing is refused: the list carries push key material.
	if w = doRequest(srv, http.MethodGet, pushSubPath, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list: want 401, got %d (%s)", w.Code, w.Body.String())
	}

	// A contributor (admin key + admin identity) may list for fan-out.
	w = doRequestAsUser(srv, http.MethodGet, pushSubPath, adminToken, adminUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("authorized list: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var listed struct {
		Subscriptions []struct {
			ID        string `json:"id"`
			Endpoint  string `json:"endpoint"`
			P256dhKey string `json:"p256dh_key"`
			AuthKey   string `json:"auth_key"`
		} `json:"subscriptions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Subscriptions) != 1 || listed.Subscriptions[0].ID != created.ID {
		t.Fatalf("unexpected list: %s", w.Body.String())
	}
	if listed.Subscriptions[0].P256dhKey != pushP256dh || listed.Subscriptions[0].AuthKey != pushAuth {
		t.Fatalf("list did not return the key material the sender needs: %s", w.Body.String())
	}

	// Duplicate endpoint is an idempotent refresh: same id, one row, keys
	// rotated in place. (Upsert refreshes the whole row, owner included —
	// same-user re-subscription is the realistic flow.)
	rotatedAuth := base64.RawURLEncoding.EncodeToString(append([]byte{0xAB}, make([]byte, 15)...))
	w = doRequest(srv, http.MethodPost, pushSubPath, "",
		pushBody("https://push.example.com/abc123", rotatedAuth, pushP256dh, `"user_id":"u-1"`))
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate subscribe: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var again struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again.ID != created.ID {
		t.Fatalf("duplicate endpoint created a second row: want id %s, got %s", created.ID, again.ID)
	}
	w = doRequestAsUser(srv, http.MethodGet, pushSubPath+"?user_id=u-1", adminToken, adminUser, "")
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Subscriptions) != 1 {
		t.Fatalf("duplicate subscribe must not add rows, list len=%d (%s)", len(listed.Subscriptions), w.Body.String())
	}
	if listed.Subscriptions[0].AuthKey != rotatedAuth {
		t.Errorf("duplicate subscribe did not refresh keys: %s", w.Body.String())
	}

	// Unsubscribe by endpoint (the browser flow), then by id.
	w = doRequest(srv, http.MethodDelete, pushSubPath+"?endpoint=https://push.example.com/abc123", "", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("unsubscribe by endpoint: want 204, got %d (%s)", w.Code, w.Body.String())
	}
	// Idempotency boundary: a vanished subscription answers 404.
	w = doRequest(srv, http.MethodDelete, pushSubPath+"?endpoint=https://push.example.com/abc123", "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("second unsubscribe: want 404, got %d", w.Code)
	}
	w = doRequest(srv, http.MethodDelete, pushSubPath+"/"+created.ID, "", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete removed row by id: want 404, got %d", w.Code)
	}

	// Fresh subscription, deleted by id.
	w = doRequest(srv, http.MethodPost, pushSubPath, "",
		pushBody("https://push.example.com/second", pushAuth, pushP256dh, ""))
	if w.Code != http.StatusCreated {
		t.Fatalf("second subscribe: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &created)
	w = doRequest(srv, http.MethodDelete, pushSubPath+"/"+created.ID, "", "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete by id: want 204, got %d", w.Code)
	}
	w = doRequestAsUser(srv, http.MethodGet, pushSubPath, adminToken, adminUser, "")
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Subscriptions) != 0 {
		t.Fatalf("expected empty list after deletes, got %s", w.Body.String())
	}
}

func TestPushSubscriptionScopeMatrix(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	body := pushBody("https://push.example.com/xyz", pushAuth, pushP256dh, "")

	// A key without write:contracts may not subscribe.
	if w := doRequest(srv, http.MethodPost, pushSubPath, readContractsKey, body); w.Code != http.StatusForbidden {
		t.Errorf("read:contracts POST: want 403, got %d", w.Code)
	}
	// A key without read:watchdog may not list key material.
	if w := doRequest(srv, http.MethodGet, pushSubPath, readContractsKey, ""); w.Code != http.StatusForbidden {
		t.Errorf("read:contracts GET: want 403, got %d", w.Code)
	}
	// The write scope is sufficient for the self-service mutations.
	if w := doRequest(srv, http.MethodPost, pushSubPath, adminToken, body); w.Code != http.StatusCreated {
		t.Errorf("scoped POST: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	// Missing endpoint parameter on the endpoint-shaped delete is a 422.
	if w := doRequest(srv, http.MethodDelete, pushSubPath, "", ""); w.Code != http.StatusUnprocessableEntity {
		t.Errorf("unsubscribe without endpoint: want 422, got %d", w.Code)
	}
}

func TestPushSubscriptionValidation(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	good65 := func(first byte) string {
		b := append([]byte{first}, make([]byte, 64)...)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	shortAuth := base64.RawURLEncoding.EncodeToString(make([]byte, 15))
	longAuth := base64.RawURLEncoding.EncodeToString(make([]byte, 33))

	cases := []struct {
		name string
		body string
	}{
		{"missing endpoint", `{"keys":{"auth":"a","p256dh":"b"}}`},
		{"empty endpoint", pushBody("", pushAuth, pushP256dh, "")},
		{"http endpoint rejected", pushBody("http://push.example.com/x", pushAuth, pushP256dh, "")},
		{"non-url endpoint", pushBody("not a url", pushAuth, pushP256dh, "")},
		{"oversized endpoint", pushBody("https://push.example.com/"+strings.Repeat("a", 3000), pushAuth, pushP256dh, "")},
		{"missing keys", `{"endpoint":"https://push.example.com/x"}`},
		{"missing p256dh", pushBody("https://push.example.com/x", pushAuth, "", "")},
		{"missing auth", pushBody("https://push.example.com/x", "", pushP256dh, "")},
		{"auth too short", pushBody("https://push.example.com/x", shortAuth, pushP256dh, "")},
		{"auth too long", pushBody("https://push.example.com/x", longAuth, pushP256dh, "")},
		{"auth not base64url", pushBody("https://push.example.com/x", "not_base64+/", pushP256dh, "")},
		{"p256dh wrong length", pushBody("https://push.example.com/x", pushAuth, good65(0x04)[:40], "")},
		{"p256dh not a P-256 point", pushBody("https://push.example.com/x", pushAuth, good65(0x02), "")},
		{"standard alphabet rejected", pushBody("https://push.example.com/x", strings.ReplaceAll(pushAuth, "-", "+"), "BE+62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U", "")},
		{"filters not an object", pushBody("https://push.example.com/x", pushAuth, pushP256dh, `"filters":[1,2]`)},
		{"filters too large", pushBody("https://push.example.com/x", pushAuth, pushP256dh, `"filters":{"k":"`+strings.Repeat("v", 5000)+`"}`)},
		{"user_id too long", pushBody("https://push.example.com/x", pushAuth, pushP256dh, `"user_id":"`+strings.Repeat("u", 200)+`"`)},
	}
	for _, tc := range cases {
		w := doRequest(srv, http.MethodPost, pushSubPath, "", tc.body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d (%s)", tc.name, w.Code, truncate(w.Body.String()))
		}
	}

	// A well-formed request with valid optional fields is accepted.
	w := doRequest(srv, http.MethodPost, pushSubPath, "",
		pushBody("https://push.example.com/ok", pushAuth, pushP256dh, `"filters":{"severities":["Critical"]}`))
	if w.Code != http.StatusCreated {
		t.Fatalf("valid subscribe with filters: want 201, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestPushSubscriptionOversizedBody(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	// The global BodyLimit middleware (config.DefaultRequestMaxBodyBytes,
	// 1 MiB) must answer 413 before the handler sees the payload.
	huge := pushBody("https://push.example.com/x", pushAuth, pushP256dh,
		`"filters":{"pad":"`+strings.Repeat("A", (1<<20)+1024)+`"}`)
	w := doRequest(srv, http.MethodPost, pushSubPath, "", huge)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: want 413, got %d", w.Code)
	}
}

func truncate(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
