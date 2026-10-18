package sorobanrpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/sorolens/sorolens/services/indexer/internal/poller"
)

// rpcReq is the decoded request body a test server sees.
type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func decodeReq(t *testing.T, r *http.Request) rpcReq {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var req rpcReq
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode request %s: %v", body, err)
	}
	return req
}

func writeResult(w http.ResponseWriter, id int64, result any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result}) //nolint:errcheck
}

// fastClient returns a client with test-friendly backoff.
func fastClient(url string, opts ...Option) *Client {
	base := []Option{WithBackoff(time.Millisecond, 5*time.Millisecond)}
	return New(url, append(base, opts...)...)
}

func TestGetLatestLedger_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		if req.JSONRPC != "2.0" || req.Method != "getLatestLedger" {
			t.Errorf("unexpected request %+v", req)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		writeResult(w, req.ID, map[string]any{"id": "abc", "sequence": 123456, "protocolVersion": 22})
	}))
	defer srv.Close()

	got, err := fastClient(srv.URL).GetLatestLedger(context.Background())
	if err != nil {
		t.Fatalf("GetLatestLedger: %v", err)
	}
	if got.Sequence != 123456 || got.ProtocolVersion != 22 {
		t.Fatalf("got %+v", got)
	}
}

func TestCall_JSONRPCErrorCarriesCodeAndIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		req := decodeReq(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"jsonrpc": "2.0", "id": req.ID,
			"error": map[string]any{"code": -32600, "message": "startLedger must be positive"},
		})
	}))
	defer srv.Close()

	_, err := fastClient(srv.URL).GetEvents(context.Background(), 0, 10, nil)
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("want *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -32600 || rpcErr.Method != "getEvents" {
		t.Fatalf("got %+v", rpcErr)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("JSON-RPC errors must not be retried; server saw %d calls", n)
	}
}

func TestCall_RetriesHTTP429ThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeResult(w, req.ID, map[string]any{"sequence": 7, "protocolVersion": 22})
	}))
	defer srv.Close()

	got, err := fastClient(srv.URL, WithMaxRetries(3)).GetLatestLedger(context.Background())
	if err != nil {
		t.Fatalf("GetLatestLedger: %v", err)
	}
	if got.Sequence != 7 {
		t.Fatalf("sequence = %d", got.Sequence)
	}
	if n := calls.Load(); n != 3 {
		t.Fatalf("server saw %d calls, want 3 (two 429s then success)", n)
	}
}

func TestCall_RetryIsBounded(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := fastClient(srv.URL, WithMaxRetries(2)).GetLatestLedger(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want HTTP 503 error, got %v", err)
	}
	if n := calls.Load(); n != 3 {
		t.Fatalf("server saw %d calls, want 3 (1 + 2 retries)", n)
	}
}

func TestCall_DoesNotRetryHTTP400(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := fastClient(srv.URL).GetLatestLedger(context.Background())
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("want HTTP 400 error, got %v", err)
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("server saw %d calls, want 1", n)
	}
}

func TestCall_TimeoutIsRetriedThenReported(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer func() {
		close(release)
		srv.Close()
	}()

	c := fastClient(srv.URL, WithTimeout(20*time.Millisecond), WithMaxRetries(1))
	start := time.Now()
	_, err := c.GetLatestLedger(context.Background())
	if err == nil {
		t.Fatal("want timeout error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want a deadline error, got %v", err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("server saw %d calls, want 2 (timeout is retriable)", n)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("per-request timeout not enforced: took %v", elapsed)
	}
}

func TestCall_ParentContextCancelStopsRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(srv.URL, WithBackoff(time.Hour, time.Hour)).GetLatestLedger(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestGetEvents_MapsFieldsAndFollowsCursor(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		var p getEventsParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			t.Fatalf("params: %v", err)
		}
		switch calls.Add(1) {
		case 1:
			if p.StartLedger != 100 || p.EndLedger != 200 || p.Pagination.Cursor != "" {
				t.Errorf("first page params = %+v", p)
			}
			if len(p.Filters) != 1 || p.Filters[0].ContractIDs[0] != "CABC" {
				t.Errorf("filters = %+v", p.Filters)
			}
			events := make([]map[string]any, eventsPageLimit)
			for i := range events {
				events[i] = map[string]any{"id": "e", "contractId": "CABC", "ledger": 150, "txHash": "tx"}
			}
			events[0] = map[string]any{
				"type": "contract", "ledger": 150, "ledgerClosedAt": "2026-10-01T00:00:00Z",
				"contractId": "CABC", "id": "0000000644245319680-0000000001",
				"inSuccessfulContractCall": true, "txHash": "deadbeef",
				"topic": []string{"AAAADwAAAAh0cmFuc2Zlcg=="}, "value": "AAAAAQ==",
				"transactionIndex": 2,
			}
			writeResult(w, req.ID, map[string]any{"events": events, "latestLedger": 300, "cursor": "c1"})
		case 2:
			if p.StartLedger != 0 || p.EndLedger != 0 || p.Pagination.Cursor != "c1" {
				t.Errorf("second page must be cursor-only, got %+v", p)
			}
			writeResult(w, req.ID, map[string]any{
				"events":       []map[string]any{{"id": "last", "contractId": "CABC", "ledger": 199}},
				"latestLedger": 301, "cursor": "c2",
			})
		default:
			t.Errorf("unexpected extra page")
		}
	}))
	defer srv.Close()

	res, err := fastClient(srv.URL).GetEvents(context.Background(), 100, 200,
		[]poller.EventFilter{{Type: "contract", ContractIDs: []string{"CABC"}}})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(res.Events) != eventsPageLimit+1 {
		t.Fatalf("got %d events, want %d", len(res.Events), eventsPageLimit+1)
	}
	first := res.Events[0]
	want := poller.RPCEvent{
		ID: "0000000644245319680-0000000001", ContractID: "CABC", Ledger: 150,
		LedgerClosedAt: "2026-10-01T00:00:00Z", TxHash: "deadbeef", Type: "contract",
		Topic: []string{"AAAADwAAAAh0cmFuc2Zlcg=="}, Value: "AAAAAQ==",
		InSuccessfulContractCall: true, TransactionIndex: 2,
	}
	if first.ID != want.ID || first.TxHash != want.TxHash || first.Value != want.Value ||
		first.LedgerClosedAt != want.LedgerClosedAt || !first.InSuccessfulContractCall ||
		first.TransactionIndex != 2 || len(first.Topic) != 1 || first.Topic[0] != want.Topic[0] {
		t.Fatalf("first event = %+v, want %+v", first, want)
	}
	if res.Events[len(res.Events)-1].ID != "last" || res.LatestLedger != 301 || res.Cursor != "c2" {
		t.Fatalf("tail = %+v latest=%d cursor=%q", res.Events[len(res.Events)-1], res.LatestLedger, res.Cursor)
	}
}

func TestGetTransaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		var p struct{ Hash string }
		json.Unmarshal(req.Params, &p) //nolint:errcheck
		if p.Hash == "missing" {
			writeResult(w, req.ID, map[string]any{"status": "NOT_FOUND", "latestLedger": 10})
			return
		}
		writeResult(w, req.ID, map[string]any{
			"status": "SUCCESS", "ledger": 1234, "createdAt": "1700000000",
			"applicationOrder": 3, "resultXdr": "AAAA",
		})
	}))
	defer srv.Close()
	c := fastClient(srv.URL)

	tx, err := c.GetTransaction(context.Background(), "abc")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}
	if tx.Status != "SUCCESS" || tx.Ledger != 1234 || tx.ApplicationOrder != 3 || tx.ResultXDR != "AAAA" {
		t.Fatalf("got %+v", tx)
	}
	if !tx.LedgerClosedAt.Equal(time.Unix(1700000000, 0)) {
		t.Fatalf("LedgerClosedAt = %v", tx.LedgerClosedAt)
	}

	if _, err := c.GetTransaction(context.Background(), "missing"); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("want ErrTransactionNotFound, got %v", err)
	}
}

func TestGetLedgerEntries_ReportsMissingKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		writeResult(w, req.ID, map[string]any{
			"entries":      []map[string]any{{"key": "k1", "xdr": "x1", "lastModifiedLedgerSeq": 9}},
			"latestLedger": 50,
		})
	}))
	defer srv.Close()

	res, err := fastClient(srv.URL).GetLedgerEntries(context.Background(), []string{"k1", "k2"})
	if err != nil {
		t.Fatalf("GetLedgerEntries: %v", err)
	}
	if len(res.Entries) != 1 || res.Entries[0].XDR != "x1" || res.Entries[0].LastModifiedLedgerSeq != 9 {
		t.Fatalf("entries = %+v", res.Entries)
	}
	if res.LatestLedger != 50 || len(res.KeysNotFound) != 1 || res.KeysNotFound[0] != "k2" {
		t.Fatalf("latest=%d notFound=%v", res.LatestLedger, res.KeysNotFound)
	}
}

func TestGetTransactions_CursorReplacesStartLedger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		var p struct {
			StartLedger uint32     `json:"startLedger"`
			Pagination  pagination `json:"pagination"`
		}
		json.Unmarshal(req.Params, &p) //nolint:errcheck
		if p.StartLedger != 0 || p.Pagination.Cursor != "cur" || p.Pagination.Limit != 5 {
			t.Errorf("params = %+v", p)
		}
		writeResult(w, req.ID, map[string]any{
			"transactions": []map[string]any{{"status": "SUCCESS", "txHash": "h", "ledger": 8, "envelopeXdr": "env"}},
			"latestLedger": 9, "cursor": "next",
		})
	}))
	defer srv.Close()

	res, err := fastClient(srv.URL).GetTransactions(context.Background(), 100, "cur", 5)
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(res.Transactions) != 1 || res.Transactions[0].EnvelopeXDR != "env" || res.Cursor != "next" {
		t.Fatalf("got %+v", res)
	}
}

func TestGetContractWasmHash(t *testing.T) {
	contractID, err := strkey.Encode(strkey.VersionByteContract, make([]byte, 32))
	if err != nil {
		t.Fatalf("encode contract id: %v", err)
	}
	var wasmHash xdr.Hash
	for i := range wasmHash {
		wasmHash[i] = byte(i)
	}
	entry := xdr.LedgerEntryData{
		Type: xdr.LedgerEntryTypeContractData,
		ContractData: &xdr.ContractDataEntry{
			Contract: xdr.ScAddress{
				Type:       xdr.ScAddressTypeScAddressTypeContract,
				ContractId: &xdr.ContractId{},
			},
			Key:        xdr.ScVal{Type: xdr.ScValTypeScvLedgerKeyContractInstance},
			Durability: xdr.ContractDataDurabilityPersistent,
			Val: xdr.ScVal{
				Type: xdr.ScValTypeScvContractInstance,
				Instance: &xdr.ScContractInstance{
					Executable: xdr.ContractExecutable{
						Type:     xdr.ContractExecutableTypeContractExecutableWasm,
						WasmHash: &wasmHash,
					},
				},
			},
		},
	}
	entryXDR, err := xdr.MarshalBase64(entry)
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}
	wantKey, err := ContractInstanceLedgerKey(contractID)
	if err != nil {
		t.Fatalf("ContractInstanceLedgerKey: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := decodeReq(t, r)
		var p struct{ Keys []string }
		json.Unmarshal(req.Params, &p) //nolint:errcheck
		if len(p.Keys) != 1 || p.Keys[0] != wantKey {
			t.Errorf("keys = %v, want [%s]", p.Keys, wantKey)
		}
		writeResult(w, req.ID, map[string]any{
			"entries":      []map[string]any{{"key": wantKey, "xdr": entryXDR}},
			"latestLedger": 1,
		})
	}))
	defer srv.Close()

	got, err := fastClient(srv.URL).GetContractWasmHash(context.Background(), contractID)
	if err != nil {
		t.Fatalf("GetContractWasmHash: %v", err)
	}
	if want := "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"; got != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}

	// The key round-trips through the SDK as a persistent instance key.
	var key xdr.LedgerKey
	if err := xdr.SafeUnmarshalBase64(wantKey, &key); err != nil {
		t.Fatalf("decode key: %v", err)
	}
	if key.ContractData == nil || key.ContractData.Durability != xdr.ContractDataDurabilityPersistent ||
		key.ContractData.Key.Type != xdr.ScValTypeScvLedgerKeyContractInstance {
		t.Fatalf("unexpected key %+v", key)
	}
}

func TestContractInstanceLedgerKey_RejectsBadIDs(t *testing.T) {
	for _, id := range []string{"", "nothex", "CAAA"} {
		if _, err := ContractInstanceLedgerKey(id); err == nil {
			t.Errorf("ContractInstanceLedgerKey(%q): want error", id)
		}
	}
}
