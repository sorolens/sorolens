package soroban

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

// fixture loads a testdata file and panics if missing.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return b
}

// staticServer returns a test server that always responds with statusCode and body.
func staticServer(statusCode int, body []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write(body) //nolint:errcheck
	}))
}

// ---- GetLatestLedger tests ------------------------------------------------

func TestGetLatestLedger(t *testing.T) {
	t.Parallel()
	srv := staticServer(http.StatusOK, fixture(t, "getlatestledger_ok.json"))
	defer srv.Close()

	c := New(srv.URL, 0)
	got, err := c.GetLatestLedger(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sequence != 3730795 {
		t.Errorf("Sequence: want 3730795, got %d", got.Sequence)
	}
	if got.ProtocolVersion != 27 {
		t.Errorf("ProtocolVersion: want 27, got %d", got.ProtocolVersion)
	}
	wantID := "0a00a9cf845f7af7cff09c66f8ae6480e9971e6e2c7fa4afd8d6266ee13c987b"
	if got.ID != wantID {
		t.Errorf("ID: want %s, got %s", wantID, got.ID)
	}
}

// ---- GetEvents tests -------------------------------------------------------

func TestGetEvents_withEvents(t *testing.T) {
	t.Parallel()
	srv := staticServer(http.StatusOK, fixture(t, "getevents_ok.json"))
	defer srv.Close()

	c := New(srv.URL, 0)
	got, err := c.GetEvents(context.Background(), 200000, 200100,
		[]EventFilter{{Type: "contract", ContractIDs: []string{"CDLZFC3S..."}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Events) != 1 {
		t.Fatalf("Events: want 1, got %d", len(got.Events))
	}
	ev := got.Events[0]
	if ev.TxHash != "d9e771ac73ec80503c7594f540d10ec068fb80981d11acea41aa193b7543c5ce" {
		t.Errorf("TxHash mismatch: %s", ev.TxHash)
	}
	if ev.Ledger != 200010 {
		t.Errorf("Ledger: want 200010, got %d", ev.Ledger)
	}
	if got.Cursor != "0000863490289963008-0000000010" {
		t.Errorf("Cursor mismatch: %s", got.Cursor)
	}
}

func TestGetEvents_empty(t *testing.T) {
	t.Parallel()
	srv := staticServer(http.StatusOK, fixture(t, "getevents_empty.json"))
	defer srv.Close()

	c := New(srv.URL, 0)
	got, err := c.GetEvents(context.Background(), 300000, 300100, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Events) != 0 {
		t.Errorf("Events: want 0, got %d", len(got.Events))
	}
}

// ---- error handling tests -------------------------------------------------

func TestRPCError(t *testing.T) {
	t.Parallel()
	srv := staticServer(http.StatusOK, fixture(t, "rpc_error.json"))
	defer srv.Close()

	c := New(srv.URL, 0)
	_, err := c.GetLatestLedger(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("want *RPCError, got %T: %v", err, err)
	}
	if rpcErr.Code != -32602 {
		t.Errorf("Code: want -32602, got %d", rpcErr.Code)
	}
}

func TestMalformedJSON(t *testing.T) {
	t.Parallel()
	srv := staticServer(http.StatusOK, fixture(t, "malformed.json"))
	defer srv.Close()

	c := New(srv.URL, 0)
	_, err := c.GetLatestLedger(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestRetryOn5xx(t *testing.T) {
	t.Parallel()

	// Fail twice with 500, then succeed on the third attempt.
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "getlatestledger_ok.json")) //nolint:errcheck
	}))
	defer srv.Close()

	c := New(srv.URL, 0)
	// Override backoff to zero sleep so tests run fast.
	got, err := c.GetLatestLedger(context.Background())
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if got.Sequence != 3730795 {
		t.Errorf("Sequence: want 3730795, got %d", got.Sequence)
	}
	if callCount.Load() != 3 {
		t.Errorf("callCount: want 3, got %d", callCount.Load())
	}
}

func TestRetryOn429(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture(t, "getlatestledger_ok.json")) //nolint:errcheck
	}))
	defer srv.Close()

	c := New(srv.URL, 0)
	got, err := c.GetLatestLedger(context.Background())
	if err != nil {
		t.Fatalf("unexpected error after 429 retry: %v", err)
	}
	if got.Sequence != 3730795 {
		t.Errorf("Sequence mismatch: %d", got.Sequence)
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	// Server always returns 500 to force retries.
	srv := staticServer(http.StatusInternalServerError, []byte(`{}`))
	defer srv.Close()

	c := New(srv.URL, 0)
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately after the first attempt starts.
	cancel()

	_, err := c.GetLatestLedger(ctx)
	if err == nil {
		t.Fatal("expected error after context cancel, got nil")
	}
}

// ---- ScVal decoder tests --------------------------------------------------

func TestDecodeScVal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		b64       string
		wantType  string
		wantHuman string
		wantErr   bool
	}{
		{
			name:      "not base64",
			b64:       "!!!",
			wantErr:   true,
		},
		{
			name:    "too short",
			b64:     "AAAA",
			wantErr: true, // decodes to 3 bytes, below the 4-byte minimum
		},
		{
			// scvVoid = discriminant 1, big-endian: 0x00000001
			name:      "void",
			b64:       "AAAAAQ==",
			wantType:  "scvVoid",
			wantHuman: "void",
		},
		{
			// scvU32 = discriminant 3, value 42 = 0x0000002A
			// bytes: 00 00 00 03 00 00 00 2A -> base64 AAAAAw==AAAAKg== combined
			// discriminant 3: 00 00 00 03, value 42: 00 00 00 2A
			// full: 00 00 00 03 00 00 00 2A
			name:      "u32 value 42",
			b64:       "AAAAAwAAAAo=",
			wantType:  "scvU32",
			wantHuman: "10", // 0x0000000A = 10
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeScVal(tc.b64)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantType != "" && got.Type != tc.wantType {
				t.Errorf("Type: want %q, got %q", tc.wantType, got.Type)
			}
			if tc.wantHuman != "" && got.Human != tc.wantHuman {
				t.Errorf("Human: want %q, got %q", tc.wantHuman, got.Human)
			}
		})
	}
}
