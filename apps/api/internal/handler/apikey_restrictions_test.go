package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestCreateAPIKeyRestrictions covers how the admin create endpoint validates
// and canonicalizes allowed_cidrs/allowed_origins (issue #793).
func TestCreateAPIKeyRestrictions(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantMessage string
		wantCIDRs   []string
		wantOrigins []string
	}{
		{
			name:        "no allowlists keeps a key unrestricted",
			body:        `{"name":"plain","scopes":["read:contracts"]}`,
			wantStatus:  http.StatusCreated,
			wantCIDRs:   []string{},
			wantOrigins: []string{},
		},
		{
			name:        "allowlists are canonicalized",
			body:        `{"name":"restricted","scopes":["read:contracts"],"allowed_cidrs":["203.0.113.7","2001:db8::/32"],"allowed_origins":["HTTPS://App.Example.com/"]}`,
			wantStatus:  http.StatusCreated,
			wantCIDRs:   []string{"203.0.113.7/32", "2001:db8::/32"},
			wantOrigins: []string{"https://app.example.com"},
		},
		{
			name:        "invalid cidr is rejected",
			body:        `{"name":"bad","scopes":["read:contracts"],"allowed_cidrs":["not-a-network"]}`,
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "invalid CIDR range",
		},
		{
			name:        "invalid origin is rejected",
			body:        `{"name":"bad","scopes":["read:contracts"],"allowed_origins":["https://app.example.com/keys"]}`,
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "invalid origin",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest(srv, http.MethodPost, "/api/v1/api-keys", adminToken, tc.body)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (%s)", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantStatus != http.StatusCreated {
				if !strings.Contains(w.Body.String(), tc.wantMessage) {
					t.Fatalf("body = %s, want message containing %q", w.Body.String(), tc.wantMessage)
				}
				return
			}
			var created struct {
				AllowedCIDRs   []string `json:"allowed_cidrs"`
				AllowedOrigins []string `json:"allowed_origins"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
				t.Fatal(err)
			}
			if strings.Join(created.AllowedCIDRs, ",") != strings.Join(tc.wantCIDRs, ",") {
				t.Fatalf("allowed_cidrs = %v, want %v", created.AllowedCIDRs, tc.wantCIDRs)
			}
			if strings.Join(created.AllowedOrigins, ",") != strings.Join(tc.wantOrigins, ",") {
				t.Fatalf("allowed_origins = %v, want %v", created.AllowedOrigins, tc.wantOrigins)
			}
		})
	}
}

// TestMyAPIKeyRestrictions checks that a self-service key can be restricted too.
func TestMyAPIKeyRestrictions(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/me/api-keys", "", contributorUser,
		`{"name":"restricted","scopes":["read:contracts"],"allowed_cidrs":["10.0.0.0/8"],"allowed_origins":["https://app.example.com"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		AllowedCIDRs   []string `json:"allowed_cidrs"`
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.AllowedCIDRs) != 1 || created.AllowedCIDRs[0] != "10.0.0.0/8" {
		t.Fatalf("allowed_cidrs = %v", created.AllowedCIDRs)
	}
	if len(created.AllowedOrigins) != 1 || created.AllowedOrigins[0] != "https://app.example.com" {
		t.Fatalf("allowed_origins = %v", created.AllowedOrigins)
	}

	w = doRequestAsUser(srv, http.MethodPost, "/api/v1/me/api-keys", "", contributorUser,
		`{"name":"bad","allowed_cidrs":["10.0.0.0/64"]}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid cidr: want 422, got %d (%s)", w.Code, w.Body.String())
	}
}

// TestRestrictedKeyIsRejectedFromOtherAddresses exercises the whole stack: a
// key created with a network allowlist cannot be used from an address outside
// it, and the rejection uses the standard envelope with the distinct code.
func TestRestrictedKeyIsRejectedFromOtherAddresses(t *testing.T) {
	ms := seedScopedKeyStore(t)
	srv := newTestHandler(ms, true, true)

	ms.AddAPIKey(store.APIKey{
		ID:           "key-restricted",
		Name:         "restricted",
		KeyPrefix:    "sl_restr",
		KeyHash:      store.HashKey("sl_restricted_token"),
		Scopes:       []string{store.ScopeReadContracts},
		CreatedAt:    time.Now().UTC(),
		AllowedCIDRs: []string{"10.0.0.0/8"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("Authorization", "Bearer sl_restricted_token")
	req.RemoteAddr = "198.51.100.7:5000"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "API_KEY_RESTRICTED" {
		t.Fatalf("error code = %q, want API_KEY_RESTRICTED", body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Fatal("error message should explain which restriction rejected the request")
	}

	// The same key from an allowed address still works.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/contracts", nil)
	req.Header.Set("Authorization", "Bearer sl_restricted_token")
	req.RemoteAddr = "10.5.6.7:5000"
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("allowed address: status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
}
