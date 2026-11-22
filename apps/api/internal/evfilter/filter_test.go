package evfilter_test

import (
	"strings"
	"testing"

	"github.com/sorolens/sorolens/apps/api/internal/evfilter"
)

func TestParseEmpty(t *testing.T) {
	e, err := evfilter.Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if !e.Empty() {
		t.Fatal("empty string should produce empty expr")
	}
}

func TestParseTopicEquality(t *testing.T) {
	e, err := evfilter.Parse(`topic[0]="transfer"`)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var args []any
	e.AppendSQL(&sb, &args)
	sql := sb.String()
	if !strings.Contains(sql, "topic_decoded") {
		t.Errorf("expected topic_decoded in SQL, got: %s", sql)
	}
	if len(args) == 0 {
		t.Error("expected at least one bind argument")
	}
}

func TestParseValueGTE(t *testing.T) {
	e, err := evfilter.Parse("value.amount>=1000")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var args []any
	e.AppendSQL(&sb, &args)
	sql := sb.String()
	if !strings.Contains(sql, "value_decoded") {
		t.Errorf("expected value_decoded in SQL, got: %s", sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestParseAndChain(t *testing.T) {
	e, err := evfilter.Parse(`topic[0]="transfer" AND value.amount>=100`)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var args []any
	e.AppendSQL(&sb, &args)
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestParseNestedValuePath(t *testing.T) {
	e, err := evfilter.Parse("value.inner.count>5")
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	var args []any
	e.AppendSQL(&sb, &args)
	sql := sb.String()
	if !strings.Contains(sql, "#>>") {
		t.Errorf("expected #>> operator for nested path, got: %s", sql)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []string{
		`topic[-1]="x"`,
		`topic[abc]="x"`,
		`topic[0]`,
		`value.`,
		`unknown=1`,
		`topic[0]="unterminated`,
	}
	for _, tc := range cases {
		_, err := evfilter.Parse(tc)
		if err == nil {
			t.Errorf("Parse(%q) should have returned an error", tc)
		}
	}
}

func FuzzParse(f *testing.F) {
	seeds := []string{
		`topic[0]="transfer"`,
		`value.amount>=100`,
		`topic[0]="x" AND value.y!=5`,
		"",
		`topic[999]='abc'`,
		`value.a.b.c<=0`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		e, err := evfilter.Parse(s)
		if err != nil {
			return // invalid input is fine
		}
		// A successfully parsed expression must produce valid-looking SQL
		var sb strings.Builder
		var args []any
		e.AppendSQL(&sb, &args) // must not panic
	})
}
