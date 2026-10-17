package main

import (
	"math/rand"
	"regexp"
	"strings"
	"testing"
)

// randSeeded builds a deterministic *rand.Rand so tests are reproducible.
func randSeeded(seed int64) *rand.Rand { return rand.New(rand.NewSource(seed)) }

var contractPattern = regexp.MustCompile(`^C[A-Za-z0-9]{55}$`)

func TestBuildContractsDeterministicAndUnique(t *testing.T) {
	a := buildContracts(randSeeded(7), 25)
	b := buildContracts(randSeeded(7), 25)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Fatal("same seed must produce identical contract ids")
	}
	seen := map[string]struct{}{}
	for _, id := range a {
		if !contractPattern.MatchString(id) {
			t.Fatalf("contract id %q does not match ^C[A-Za-z0-9]{55}$", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate contract id %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(a) != 25 {
		t.Fatalf("want 25 contracts, got %d", len(a))
	}
}

func TestTuplePlacesCastOnJsonbColumn(t *testing.T) {
	got := tuple(0, 3, map[int]string{2: "jsonb"})
	want := "($1,$2,$3::jsonb)"
	if got != want {
		t.Fatalf("tuple() = %q, want %q", got, want)
	}
	// Offsets accumulate across rows so a multi-VALUES batch keeps unique $n.
	got2 := tuple(3, 2, nil)
	if got2 != "($4,$5)" {
		t.Fatalf("tuple() offset = %q, want ($4,$5)", got2)
	}
}

func TestRandHexLengthAndAlphabet(t *testing.T) {
	h := randHex(randSeeded(1), 64)
	if len(h) != 64 {
		t.Fatalf("randHex length = %d, want 64", len(h))
	}
	if strings.TrimLeft(h, "0123456789abcdef") != "" {
		t.Fatalf("randHex produced non-hex characters: %q", h)
	}
}

func TestPickReturnsMember(t *testing.T) {
	rng := randSeeded(3)
	items := strings.Split(eventTypes, ",")
	for i := 0; i < 50; i++ {
		got := pick(rng, eventTypes)
		found := false
		for _, it := range items {
			if it == got {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("pick() returned %q, not a member of eventTypes", got)
		}
	}
}
