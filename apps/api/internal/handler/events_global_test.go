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

func TestListAllEventsTimeRange(t *testing.T) {
	srv := newTestHandler(seedGlobalEvents(t), true, true)

	t.Run("boundary values from and to inclusive", func(t *testing.T) {
		body := listGlobal(t, srv, "?from=2026-09-01T02:00:00Z&to=2026-09-01T04:00:00Z")
		if len(body.Events) != 3 {
			t.Fatalf("want 3 events, got %d", len(body.Events))
		}
	})

	t.Run("boundary value from equal to to", func(t *testing.T) {
		body := listGlobal(t, srv, "?from=2026-09-01T03:00:00Z&to=2026-09-01T03:00:00Z")
		if len(body.Events) != 1 {
			t.Fatalf("want 1 event, got %d", len(body.Events))
		}
	})

	t.Run("offset timestamps normalized to UTC", func(t *testing.T) {
		// +02:00 -> 04:00 is 02:00 UTC
		// -03:00 -> 01:00 is 04:00 UTC
		body := listGlobal(t, srv, "?from=2026-09-01T04:00:00%2B02:00&to=2026-09-01T01:00:00-03:00")
		if len(body.Events) != 3 {
			t.Fatalf("want 3 events with offset timestamps, got %d", len(body.Events))
		}
	})

	t.Run("old aliases still work", func(t *testing.T) {
		body := listGlobal(t, srv, "?since=2026-09-01T02:00:00Z&until=2026-09-01T04:00:00Z")
		if len(body.Events) != 3 {
			t.Fatalf("want 3 events with since/until, got %d", len(body.Events))
		}
	})

	t.Run("primary parameters win over old aliases", func(t *testing.T) {
		// from=02:00 wins over since=00:00; to=03:00 wins over until=06:00 -> hours 2 and 3 = 2 events
		body := listGlobal(t, srv, "?from=2026-09-01T02:00:00Z&since=2026-09-01T00:00:00Z&to=2026-09-01T03:00:00Z&until=2026-09-01T06:00:00Z")
		if len(body.Events) != 2 {
			t.Fatalf("want 2 events (primary wins), got %d", len(body.Events))
		}
	})

	t.Run("invalid from returns 422 standard envelope", func(t *testing.T) {
		w := doRequest(srv, http.MethodGet, "/api/v1/events?from=invalid", "", "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", w.Code)
		}
		var errResp struct {
			Error struct {
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("decode error envelope: %v", err)
		}
		if errResp.Error.Code != "INVALID_INPUT" {
			t.Errorf("want code INVALID_INPUT, got %q", errResp.Error.Code)
		}
		if errResp.Error.Message != "from must be an RFC 3339 timestamp" {
			t.Errorf("unexpected message: %q", errResp.Error.Message)
		}
	})

	t.Run("inverted range returns 422 standard envelope", func(t *testing.T) {
		w := doRequest(srv, http.MethodGet, "/api/v1/events?from=2026-09-01T04:00:00Z&to=2026-09-01T02:00:00Z", "", "")
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d", w.Code)
		}
		var errResp struct {
			Error struct {
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("decode error envelope: %v", err)
		}
		if errResp.Error.Code != "INVALID_INPUT" {
			t.Errorf("want code INVALID_INPUT, got %q", errResp.Error.Code)
		}
		if errResp.Error.Message != "to must not be before from" {
			t.Errorf("unexpected message: %q", errResp.Error.Message)
		}
	})
}
