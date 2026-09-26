package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// 56-character contract IDs (C followed by 55 repeated characters).
var (
	sortContractA = "C" + strings.Repeat("A", 55)
	sortContractB = "C" + strings.Repeat("B", 55)
	sortContractC = "C" + strings.Repeat("C", 55)
)

// seedSortStore builds three contracts that differ on every sortable column:
//
//	contract  added_at   events  last activity
//	A         Jan 1      1       Mar 1
//	B         Jan 2      3       Apr 1
//	C         Jan 3      0       none (epoch)
func seedSortStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, c := range []store.Contract{
		{ID: sortContractA, Network: "testnet", Label: "A", Status: "active", AddedAt: base},
		{ID: sortContractB, Network: "testnet", Label: "B", Status: "active", AddedAt: base.Add(24 * time.Hour)},
		{ID: sortContractC, Network: "testnet", Label: "C", Status: "active", AddedAt: base.Add(48 * time.Hour)},
	} {
		if err := ms.UpsertContract(nil, c); err != nil {
			t.Fatalf("upsert contract %d: %v", i, err)
		}
	}

	if err := ms.BatchInsertEvents(nil, []store.Event{
		{ID: "ev-a1", ContractID: sortContractA, Network: "testnet", Ledger: 10, LedgerClosedAt: base.AddDate(0, 2, 0), TxHash: "txa1", Type: "contract"},
		{ID: "ev-b1", ContractID: sortContractB, Network: "testnet", Ledger: 20, LedgerClosedAt: base.AddDate(0, 0, 10), TxHash: "txb1", Type: "contract"},
		{ID: "ev-b2", ContractID: sortContractB, Network: "testnet", Ledger: 21, LedgerClosedAt: base.AddDate(0, 0, 11), TxHash: "txb2", Type: "contract"},
		{ID: "ev-b3", ContractID: sortContractB, Network: "testnet", Ledger: 22, LedgerClosedAt: base.AddDate(0, 1, 0), TxHash: "txb3", Type: "contract"},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	// B's most recent activity is an invocation, later than any of its events.
	if err := ms.BatchInsertInvocations(nil, []store.Invocation{
		{TxHash: "inv-b1", ContractID: sortContractB, Network: "testnet", Ledger: 30, LedgerClosedAt: base.AddDate(0, 3, 0), Status: "SUCCESS"},
	}); err != nil {
		t.Fatalf("insert invocations: %v", err)
	}
	return ms
}

// listContractIDs issues a contracts list request and returns the ordered IDs.
func listContractIDs(t *testing.T, srv http.Handler, query string) (int, []string, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/contracts"+query, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var ids []string
	var next string
	if w.Code == http.StatusOK {
		contracts := decodeContracts(t, w.Body.Bytes())
		ids = make([]string, len(contracts))
		for i, c := range contracts {
			ids[i], _ = c["id"].(string)
		}
		var parsed struct {
			NextCursor string `json:"next_cursor"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err == nil {
			next = parsed.NextCursor
		}
	}
	return w.Code, ids, next
}

func TestListContractsSortCombinations(t *testing.T) {
	cases := []struct {
		sort, order string
		want        []string
	}{
		{"added_at", "asc", []string{sortContractA, sortContractB, sortContractC}},
		{"added_at", "desc", []string{sortContractC, sortContractB, sortContractA}},
		{"last_activity", "asc", []string{sortContractC, sortContractA, sortContractB}},
		{"last_activity", "desc", []string{sortContractB, sortContractA, sortContractC}},
		{"events_count", "asc", []string{sortContractC, sortContractA, sortContractB}},
		{"events_count", "desc", []string{sortContractB, sortContractA, sortContractC}},
	}
	for _, tc := range cases {
		t.Run(tc.sort+"_"+tc.order, func(t *testing.T) {
			srv := newTestHandler(seedSortStore(t), true, true)
			code, got, _ := listContractIDs(t, srv, "?sort="+tc.sort+"&order="+tc.order)
			if code != http.StatusOK {
				t.Fatalf("want 200, got %d", code)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("sort=%s order=%s: want %v, got %v", tc.sort, tc.order, tc.want, got)
			}
		})
	}
}

func TestListContractsDefaultSort(t *testing.T) {
	srv := newTestHandler(seedSortStore(t), true, true)
	code, got, _ := listContractIDs(t, srv, "")
	if code != http.StatusOK {
		t.Fatalf("want 200, got %d", code)
	}
	// Default is newest first: added_at descending.
	want := []string{sortContractC, sortContractB, sortContractA}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default order: want %v, got %v", want, got)
	}
}

func TestListContractsRejectsInvalidSort(t *testing.T) {
	srv := newTestHandler(seedSortStore(t), true, true)
	for _, q := range []string{"sort=label", "sort=id", "sort=events", "sort=added_at%20desc"} {
		code, _, _ := listContractIDs(t, srv, "?"+q)
		if code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", q, code)
		}
	}
}

func TestListContractsRejectsInvalidOrder(t *testing.T) {
	srv := newTestHandler(seedSortStore(t), true, true)
	for _, q := range []string{"order=sideways", "order=ASCENDING", "order=up"} {
		code, _, _ := listContractIDs(t, srv, "?"+q)
		if code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", q, code)
		}
	}
}

// TestListContractsSortPagination walks a descending events_count list one row
// at a time to prove the opaque cursor continues the same ordering.
func TestListContractsSortPagination(t *testing.T) {
	srv := newTestHandler(seedSortStore(t), true, true)

	wantPages := [][]string{
		{sortContractB},
		{sortContractA},
		{sortContractC},
	}

	cursor := ""
	for i, want := range wantPages {
		query := "?sort=events_count&order=desc&limit=1"
		if cursor != "" {
			query += "&cursor=" + url.QueryEscape(cursor)
		}
		code, got, next := listContractIDs(t, srv, query)
		if code != http.StatusOK {
			t.Fatalf("page %d: want 200, got %d", i, code)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("page %d: want %v, got %v", i, want, got)
		}
		if i < len(wantPages)-1 && next == "" {
			t.Fatalf("page %d: expected a next cursor", i)
		}
		if i == len(wantPages)-1 && next != "" {
			t.Fatalf("last page: expected no next cursor, got %q", next)
		}
		cursor = next
	}
}
