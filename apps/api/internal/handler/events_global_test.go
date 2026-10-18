package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const (
	explorerContractA = "CAAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQCAIBAEAQDZ7H"
	explorerContractB = "CABQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGAYDAMBQGCK3"
)

type globalEventsBody struct {
	Events []struct {
		ID         string `json:"id"`
		ContractID string `json:"contract_id"`
		Type       string `json:"type"`
	} `json:"events"`
	NextCursor string `json:"next_cursor"`
}

// seedGlobalEvents inserts 8 events one hour apart with IDs in ledger order:
// 6 contract events for A and 2 system events for B (i = 2 and 5).
func seedGlobalEvents(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var events []store.Event
	for i := 0; i < 8; i++ {
		cid, typ := explorerContractA, "contract"
		if i%3 == 2 {
			cid, typ = explorerContractB, "system"
		}
		events = append(events, store.Event{
			ID:             fmt.Sprintf("%019d-0000000001", 1000+i),
			ContractID:     cid,
			Network:        "testnet",
			Ledger:         uint32(1000 + i),
			LedgerClosedAt: base.Add(time.Duration(i) * time.Hour),
			Type:           typ,
		})
	}
	if err := ms.BatchInsertEvents(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	return ms
}

func listGlobal(t *testing.T, srv http.Handler, query string) globalEventsBody {
	t.Helper()
	w := doRequest(srv, http.MethodGet, "/api/v1/events"+query, "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/events%s: %d %s", query, w.Code, w.Body.String())
	}
	var body globalEventsBody
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestListAllEventsPaginatesNewestFirst(t *testing.T) {
	srv := newTestHandler(seedGlobalEvents(t), true, true)

	var ids []string
	query := "?limit=3"
	for page := 0; page < 5; page++ {
		body := listGlobal(t, srv, query)
		for _, e := range body.Events {
			ids = append(ids, e.ID)
		}
		if body.NextCursor == "" {
			break
		}
		query = "?limit=3&cursor=" + url.QueryEscape(body.NextCursor)
	}
	if len(ids) != 8 {
		t.Fatalf("want 8 events across pages, got %d", len(ids))
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] >= ids[i-1] {
			t.Fatalf("not newest first: %v", ids)
		}
	}
}

func TestListAllEventsFilters(t *testing.T) {
	srv := newTestHandler(seedGlobalEvents(t), true, true)

	// Contract ID prefix, case-insensitive input.
	body := listGlobal(t, srv, "?contract_id=cabq")
	if len(body.Events) != 2 {
		t.Fatalf("contract prefix: want 2, got %d", len(body.Events))
	}
	for _, e := range body.Events {
		if e.ContractID != explorerContractB {
			t.Fatalf("contract filter leaked %s", e.ContractID)
		}
	}

	if body := listGlobal(t, srv, "?type=system"); len(body.Events) != 2 {
		t.Fatalf("type filter: want 2, got %d", len(body.Events))
	}

	// Inclusive date range: hours 2..4.
	body = listGlobal(t, srv, "?since=2026-09-01T02:00:00Z&until=2026-09-01T04:00:00Z")
	if len(body.Events) != 3 {
		t.Fatalf("date range: want 3, got %d", len(body.Events))
	}

	if body := listGlobal(t, srv, "?contract_id=CZZZ"); len(body.Events) != 0 || body.NextCursor != "" {
		t.Fatalf("no matches: %+v", body)
	}
}

func TestListAllEventsValidation(t *testing.T) {
	srv := newTestHandler(store.NewMockStore(), true, true)
	for _, q := range []string{
		"?type=bogus",
		"?since=yesterday",
		"?until=2026-13-01",
		"?since=2026-09-02T00:00:00Z&until=2026-09-01T00:00:00Z",
		"?network=moon",
		"?cursor=!!!",
	} {
		if w := doRequest(srv, http.MethodGet, "/api/v1/events"+q, "", ""); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: want 422, got %d", q, w.Code)
		}
	}
}

func TestListAllEventsEmpty(t *testing.T) {
	body := listGlobal(t, newTestHandler(store.NewMockStore(), true, true), "")
	if len(body.Events) != 0 || body.NextCursor != "" {
		t.Fatalf("empty store: %+v", body)
	}
}
