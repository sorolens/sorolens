package alertgroup_test

import (
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/alertgroup"
)

var (
	t0 = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

func alert(contractID, severity, rule, msg string, ts time.Time) alertgroup.IncomingAlert {
	return alertgroup.IncomingAlert{
		ContractID: contractID,
		Severity:   severity,
		Rule:       rule,
		Message:    msg,
		Timestamp:  ts,
	}
}

func TestBuildGroupKey(t *testing.T) {
	got := alertgroup.BuildGroupKey("CONTRACT_A", "Critical", "high_error_rate")
	want := "CONTRACT_A|Critical|high_error_rate"
	if got != want {
		t.Errorf("BuildGroupKey = %q, want %q", got, want)
	}
}

func TestBuildGroupKeyEmptyRule(t *testing.T) {
	got := alertgroup.BuildGroupKey("CONTRACT_A", "Warning", "")
	want := "CONTRACT_A|Warning|"
	if got != want {
		t.Errorf("BuildGroupKey = %q, want %q", got, want)
	}
}

func TestMerge_NewGroupWhenNoExisting(t *testing.T) {
	a := alert("C1", "Critical", "rule1", "first alert", t0)
	g, result := alertgroup.Merge(nil, a, 300)

	if result != alertgroup.MergeResultNew {
		t.Fatalf("result = %v, want MergeResultNew", result)
	}
	if g.Count != 1 {
		t.Errorf("Count = %d, want 1", g.Count)
	}
	if g.ContractID != "C1" {
		t.Errorf("ContractID = %q, want C1", g.ContractID)
	}
	if g.FirstSeen != t0 || g.LastSeen != t0 {
		t.Errorf("FirstSeen/LastSeen not set to alert timestamp")
	}
	if g.LastMessage != "first alert" {
		t.Errorf("LastMessage = %q, want %q", g.LastMessage, "first alert")
	}
	if g.DedupeWindowSecs != 300 {
		t.Errorf("DedupeWindowSecs = %d, want 300", g.DedupeWindowSecs)
	}
}

func TestMerge_MergesWithinWindow(t *testing.T) {
	a1 := alert("C1", "Critical", "rule1", "first", t0)
	g, _ := alertgroup.Merge(nil, a1, 300)

	// Second alert arrives 100s later — within the 300s window.
	a2 := alert("C1", "Critical", "rule1", "second", t0.Add(100*time.Second))
	g2, result := alertgroup.Merge(g, a2, 300)

	if result != alertgroup.MergeResultMerged {
		t.Fatalf("result = %v, want MergeResultMerged", result)
	}
	if g2.Count != 2 {
		t.Errorf("Count = %d, want 2", g2.Count)
	}
	if g2.LastSeen != a2.Timestamp {
		t.Errorf("LastSeen not updated")
	}
	if g2.LastMessage != "second" {
		t.Errorf("LastMessage = %q, want %q", g2.LastMessage, "second")
	}
	// FirstSeen must not change on merge.
	if g2.FirstSeen != t0 {
		t.Errorf("FirstSeen changed on merge")
	}
}

func TestMerge_OpensNewGroupOutsideWindow(t *testing.T) {
	a1 := alert("C1", "Critical", "rule1", "first", t0)
	g, _ := alertgroup.Merge(nil, a1, 300)

	// Third alert arrives 400s later — outside the 300s window.
	a3 := alert("C1", "Critical", "rule1", "third", t0.Add(400*time.Second))
	g2, result := alertgroup.Merge(g, a3, 300)

	if result != alertgroup.MergeResultNew {
		t.Fatalf("result = %v, want MergeResultNew", result)
	}
	if g2.Count != 1 {
		t.Errorf("Count = %d, want 1 (new group)", g2.Count)
	}
	if g2.FirstSeen != a3.Timestamp {
		t.Errorf("FirstSeen not reset for new group")
	}
}

func TestMerge_AtWindowBoundaryMerges(t *testing.T) {
	a1 := alert("C1", "Warning", "", "first", t0)
	g, _ := alertgroup.Merge(nil, a1, 300)

	// Exactly at the window boundary (300s) — should merge (not after).
	a2 := alert("C1", "Warning", "", "boundary", t0.Add(300*time.Second))
	_, result := alertgroup.Merge(g, a2, 300)

	if result != alertgroup.MergeResultMerged {
		t.Errorf("result = %v, want MergeResultMerged at exact boundary", result)
	}
}

func TestMerge_DefaultWindowUsedWhenZero(t *testing.T) {
	a1 := alert("C1", "Info", "", "first", t0)
	g, _ := alertgroup.Merge(nil, a1, 0) // 0 → use DefaultDedupeWindowSecs

	if g.DedupeWindowSecs != alertgroup.DefaultDedupeWindowSecs {
		t.Errorf("DedupeWindowSecs = %d, want %d", g.DedupeWindowSecs, alertgroup.DefaultDedupeWindowSecs)
	}

	// Alert within the default 300s window should merge.
	a2 := alert("C1", "Info", "", "second", t0.Add(100*time.Second))
	_, result := alertgroup.Merge(g, a2, 0)
	if result != alertgroup.MergeResultMerged {
		t.Errorf("result = %v, want MergeResultMerged within default window", result)
	}
}

func TestMerge_GroupKeyIncludesRule(t *testing.T) {
	a := alert("C2", "Critical", "my_rule", "msg", t0)
	g, _ := alertgroup.Merge(nil, a, 300)

	want := "C2|Critical|my_rule"
	if g.GroupKey != want {
		t.Errorf("GroupKey = %q, want %q", g.GroupKey, want)
	}
}
