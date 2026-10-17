package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sorolens/go-client/examples/internal/harness"
)

func TestRunPagesThroughEveryContract(t *testing.T) {
	server := harness.NewServer()
	defer server.Close()

	var out bytes.Buffer
	if err := run(context.Background(), server.URL, "sl_test", &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := out.String()
	for _, suffix := range []string{"AD4", "AE5"} { // only present on the second page
		if !strings.Contains(got, suffix) {
			t.Errorf("output missing second-page contract %s:\n%s", suffix, got)
		}
	}
	if !strings.Contains(got, "fetched 4 contract(s)") {
		t.Errorf("output missing summary:\n%s", got)
	}
}
