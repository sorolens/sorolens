package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/cli/internal/client"
)

func TestReportCommandSuccess(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/reports/"+contractID {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("month"); got != "2026-02" {
			t.Errorf("month query: got %q, want 2026-02", got)
		}
		resp := client.ContractReport{
			ContractID:    contractID,
			Month:         "2026-02",
			UptimePct:     99.98,
			TotalChecks:   5000,
			HealthyChecks: 4999,
			Incidents:     1,
			MTTRSeconds:   45.5,
			Signed:        true,
			Signature:     "123456",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"report", contractID, "--month", "2026-02", "--api-url", srv.URL, "--no-color"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "99.98%") {
		t.Errorf("output missing uptime, got:\n%s", out)
	}
	if !strings.Contains(out, "Met (>=99.9%)") {
		t.Errorf("output missing SLA status, got:\n%s", out)
	}
	if !strings.Contains(out, "2026-02") {
		t.Errorf("output missing month, got:\n%s", out)
	}
}

func TestReportCommandJSON(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := client.ContractReport{
			ContractID:    contractID,
			Month:         "2026-02",
			UptimePct:     99.90,
			TotalChecks:   1000,
			HealthyChecks: 999,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"report", contractID, "--month", "2026-02", "--api-url", srv.URL, "--json"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var parsed client.ContractReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &parsed); err != nil {
		t.Fatalf("invalid json output: %v\nOutput: %s", err, out)
	}
	if parsed.ContractID != contractID || parsed.Month != "2026-02" {
		t.Errorf("unexpected parsed report: %+v", parsed)
	}
}

func TestReportCommandInvalidMonthFormat(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	rootCmd.SetArgs([]string{"report", contractID, "--month", "invalid-month", "--api-url", "http://localhost:1"})
	_, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error for invalid month format, got nil")
	}
	if !strings.Contains(err.Error(), "must be YYYY-MM") {
		t.Errorf("expected error message to mention YYYY-MM, got: %v", err)
	}
}

func TestReportCommandErrorNon2xx(t *testing.T) {
	contractID := "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{
				"code":    "NOT_IMPLEMENTED",
				"message": "sla reporting not supported by this deployment",
			},
		})
	}))
	defer srv.Close()

	rootCmd.SetArgs([]string{"report", contractID, "--month", "2026-02", "--api-url", srv.URL})
	_, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReportHelp(t *testing.T) {
	rootCmd.SetArgs([]string{"report", "--help"})
	out, err := captureOutput(t, func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out, "monthly service-level agreement") {
		t.Errorf("output missing long description: %s", out)
	}
	if !strings.Contains(out, "--month string") {
		t.Errorf("output missing --month flag description: %s", out)
	}
}
