package store

import "testing"

func TestValidContractSort(t *testing.T) {
	for _, s := range []string{ContractSortAddedAt, ContractSortLastActivity, ContractSortEventsCount} {
		if !ValidContractSort(s) {
			t.Errorf("ValidContractSort(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "id", "label", "added_at desc", "events"} {
		if ValidContractSort(s) {
			t.Errorf("ValidContractSort(%q) = true, want false", s)
		}
	}
}

func TestValidContractOrder(t *testing.T) {
	for _, o := range []string{"asc", "desc", "ASC", "DESC", " asc "} {
		if !ValidContractOrder(o) {
			t.Errorf("ValidContractOrder(%q) = false, want true", o)
		}
	}
	for _, o := range []string{"", "ascending", "up", "sideways"} {
		if ValidContractOrder(o) {
			t.Errorf("ValidContractOrder(%q) = true, want false", o)
		}
	}
}

func TestNormalizeContractSort(t *testing.T) {
	cases := []struct {
		sort, order string
		wantCol     string
		wantDesc    bool
	}{
		{ContractSortAddedAt, SortAsc, ContractSortAddedAt, false},
		{ContractSortEventsCount, SortDesc, ContractSortEventsCount, true},
		{ContractSortLastActivity, "", ContractSortLastActivity, true},
		{"", "", ContractSortAddedAt, true},
		{"bogus", "bogus", ContractSortAddedAt, true},
		{" Added_At ", " ASC ", ContractSortAddedAt, false},
	}
	for _, tc := range cases {
		col, desc := NormalizeContractSort(tc.sort, tc.order)
		if col != tc.wantCol || desc != tc.wantDesc {
			t.Errorf("NormalizeContractSort(%q, %q) = (%q, %v), want (%q, %v)",
				tc.sort, tc.order, col, desc, tc.wantCol, tc.wantDesc)
		}
	}
}
