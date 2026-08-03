package format_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/cli/internal/format"
)

var update = flag.Bool("update", false, "update golden files")

func TestRenderTable(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	headers := []string{"Name", "Value", "Status"}
	rows := [][]string{
		{"alpha", "100", "active"},
		{"beta-contract", "200", "pending"},
	}
	got := format.RenderTable(headers, rows)
	checkGolden(t, "render_table.txt", got)
}

func TestRenderTableEmpty(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := format.RenderTable([]string{}, [][]string{})
	if got != "" {
		t.Errorf("expected empty string for empty table, got %q", got)
	}
}

func TestRenderTableSingleRow(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := format.RenderTable([]string{"Key", "Val"}, [][]string{{"k1", "v1"}})
	if !strings.Contains(got, "Key") || !strings.Contains(got, "Val") {
		t.Error("headers missing from output")
	}
	if !strings.Contains(got, "k1") || !strings.Contains(got, "v1") {
		t.Error("data missing from output")
	}
}

func TestColorByUrgencyThresholds(t *testing.T) {
	tests := []struct {
		ledgersLeft int
		desc        string
	}{
		{500, "red (<=1000)"},
		{1000, "red (<=1000 boundary)"},
		{1001, "yellow (>1000)"},
		{10000, "yellow (<=10000 boundary)"},
		{10001, "green (>10000)"},
		{100000, "green"},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			style := format.ColorByUrgency(tc.ledgersLeft)
			// With NO_COLOR, all styles render plainly. Just verify Render works.
			got := style.Render("test")
			if got != "test" {
				t.Errorf("ColorByUrgency(%d).Render(\"test\"): got %q, want \"test\"", tc.ledgersLeft, got)
			}
		})
	}
}

// checkGolden compares got against the golden file. If -update is set, writes the file.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden file: %v", err)
		}
		t.Logf("updated golden file: %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden file %s not found; run with -update to create it", goldenPath)
	}
	if string(want) != got {
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, string(want))
	}
}
