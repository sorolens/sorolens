package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fuzzReportEntry struct {
	Method       string `json:"method"`
	Path         string `json:"path"`
	Status       int    `json:"status"`
	DurationMs   int64  `json:"duration_ms"`
	ResponseOK   bool   `json:"response_ok"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type fuzzReport struct {
	GeneratedAt time.Time         `json:"generated_at"`
	TotalTested int               `json:"total_tested"`
	Passed      int               `json:"passed"`
	Failed      int               `json:"failed"`
	Entries     []fuzzReportEntry `json:"entries"`
}

// TestOpenAPIContractFuzz exercises documented endpoints against the running API (issue #850).
func TestOpenAPIContractFuzz(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping contract fuzzing in short mode")
	}

	root := "../../"
	specPath := filepath.Join(root, "docs", "openapi.yaml")
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Skipf("cannot read openapi spec at %s: %v", specPath, err)
	}

	lines := strings.Split(string(specBytes), "\n")
	var paths []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "/") && strings.HasSuffix(trimmed, ":") {
			p := strings.TrimSuffix(trimmed, ":")
			paths = append(paths, p)
		}
	}

	exclusions := map[string]string{
		"/api/v1/stream/events":        "streaming SSE connection",
		"/api/v1/subscribe":            "websocket connection",
		"/debug/pprof":                 "runtime pprof profiler",
		"/integrations/slack/commands": "slack signed HMAC requests",
	}

	var report fuzzReport
	report.GeneratedAt = time.Now().UTC()

	methods := []string{"GET", "POST", "DELETE"}

	for _, p := range paths {
		if _, excluded := exclusions[p]; excluded {
			continue
		}

		resolvedPath := strings.ReplaceAll(p, "{id}", contractID)
		resolvedPath = strings.ReplaceAll(resolvedPath, "{contractId}", contractID)
		resolvedPath = strings.ReplaceAll(resolvedPath, "{tx_hash}", txHash)
		resolvedPath = strings.ReplaceAll(resolvedPath, "{tag}", "fuzz-tag")
		resolvedPath = strings.ReplaceAll(resolvedPath, "{noteID}", "fuzz-note")

		for _, method := range methods {
			testBodies := []string{
				"",
				"{}",
				`{"invalid": true}`,
			}

			for _, b := range testBodies {
				var body io.Reader
				if method != "GET" {
					body = strings.NewReader(b)
				}

				req, err := http.NewRequest(method, apiURL+resolvedPath, body)
				if err != nil {
					continue
				}
				if sessionToken != "" {
					req.Header.Set("Authorization", "Bearer "+sessionToken)
				}
				if b != "" {
					req.Header.Set("Content-Type", "application/json")
				}

				start := time.Now()
				resp, err := apiHTTPClient.Do(req)
				duration := time.Since(start).Milliseconds()

				report.TotalTested++
				entry := fuzzReportEntry{
					Method:     method,
					Path:       resolvedPath,
					DurationMs: duration,
				}

				if err != nil {
					entry.ResponseOK = false
					entry.ErrorMessage = err.Error()
					report.Failed++
				} else {
					_ = resp.Body.Close()
					entry.Status = resp.StatusCode
					// Any 500 status code is an unhandled server defect
					if resp.StatusCode >= 500 {
						entry.ResponseOK = false
						entry.ErrorMessage = fmt.Sprintf("HTTP %d server error", resp.StatusCode)
						report.Failed++
						t.Errorf("[%s %s] server error: %d", method, resolvedPath, resp.StatusCode)
					} else {
						entry.ResponseOK = true
						report.Passed++
					}
				}
				report.Entries = append(report.Entries, entry)
			}
		}
	}

	reportDir := filepath.Join(root, "tests", "fuzz", "reports")
	_ = os.MkdirAll(reportDir, 0755)
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(reportDir, "fuzz-report.json"), reportJSON, 0644)
	}

	t.Logf("OpenAPI contract fuzzing finished: %d requests tested (%d passed, %d failed)",
		report.TotalTested, report.Passed, report.Failed)
}
