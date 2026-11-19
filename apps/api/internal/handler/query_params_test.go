package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/handler"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// decodeErrorCode returns the error.code of a 422 response, failing the test
// when the body is not the standard error envelope.
func decodeErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v (body %q)", err, w.Body.String())
	}
	if env.Error.Code == "" {
		t.Fatalf("error envelope has no code: %s", w.Body.String())
	}
	return env.Error.Code
}

// TestQueryParameterValidation pins the 422 code each class of bad query
// parameter returns (issue #776): the same class must report the same code on
// every list endpoint instead of one endpoint treating it as a default and
// another as an error.
func TestQueryParameterValidation(t *testing.T) {
	srv := newTestHandler(seedInvocationStore(t), true, true)

	cases := []struct {
		name string
		path string
		want string
	}{
		{"contracts invalid cursor", "/api/v1/contracts?cursor=not-base64!!", handler.CodeInvalidCursor},
		{"contract events invalid cursor", "/api/v1/contracts/CAAA/events?cursor=not-base64!!", handler.CodeInvalidCursor},
		{"contract invocations invalid cursor", "/api/v1/contracts/CAAA/invocations?cursor=not-base64!!", handler.CodeInvalidCursor},
		{"contract storage invalid cursor", "/api/v1/contracts/CAAA/storage?cursor=not-base64!!", handler.CodeInvalidCursor},
		{"global invocations invalid cursor", "/api/v1/invocations?cursor=not-base64!!", handler.CodeInvalidCursor},
		{"global invocations well-formed but unusable cursor", "/api/v1/invocations?cursor=YWJj", handler.CodeInvalidCursor},
		{"global events invalid cursor", "/api/v1/events?cursor=not-base64!!", handler.CodeInvalidCursor},

		{"contracts invalid network", "/api/v1/contracts?network=mainet", handler.CodeInvalidNetwork},
		{"contract events invalid network", "/api/v1/contracts/CAAA/events?network=mainet", handler.CodeInvalidNetwork},
		{"contract invocations invalid network", "/api/v1/contracts/CAAA/invocations?network=mainet", handler.CodeInvalidNetwork},
		{"contract storage invalid network", "/api/v1/contracts/CAAA/storage?network=mainet", handler.CodeInvalidNetwork},
		{"global invocations invalid network", "/api/v1/invocations?network=mainet", handler.CodeInvalidNetwork},
		{"global events invalid network", "/api/v1/events?network=mainet", handler.CodeInvalidNetwork},

		{"contracts invalid limit", "/api/v1/contracts?limit=abc", handler.CodeInvalidLimit},
		{"contracts zero limit", "/api/v1/contracts?limit=0", handler.CodeInvalidLimit},
		{"contract events invalid limit", "/api/v1/contracts/CAAA/events?limit=-1", handler.CodeInvalidLimit},
		{"contract invocations invalid limit", "/api/v1/contracts/CAAA/invocations?limit=1.5", handler.CodeInvalidLimit},
		{"contract storage invalid limit", "/api/v1/contracts/CAAA/storage?limit=abc", handler.CodeInvalidLimit},
		{"global invocations invalid limit", "/api/v1/invocations?limit=abc", handler.CodeInvalidLimit},
		{"global events invalid limit", "/api/v1/events?limit=abc", handler.CodeInvalidLimit},
		{"search invalid limit", "/api/v1/search?q=a&limit=abc", handler.CodeInvalidLimit},

		{"contracts invalid sort", "/api/v1/contracts?sort=bogus", handler.CodeInvalidEnum},
		{"contracts invalid dir", "/api/v1/contracts?dir=sideways", handler.CodeInvalidEnum},
		{"contract events invalid flag", "/api/v1/contracts/CAAA/events?in_successful_call=maybe", handler.CodeInvalidEnum},
		{"global events invalid type", "/api/v1/events?type=bogus", handler.CodeInvalidEnum},

		{"global invocations invalid since", "/api/v1/invocations?since=yesterday", handler.CodeInvalidDate},
		{"global invocations invalid until", "/api/v1/invocations?until=2026-13-01", handler.CodeInvalidDate},
		{"global events invalid since", "/api/v1/events?since=not-a-date", handler.CodeInvalidDate},
		{"global events inverted range", "/api/v1/events?since=2026-09-02T00:00:00Z&until=2026-09-01T00:00:00Z", handler.CodeInvalidDate},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := doRequest(srv, http.MethodGet, tc.path, "", "")
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("GET %s: status %d, want 422 (%s)", tc.path, w.Code, w.Body.String())
			}
			if got := decodeErrorCode(t, w); got != tc.want {
				t.Errorf("GET %s: code %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// TestPageSizeClamped proves no list endpoint returns more rows than the
// documented maximum, however large a ?limit= it is handed.
func TestPageSizeClamped(t *testing.T) {
	ms := store.NewMockStore()
	const seeded = handler.MaxPageSize + 25
	for i := 0; i < seeded; i++ {
		id := fmt.Sprintf("C%055d", i)
		if err := ms.UpsertContract(context.Background(), store.Contract{
			ID: id, Network: "testnet", Status: "active",
		}); err != nil {
			t.Fatalf("seed contract %d: %v", i, err)
		}
	}
	srv := newTestHandler(ms, true, true)

	w := doRequest(srv, http.MethodGet, "/api/v1/contracts?limit=10000000", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Contracts []map[string]any `json:"contracts"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Contracts) != handler.MaxPageSize {
		t.Fatalf("returned %d contracts, want the %d-row page maximum", len(body.Contracts), handler.MaxPageSize)
	}
}

// TestGlobalStatsApproximateFlag checks the response surfaces the store's
// approximate flag so a caller can label estimated totals (issue #776).
func TestGlobalStatsApproximateFlag(t *testing.T) {
	for _, approx := range []bool{true, false} {
		ms := store.NewMockStore()
		ms.SetGlobalStats(store.GlobalStats{
			TrackedContracts: 2, TotalEvents: 15000, TotalInvocations: 900, TotalStorageEntries: 40,
			Approximate: approx,
		})
		srv := newTestHandler(ms, true, true)

		w := doRequest(srv, http.MethodGet, "/api/v1/stats/global", "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		got, ok := resp["approximate"].(bool)
		if !ok {
			t.Fatalf("response has no boolean approximate field: %v", resp)
		}
		if got != approx {
			t.Errorf("approximate = %v, want %v", got, approx)
		}
	}
}
