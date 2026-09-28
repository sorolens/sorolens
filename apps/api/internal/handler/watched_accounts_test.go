package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	watchedAccountA = "GAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQDZ7H"
	watchedAccountB = "GABAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEJXA"
)

type watchedAccountBody struct {
	AccountID       string `json:"account_id"`
	AddedBy         string `json:"added_by"`
	DiscoveredCount int64  `json:"discovered_count"`
}

func TestAddWatchedAccount(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)

	w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watched-accounts", "", contributorUser,
		`{"account_id":"`+watchedAccountA+`"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d (%s)", w.Code, w.Body.String())
	}
	var got watchedAccountBody
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.AccountID != watchedAccountA || got.AddedBy != contributorUser || got.DiscoveredCount != 0 {
		t.Fatalf("unexpected body: %+v", got)
	}

	// Re-adding is idempotent.
	w = doRequestAsUser(srv, http.MethodPost, "/api/v1/watched-accounts", "", adminUser,
		`{"account_id":"`+watchedAccountA+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("re-add: want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.AddedBy != contributorUser {
		t.Fatalf("re-add must keep original added_by, got %q", got.AddedBy)
	}
}

func TestAddWatchedAccountValidation(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)

	cases := []struct {
		name string
		body string
	}{
		{"invalid json", `{`},
		{"missing", `{}`},
		{"contract id", `{"account_id":"CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3"}`},
		{"bad checksum", `{"account_id":"GAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQDZ7A"}`},
		{"too short", `{"account_id":"GAAQCAIB"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watched-accounts", "", contributorUser, tc.body)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

func TestWatchedAccountsRoles(t *testing.T) {
	srv := newTestHandler(seedRBACUsers(t), true, true)
	body := `{"account_id":"` + watchedAccountA + `"}`

	if w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watched-accounts", "", viewerUser, body); w.Code != http.StatusForbidden {
		t.Fatalf("viewer POST: want 403, got %d", w.Code)
	}
	if w := doRequestAsUser(srv, http.MethodPost, "/api/v1/watched-accounts", "", "", body); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous POST: want 401, got %d", w.Code)
	}
	if w := doRequestAsUser(srv, http.MethodDelete, "/api/v1/watched-accounts/"+watchedAccountA, "", viewerUser, ""); w.Code != http.StatusForbidden {
		t.Fatalf("viewer DELETE: want 403, got %d", w.Code)
	}
	if w := doRequestAsUser(srv, http.MethodGet, "/api/v1/watched-accounts", "", "", ""); w.Code != http.StatusOK {
		t.Fatalf("anonymous GET: want 200, got %d", w.Code)
	}
}

func TestListAndDeleteWatchedAccounts(t *testing.T) {
	ms := seedRBACUsers(t)
	srv := newTestHandler(ms, true, true)
	for _, id := range []string{watchedAccountA, watchedAccountB} {
		if _, _, err := ms.AddWatchedAccount(context.Background(), store.WatchedAccount{AccountID: id}); err != nil {
			t.Fatal(err)
		}
	}
	// A contract discovered from account A shows up in its count.
	if _, err := ms.RecordDiscoveredContract(context.Background(), store.DiscoveredContract{
		ContractID: "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3",
		Network:    "testnet",
		AccountID:  watchedAccountA,
	}); err != nil {
		t.Fatal(err)
	}

	w := doRequestAsUser(srv, http.MethodGet, "/api/v1/watched-accounts", "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	var list struct {
		WatchedAccounts []watchedAccountBody `json:"watched_accounts"`
	}
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.WatchedAccounts) != 2 {
		t.Fatalf("want 2 accounts, got %d", len(list.WatchedAccounts))
	}
	counts := map[string]int64{}
	for _, a := range list.WatchedAccounts {
		counts[a.AccountID] = a.DiscoveredCount
	}
	if counts[watchedAccountA] != 1 || counts[watchedAccountB] != 0 {
		t.Fatalf("unexpected discovered counts: %v", counts)
	}

	w = doRequestAsUser(srv, http.MethodDelete, "/api/v1/watched-accounts/"+watchedAccountA, "", contributorUser, "")
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d (%s)", w.Code, w.Body.String())
	}
	w = doRequestAsUser(srv, http.MethodDelete, "/api/v1/watched-accounts/"+watchedAccountA, "", contributorUser, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("second delete: want 404, got %d", w.Code)
	}

	// The discovered contract stays tracked after the account is removed.
	w = doRequestAsUser(srv, http.MethodGet, "/api/v1/contracts", "", "", "")
	var contracts struct {
		Contracts []struct {
			ID     string `json:"id"`
			Label  string `json:"label"`
			Status string `json:"status"`
		} `json:"contracts"`
	}
	if err := json.NewDecoder(w.Body).Decode(&contracts); err != nil {
		t.Fatal(err)
	}
	if len(contracts.Contracts) != 1 {
		t.Fatalf("want 1 contract, got %d", len(contracts.Contracts))
	}
	if got := contracts.Contracts[0].Label; got != "discovered_by:"+watchedAccountA {
		t.Fatalf("want discovered_by label, got %q", got)
	}
}
