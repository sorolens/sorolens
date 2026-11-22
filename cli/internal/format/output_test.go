package format_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sorolens/sorolens/cli/internal/format"
)

type row struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

var rows = []row{{"a", 1}, {"b", 2}}

func render(t *testing.T, v any, f, q string) string {
	t.Helper()
	o := format.Options{Format: f}
	if q != "" {
		parsed, err := format.ParseQuery(q)
		if err != nil {
			t.Fatalf("parse %q: %v", q, err)
		}
		o.Query = parsed
	}
	var buf bytes.Buffer
	if err := format.Fprint(&buf, v, o); err != nil {
		t.Fatalf("print: %v", err)
	}
	return buf.String()
}

func TestOutputGolden(t *testing.T) {
	cases := map[string]string{
		"output_json.txt":        render(t, rows, "json", ""),
		"output_yaml.txt":        render(t, rows, "yaml", ""),
		"output_jsonl.txt":       render(t, rows, "jsonl", ""),
		"output_query.txt":       render(t, rows, "json", ".[] | .id"),
		"output_query_jsonl.txt": render(t, rows, "jsonl", ".[]"),
		"output_query_yaml.txt":  render(t, rows, "yaml", ".[].count"),
	}
	for name, got := range cases {
		checkGolden(t, name, got)
	}
}

func TestDefaultFormatIsIndentedJSON(t *testing.T) {
	got := render(t, row{"a", 1}, "", "")
	if got != "{\n  \"id\": \"a\",\n  \"count\": 1\n}\n" {
		t.Fatalf("default changed: %q", got)
	}
}

func TestJSONLStreamsOneLinePerRow(t *testing.T) {
	got := render(t, rows, "jsonl", "")
	if lines := strings.Split(strings.TrimSpace(got), "\n"); len(lines) != 2 {
		t.Fatalf("want 2 lines, got %q", got)
	}
}

func TestQueryErrorsReportPosition(t *testing.T) {
	for expr, want := range map[string]string{
		".a | | .b": "position 6",
		".a$":       "position 3",
		".a[x]":     "position 4",
		"":          "position 1",
	} {
		_, err := format.ParseQuery(expr)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: want error containing %q, got %v", expr, want, err)
		}
	}
	q, _ := format.ParseQuery(".[] | .id.x")
	var buf bytes.Buffer
	err := format.Fprint(&buf, rows, format.Options{Format: "json", Query: q})
	if err == nil || !strings.Contains(err.Error(), "position") {
		t.Errorf("runtime error should report position, got %v", err)
	}
}

func TestQueryIndexAndMissing(t *testing.T) {
	if got := render(t, rows, "json", ".[-1].id"); got != "\"b\"\n" {
		t.Errorf("got %q", got)
	}
	if got := render(t, rows, "json", ".[5]"); got != "null\n" {
		t.Errorf("got %q", got)
	}
}
