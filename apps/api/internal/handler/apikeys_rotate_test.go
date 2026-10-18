package handler_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// keyDetail mirrors the metadata subset returned by GET /api/v1/api-keys/{id}
// and the rotation response.
type keyDetail struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	CreatedAt string   `json:"created_at"`
	RotatedAt *string  `json:"rotated_at"`
	Key       string   `json:"key"`
}

func TestRotateAPIKeyFlow(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)

	// Create a key we can then rotate.
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/api-keys", adminToken, adminUser,
		`{"name":"rotate me","scopes":["read:watchdog"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created keyDetail
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.RotatedAt != nil {
		t.Fatalf("new key should have null rotated_at, got %v", *created.RotatedAt)
	}
	oldKey := created.Key

	// The original key authenticates its scope before rotation.
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", oldKey, "").Code; got != http.StatusOK {
		t.Fatalf("pre-rotation old key: want 200, got %d", got)
	}

	// Rotate.
	w = doRequestAsUser(srv, http.MethodPost, "/api/v1/api-keys/"+created.ID+"/rotate", adminToken, adminUser, "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("rotate: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var rotated keyDetail
	if err := json.Unmarshal(w.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	newKey := rotated.Key
	if newKey == "" || newKey == oldKey {
		t.Fatalf("rotation must return a fresh plaintext key, got %q (old %q)", newKey, oldKey)
	}
	// Metadata is preserved across rotation.
	if rotated.ID != created.ID || rotated.Name != created.Name || rotated.CreatedAt != created.CreatedAt {
		t.Fatalf("rotation changed identity: %+v vs %+v", rotated, created)
	}
	if len(rotated.Scopes) != 1 || rotated.Scopes[0] != "read:watchdog" {
		t.Fatalf("rotation changed scopes: %+v", rotated.Scopes)
	}
	if rotated.RotatedAt == nil {
		t.Fatal("rotated_at must be set after rotation")
	}

	// Old key is rejected immediately; new key works.
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", oldKey, "").Code; got != http.StatusUnauthorized {
		t.Fatalf("old key after rotation: want 401, got %d", got)
	}
	if got := doRequest(srv, http.MethodGet, "/api/v1/watchdog/stats", newKey, "").Code; got != http.StatusOK {
		t.Fatalf("new key after rotation: want 200, got %d", got)
	}

	// GET /{id} exposes rotated_at and never leaks key material.
	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/api-keys/"+created.ID, adminToken, adminUser, "")
	if w.Code != http.StatusOK {
		t.Fatalf("get key: want 200, got %d", w.Code)
	}
	if bs := w.Body.String(); jsonHasField(bs, "key") {
		t.Errorf("GET key must not return plaintext material: %s", bs)
	}
	var fetched keyDetail
	if err := json.Unmarshal(w.Body.Bytes(), &fetched); err != nil {
		t.Fatal(err)
	}
	if fetched.RotatedAt == nil {
		t.Error("GET /{id} must include rotated_at after a rotation")
	}
}

func TestRotateUnknownAPIKey(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/api-keys/does-not-exist/rotate", adminToken, adminUser, "{}")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRotateAPIKeyRequiresAdminScope(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	// A non-admin caller (read:contracts key) may not rotate keys.
	w := doRequest(srv, http.MethodPost, "/api/v1/api-keys/key-admin/rotate", readContractsKey, "{}")
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin rotate: want 403, got %d (%s)", w.Code, w.Body.String())
	}
}

// jsonHasField reports whether a JSON object literally contains the given
// top-level field key.
func jsonHasField(body, field string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return false
	}
	_, ok := m[field]
	return ok
}
