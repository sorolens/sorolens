package failurecategory

import "testing"

func TestParse(t *testing.T) {
	for _, c := range All() {
		got, ok := Parse(string(c))
		if !ok || got != c {
			t.Errorf("Parse(%q) = %q, %v; want %q, true", c, got, ok, c)
		}
	}
	for _, bad := range []string{"", "OTHER", "Budget_Exceeded", "budget", "unknown"} {
		if _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) accepted an undefined category", bad)
		}
	}
}

func TestAllIsACopy(t *testing.T) {
	a := All()
	a[0] = "mutated"
	if All()[0] != BudgetExceeded {
		t.Fatal("All() exposes the package's backing slice")
	}
}

func TestAllEndsWithOther(t *testing.T) {
	a := All()
	if a[len(a)-1] != Other {
		t.Fatalf("Other must be listed last, got %v", a)
	}
}
