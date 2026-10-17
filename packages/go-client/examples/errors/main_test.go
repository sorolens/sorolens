package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sorolens/go-client/examples/internal/harness"
)

func TestRunReportsNotFoundAndRateLimit(t *testing.T) {
	server := harness.NewServer()
	defer server.Close()

	var out bytes.Buffer
	if err := run(context.Background(), server.URL, "sl_test", &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "not found:") {
		t.Errorf("missing 404 handling:\n%s", got)
	}
	if !strings.Contains(got, "rate limited:") {
		t.Errorf("missing 429 handling:\n%s", got)
	}
}
