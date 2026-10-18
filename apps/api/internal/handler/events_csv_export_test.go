package handler_test

import (
	"encoding/csv"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

const csvContractID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// csvEventColumns is the header the export promises. It is spelled out here
// rather than derived from the store so a change to the file format has to be
// made deliberately in the test too.
var csvEventColumns = []string{
	"id", "contract_id", "network", "ledger", "ledger_closed_at", "tx_hash",
	"type", "topic_xdr", "value_xdr", "topic_decoded", "value_decoded", "in_successful_call",
}

// csvClosedAt is a fixed timestamp so exports of the same store are byte-equal
// and can be compared directly.
var csvClosedAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func seedCSVStore(t *testing.T) *store.MockStore {
	t.Helper()
	ms := store.NewMockStore()
	if err := ms.UpsertContract(nil, store.Contract{
		ID:      csvContractID,
		Network: "testnet",
		Label:   "csv contract",
		Status:  "active",
	}); err != nil {
		t.Fatal(err)
	}
	// Inserted out of ledger order on purpose: the export must sort by
	// (ledger, id) rather than echo insertion order.
	err := ms.BatchInsertEvents(nil, []store.Event{
		{
			ID: "evt-b", ContractID: csvContractID, Network: "testnet",
			Ledger: 200, LedgerClosedAt: csvClosedAt, TxHash: "tx-b", Type: "transfer",
			TopicXDR: []string{"AAAABBBB"}, ValueXDR: "CCCCDDDD",
			TopicDecoded: []any{"transfer"}, ValueDecoded: map[string]any{"amount": "100"},
			InSuccessfulCall: true,
		},
		{
			ID: "evt-a", ContractID: csvContractID, Network: "testnet",
			Ledger: 100, LedgerClosedAt: csvClosedAt, TxHash: "tx-a", Type: "contract",
			TopicXDR: []string{"ZZZZ", "YYYY"}, ValueXDR: "XXXX",
			TopicDecoded: []any{"mint", "burn"}, ValueDecoded: map[string]any{"amount": "-42"},
			InSuccessfulCall: false,
		},
		{
			ID: "evt-mainnet", ContractID: csvContractID, Network: "mainnet",
			Ledger: 150, LedgerClosedAt: csvClosedAt, TxHash: "tx-m", Type: "contract",
		},
		{
			ID: "evt-other", ContractID: netContractB, Network: "mainnet",
			Ledger: 100, LedgerClosedAt: csvClosedAt, TxHash: "tx-o", Type: "contract",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func getCSVRows(t *testing.T, srv http.Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func parseCSV(t *testing.T, body string) [][]string {
	t.Helper()
	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v\nbody:\n%s", err, body)
	}
	return rows
}

func TestExportEventsCSV(t *testing.T) {
	srv := newTestHandler(seedCSVStore(t), true, true)
	w := getCSVRows(t, srv, "/api/v1/contracts/"+csvContractID+"/events.csv")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/csv; charset=utf-8", got)
	}
	if got, want := w.Header().Get("Content-Disposition"),
		`attachment; filename="`+csvContractID+`-events.csv"`; got != want {
		t.Errorf("Content-Disposition = %q, want %q", got, want)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	rows := parseCSV(t, w.Body.String())
	if len(rows) != 4 {
		t.Fatalf("got %d rows (header + events), want 4: %v", len(rows), rows)
	}
	if got := strings.Join(rows[0], ","); got != strings.Join(csvEventColumns, ",") {
		t.Errorf("header = %q, want %q", got, strings.Join(csvEventColumns, ","))
	}

	// Sorted by ledger ascending, and only the requested contract.
	wantIDs := []string{"evt-a", "evt-mainnet", "evt-b"}
	for i, want := range wantIDs {
		if got := rows[i+1][0]; got != want {
			t.Errorf("row %d id = %q, want %q (rows: %v)", i, got, want, rows)
		}
	}

	// Spot-check the columns that are easy to get subtly wrong.
	if got := rows[1][3]; got != "100" {
		t.Errorf("ledger = %q, want 100", got)
	}
	if got, want := rows[1][4], csvClosedAt.Format(time.RFC3339); got != want {
		t.Errorf("ledger_closed_at = %q, want %q", got, want)
	}
	if got := rows[1][7]; got != `["ZZZZ","YYYY"]` {
		t.Errorf("topic_xdr = %q, want [\"ZZZZ\",\"YYYY\"]", got)
	}
	if got := rows[3][11]; got != "true" {
		t.Errorf("in_successful_call = %q, want true", got)
	}
	// A negative number must survive the spreadsheet-injection guard as a
	// number, not be turned into text.
	if got := rows[1][10]; !strings.Contains(got, `"-42"`) {
		t.Errorf("value_decoded = %q, want the amount to stay the JSON number -42", got)
	}
}

// TestExportEventsCSVDeterministic pins the promise that two exports of an
// unchanged store are byte-identical, which is what makes a saved export
// archivable and diffable.
func TestExportEventsCSVDeterministic(t *testing.T) {
	srv := newTestHandler(seedCSVStore(t), true, true)
	url := "/api/v1/contracts/" + csvContractID + "/events.csv"

	first := getCSVRows(t, srv, url).Body.String()
	second := getCSVRows(t, srv, url).Body.String()
	if first != second {
		t.Errorf("two exports of an unchanged store differ:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestExportEventsCSVFilters(t *testing.T) {
	srv := newTestHandler(seedCSVStore(t), true, true)
	base := "/api/v1/contracts/" + csvContractID + "/events.csv"

	cases := []struct {
		name   string
		query  string
		wantID []string
	}{
		{"network", "?network=testnet", []string{"evt-a", "evt-b"}},
		{"type", "?type=transfer", []string{"evt-b"}},
		{"ledger range", "?from=150&to=200", []string{"evt-mainnet", "evt-b"}},
		{"from only", "?from=150", []string{"evt-mainnet", "evt-b"}},
		{"to only", "?to=150", []string{"evt-a", "evt-mainnet"}},
		{"combined", "?network=testnet&type=contract&from=100&to=150", []string{"evt-a"}},
		{"no matches", "?type=nonexistent", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := getCSVRows(t, srv, base+tc.query)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}
			rows := parseCSV(t, w.Body.String())
			if len(rows) != len(tc.wantID)+1 {
				t.Fatalf("got %d rows, want %d (header + %v): %v", len(rows), len(tc.wantID)+1, tc.wantID, rows)
			}
			for i, want := range tc.wantID {
				if got := rows[i+1][0]; got != want {
					t.Errorf("row %d id = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestExportEventsCSVUnknownContractIsHeaderOnly(t *testing.T) {
	srv := newTestHandler(seedCSVStore(t), true, true)
	w := getCSVRows(t, srv, "/api/v1/contracts/CUNKNOWNCONTRACT/events.csv")

	// The JSON listing answers 200 with an empty page for an unknown contract,
	// and the export matches that: a header row, not a 404.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	rows := parseCSV(t, w.Body.String())
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want just the header: %v", len(rows), rows)
	}
	if got := strings.Join(rows[0], ","); got != strings.Join(csvEventColumns, ",") {
		t.Errorf("header = %q", got)
	}
}

func TestExportEventsCSVRejectsBadInput(t *testing.T) {
	srv := newTestHandler(seedCSVStore(t), true, true)
	base := "/api/v1/contracts/" + csvContractID + "/events.csv"

	cases := []struct {
		name  string
		query string
	}{
		{"unknown network", "?network=bogus"},
		{"from is zero", "?from=0"},
		{"from is not a number", "?from=abc"},
		{"from overflows uint32", "?from=99999999999999"},
		{"to is negative", "?to=-5"},
		{"inverted range", "?from=200&to=100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := getCSVRows(t, srv, base+tc.query)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422: %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("Content-Type = %q, want JSON error", got)
			}
		})
	}
}

// TestExportEventsCSVStoreErrorBeforeAnyBytes guards the case the original
// implementation got wrong: when the store fails before producing a row, the
// caller must get a 500 rather than a 200 that looks like a contract with no
// events.
func TestExportEventsCSVStoreErrorBeforeAnyBytes(t *testing.T) {
	ms := seedCSVStore(t)
	ms.ListEventsErr = errors.New("database is on fire")

	srv := newTestHandler(ms, true, true)
	w := getCSVRows(t, srv, "/api/v1/contracts/"+csvContractID+"/events.csv")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want JSON error", got)
	}
	// A JSON error body is the only thing a client can act on; a stray CSV
	// header row in front of it would corrupt the parse.
	if strings.HasPrefix(w.Body.String(), "id,") {
		t.Errorf("body starts with the CSV header, want a JSON error: %s", w.Body.String())
	}
	if got := w.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("Content-Disposition = %q, want it dropped on the error path", got)
	}
}

// TestExportEventsCSVFormulaInjection checks that a hostile contract cannot
// make a spreadsheet execute code when an operator opens the export.
func TestExportEventsCSVFormulaInjection(t *testing.T) {
	ms := store.NewMockStore()
	err := ms.BatchInsertEvents(nil, []store.Event{{
		ID: "evt-evil", ContractID: csvContractID, Network: "testnet",
		Ledger: 7, LedgerClosedAt: csvClosedAt, TxHash: "tx",
		Type:         `=cmd|'/c calc'!A1`,
		ValueXDR:     `@SUM(1+1)*cmd|' /C calc'!A0`,
		TopicDecoded: []any{"@SUM(A1:A9)"},
		ValueDecoded: map[string]any{"amount": -42},
		TopicXDR:     []string{"+SUM(A1)"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	srv := newTestHandler(ms, true, true)
	w := getCSVRows(t, srv, "/api/v1/contracts/"+csvContractID+"/events.csv")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	rows := parseCSV(t, w.Body.String())
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want header + 1: %v", len(rows), rows)
	}
	row := rows[1]
	// The two free-text columns a contract can choose are neutralized.
	if want := `'=cmd|'/c calc'!A1`; row[6] != want {
		t.Errorf("type = %q, want %q (a leading = must be neutralized)", row[6], want)
	}
	if want := `'@SUM(1+1)*cmd|' /C calc'!A0`; row[8] != want {
		t.Errorf("value_xdr = %q, want %q (a leading @ must be neutralized)", row[8], want)
	}
	// The JSON columns always open with a bracket, so a formula lead inside
	// them is inert and must be left alone — and a negative number must never
	// be turned into text.
	if want := `["@SUM(A1:A9)"]`; row[9] != want {
		t.Errorf("topic_decoded = %q, want %q (JSON cells are not formulas)", row[9], want)
	}
	if want := `["+SUM(A1)"]`; row[7] != want {
		t.Errorf("topic_xdr = %q, want %q", row[7], want)
	}
	if want := `{"amount":-42}`; row[10] != want {
		t.Errorf("value_decoded = %q, want %q (the amount stays the JSON number -42)", row[10], want)
	}
}

// TestExportEventsCSVRequiresReadContractsScope guards the route's entry in the
// scope table. A route missing from that table is not merely unscoped by
// accident: the middleware treats it as "no rule" and lets any valid API key
// through, so a watchdog-only key would have been able to download contract
// events the spec documents as read:contracts.
func TestExportEventsCSVRequiresReadContractsScope(t *testing.T) {
	srv := newTestHandler(seedScopedKeyStore(t), true, true)
	url := "/api/v1/contracts/" + csvContractID + "/events.csv"

	cases := []struct {
		name  string
		token string
		want  int
	}{
		{"read:contracts allowed", readContractsKey, http.StatusOK},
		{"read:watchdog denied", readWatchdogKey, http.StatusForbidden},
		{"admin allowed", adminToken, http.StatusOK},
		// The public v0.1 read surface stays open: no credential is a pass,
		// matching GET /api/v1/contracts/{id}/events.
		{"anonymous allowed", "", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doRequestAsUser(srv, http.MethodGet, url, tc.token, "", "")
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// TestExportEventsCSVFilenameSanitized covers the Content-Disposition header
// being attacker-controlled: a quote in the id must not terminate the filename.
func TestExportEventsCSVFilenameSanitized(t *testing.T) {
	hostile := `evil".html;`
	ms := store.NewMockStore()
	if err := ms.BatchInsertEvents(nil, []store.Event{{
		ID: "evt-x", ContractID: hostile, Network: "testnet",
		Ledger: 1, LedgerClosedAt: csvClosedAt, Type: "contract",
	}}); err != nil {
		t.Fatal(err)
	}

	srv := newTestHandler(ms, true, true)
	w := getCSVRows(t, srv, "/api/v1/contracts/"+hostile+"/events.csv")

	got := w.Header().Get("Content-Disposition")
	if strings.Count(got, `"`) != 2 {
		t.Errorf("Content-Disposition = %q, want exactly one quoted filename", got)
	}
	if !strings.HasSuffix(got, `-events.csv"`) {
		t.Errorf("Content-Disposition = %q, want a -events.csv filename", got)
	}
	if strings.ContainsAny(got, ";/\r\n") && !strings.HasPrefix(got, "attachment; ") {
		t.Errorf("Content-Disposition = %q, want the separator escaped out of the filename", got)
	}
}
