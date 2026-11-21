package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestFailureCategoryPostgres covers the issue #806 SQL against a migrated
// database: the failure_category filter on both invocation listings, the
// failure fields in returned rows, and the per-category stats counts. Rows
// use a unique contract id and are removed afterwards, so the test does not
// truncate tables other tests read.
func TestFailureCategoryPostgres(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	s := store.NewFullStore(pool)

	contract := fmt.Sprintf("ZZFAILCAT_%d", time.Now().UnixNano())
	if err := s.UpsertContract(ctx, store.Contract{ID: contract, Network: "testnet", Status: "active"}); err != nil {
		t.Fatalf("UpsertContract: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM invocations WHERE contract_id = $1`, contract) //nolint:errcheck
		pool.Exec(ctx, `DELETE FROM contracts WHERE id = $1`, contract)            //nolint:errcheck
	})

	now := time.Now().UTC()
	rows := []struct {
		hash     string
		ledger   int
		age      time.Duration
		status   string
		category any
		code     any
	}{
		{"ok", 1, time.Hour, "SUCCESS", nil, nil},
		{"budget", 2, 2 * time.Hour, "FAILED", "budget_exceeded", "Error(Budget, ExceededLimit)"},
		{"contract-1", 3, 3 * time.Hour, "FAILED", "contract_error", "Error(Contract, #4)"},
		{"contract-2", 4, 72 * time.Hour, "FAILED", "contract_error", "Error(Contract, #1)"},
		{"other", 5, 10 * 24 * time.Hour, "FAILED", "other", "Error(#12, #3)"},
		{"legacy", 6, 4 * time.Hour, "FAILED", nil, nil},
	}
	for _, r := range rows {
		if _, err := pool.Exec(ctx, `
			INSERT INTO invocations (tx_hash, contract_id, network, ledger, ledger_closed_at, status, failure_category, failure_code)
			VALUES ($1, $2, 'testnet', $3, $4, $5, $6, $7)`,
			contract+"-"+r.hash, contract, r.ledger, now.Add(-r.age), r.status, r.category, r.code); err != nil {
			t.Fatalf("insert %s: %v", r.hash, err)
		}
	}

	hashes := func(invs []store.Invocation) []string {
		out := make([]string, len(invs))
		for i, inv := range invs {
			out[i] = inv.TxHash[len(contract)+1:]
		}
		return out
	}

	// Global listing, newest first.
	all, _, _, err := s.ListAllInvocations(ctx, 0, "", 50, store.InvocationFilters{ContractID: contract, FailureCategory: "contract_error"})
	if err != nil {
		t.Fatalf("ListAllInvocations: %v", err)
	}
	if got := fmt.Sprint(hashes(all)); got != "[contract-2 contract-1]" {
		t.Fatalf("global contract_error = %s", got)
	}
	if all[0].FailureCategory != "contract_error" || all[0].FailureCode != "Error(Contract, #1)" {
		t.Fatalf("failure fields not returned: %+v", all[0])
	}

	// Without a contract filter every returned row has the category.
	anyContract, _, _, err := s.ListAllInvocations(ctx, 0, "", 200, store.InvocationFilters{FailureCategory: "other"})
	if err != nil {
		t.Fatalf("ListAllInvocations(other): %v", err)
	}
	foundOther := false
	for _, inv := range anyContract {
		if inv.FailureCategory != "other" {
			t.Fatalf("row %s has category %q under failure_category=other", inv.TxHash, inv.FailureCategory)
		}
		foundOther = foundOther || inv.TxHash == contract+"-other"
	}
	if !foundOther {
		t.Fatal("failure_category=other did not return the seeded row")
	}

	// Per-contract listing, oldest first.
	per, _, err := s.ListInvocations(ctx, contract, "", 50, store.InvocationFilters{FailureCategory: "contract_error"})
	if err != nil {
		t.Fatalf("ListInvocations: %v", err)
	}
	if got := fmt.Sprint(hashes(per)); got != "[contract-1 contract-2]" {
		t.Fatalf("per-contract contract_error = %s", got)
	}

	// Unfiltered rows keep empty failure fields for success and legacy rows.
	inv, err := s.GetInvocation(ctx, contract+"-ok")
	if err != nil {
		t.Fatalf("GetInvocation: %v", err)
	}
	if inv.FailureCategory != "" || inv.FailureCode != "" {
		t.Fatalf("successful invocation has failure fields: %+v", inv)
	}
	detail, _, err := s.GetInvocationWithEvents(ctx, contract, contract+"-budget")
	if err != nil {
		t.Fatalf("GetInvocationWithEvents: %v", err)
	}
	if detail.FailureCategory != "budget_exceeded" || detail.FailureCode != "Error(Budget, ExceededLimit)" {
		t.Fatalf("detail failure fields: %+v", detail)
	}

	// Stats: every category listed, zeros included; the unclassified legacy
	// failure counts toward failed_invocations only.
	stats, err := s.GetContractInvocationStats(ctx, contract)
	if err != nil {
		t.Fatalf("GetContractInvocationStats: %v", err)
	}
	for name, tc := range map[string]struct {
		w      store.InvocationWindowStats
		failed int64
		want   map[string]int64
	}{
		"24h": {stats.Last24Hours, 3, map[string]int64{"budget_exceeded": 1, "contract_error": 1}},
		"7d":  {stats.Last7Days, 4, map[string]int64{"budget_exceeded": 1, "contract_error": 2}},
		"30d": {stats.Last30Days, 5, map[string]int64{"budget_exceeded": 1, "contract_error": 2, "other": 1}},
	} {
		if tc.w.FailedInvocations != tc.failed {
			t.Errorf("%s failed = %d, want %d", name, tc.w.FailedInvocations, tc.failed)
		}
		if len(tc.w.FailuresByCategory) != 6 {
			t.Errorf("%s: want 6 categories, got %v", name, tc.w.FailuresByCategory)
		}
		for cat, n := range tc.w.FailuresByCategory {
			if n != tc.want[cat] {
				t.Errorf("%s %s = %d, want %d", name, cat, n, tc.want[cat])
			}
		}
	}
}
