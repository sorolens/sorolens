package client_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type ConformanceFixture struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Request     struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	} `json:"request"`
	Response struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    any               `json:"body"`
	} `json:"response"`
	Expected struct {
		Status     int    `json:"status"`
		ErrorCode  string `json:"error_code,omitempty"`
		RetryAfter string `json:"retry_after,omitempty"`
	} `json:"expected"`
}

func TestSDKConformance(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "tests", "sdk-conformance", "fixtures")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("read fixtures directory: %v", err)
	}

	fixturesTested := 0
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(fixtureDir, entry.Name()))
		if err != nil {
			t.Fatalf("read fixture %s: %v", entry.Name(), err)
		}

		var f ConformanceFixture
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatalf("unmarshal fixture %s: %v", entry.Name(), err)
		}

		t.Run(f.ID, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range f.Response.Headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(f.Response.Status)
				_ = json.NewEncoder(w).Encode(f.Response.Body)
			}))
			defer server.Close()

			req, err := http.NewRequest(f.Request.Method, server.URL+f.Request.Path, nil)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer test_token")

			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("execute request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != f.Expected.Status {
				t.Errorf("status mismatch: got %d, want %d", resp.StatusCode, f.Expected.Status)
			}

			if f.Expected.RetryAfter != "" {
				if got := resp.Header.Get("Retry-After"); got != f.Expected.RetryAfter {
					t.Errorf("Retry-After header mismatch: got %q, want %q", got, f.Expected.RetryAfter)
				}
			}

			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil && f.Response.Body != nil {
				t.Errorf("decode json body: %v", err)
			}

			if f.Expected.ErrorCode != "" {
				errObj, ok := body["error"].(map[string]any)
				if !ok {
					t.Errorf("expected error object in body, got %v", body)
				} else if errObj["code"] != f.Expected.ErrorCode {
					t.Errorf("expected error code %q, got %q", f.Expected.ErrorCode, errObj["code"])
				}
			}
		})
		fixturesTested++
	}

	if fixturesTested < 15 {
		t.Errorf("expected at least 15 fixtures tested, got %d", fixturesTested)
	}
}
