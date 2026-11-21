package verifier

import (
	"context"
	"testing"
)

func TestVerifier_CleanRange(t *testing.T) {
	v := NewVerifier(nil, nil)
	ctx := context.Background()
	results, err := v.VerifyRange(ctx, VerifyOptions{
		Network:     "testnet",
		StartLedger: 100,
		EndLedger:   105,
		Repair:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(results))
	}
	for _, r := range results {
		if !r.Match {
			t.Errorf("expected match for ledger %d", r.LedgerSeq)
		}
	}
}
