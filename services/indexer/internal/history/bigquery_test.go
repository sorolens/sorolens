package history

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type staticToken string

func (s staticToken) Token(context.Context) (string, error) { return string(s), nil }

var hubbleSchema = []map[string]string{
	{"name": "ledger_sequence"}, {"name": "closed_at"}, {"name": "transaction_hash"},
	{"name": "transaction_id"}, {"name": "event_index"}, {"name": "contract_id"},
	{"name": "contract_event_xdr"}, {"name": "in_successful_contract_call"},
}

func bqRow(ledger, txID, index string) map[string]any {
	cells := []string{ledger, "1709251200.5", "hash" + ledger, txID, index, contractID, "AAAA", "true"}
	f := make([]map[string]any, len(cells))
	for i, c := range cells {
		f[i] = map[string]any{"v": c}
	}
	return map[string]any{"f": f}
}

// fakeBigQuery answers jobs.query with an unfinished job, then serves its
// results in two pages.
func fakeBigQuery(t *testing.T) (*httptest.Server, *[]string, *bqQueryRequest) {
	t.Helper()
	var mu sync.Mutex
	var calls []string
	var query bqQueryRequest
	ref := map[string]string{"projectId": "billing", "jobId": "job-1", "location": "US"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		n := len(calls)
		mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"error":{"message":"unauthenticated"}}`, http.StatusUnauthorized)
			return
		}
		var body map[string]any
		switch {
		case r.Method == http.MethodPost:
			if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
				t.Errorf("decode query: %v", err)
			}
			body = map[string]any{"jobComplete": false, "jobReference": ref}
		case n == 2:
			body = map[string]any{
				"jobComplete": true, "jobReference": ref, "pageToken": "p2",
				"schema": map[string]any{"fields": hubbleSchema},
				"rows":   []any{bqRow("100", "7", "0"), bqRow("100", "7", "1")},
			}
		default:
			body = map[string]any{
				"jobComplete": true,
				"schema":      map[string]any{"fields": hubbleSchema},
				"rows":        []any{bqRow("101", "8", "0")},
			}
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &query
}

func TestBigQueryReaderPollsAndPages(t *testing.T) {
	srv, calls, query := fakeBigQuery(t)
	r := &BigQueryReader{Endpoint: srv.URL, BillingProject: "billing", Tokens: staticToken("tok"),
		MaxBytesBilled: 1 << 30, PollInterval: time.Millisecond}

	at := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	rows, err := r.Rows(context.Background(), "SELECT 1", map[string]any{
		"contract_id": contractID, "from_ledger": int64(100), "from_time": at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[2].LedgerSequence != 101 || rows[1].EventIndex != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	if !rows[0].InSuccessfulContractCall || rows[0].ContractID != contractID || rows[0].TransactionHash != "hash100" {
		t.Fatalf("row = %+v", rows[0])
	}
	if want := time.Unix(1709251200, 500_000_000).UTC(); !rows[0].ClosedAt.Equal(want) {
		t.Fatalf("closed at %v, want %v", rows[0].ClosedAt, want)
	}

	if len(*calls) != 3 || !strings.HasPrefix((*calls)[0], "POST /projects/billing/queries") {
		t.Fatalf("calls = %v", *calls)
	}
	if !strings.Contains((*calls)[1], "/queries/job-1") || !strings.Contains((*calls)[1], "location=US") {
		t.Fatalf("poll = %s", (*calls)[1])
	}
	if !strings.Contains((*calls)[2], "pageToken=p2") {
		t.Fatalf("second page = %s", (*calls)[2])
	}

	if query.UseLegacySQL || query.ParameterMode != "NAMED" || query.MaximumBytes != "1073741824" {
		t.Fatalf("query = %+v", query)
	}
	params := map[string]bqParam{}
	for _, p := range query.QueryParameters {
		params[p.Name] = p
	}
	if params["from_ledger"].Type.Type != "INT64" || params["from_ledger"].Value.Value != "100" {
		t.Fatalf("from_ledger = %+v", params["from_ledger"])
	}
	if params["from_time"].Type.Type != "TIMESTAMP" || params["from_time"].Value.Value != "2024-03-01 00:00:00 UTC" {
		t.Fatalf("from_time = %+v", params["from_time"])
	}
}

func TestBigQueryReaderReportsAPIErrors(t *testing.T) {
	srv, _, _ := fakeBigQuery(t)
	r := &BigQueryReader{Endpoint: srv.URL, BillingProject: "billing", Tokens: staticToken("wrong")}
	_, err := r.Rows(context.Background(), "SELECT 1", nil)
	if err == nil || !strings.Contains(err.Error(), "unauthenticated") {
		t.Fatalf("err = %v", err)
	}

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jobComplete":true,"error":{"message":"Query exceeded limit for bytes billed"}}`))
	}))
	defer srv2.Close()
	r = &BigQueryReader{Endpoint: srv2.URL, BillingProject: "billing", Tokens: staticToken("tok")}
	if _, err := r.Rows(context.Background(), "SELECT 1", nil); err == nil || !strings.Contains(err.Error(), "bytes billed") {
		t.Fatalf("err = %v", err)
	}
}

func TestBigQueryReaderNeedsConfig(t *testing.T) {
	if _, err := (&BigQueryReader{}).Rows(context.Background(), "SELECT 1", nil); err == nil {
		t.Fatal("a reader without a billing project ran")
	}
	r := &BigQueryReader{BillingProject: "b", Tokens: staticToken("t")}
	if _, err := r.Rows(context.Background(), "SELECT 1", map[string]any{"bad": 1.5}); err == nil {
		t.Fatal("an unsupported parameter type was accepted")
	}
}

func TestDecodeRowsRejectsMissingColumns(t *testing.T) {
	resp := bqResponse{JobComplete: true}
	resp.Schema.Fields = []bqField{{Name: "ledger_sequence"}}
	resp.Rows = append(resp.Rows, struct {
		F []struct {
			V any `json:"v"`
		} `json:"f"`
	}{})
	if _, err := decodeRows(resp); err == nil {
		t.Fatal("rows without the expected columns were decoded")
	}
}
