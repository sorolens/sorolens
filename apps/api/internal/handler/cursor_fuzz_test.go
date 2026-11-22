package handler

import (
	"github.com/sorolens/sorolens/apps/api/internal/cursor"
	"fmt"
	"testing"
)

// FuzzDecodeCursor verifies that cursor decoding and invocation cursor parsing
// never panic on arbitrary inputs and adhere to formatting contracts.
func FuzzDecodeCursor(f *testing.F) {
	seeds := []string{
		// Empty and edge cases
		"",
		" ",
		"!!!",
		"===",
		"abc",
		"123",
		":",
		"::",
		"0:",
		":abc",
		"-1:abc",
		"1:2:3",
		"0:0000000000000000000000000000000000000000000000000000000000000000",
		"100:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		"9999999999999999999999999999999:abc",
		// Valid encoded cursors
		cursor.Encode(""),
		cursor.Encode("abc"),
		cursor.Encode("123:abc"),
		cursor.Encode("500000:7b96058fa85b8c66e2c366ffbc82798e21183ff1c8018e77a1027116df63adbb"),
		cursor.Encode("1:tx1"),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// Test generic decodeCursor
		raw, ok := cursor.Decode(data)
		if ok && raw != "" {
			// If base64 decoding succeeded, test invocation cursor parsing on the raw value
			ledger, txHash, invOk := cursor.ParseInvocation(raw)
			if invOk {
				if ledger == 0 || txHash == "" {
					t.Errorf("parseInvocationCursor succeeded with invalid output: ledger=%d, txHash=%q", ledger, txHash)
				}
			}
		}

		// Test parseInvocationCursor directly on raw data
		ledger, txHash, invOk := cursor.ParseInvocation(data)
		if invOk {
			if data != "" && (ledger == 0 || txHash == "") {
				t.Errorf("parseInvocationCursor on %q returned ok=true but ledger=%d, txHash=%q", data, ledger, txHash)
			}
			if ledger > 0 && txHash != "" {
				// Roundtrip check: re-encoding and re-parsing must produce the same result
				reconstructed := fmt.Sprintf("%d:%s", ledger, txHash)
				l2, h2, ok2 := cursor.ParseInvocation(reconstructed)
				if !ok2 || l2 != ledger || h2 != txHash {
					t.Errorf("reconstructed cursor mismatch: got (%d, %q), want (%d, %q)", l2, h2, ledger, txHash)
				}
			}
		}
	})
}
