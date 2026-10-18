package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/cli/internal/client"
)

func TestSnapshotCommandSuccessWithLedger(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/contracts/"+contractID+"/snapshot" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("ledger"); got != "12345" {
			t.Errorf("ledger query: got %q, want 12345", got)
		}
		resp := client.ContractSnapshot{
			ContractID:         contractID,
			Network:            "testnet",
			Ledger:             12345,
			FirstTrackedLedger: 1000,
			Storage: []client.StorageEntry{
				{
					ContractID:      contractID,
					KeyXDR:          "AAAAFA==",
					Durability:      "persistent",
					LiveUntilLedger: 20000,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"snapshot", contractID, "--ledger", "12345", "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "12345") {
		t.Errorf("output missing ledger 12345, got:\n%s", out)
	}
	if !strings.Contains(out, "persistent") {
		t.Errorf("output missing durability, got:\n%s", out)
	}
}

func TestSnapshotCommandSuccessCurrentExport(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/contracts/"+contractID+"/snapshot.json" {
			http.NotFound(w, r)
			return
		}
		resp := client.ContractSnapshotExport{
			SchemaVersion: 1,
			ContractID:    contractID,
			Network:       "testnet",
			Ledger:        54321,
			Storage: []client.StorageEntry{
				{
					ContractID:      contractID,
					KeyXDR:          "BBBBFA==",
					Durability:      "instance",
					LiveUntilLedger: 30000,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"snapshot", contractID, "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "54321") {
		t.Errorf("output missing ledger 54321, got:\n%s", out)
	}
	if !strings.Contains(out, "instance") {
		t.Errorf("output missing durability, got:\n%s", out)
	}
}

func TestSnapshotCommandJSON(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := client.ContractSnapshot{
			ContractID: contractID,
			Network:    "testnet",
			Ledger:     12345,
			Storage: []client.StorageEntry{
				{ContractID: contractID, KeyXDR: "AAAAFA==", Durability: "persistent", LiveUntilLedger: 20000},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"snapshot", contractID, "--ledger", "12345", "--api-url", srv.URL, "--json"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var parsed client.ContractSnapshot
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("invalid json output: %v\nOutput: %s", err, out)
	}
	if parsed.ContractID != contractID || parsed.Ledger != 12345 {
		t.Errorf("unexpected parsed snapshot: %+v", parsed)
	}
}

func TestSnapshotCommandErrorNon2xx(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "NOT_FOUND",
				"message": "contract not found",
			},
		})
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"snapshot", contractID, "--api-url", srv.URL})
	_, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSnapshotCommandMissingArgument(t *testing.T) {
	rootCmd.SetArgs([]string{"snapshot"})
	_, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error when contract-id argument is missing, got nil")
	}
}

func TestSnapshotHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"snapshot", "--help"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "snapshot of a contract's storage entries and state") {
		t.Errorf("output missing long description: %s", out)
	}
	if !strings.Contains(out, "--ledger uint32") {
		t.Errorf("output missing --ledger flag description: %s", out)
	}
}
