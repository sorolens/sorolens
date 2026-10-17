package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sorolens/go-client/examples/internal/harness"
)

func TestRunReadsServerSentEvents(t *testing.T) {
	server := harness.NewServer()
	defer server.Close()

	var out bytes.Buffer
	if err := run(context.Background(), server.URL, "sl_test", &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	got := out.String()
	for _, id := range []string{"evt_1", "evt_2"} {
		if !strings.Contains(got, id) {
			t.Errorf("output missing %s:\n%s", id, got)
		}
	}
}
