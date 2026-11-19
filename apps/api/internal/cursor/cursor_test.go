package cursor

import (
	"encoding/base64"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for _, raw := range []string{"12345:abc", "ledger", "a", "Ünïcödé"} {
		encoded := Encode(raw)
		if encoded == "" {
			t.Fatalf("Encode(%q) unexpectedly returned empty", raw)
		}
		got, ok := Decode(encoded)
		if !ok || got != raw {
			t.Errorf("Decode(Encode(%q)) = (%q, %v), want (%q, true)", raw, got, ok, raw)
		}
	}
}

// An empty cursor is a valid "start of the list", not an error: every list
// endpoint must keep treating the first page as fetchable.
func TestEmptyCursorIsEmpty(t *testing.T) {
	if got := Encode(""); got != "" {
		t.Errorf(`Encode("") = %q, want ""`, got)
	}
	raw, ok := Decode("")
	if !ok || raw != "" {
		t.Errorf(`Decode("") = (%q, %v), want ("", true)`, raw, ok)
	}
}

// A malformed cursor must be rejected, never silently reinterpreted as
// something else: it feeds keyset pagination in the store layer.
func TestDecodeRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"!!!!not base64!!!!", "aGVsbG", "\x00\xff"} {
		if raw, ok := Decode(bad); ok {
			t.Errorf("Decode(%q) = (%q, true), want ok=false", bad, raw)
		}
	}
}

func TestParseInvocationEmpty(t *testing.T) {
	ledger, txHash, ok := ParseInvocation("")
	if !ok || ledger != 0 || txHash != "" {
		t.Errorf(`ParseInvocation("") = (%d, %q, %v), want (0, "", true)`, ledger, txHash, ok)
	}
}

func TestParseInvocationValid(t *testing.T) {
	ledger, txHash, ok := ParseInvocation("4294967295:deadbeef")
	if !ok {
		t.Fatal("ParseInvocation rejected a valid cursor")
	}
	if ledger != 4294967295 {
		t.Errorf("ledger = %d, want 4294967295 (the uint32 ceiling)", ledger)
	}
	if txHash != "deadbeef" {
		t.Errorf("txHash = %q, want %q", txHash, "deadbeef")
	}
}

func TestParseInvocationRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"no separator":       "12345",
		"empty tx hash":      "12345:",
		"non-numeric ledger": "abc:tx",
		"zero ledger":        "0:tx",
		"negative ledger":    "-1:tx",
		"ledger overflows":   "4294967296:tx",
		"float ledger":       "1.5:tx",
	}
	for name, raw := range cases {
		if _, _, ok := ParseInvocation(raw); ok {
			t.Errorf("ParseInvocation(%q) (%s) accepted an invalid cursor", raw, name)
		}
	}
}

// The encoded form is standard base64 of the raw keyset, a format other
// clients (and the SDK examples) rely on.
func TestEncodeIsStdBase64(t *testing.T) {
	raw := "12345:abcdef"
	want := base64.StdEncoding.EncodeToString([]byte(raw))
	if got := Encode(raw); got != want {
		t.Errorf("Encode(%q) = %q, want %q", raw, got, want)
	}
}
