package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sorolens/sorolens/apps/api/internal/store"
)

// TestListMonitoredContractsKeysetPagination exercises the real SQL keyset
// query (contract_id > cursor ORDER BY contract_id LIMIT limit+1).
func TestListMonitoredContractsKeysetPagination(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_URL")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_URL not set, skipping DB test")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()
	s := store.NewFullStore(pool)

	// seed replaces the table contents with n testnet and 3 mainnet contracts
	// that differ only in their ID, and returns the testnet IDs in order.
	seed := func(t *testing.T, n int) []string {
		t.Helper()
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE monitored_contracts CASCADE"); err != nil {
			t.Fatal(err)
		}
		registered := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		var ids []string
		for i := range n + 3 {
			m := store.MonitoredContract{
				ContractID:   fmt.Sprintf("CPAGE_TESTNET_%03d", i),
				Network:      "testnet",
				Name:         "same-name",
				Owner:        "GOWNER",
				Status:       "Healthy",
				RegisteredAt: registered,
			}
			if i >= n {
				m.ContractID = fmt.Sprintf("CPAGE_MAINNET_%03d", i)
				m.Network = "mainnet"
			} else {
				ids = append(ids, m.ContractID)
			}
			if err := s.UpsertMonitoredContract(ctx, m); err != nil {
				t.Fatal(err)
			}
		}
		return ids
	}

	tests := []struct {
		name      string
		seed      int
		limit     int
		wantPages []int
	}{
		{"empty dataset", 0, 3, []int{0}},
		{"fewer items than limit", 2, 3, []int{2}},
		{"exactly limit items", 3, 3, []int{3}},
		{"limit plus one items", 4, 3, []int{3, 1}},
		{"multiple pages", 7, 3, []int{3, 3, 1}},
		{"limit zero uses default", 51, 0, []int{50, 1}},
		{"limit above max uses default", 51, 201, []int{50, 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := seed(t, tc.seed)

			var got []string
			var sizes []int
			cursor := ""
			for {
				page, next, err := s.ListMonitoredContracts(ctx, cursor, tc.limit, "testnet")
				if err != nil {
					t.Fatal(err)
				}
				sizes = append(sizes, len(page))
				for _, m := range page {
					got = append(got, m.ContractID)
				}
				if next == "" {
					break
				}
				if next != page[len(page)-1].ContractID {
					t.Fatalf("next cursor %q is not the last id on the page", next)
				}
				if len(sizes) > len(tc.wantPages) {
					t.Fatalf("too many pages: %v", sizes)
				}
				cursor = next
			}

			if fmt.Sprint(sizes) != fmt.Sprint(tc.wantPages) {
				t.Errorf("page sizes: want %v, got %v", tc.wantPages, sizes)
			}
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("traversal mismatch:\nwant %v\ngot  %v", want, got)
			}
		})
	}

	t.Run("cursor past the last item", func(t *testing.T) {
		seed(t, 3)
		page, next, err := s.ListMonitoredContracts(ctx, "CPAGE_TESTNET_999", 3, "testnet")
		if err != nil {
			t.Fatal(err)
		}
		if len(page) != 0 || next != "" {
			t.Fatalf("want empty page and no cursor, got %d items, cursor %q", len(page), next)
		}
	})
}
