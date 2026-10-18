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

const contractsTestID = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func contractsTestServer(t *testing.T, contracts []client.Contract) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/contracts":
			if got := r.URL.Query().Get("network"); got != "mainnet" {
				t.Errorf("network query: got %q, want mainnet", got)
			}
			if got := r.URL.Query().Get("limit"); got != "5" {
				t.Errorf("limit query: got %q, want 5", got)
			}
			_ = json.NewEncoder(w).Encode(client.ContractsResponse{Contracts: contracts})
		case r.URL.Path == "/api/v1/contracts/"+contractsTestID:
			_ = json.NewEncoder(w).Encode(contracts[0])
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"contract not found"}}`))
		}
	}))
}

func sampleContracts() []client.Contract {
	return []client.Contract{{
		ID: contractsTestID, Network: "mainnet", Label: "my-token", Status: "active",
		WasmHash: "abc123", CreatedAtLedger: 42,
		AddedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}}
}

func TestContractsListTable(t *testing.T) {
	srv := contractsTestServer(t, sampleContracts())
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "list", "--network", "mainnet", "--limit", "5", "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"ID", "Label", "Network", "Status", contractsTestID, "my-token", "mainnet", "active", "2026-01-02T03:04:05Z"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestContractsListJSON(t *testing.T) {
	srv := contractsTestServer(t, sampleContracts())
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "list", "--network", "mainnet", "--limit", "5", "--json", "--api-url", srv.URL})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []client.Contract
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not a JSON array: %v\n%s", err, out)
	}
	if len(got) != 1 || got[0].ID != contractsTestID {
		t.Errorf("unexpected contracts: %+v", got)
	}
}

func TestContractsListEmpty(t *testing.T) {
	srv := contractsTestServer(t, nil)
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "list", "--network", "mainnet", "--limit", "5", "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No tracked contracts found.") {
		t.Errorf("expected empty message, got:\n%s", out)
	}
}

func TestContractsListEmptyJSON(t *testing.T) {
	srv := contractsTestServer(t, nil)
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "list", "--network", "mainnet", "--limit", "5", "--json", "--api-url", srv.URL})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("expected [], got %q", out)
	}
}

func TestContractsListInvalidLimit(t *testing.T) {
	rootCmd.SetArgs([]string{"contracts", "list", "--limit", "0", "--api-url", "http://127.0.0.1:1"})
	_, err := captureOutput(t, rootCmd.Execute)
	if err == nil || !strings.Contains(err.Error(), "--limit") {
		t.Fatalf("expected --limit error, got %v", err)
	}
}

func TestContractsGetTable(t *testing.T) {
	srv := contractsTestServer(t, sampleContracts())
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "get", contractsTestID, "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{contractsTestID, "my-token", "mainnet", "active", "abc123", "42", "pending"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestContractsGetJSON(t *testing.T) {
	srv := contractsTestServer(t, sampleContracts())
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "get", contractsTestID, "--json", "--api-url", srv.URL})
	out, err := captureOutput(t, rootCmd.Execute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got client.Contract
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, out)
	}
	if got.ID != contractsTestID || got.Label != "my-token" {
		t.Errorf("unexpected contract: %+v", got)
	}
}

func TestContractsGetNotFound(t *testing.T) {
	srv := contractsTestServer(t, sampleContracts())
	defer srv.Close()

	rootCmd.SetArgs([]string{"contracts", "get", "CBBB", "--api-url", srv.URL, "--no-color"})
	_, err := captureOutput(t, rootCmd.Execute)
	if err == nil || !strings.Contains(err.Error(), "contract not found: CBBB") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestContractsGetRequiresID(t *testing.T) {
	rootCmd.SetArgs([]string{"contracts", "get"})
	_, err := captureOutput(t, rootCmd.Execute)
	if err == nil {
		t.Fatal("expected error for missing contract id")
	}
}
