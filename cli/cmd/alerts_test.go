package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func resetCommandFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Changed = false
		_ = f.Value.Set(f.DefValue)
	})
	for _, sub := range cmd.Commands() {
		resetCommandFlags(sub)
	}
}

func captureOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	globalConfig = Config{Timeout: 10 * time.Second}
	resetCommandFlags(rootCmd)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	fnErr := fn()

	_ = w.Close()
	os.Stdout = oldStdout
	out := <-outChan
	_ = r.Close()

	return out, fnErr
}

func TestAlertsListCommandSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/watchdog/alerts" {
			http.NotFound(w, r)
			return
		}
		resp := client.AlertsResponse{
			Alerts: []client.ContractAlert{
				{
					ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
					Severity:   "critical",
					Message:    "Storage TTL under 500 ledgers",
					Ledger:     12345,
					Timestamp:  time.Date(2026, 2, 15, 10, 30, 0, 0, time.UTC),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"alerts", "list", "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "Storage TTL under 500 ledgers") {
		t.Errorf("output missing alert message, got:\n%s", out)
	}
	if !strings.Contains(out, "critical") {
		t.Errorf("output missing severity, got:\n%s", out)
	}
}

func TestAlertsListCommandJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := client.AlertsResponse{
			Alerts: []client.ContractAlert{
				{
					ContractID: "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
					Severity:   "warning",
					Message:    "Invocation spike detected",
					Ledger:     54321,
					Timestamp:  time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"alerts", "list", "--api-url", srv.URL, "--json"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var parsed []client.ContractAlert
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("invalid json output: %v\nOutput: %s", err, out)
	}
	if len(parsed) != 1 || parsed[0].Message != "Invocation spike detected" {
		t.Errorf("unexpected parsed alerts: %+v", parsed)
	}
}

func TestAlertsListCommandErrorNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "database unavailable",
			},
		})
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"alerts", "list", "--api-url", srv.URL})
	_, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAlertsHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"alerts", "--help"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "alerts commands allow you to query and inspect alerts") {
		t.Errorf("output missing long description: %s", out)
	}
	if !strings.Contains(out, "sorolens alerts list") {
		t.Errorf("output missing examples: %s", out)
	}
}
