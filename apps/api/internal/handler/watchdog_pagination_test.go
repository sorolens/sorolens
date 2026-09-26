package handler_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// monitoredPage is the GET /api/v1/watchdog/contracts response envelope.
type monitoredPage struct {
	Contracts []struct {
		ContractID string `json:"contract_id"`
	} `json:"contracts"`
	NextCursor string `json:"next_cursor"`
}

func (p monitoredPage) ids() []string {
	out := make([]string, len(p.Contracts))
	for i, c := range p.Contracts {
		out[i] = c.ContractID
	}
	return out
}

// seedManyMonitored registers n contracts that differ only in their ID, so page
// boundaries can only come from the contract_id keyset. IDs are zero-padded
// so lexicographic order matches creation order.
func seedManyMonitored(t *testing.T, ms *store.MockStore, n int, network string) []string {
	t.Helper()
	registered := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ids := make([]string, n)
	for i := range n {
		ids[i] = fmt.Sprintf("C%s%03d", network, i)
		if err := ms.UpsertMonitoredContract(nil, store.MonitoredContract{
			ContractID:    ids[i],
			Network:       network,
			Name:          "same-name",
			Owner:         "GOWNER",
			Status:        "Healthy",
			CheckInterval: 60,
			RegisteredAt:  registered,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

func getMonitoredPage(t *testing.T, srv http.Handler, query url.Values) monitoredPage {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/watchdog/contracts?"+query.Encode(), nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	var body monitoredPage
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestListMonitoredContractsPageBoundaries(t *testing.T) {
	cursorFor := func(id string) string { return base64.StdEncoding.EncodeToString([]byte(id)) }

	tests := []struct {
		name     string
		seed     int
		query    url.Values
		wantLen  int
		wantNext bool
	}{
		{"empty dataset", 0, url.Values{"limit": {"5"}}, 0, false},
		{"fewer items than limit", 3, url.Values{"limit": {"5"}}, 3, false},
		{"exactly limit items", 5, url.Values{"limit": {"5"}}, 5, false},
		{"limit plus one items", 6, url.Values{"limit": {"5"}}, 5, true},
		{"default limit is 50", 60, url.Values{}, 50, true},
		{"limit at max of 200", 250, url.Values{"limit": {"200"}}, 200, true},
		{"limit above max falls back to default", 250, url.Values{"limit": {"201"}}, 50, true},
		{"limit zero falls back to default", 60, url.Values{"limit": {"0"}}, 50, true},
		{"negative limit falls back to default", 60, url.Values{"limit": {"-1"}}, 50, true},
		{"non-numeric limit falls back to default", 60, url.Values{"limit": {"abc"}}, 50, true},
		{"cursor past the last item", 3, url.Values{"cursor": {cursorFor("Ctestnet999")}}, 0, false},
		{"cursor on the last item", 3, url.Values{"cursor": {cursorFor("Ctestnet002")}}, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := store.NewMockStore()
			seedManyMonitored(t, ms, tc.seed, "testnet")
			page := getMonitoredPage(t, newTestHandler(ms, true, true), tc.query)

			if len(page.Contracts) != tc.wantLen {
				t.Fatalf("want %d contracts, got %d", tc.wantLen, len(page.Contracts))
			}
			if got := page.NextCursor != ""; got != tc.wantNext {
				t.Fatalf("want next_cursor present=%v, got %q", tc.wantNext, page.NextCursor)
			}
			if tc.wantNext {
				// The cursor is the base64 of the last returned contract_id,
				// the same opaque format /contracts uses.
				last := page.Contracts[len(page.Contracts)-1].ContractID
				if page.NextCursor != cursorFor(last) {
					t.Errorf("next_cursor %q does not encode last id %q", page.NextCursor, last)
				}
			}
		})
	}
}

func TestListMonitoredContractsSecondPageAfterLimitPlusOne(t *testing.T) {
	ms := store.NewMockStore()
	ids := seedManyMonitored(t, ms, 6, "testnet")
	srv := newTestHandler(ms, true, true)

	first := getMonitoredPage(t, srv, url.Values{"limit": {"5"}})
	second := getMonitoredPage(t, srv, url.Values{"limit": {"5"}, "cursor": {first.NextCursor}})

	if len(second.Contracts) != 1 || second.Contracts[0].ContractID != ids[5] {
		t.Fatalf("want second page [%s], got %v", ids[5], second.ids())
	}
	if second.NextCursor != "" {
		t.Fatalf("want no next_cursor on the last page, got %q", second.NextCursor)
	}
}

func TestListMonitoredContractsFullTraversal(t *testing.T) {
	tests := []struct {
		name      string
		seed      int
		limit     int
		wantPages int
	}{
		{"uneven last page", 23, 5, 5},
		{"even last page", 20, 5, 4},
		{"single item pages", 4, 1, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ms := store.NewMockStore()
			want := seedManyMonitored(t, ms, tc.seed, "testnet")
			// Contracts on another network must never leak into the pages.
			seedManyMonitored(t, ms, 7, "mainnet")
			srv := newTestHandler(ms, true, true)

			var got []string
			seen := map[string]bool{}
			cursor := ""
			pages := 0
			for {
				q := url.Values{"limit": {fmt.Sprint(tc.limit)}, "network": {"testnet"}}
				if cursor != "" {
					q.Set("cursor", cursor)
				}
				page := getMonitoredPage(t, srv, q)
				pages++
				if pages > tc.wantPages {
					t.Fatalf("more than %d pages; last cursor %q", tc.wantPages, cursor)
				}
				for _, id := range page.ids() {
					if seen[id] {
						t.Fatalf("contract %s returned twice", id)
					}
					seen[id] = true
					got = append(got, id)
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}

			if pages != tc.wantPages {
				t.Errorf("want %d pages, got %d", tc.wantPages, pages)
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("traversal mismatch:\nwant %v\ngot  %v", want, got)
			}
		})
	}
}

func TestListMonitoredContractsInvalidCursor(t *testing.T) {
	type errBody struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	get := func(t *testing.T, path, cursor string) (int, errBody) {
		t.Helper()
		ms := store.NewMockStore()
		seedManyMonitored(t, ms, 3, "testnet")
		req := httptest.NewRequest(http.MethodGet, path+"?cursor="+url.QueryEscape(cursor), nil)
		w := httptest.NewRecorder()
		newTestHandler(ms, true, true).ServeHTTP(w, req)
		var body errBody
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return w.Code, body
	}

	for _, cursor := range []string{"not-base64!", "abc", "%%%%"} {
		t.Run(cursor, func(t *testing.T) {
			code, body := get(t, "/api/v1/watchdog/contracts", cursor)
			if code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d", code)
			}
			if body.Error.Code != "INVALID_INPUT" || body.Error.Message != "invalid cursor" {
				t.Fatalf("unexpected error body: %+v", body.Error)
			}

			// Must match what /contracts returns for the same cursor.
			refCode, refBody := get(t, "/api/v1/contracts", cursor)
			if code != refCode || body != refBody {
				t.Errorf("watchdog (%d %+v) differs from /contracts (%d %+v)", code, body, refCode, refBody)
			}
		})
	}
}
