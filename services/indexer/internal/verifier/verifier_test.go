package verifier

import (
	"context"
	"testing"
)

// VerifyRange needs a database pool; without one it must return an error
// rather than panic. A clean-range test requires a live Postgres and is
// covered by integration tests.
func TestVerifier_RequiresDB(t *testing.T) {
	v := NewVerifier(nil, nil)
	results, err := v.VerifyRange(context.Background(), VerifyOptions{
		Network:     "testnet",
		StartLedger: 100,
		EndLedger:   105,
	})
	if err == nil {
		t.Fatalf("expected error for nil db, got results %v", results)
	}
}
