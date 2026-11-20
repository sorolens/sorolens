package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
)

const (
	invTestContract = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	invTestHash     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func sampleInvocation() client.Invocation {
	return client.Invocation{
		TxHash: invTestHash, ContractID: invTestContract, Ledger: 1234,
		LedgerClosedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Status:         "SUCCESS", FunctionName: "transfer", CPUInsn: 777, ResourceFeeCharged: 55,
	}
}

func invocationsTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	notFound := func(w http.ResponseWriter, msg string) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"` + msg + `"}}`))
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/contracts/" + invTestContract + "/invocations":
			q := r.URL.Query()
			if q.Get("status") != "FAILED" || q.Get("function_name") != "transfer" || q.Get("limit") != "5" {
				t.Errorf("unexpected query: %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(client.InvocationsResponse{Invocations: []client.Invocation{sampleInvocation()}})
		case "/api/v1/invocations/" + invTestHash + "/trace":
			_, _ = w.Write([]byte(`{"tx_hash":"` + invTestHash + `","root":{"contract_id":"` + invTestContract + `","function_name":"transfer"}}`))
		case "/api/v1/invocations/" + strings.Repeat("b", 64) + "/trace":
			notFound(w, "transaction not found")
		case "/api/v1/invocations/" + strings.Repeat("c", 64) + "/trace":
			_, _ = w.Write([]byte(`{"tx_hash":"x","root":{}}`))
		case "/api/v1/contracts/" + invTestContract + "/invocations/" + invTestHash:
			_ = json.NewEncoder(w).Encode(client.InvocationDetail{
				Invocation: sampleInvocation(),
				Events:     []client.Event{{Type: "contract", ContractID: invTestContract, TopicDecoded: []any{"transfer"}, ValueDecoded: "42"}},
			})
		default:
			notFound(w, "not found")
		}
	}))
}

func TestInvocationsTable(t *testing.T) {
	srv := invocationsTestServer(t)
	defer srv.Close()
	rootCmd.SetArgs([]string{"invocations", invTestContract, "--status", "failed", "--fn", "transfer", "--limit", "5", "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Tx Hash", "Function", invTestHash, "1234", "SUCCESS", "transfer", "777", "55"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestInvocationsListSubcommandJSON(t *testing.T) {
	srv := invocationsTestServer(t)
	defer srv.Close()
	rootCmd.SetArgs([]string{"invocations", "list", invTestContract, "--status", "FAILED", "--fn", "transfer", "--limit", "5", "--json", "--api-url", srv.URL})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []client.Invocation
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0].TxHash != invTestHash {
		t.Fatalf("got %+v", got)
	}
}

func TestInvocationsRejectsBadFlags(t *testing.T) {
	for name, args := range map[string][]string{
		"status": {"invocations", invTestContract, "--status", "bogus"},
		"limit":  {"invocations", invTestContract, "--limit", "0"},
	} {
		rootCmd.SetArgs(append(args, "--api-url", "http://127.0.0.1:1"))
		if _, err := captureOutput(t, rootCmd.Execute); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestInspectTxResolvesContractAndPrintsEvents(t *testing.T) {
	srv := invocationsTestServer(t)
	defer srv.Close()
	rootCmd.SetArgs([]string{"inspect", "--tx", strings.ToUpper(invTestHash), "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Tx Hash", invTestHash, invTestContract, "transfer", "Events (1)", "42"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestInspectTxJSONWithContractHint(t *testing.T) {
	srv := invocationsTestServer(t)
	defer srv.Close()
	rootCmd.SetArgs([]string{"inspect", "--tx", invTestHash, invTestContract, "--json", "--api-url", srv.URL})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got client.InvocationDetail
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.TxHash != invTestHash || len(got.Events) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestInspectTxErrors(t *testing.T) {
	srv := invocationsTestServer(t)
	defer srv.Close()
	cases := map[string]struct {
		args []string
		want string
	}{
		"malformed hash":   {[]string{"inspect", "--tx", "xyz"}, "invalid transaction hash"},
		"unknown hash":     {[]string{"inspect", "--tx", strings.Repeat("b", 64)}, "transaction not found"},
		"no contract":      {[]string{"inspect", "--tx", strings.Repeat("c", 64)}, "pass the contract ID"},
		"unknown for ctr":  {[]string{"inspect", "--tx", strings.Repeat("d", 64), invTestContract}, "not found for contract"},
		"missing argument": {[]string{"inspect"}, "--tx"},
	}
	for name, tc := range cases {
		rootCmd.SetArgs(append(tc.args, "--api-url", srv.URL))
		_, err := captureOutput(t, rootCmd.Execute)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want containing %q", name, err, tc.want)
		}
	}
}
