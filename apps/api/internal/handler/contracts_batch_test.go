package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// 56-character strkeys, matching the API's contract ID validation.
var (
	batchIDOne = "C" + strings.Repeat("A", 55)
	batchIDTwo = "C" + strings.Repeat("B", 55)
)

func seedBatchStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := seedRBACUsers(t)
	ctx := context.Background()
	for _, id := range []string{batchIDOne, batchIDTwo} {
		if err := ms.UpsertContract(ctx, store.Contract{
			ID: id, Network: "testnet", Status: "active", Label: "old",
		}); err != nil {
			t.Fatal(err)
		}
	}
	return ms
}

func batchBody(t *testing.T, ids []string, action string, args map[string]any) string {
	t.Helper()
	body := map[string]any{"ids": ids, "action": action}
	if args != nil {
		body["args"] = args
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type batchResponse struct {
	Action    string `json:"action"`
	Requested int    `json:"requested"`
	Affected  int64  `json:"affected"`
}

func TestBatchContractsUntrack(t *testing.T) {
	ms := seedBatchStore(t)
	srv := newTestHandler(ms, true, true)
	ctx := context.Background()

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", "", contributorUser,
		batchBody(t, []string{batchIDOne}, "untrack", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp batchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Action != "untrack" || resp.Requested != 1 || resp.Affected != 1 {
		t.Fatalf("resp = %+v, want action=untrack requested=1 affected=1", resp)
	}

	if _, err := ms.GetContract(ctx, batchIDOne); err == nil {
		t.Error("selected contract should have been untracked")
	}
	if _, err := ms.GetContract(ctx, batchIDTwo); err != nil {
		t.Errorf("unselected contract should survive, got err=%v", err)
	}
}

func TestBatchContractsTagAppliesToAllSelected(t *testing.T) {
	ms := seedBatchStore(t)
	srv := newTestHandler(ms, true, true)
	ctx := context.Background()

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", "", contributorUser,
		batchBody(t, []string{batchIDOne, batchIDTwo}, "tag", map[string]any{"label": "payments"}))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var resp batchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Action != "tag" || resp.Requested != 2 || resp.Affected != 2 {
		t.Fatalf("resp = %+v, want action=tag requested=2 affected=2", resp)
	}

	for _, id := range []string{batchIDOne, batchIDTwo} {
		c, err := ms.GetContract(ctx, id)
		if err != nil {
			t.Fatalf("GetContract(%s): %v", id, err)
		}
		if c.Label != "payments" {
			t.Errorf("%s label = %q, want payments", id, c.Label)
		}
	}
}

func TestBatchContractsValidation(t *testing.T) {
	srv := newTestHandler(seedBatchStore(t), true, true)

	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = batchIDOne
	}

	cases := []struct {
		name string
		body string
	}{
		{"empty ids", batchBody(t, []string{}, "untrack", nil)},
		{"too many ids", batchBody(t, tooMany, "untrack", nil)},
		{"malformed id", batchBody(t, []string{"not-a-contract"}, "untrack", nil)},
		{"unknown action", batchBody(t, []string{batchIDOne}, "explode", nil)},
		{"tag without label", batchBody(t, []string{batchIDOne}, "tag", nil)},
		{"tag with blank label", batchBody(t, []string{batchIDOne}, "tag", map[string]any{"label": "   "})},
		{"malformed json", `{"ids":`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", "", contributorUser, tc.body)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestBatchContractsRequiresContributorRole(t *testing.T) {
	srv := newTestHandler(seedBatchStore(t), true, true)
	body := batchBody(t, []string{batchIDOne}, "tag", map[string]any{"label": "x"})

	if w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", "", viewerUser, body); w.Code != http.StatusForbidden {
		t.Errorf("viewer: want 403, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", "", "", body); w.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: want 401, got %d (%s)", w.Code, w.Body.String())
	}
	if w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", "", contributorUser, body); w.Code != http.StatusOK {
		t.Errorf("contributor: want 200, got %d (%s)", w.Code, w.Body.String())
	}
}

// A read-only API key must not reach a write route even for anonymous callers.
func TestBatchContractsDeniesReadOnlyScope(t *testing.T) {
	srv := newTestHandler(seedBatchStore(t), true, true)
	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/contracts/batch", readContractsKey, contributorUser,
		batchBody(t, []string{batchIDOne}, "tag", map[string]any{"label": "x"}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d (%s)", w.Code, w.Body.String())
	}
}
